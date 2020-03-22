// Copyright 2019 the orbs-network-go authors
// This file is part of the orbs-network-go library in the Orbs project.
//
// This source code is licensed under the MIT license found in the LICENSE file in the root directory of this source tree.
// The above notice should be included in all copies or substantial portions of the software.

package sdk

import (
	"context"
	"errors"
	"fmt"
	"github.com/orbs-network/orbs-network-go/config"
	"github.com/orbs-network/orbs-network-go/crypto/hash"
	"github.com/orbs-network/orbs-network-go/instrumentation/metric"
	"github.com/orbs-network/orbs-network-go/services/processor/native"
	"github.com/orbs-network/orbs-network-go/services/processor/native/testkit"
	"github.com/orbs-network/orbs-network-go/services/statestorage"
	stateAdapter "github.com/orbs-network/orbs-network-go/services/statestorage/adapter/memory"
	"github.com/orbs-network/orbs-network-go/services/virtualmachine"
	testCommitteeProvider "github.com/orbs-network/orbs-network-go/services/virtualmachine/test"
	"github.com/orbs-network/orbs-network-go/test/builders"
	"github.com/orbs-network/orbs-spec/types/go/protocol"
	"github.com/orbs-network/orbs-spec/types/go/services"
	"github.com/orbs-network/orbs-spec/types/go/services/handlers"
	"github.com/orbs-network/scribe/log"
)

const ContractName = "TestContract"

func generateDeployTx() *protocol.SignedTransaction {
	return builders.Transaction().WithMethod("_Deployments", "deployService").
		WithArgs(ContractName, uint32(protocol.PROCESSOR_TYPE_NATIVE), []byte("irrelevant data - contract is already registered")).
		Build()
}

type harness struct {
	vm                services.VirtualMachine
	repository        *testkit.ManualRepository
	committeeProvider virtualmachine.CommitteeProvider
}

func newVmHarness(logger log.Logger) *harness {
	registry := metric.NewRegistry()

	ssCfg := config.ForStateStorageTest(10, 5, 5000)
	ssPersistence := stateAdapter.NewStatePersistence(registry)
	stateStorage := statestorage.NewStateStorage(ssCfg, ssPersistence, nil, logger, registry)
	committeeProvider := testCommitteeProvider.NewTestCommitteeProvider(5)

	sdkCallHandler := &handlers.MockContractSdkCallHandler{}
	psCfg := config.ForNativeProcessorTests(42)
	repo := testkit.NewRepository()

	processorService := native.NewProcessorWithContractRepository(repo, psCfg, logger, registry)
	processorService.RegisterContractSdkCallHandler(sdkCallHandler)

	processorMap := map[protocol.ProcessorType]services.Processor{protocol.PROCESSOR_TYPE_NATIVE: processorService}
	crosschainConnectors := make(map[protocol.CrosschainConnectorType]services.CrosschainConnector)
	crosschainConnectors[protocol.CROSSCHAIN_CONNECTOR_TYPE_ETHEREUM] = &services.MockCrosschainConnector{}
	vm := virtualmachine.NewVirtualMachine(stateStorage, processorMap, crosschainConnectors, committeeProvider, logger)

	return &harness{
		committeeProvider: committeeProvider,
		vm:         vm,
		repository: repo,
	}
}

func (h *harness) processSuccessfully(ctx context.Context, txs ...*protocol.SignedTransaction) ([]*protocol.TransactionReceipt, error) {
	out, err := h.process(ctx, txs...)
	if err != nil {
		return nil, err
	}

	for i := 0; i < len(txs); i++ {
		executionResult := out.TransactionReceipts[i].ExecutionResult()
		if protocol.EXECUTION_RESULT_SUCCESS != out.TransactionReceipts[i].ExecutionResult() {
			return nil, errors.New(fmt.Sprintf("tx %d should succeed. execution res was %s", i, executionResult))
		}
	}

	return out.TransactionReceipts, nil
}

func (h *harness) process(ctx context.Context, txs ...*protocol.SignedTransaction) (*services.ProcessTransactionSetOutput, error) {
	return h.vm.ProcessTransactionSet(ctx, &services.ProcessTransactionSetInput{
		CurrentBlockHeight:    1,
		CurrentBlockTimestamp: 66,
		SignedTransactions:    txs,
		BlockProposerAddress:  hash.Make32BytesWithFirstByte(5),
	})
}
