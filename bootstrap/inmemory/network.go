// Copyright 2019 the orbs-network-go authors
// This file is part of the orbs-network-go library in the Orbs project.
//
// This source code is licensed under the MIT license found in the LICENSE file in the root directory of this source tree.
// The above notice should be included in all copies or substantial portions of the software.

package inmemory

import (
	"context"
	"fmt"
	"github.com/orbs-network/govnr"
	"github.com/orbs-network/orbs-network-go/bootstrap"
	"github.com/orbs-network/orbs-network-go/config"
	"github.com/orbs-network/orbs-network-go/crypto/digest"
	"github.com/orbs-network/orbs-network-go/instrumentation/metric"
	blockStorageAdapter "github.com/orbs-network/orbs-network-go/services/blockstorage/adapter"
	blockStorageMemoryAdapter "github.com/orbs-network/orbs-network-go/services/blockstorage/adapter/memory"
	ethereumAdapter "github.com/orbs-network/orbs-network-go/services/crosschainconnector/ethereum/adapter"
	"github.com/orbs-network/orbs-network-go/services/gossip/adapter"
	"github.com/orbs-network/orbs-network-go/services/management"
	managementAdapter "github.com/orbs-network/orbs-network-go/services/management/adapter"
	nativeProcessorAdapter "github.com/orbs-network/orbs-network-go/services/processor/native/adapter"
	stateStorageAdapter "github.com/orbs-network/orbs-network-go/services/statestorage/adapter"
	stateStorageMemoryAdapter "github.com/orbs-network/orbs-network-go/services/statestorage/adapter/memory"
	txPoolAdapter "github.com/orbs-network/orbs-network-go/services/transactionpool/adapter"
	"github.com/orbs-network/orbs-network-go/synchronization"
	"github.com/orbs-network/orbs-network-go/test/builders"
	"github.com/orbs-network/orbs-spec/types/go/primitives"
	"github.com/orbs-network/orbs-spec/types/go/protocol"
	"github.com/orbs-network/orbs-spec/types/go/protocol/client"
	"github.com/orbs-network/orbs-spec/types/go/services"
	"github.com/orbs-network/scribe/log"
	"github.com/pkg/errors"
	"math"
	"sync"
)

// Represents an in-process network connecting a group of in-memory Nodes together using the provided Transport
type Network struct {
	govnr.TreeSupervisor
	Nodes              []*Node
	Logger             log.Logger
	Transport          adapter.Transport
	VirtualChainId     primitives.VirtualChainId
	MaybeClock         txPoolAdapter.Clock
}

type NodeDependencies struct {
	Compiler                           nativeProcessorAdapter.Compiler
	EtherConnection                    ethereumAdapter.EthereumConnection
	BlockPersistence                   blockStorageAdapter.BlockPersistence
	ManagementProvider                 management.Provider
	StatePersistence                   stateStorageAdapter.StatePersistence
	StateBlockHeightReporter           stateStorageAdapter.BlockHeightReporter
	TransactionPoolBlockHeightReporter *synchronization.BlockTracker
}
type nodeDependencyProvider func(idx int, nodeConfig config.NodeConfig, logger log.Logger, metricRegistry metric.Registry) *NodeDependencies

func NewNetworkWithNumOfNodes(validators map[string]config.ValidatorNode, nodeOrder []primitives.NodeAddress, privateKeys map[string]primitives.EcdsaSecp256K1PrivateKey, parent log.Logger, cfgTemplate config.OverridableConfig, transport adapter.Transport, maybeClock txPoolAdapter.Clock, provider nodeDependencyProvider) *Network {

	network := &Network{
		Logger:             parent,
		Transport:          transport,
		MaybeClock:         maybeClock,
		VirtualChainId:     cfgTemplate.VirtualChainId(),
	}
	parent.Info("acceptance network node order", log.StringableSlice("addresses", nodeOrder))
	parent.Info(configToStr(cfgTemplate))

	for _, address := range nodeOrder {
		validatorNode := validators[address.KeyForMap()]
		cfg := cfgTemplate.ForNode(address, privateKeys[address.KeyForMap()])
		metricRegistry := metric.NewRegistry()

		nodeLogger := parent.WithTags(log.Node(cfg.NodeAddress().String()))
		dep := &NodeDependencies{}
		if provider == nil {
			dep.BlockPersistence = blockStorageMemoryAdapter.NewBlockPersistence(nodeLogger, metricRegistry)
			dep.Compiler = nativeProcessorAdapter.NewNativeCompiler(cfgTemplate, nodeLogger, metricRegistry)
			dep.EtherConnection = ethereumAdapter.NewEthereumRpcConnection(cfgTemplate, nodeLogger, metricRegistry)
			dep.ManagementProvider = managementAdapter.NewMemoryProvider(cfgTemplate, nodeLogger)
			dep.StatePersistence = stateStorageMemoryAdapter.NewStatePersistence(metricRegistry)
			dep.StateBlockHeightReporter = synchronization.NopHeightReporter{}
			dep.TransactionPoolBlockHeightReporter = synchronization.NewBlockTracker(nodeLogger, 0, math.MaxUint16)
		} else {
			dep = provider(len(network.Nodes), cfg, nodeLogger, metricRegistry)
		}

		network.addNode(fmt.Sprintf("%s", validatorNode.NodeAddress()[:3]), cfg, dep, metricRegistry, nodeLogger)
	}

	network.Supervise(transport)

	return network // call network.CreateAndStartNodes to launch nodes in the network
}

func configToStr(cfgTemplate config.OverridableConfig) string {
	// This is an OPINIONATED list of important config properties to print to aid debugging
	configStr := fmt.Sprintf("CONFIG_PROPS: public-api-tx-timeout=%s lh-election-timeout=%s node-sync-nocommit-interval=%s node-sync-collect-chunks-timeout=%s node-sync-collect-response-timeout=%s block-tracker-grace-timeout=%s gossip-timeout=%s, block-sync-num-blocks-in-batch=%d papi-node-sync-warning-time=%s txpool-time-between-empty-blocks=%s",
		cfgTemplate.PublicApiSendTransactionTimeout(),
		cfgTemplate.LeanHelixConsensusRoundTimeoutInterval(),
		cfgTemplate.BlockSyncNoCommitInterval(),
		cfgTemplate.BlockSyncCollectChunksTimeout(),
		cfgTemplate.BlockSyncCollectResponseTimeout(),
		cfgTemplate.BlockTrackerGraceTimeout(),
		cfgTemplate.GossipNetworkTimeout(),
		cfgTemplate.BlockSyncNumBlocksInBatch(),
		cfgTemplate.PublicApiNodeSyncWarningTime(),
		cfgTemplate.TransactionPoolTimeBetweenEmptyBlocks(),
	)
	return configStr
}

func (n *Network) addNode(name string, cfg config.NodeConfig, nodeDependencies *NodeDependencies, metricRegistry metric.Registry, logger log.Logger) {

	node := &Node{}
	node.index = len(n.Nodes)
	node.name = name
	node.config = cfg
	node.statePersistence = nodeDependencies.StatePersistence
	node.stateBlockHeightReporter = nodeDependencies.StateBlockHeightReporter
	node.transactionPoolBlockTracker = nodeDependencies.TransactionPoolBlockHeightReporter
	node.blockPersistence = nodeDependencies.BlockPersistence
	node.managementProvider = nodeDependencies.ManagementProvider
	node.nativeCompiler = nodeDependencies.Compiler
	node.ethereumConnection = nodeDependencies.EtherConnection
	node.metricRegistry = metricRegistry

	n.Nodes = append(n.Nodes, node)
}

func (n *Network) CreateAndStartNodes(ctx context.Context, numOfNodesToStart int) {
	nodes := reverse(n.Nodes[:numOfNodesToStart]) // this is to reduce the chances of getting to view=1 in the first block; since the committee is ordered by num of nodes, this will increase the chances that the leader of block height 1 starts after its validators, thereby reducing the chances that it sends PREPREPARE before its validators have started

	wg := &sync.WaitGroup{}
	for _, node := range nodes {
		wg.Add(1)

		nodeLogger := n.Logger.WithTags(log.Node(node.name))
		node.nodeLogic = bootstrap.NewNodeLogic(
			ctx,
			n.Transport,
			node.blockPersistence,
			node.statePersistence,
			node.stateBlockHeightReporter,
			node.transactionPoolBlockTracker,
			n.MaybeClock,
			node.nativeCompiler,
			node.managementProvider,
			nodeLogger,
			node.metricRegistry,
			node.config,
			node.ethereumConnection,
		)
		go func(nx *Node) { // nodes should not block each other from executing wait
			if err := nx.transactionPoolBlockTracker.WaitForBlock(ctx, 1); err != nil {
				msg := fmt.Sprintf("node %v did not reach block 1: %s", nx.name, err)
				nodeLogger.Error(msg)
				panic(msg)
			} else {
				nodeLogger.Info(fmt.Sprintf("node %v reached block 1", nx.name))
			}
			wg.Done()
		}(node)

		n.Supervise(node.nodeLogic)
	}

	wg.Wait()
}

func reverse(nodes []*Node) (reversed []*Node) {
	for i := len(nodes) - 1; i >= 0; i-- {
		reversed = append(reversed, nodes[i])
	}
	return
}

func (n *Network) PublicApi(nodeIndex int) services.PublicApi {
	return n.Nodes[nodeIndex].nodeLogic.PublicApi()
}

type sendTxResp struct {
	res *services.SendTransactionOutput
	err error
}

func (n *Network) SendTransaction(ctx context.Context, builder *protocol.SignedTransactionBuilder, nodeIndex int) (*client.SendTransactionResponse, primitives.Sha256) {
	n.assertStarted(nodeIndex)
	ch := make(chan sendTxResp)

	transactionRequestBuilder := &client.SendTransactionRequestBuilder{SignedTransaction: builder}
	txHash := digest.CalcTxHash(transactionRequestBuilder.SignedTransaction.Transaction.Build())

	go func() {
		defer close(ch)
		publicApi := n.Nodes[nodeIndex].GetPublicApi()
		output, err := publicApi.SendTransaction(ctx, &services.SendTransactionInput{
			ClientRequest: transactionRequestBuilder.Build(),
		})

		select {
		case ch <- sendTxResp{res: output, err: err}:
		case <-ctx.Done():
			ch <- sendTxResp{err: errors.Wrap(ctx.Err(), "aborted send tx")}
		}
	}()

	out := <-ch
	if out.res == nil {
		panic(fmt.Sprintf("error in send transaction: %s", out.err.Error())) // TODO(https://github.com/orbs-network/orbs-network-go/issues/531): improve
	}
	return out.res.ClientResponse, txHash
}

func (n *Network) SendTransactionInBackground(ctx context.Context, builder *protocol.SignedTransactionBuilder, nodeIndex int) {
	n.assertStarted(nodeIndex)

	go func() {
		publicApi := n.Nodes[nodeIndex].GetPublicApi()
		output, err := publicApi.SendTransactionAsync(ctx, &services.SendTransactionInput{
			ClientRequest: (&client.SendTransactionRequestBuilder{SignedTransaction: builder}).Build(),
		})
		if output == nil {
			panic(fmt.Sprintf("error sending transaction: %s", err.Error())) // TODO(https://github.com/orbs-network/orbs-network-go/issues/531): improve
		}
	}()
}

type getTxStatResp struct {
	res *services.GetTransactionStatusOutput
	err error
}

func (n *Network) GetTransactionStatus(ctx context.Context, txHash primitives.Sha256, nodeIndex int) *client.GetTransactionStatusResponse {
	n.assertStarted(nodeIndex)

	ch := make(chan getTxStatResp)
	go func() {
		defer close(ch)
		publicApi := n.Nodes[nodeIndex].GetPublicApi()
		output, err := publicApi.GetTransactionStatus(ctx, &services.GetTransactionStatusInput{
			ClientRequest: (&client.GetTransactionStatusRequestBuilder{
				TransactionRef: builders.TransactionRef().WithTxHash(txHash).Builder(),
			}).Build(),
		})
		select {
		case ch <- getTxStatResp{res: output, err: err}:
		case <-ctx.Done():
			ch <- getTxStatResp{err: errors.Wrap(ctx.Err(), "aborted get tx status")}
		}
	}()
	out := <-ch
	if out.res == nil {
		panic(fmt.Sprintf("error in get tx status: %s", out.err.Error())) // TODO(https://github.com/orbs-network/orbs-network-go/issues/531): improve
	}
	return out.res.ClientResponse
}

type getTxProofResp struct {
	res *services.GetTransactionReceiptProofOutput
	err error
}

func (n *Network) GetTransactionReceiptProof(ctx context.Context, txHash primitives.Sha256, nodeIndex int) *client.GetTransactionReceiptProofResponse {
	n.assertStarted(nodeIndex)

	ch := make(chan getTxProofResp)
	go func() {
		defer close(ch)
		publicApi := n.Nodes[nodeIndex].GetPublicApi()
		output, err := publicApi.GetTransactionReceiptProof(ctx, &services.GetTransactionReceiptProofInput{
			ClientRequest: (&client.GetTransactionReceiptProofRequestBuilder{
				TransactionRef: builders.TransactionRef().WithTxHash(txHash).Builder(),
			}).Build(),
		})
		select {
		case ch <- getTxProofResp{res: output, err: err}:
		case <-ctx.Done():
			ch <- getTxProofResp{err: errors.Wrap(ctx.Err(), "aborted get tx receipt proof")}
		}
	}()
	out := <-ch
	if out.res == nil {
		panic(fmt.Sprintf("error in get tx receipt proof: %s", out.err.Error())) // TODO(https://github.com/orbs-network/orbs-network-go/issues/531): improve
	}
	return out.res.ClientResponse
}

type runQueryResp struct {
	res *services.RunQueryOutput
	err error
}

func (n *Network) RunQuery(ctx context.Context, builder *protocol.SignedQueryBuilder, nodeIndex int) *client.RunQueryResponse {
	n.assertStarted(nodeIndex)

	ch := make(chan runQueryResp)
	go func() {
		defer close(ch)
		publicApi := n.Nodes[nodeIndex].GetPublicApi()
		output, err := publicApi.RunQuery(ctx, &services.RunQueryInput{
			ClientRequest: (&client.RunQueryRequestBuilder{SignedQuery: builder}).Build(),
		})

		select {
		case ch <- runQueryResp{res: output, err: err}:
		case <-ctx.Done():
			ch <- runQueryResp{err: errors.Wrap(ctx.Err(), "aborted run query")}
		}
	}()
	out := <-ch
	if out.res == nil {
		panic(fmt.Sprintf("error in run query: %s", out.err.Error())) // TODO(https://github.com/orbs-network/orbs-network-go/issues/531): improve
	}
	return out.res.ClientResponse
}

func (n *Network) assertStarted(nodeIndex int) {
	if !n.Nodes[nodeIndex].Started() {
		panic(fmt.Sprintf("accessing a stopped node %d", nodeIndex))
	}
}

func (n *Network) Destroy() {
	for _, node := range n.Nodes {
		node.Destroy()
	}
}

func (n *Network) MetricRegistry(nodeIndex int) metric.Registry {
	return n.Nodes[nodeIndex].metricRegistry
}

func (n *Network) GetVirtualChainId() primitives.VirtualChainId {
	return n.VirtualChainId
}
