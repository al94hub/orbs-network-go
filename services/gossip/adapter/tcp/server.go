// Copyright 2019 the orbs-network-go authors
// This file is part of the orbs-network-go library in the Orbs project.
//
// This source code is licensed under the MIT license found in the LICENSE file in the root directory of this source tree.
// The above notice should be included in all copies or substantial portions of the software.

package tcp

import (
	"context"
	"fmt"
	"github.com/orbs-network/govnr"
	"github.com/orbs-network/membuffers/go"
	"github.com/orbs-network/orbs-network-go/instrumentation/logfields"
	"github.com/orbs-network/orbs-network-go/instrumentation/metric"
	"github.com/orbs-network/orbs-network-go/instrumentation/trace"
	"github.com/orbs-network/orbs-network-go/services/gossip/adapter"
	"github.com/orbs-network/scribe/log"
	"github.com/pkg/errors"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

var ACK_BUFFER = []byte{0x11, 0x22, 0x33, 0x44}

type serverConfig interface {
	GossipListenPort() uint16
	GossipNetworkTimeout() time.Duration
}

type transportServer struct {
	govnr.TreeSupervisor

	sync.RWMutex
	port        int
	listener    adapter.TransportListener
	netListener net.Listener

	logger         log.Logger
	metrics        incomingConnectionMetrics
	config         serverConfig
	shutdownServer context.CancelFunc
}

type incomingConnectionMetrics struct {
	acceptSuccesses   *metric.Gauge
	acceptErrors      *metric.Gauge
	transportErrors   *metric.Gauge
	activeConnections *metric.Gauge
}

func newServer(config serverConfig, logger log.Logger, registry metric.Registry) *transportServer {
	server := &transportServer{
		config:  config,
		logger:  logger,
		metrics: createServerMetrics(registry),
	}

	return server
}

func createServerMetrics(registry metric.Registry) incomingConnectionMetrics {
	return incomingConnectionMetrics{
		acceptSuccesses:   registry.NewGauge("Gossip.IncomingConnection.ListeningOnTCPPortSuccess.Count"),
		acceptErrors:      registry.NewGauge("Gossip.IncomingConnection.ListeningOnTCPPortErrors.Count"),
		transportErrors:   registry.NewGauge("Gossip.IncomingConnection.TransportErrors.Count"),
		activeConnections: registry.NewGauge("Gossip.IncomingConnection.Active.Count"),
	}
}

func (t *transportServer) getPort() int {
	t.RLock()
	defer t.RUnlock()

	return t.port
}

func (t *transportServer) listenForIncomingConnections(ctx context.Context) (net.Listener, error) {
	// TODO(v1): migrate to ListenConfig which has better support of contexts (go 1.11 required)
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", t.config.GossipListenPort()))
	if err != nil {
		return nil, err
	}

	t.Lock()
	defer t.Unlock()
	t.port = listener.Addr().(*net.TCPAddr).Port
	t.netListener = listener
	t.logger.Info("gossip transport server listening", log.Int("port", t.port))

	return listener, err
}

func (t *transportServer) IsListening() bool {
	t.RLock()
	defer t.RUnlock()

	return t.netListener != nil
}

func (t *transportServer) mainLoop(parentCtx context.Context, listener net.Listener) {
	ctx := trace.NewContext(parentCtx, "Gossip.Transport.TCP.Server")
	logger := t.logger.WithTags(trace.LogFieldFrom(ctx))

	var numOfConnections int32

	connClosed := make(chan struct{})
	connCtx, closeAllDanglingConnections := context.WithCancel(parentCtx)

	defer func() {
		closeAllDanglingConnections()
		for ; atomic.LoadInt32(&numOfConnections) > 0; atomic.AddInt32(&numOfConnections, -1) {
			<-connClosed
		}
		logger.Info("all connections have been closed, returning")
	}()

	for {
		if ctx.Err() != nil {
			logger.Info("server main loop quitting because system is shutting down")
			return
		}
		conn, err := listener.Accept()
		if err != nil {
			if !t.IsListening() {
				logger.Info("incoming connection accept stopped since server is shutting down")
				return
			}
			t.metrics.acceptErrors.Inc()
			logger.Info("incoming connection accept error", log.Error(err))
			continue
		}

		logger.Info("got incoming connection", log.Stringable("remote-address", conn.RemoteAddr()))
		t.metrics.acceptSuccesses.Inc()

		atomic.AddInt32(&numOfConnections, 1)

		govnr.Once(logfields.GovnrErrorer(logger), func() {
			t.handleIncomingConnection(connCtx, conn)
			if connCtx.Err() == nil { // server is not shutting down
				atomic.AddInt32(&numOfConnections, -1) // don't have to wait for this connection to close
			} else {
				connClosed <- struct{}{}
			}
		})
	}
}

func (t *transportServer) handleIncomingConnection(ctx context.Context, conn net.Conn) {
	t.logger.Info("successful incoming gossip transport connection", log.String("peer", conn.RemoteAddr().String()), trace.LogFieldFrom(ctx))
	// TODO(https://github.com/orbs-network/orbs-network-go/issues/182): add a white list for IPs we're willing to accept connections from
	// TODO(https://github.com/orbs-network/orbs-network-go/issues/182): make sure each IP from the white list connects only once
	t.metrics.activeConnections.Inc()
	defer t.metrics.activeConnections.Dec()

	defer func() { _ = conn.Close() }()
	for {
		payloads, err := t.receiveTransportData(ctx, conn)
		if err != nil {
			t.metrics.transportErrors.Inc()
			t.logger.Info("failed receiving transport data, disconnecting", log.Error(err), log.String("peer", conn.RemoteAddr().String()), trace.LogFieldFrom(ctx))

			return
		}

		// notify if not keepalive
		if len(payloads) > 0 {
			ctxWithPeer := context.WithValue(ctx, "peer-ip", conn.RemoteAddr().String())
			t.notifyListener(ctxWithPeer, payloads)
		}
	}
}

func (t *transportServer) receiveTransportData(ctx context.Context, conn net.Conn) ([][]byte, error) {
	// TODO(https://github.com/orbs-network/orbs-network-go/issues/182): think about timeout policy on receive, we might not want it
	timeout := t.config.GossipNetworkTimeout()
	var res [][]byte

	// receive num payloads
	sizeBuffer, err := readTotal(ctx, conn, 4, timeout)
	if err != nil {
		return nil, err
	}
	numPayloads := membuffers.GetUint32(sizeBuffer)

	if numPayloads > MAX_PAYLOADS_IN_MESSAGE {
		return nil, errors.Errorf("received message with too many payloads: %d", numPayloads)
	}

	for i := uint32(0); i < numPayloads; i++ {
		// receive payload size
		sizeBuffer, err := readTotal(ctx, conn, 4, timeout)
		if err != nil {
			return nil, err
		}
		payloadSize := membuffers.GetUint32(sizeBuffer)
		if payloadSize > MAX_PAYLOAD_SIZE_BYTES {
			return nil, errors.Errorf("received message with a payload too big: %d bytes", payloadSize)
		}

		// receive payload data
		payload, err := readTotal(ctx, conn, payloadSize, timeout)
		if err != nil {
			return nil, err
		}
		res = append(res, payload)

		// receive padding
		paddingSize := calcPaddingSize(uint32(len(payload)))
		if paddingSize > 0 {
			_, err := readTotal(ctx, conn, paddingSize, timeout)
			if err != nil {
				return nil, err
			}
		}
	}

	// send ack
	err = write(ctx, conn, ACK_BUFFER, timeout)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (t *transportServer) notifyListener(ctx context.Context, payloads [][]byte) {
	listener := t.getListener()

	if listener == nil {
		return
	}

	listener.OnTransportMessageReceived(ctx, payloads)
}

func (t *transportServer) getListener() adapter.TransportListener {
	t.RLock()
	defer t.RUnlock()

	return t.listener
}

func (t *transportServer) startSupervisedMainLoop(parent context.Context) {
	ctx, cancel := context.WithCancel(parent)
	t.shutdownServer = cancel

	listener, err := t.listenForIncomingConnections(ctx)
	if err != nil {
		panic(fmt.Sprintf("gossip transport failed to listen on port %d: %s", t.config.GossipListenPort(), err.Error()))
	}
	t.Supervise(govnr.Forever(ctx, "TCP server", logfields.GovnrErrorer(t.logger), func() {
		t.mainLoop(ctx, listener)
	}))
}

func (t *transportServer) GracefulShutdown(shutdownContext context.Context) {
	t.Lock()
	defer t.Unlock()
	l := t.netListener
	t.netListener = nil
	if l != nil {
		err := l.Close()
		if err != nil {
			t.logger.Error("Failed to close direct transport lister", log.Error(err))
		}
	}
	t.shutdownServer()
}

func readTotal(ctx context.Context, conn net.Conn, totalSize uint32, timeout time.Duration) ([]byte, error) {
	// TODO(https://github.com/orbs-network/orbs-network-go/issues/182): consider whether the right approach is to poll context this way or have a single watchdog goroutine that closes all active connections when context is cancelled
	// make sure context is still open
	err := ctx.Err()
	if err != nil {
		return nil, err
	}

	buffer := make([]byte, totalSize)
	totalRead := uint32(0)
	for totalRead < totalSize {
		err := conn.SetReadDeadline(time.Now().Add(timeout))
		if err != nil {
			return nil, err
		}
		read, err := conn.Read(buffer[totalRead:])
		totalRead += uint32(read)
		if totalRead == totalSize {
			break
		}
		if err != nil {
			return nil, err
		}
	}
	return buffer, nil
}
