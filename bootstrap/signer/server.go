// Copyright 2019 the orbs-network-go authors
// This file is part of the orbs-network-go library in the Orbs project.
//
// This source code is licensed under the MIT license found in the LICENSE file in the root directory of this source tree.
// The above notice should be included in all copies or substantial portions of the software.

package signer

import (
	"context"
	"github.com/orbs-network/govnr"
	"github.com/orbs-network/orbs-network-go/config"
	"github.com/orbs-network/orbs-network-go/services/signer"
	"github.com/orbs-network/orbs-network-go/synchronization/supervised"
	"github.com/orbs-network/orbs-spec/types/go/services"
	"github.com/orbs-network/scribe/log"
)

type Server struct {
	govnr.TreeSupervisor
	service    services.Vault
	cancelFunc context.CancelFunc
	httpServer *httpServer
}

func StartSignerServer(cfg config.SignerServiceConfig, logger log.Logger) (*Server, error) {
	if err := config.ValidateSigner(cfg); err != nil {
		return nil, err
	}

	service := signer.NewService(cfg, logger)
	api := &api{
		service, logger,
	}

	httpServer, err := NewHttpServer(cfg.HttpAddress(), logger)
	if err != nil {
		return nil, err
	}

	httpServer.Router().HandleFunc("/", api.IndexHandler)
	httpServer.Router().HandleFunc("/sign", api.SignHandler)

	_, cancel := context.WithCancel(context.Background())
	s := &Server{
		service:    service,
		cancelFunc: cancel,
		httpServer: httpServer,
	}

	s.Supervise(httpServer)

	return s, nil
}

func (s *Server) GracefulShutdown(shutdownContext context.Context) {
	supervised.ShutdownAllGracefully(shutdownContext, s.httpServer)
}
