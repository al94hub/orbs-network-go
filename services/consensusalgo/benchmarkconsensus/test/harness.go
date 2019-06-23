// Copyright 2019 the orbs-network-go authors
// This file is part of the orbs-network-go library in the Orbs project.
//
// This source code is licensed under the MIT license found in the LICENSE file in the root directory of this source tree.
// The above notice should be included in all copies or substantial portions of the software.

package test

import (
	"context"
	"github.com/orbs-network/go-mock"
	"github.com/orbs-network/orbs-network-go/config"
	"github.com/orbs-network/orbs-network-go/crypto/signer"
	"github.com/orbs-network/orbs-network-go/instrumentation/metric"
	"github.com/orbs-network/orbs-network-go/services/consensusalgo/benchmarkconsensus"
	testKeys "github.com/orbs-network/orbs-network-go/test/crypto/keys"
	"github.com/orbs-network/orbs-spec/types/go/protocol"
	"github.com/orbs-network/orbs-spec/types/go/services"
	"github.com/orbs-network/orbs-spec/types/go/services/gossiptopics"
	"github.com/orbs-network/orbs-spec/types/go/services/handlers"
	"github.com/orbs-network/scribe/log"
	"github.com/stretchr/testify/require"
	"testing"
)

const NETWORK_SIZE = 5

type harness struct {
	gossip           *gossiptopics.MockBenchmarkConsensus
	blockStorage     *services.MockBlockStorage
	consensusContext *services.MockConsensusContext
	signer           signer.Signer
	reporting        log.Logger
	config           benchmarkconsensus.Config
	service          services.ConsensusAlgoBenchmark
	registry         metric.Registry
}

func leaderKeyPair() *testKeys.TestEcdsaSecp256K1KeyPair {
	return testKeys.EcdsaSecp256K1KeyPairForTests(0)
}

func nonLeaderKeyPair() *testKeys.TestEcdsaSecp256K1KeyPair {
	return testKeys.EcdsaSecp256K1KeyPairForTests(1)
}

func otherNonLeaderKeyPair() *testKeys.TestEcdsaSecp256K1KeyPair {
	return testKeys.EcdsaSecp256K1KeyPairForTests(2)
}

func newHarness(tb testing.TB, isLeader bool) *harness {

	genesisValidatorNodes := make(map[string]config.ValidatorNode)
	for i := 0; i < NETWORK_SIZE; i++ {
		nodeAddress := testKeys.EcdsaSecp256K1KeyPairForTests(i).NodeAddress()
		genesisValidatorNodes[nodeAddress.KeyForMap()] = config.NewHardCodedValidatorNode(nodeAddress)
	}

	nodeKeyPair := leaderKeyPair()
	if !isLeader {
		nodeKeyPair = nonLeaderKeyPair()
	}

	//TODO(v1) don't use acceptance tests config! use a per-service config
	cfg := config.ForBenchmarkConsensusTests(nodeKeyPair, leaderKeyPair(), genesisValidatorNodes)
	log := log.DefaultTestingLogger(tb)

	gossip := &gossiptopics.MockBenchmarkConsensus{}
	gossip.When("RegisterBenchmarkConsensusHandler", mock.Any).Return().Times(1)

	blockStorage := &services.MockBlockStorage{}
	blockStorage.When("RegisterConsensusBlocksHandler", mock.Any).Return().Times(1)

	consensusContext := &services.MockConsensusContext{}

	signer, err := signer.New(cfg)
	require.NoError(tb, err)

	return &harness{
		gossip:           gossip,
		blockStorage:     blockStorage,
		consensusContext: consensusContext,
		signer:           signer,
		reporting:        log,
		config:           cfg,
		service:          nil,
		registry:         metric.NewRegistry(),
	}
}

func (h *harness) createService(ctx context.Context) {
	h.service = benchmarkconsensus.NewBenchmarkConsensusAlgo(
		ctx,
		h.gossip,
		h.blockStorage,
		h.consensusContext,
		h.signer,
		h.reporting,
		h.config,
		h.registry,
	)
}

func (h *harness) verifyHandlerRegistrations(t *testing.T) {
	ok, err := h.gossip.Verify()
	if !ok {
		t.Fatal("Did not register with Gossip:", err)
	}
	ok, err = h.blockStorage.Verify()
	if !ok {
		t.Fatal("Did not register with BlockStorage:", err)
	}
}

func (h *harness) handleBlockConsensus(ctx context.Context, mode handlers.HandleBlockConsensusMode, blockPair *protocol.BlockPairContainer, prevBlockPair *protocol.BlockPairContainer) error {
	_, err := h.service.HandleBlockConsensus(ctx, &handlers.HandleBlockConsensusInput{
		Mode:                   mode,
		BlockType:              protocol.BLOCK_TYPE_BLOCK_PAIR,
		BlockPair:              blockPair,
		PrevCommittedBlockPair: prevBlockPair,
	})
	return err
}
