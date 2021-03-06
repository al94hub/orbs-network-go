// Copyright 2019 the orbs-network-go authors
// This file is part of the orbs-network-go library in the Orbs project.
//
// This source code is licensed under the MIT license found in the LICENSE file in the root directory of this source tree.
// The above notice should be included in all copies or substantial portions of the software.

package ethereum

import (
	"bytes"
	"context"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/orbs-network/orbs-network-go/config"
	"github.com/orbs-network/orbs-network-go/instrumentation/logfields"
	"github.com/orbs-network/orbs-network-go/instrumentation/metric"
	"github.com/orbs-network/orbs-network-go/instrumentation/trace"
	"github.com/orbs-network/orbs-network-go/services/crosschainconnector/ethereum/adapter"
	"github.com/orbs-network/orbs-network-go/services/crosschainconnector/ethereum/timestampfinder"
	"github.com/orbs-network/orbs-spec/types/go/primitives"
	"github.com/orbs-network/orbs-spec/types/go/services"
	"github.com/orbs-network/scribe/log"
	"github.com/pkg/errors"
	"math/big"
	"strings"
)

var LogTag = log.Service("crosschain-connector")

type service struct {
	connection      adapter.EthereumConnection
	logger          log.Logger
	blockTimeGetter timestampfinder.BlockTimeGetter
	timestampFinder timestampfinder.TimestampFinder
	config          config.EthereumCrosschainConnectorConfig
}

func NewEthereumCrosschainConnector(connection adapter.EthereumConnection, config config.EthereumCrosschainConnectorConfig, parent log.Logger, metrics metric.Factory) services.CrosschainConnector {
	logger := parent.WithTags(LogTag)
	blockTimeGetter := timestampfinder.NewEthereumBasedBlockTimeGetter(connection)
	s := &service{
		connection:      connection,
		blockTimeGetter: blockTimeGetter,
		timestampFinder: timestampfinder.NewTimestampFinder(blockTimeGetter, logger, metrics),
		logger:          logger,
		config:          config,
	}
	return s
}

func (s *service) EthereumCallContract(ctx context.Context, input *services.EthereumCallContractInput) (*services.EthereumCallContractOutput, error) {
	logger := s.logger.WithTags(trace.LogFieldFrom(ctx))

	var ethereumBlockNumber int64

	if input.EthereumBlockNumber == 0 { // caller specified the latest block number possible
		ethereumBlockNumberAndTime, err := s.getFinalitySafeBlockNumber(ctx, input.ReferenceTimestamp)
		if err != nil {
			return nil, err
		}
		// TODO	https://github.com/orbs-network/orbs-network-go/issues/1214 simulator returns nil from FindBlockByTimestamp
		// TODO this could be a bug because it avoids stopping the func if we can't get latest finality block number !!!
		if ethereumBlockNumberAndTime != nil {
			ethereumBlockNumber = ethereumBlockNumberAndTime.BlockNumber
		}
	} else { // caller specified a non-zero block number
		ethereumBlockNumber = int64(input.EthereumBlockNumber)
		err := s.verifyBlockNumberIsFinalitySafe(ctx, input.EthereumBlockNumber, input.ReferenceTimestamp)
		if err != nil {
			return nil, err
		}
	}

	if ethereumBlockNumber != 0 { // TODO https://github.com/orbs-network/orbs-network-go/issues/1214  simulator returns nil from FindBlockByTimestamp
		logger.Info("calling contract from ethereum",
			log.String("address", input.EthereumContractAddress),
			log.Uint64("requested-block", input.EthereumBlockNumber),
			log.Int64("actual-block-requested", ethereumBlockNumber))
	}

	ethereumContractAddress, err := hexutil.Decode(input.EthereumContractAddress)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to decode the contract address %s", input.EthereumContractAddress)
	}

	var ethereumBlockNumberBigInt *big.Int
	if ethereumBlockNumber != 0 {
		ethereumBlockNumberBigInt = new(big.Int).SetInt64(ethereumBlockNumber)
	}
	output, err := s.connection.CallContract(ctx, ethereumContractAddress, input.EthereumAbiPackedInputArguments, ethereumBlockNumberBigInt)
	if err != nil {
		return nil, errors.Wrap(err, "ethereum call failed")
	}

	return &services.EthereumCallContractOutput{
		EthereumAbiPackedOutput: output,
	}, nil
}

func (s *service) EthereumGetTransactionLogs(ctx context.Context, input *services.EthereumGetTransactionLogsInput) (*services.EthereumGetTransactionLogsOutput, error) {
	logger := s.logger.WithTags(trace.LogFieldFrom(ctx))
	logger.Info("getting transaction logs", log.String("contract-address", input.EthereumContractAddress), log.String("event", input.EthereumEventName), logfields.Transaction(primitives.Sha256(input.EthereumTxhash)))

	ethereumTxHash, err := hexutil.Decode(input.EthereumTxhash)
	if err != nil {
		return nil, err
	}

	ethereumContractAddress, err := hexutil.Decode(input.EthereumContractAddress)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to decode the contract address %s", input.EthereumContractAddress)
	}

	parsedABI, err := abi.JSON(strings.NewReader(input.EthereumJsonAbi))
	if err != nil {
		return nil, err
	}

	eventABI, found := parsedABI.Events[input.EthereumEventName]
	if !found {
		return nil, errors.Errorf("event with name '%s' not found in given ABI", input.EthereumEventName)
	}

	logs, err := s.connection.GetTransactionLogs(ctx, ethereumTxHash, eventABI.ID().Bytes())
	if err != nil {
		return nil, errors.Wrapf(err, "failed getting logs for Ethereum txhash %s of contract %s", input.EthereumTxhash, input.EthereumContractAddress)
	}

	// TODO(https://github.com/orbs-network/orbs-network-go/issues/597): support multiple logs
	if len(logs) != 1 {
		return nil, errors.Errorf("expected exactly one log entry for txhash %s of contract %s but got %d", input.EthereumTxhash, input.EthereumContractAddress, len(logs))
	}

	ethereumContractAddressResult := logs[0].ContractAddress
	ethereumBlockNumberResult := logs[0].BlockNumber
	ethereumTxIndexResult := logs[0].TxIndex

	if !bytes.Equal(ethereumContractAddress, ethereumContractAddressResult) {
		return nil, errors.Errorf("Ethereum txhash %s is under contract %s and not %s", input.EthereumTxhash, hexutil.Encode(ethereumContractAddressResult), input.EthereumContractAddress)
	}

	err = s.verifyBlockNumberIsFinalitySafe(ctx, ethereumBlockNumberResult, input.ReferenceTimestamp)
	if err != nil {
		return nil, err
	}

	output, err := repackEventABIWithTopics(eventABI, logs[0])
	if err != nil {
		return nil, err
	}

	return &services.EthereumGetTransactionLogsOutput{
		EthereumAbiPackedOutputs: [][]byte{output},
		EthereumBlockNumber:      ethereumBlockNumberResult,
		EthereumTxindex:          ethereumTxIndexResult,
	}, nil
}

func (s *service) EthereumGetBlockNumber(ctx context.Context, input *services.EthereumGetBlockNumberInput) (*services.EthereumGetBlockNumberOutput, error) {
	//	logger := s.logger.WithTags(trace.LogFieldFrom(ctx))
	//	logger.Info("getting current safe Ethereum block number")

	blockNumberAndTime, err := s.getFinalitySafeBlockNumber(ctx, input.ReferenceTimestamp)
	if err != nil {
		return nil, err
	}

	return &services.EthereumGetBlockNumberOutput{
		EthereumBlockNumber: uint64(blockNumberAndTime.BlockNumber),
	}, nil
}

func (s *service) EthereumGetBlockNumberByTime(ctx context.Context, input *services.EthereumGetBlockNumberByTimeInput) (*services.EthereumGetBlockNumberByTimeOutput, error) {
	blockNumberAndTime, err := s.timestampFinder.FindBlockByTimestamp(ctx, input.EthereumTimestamp)
	if err != nil {
		return nil, err
	}

	err = s.verifyBlockNumberIsFinalitySafe(ctx, uint64(blockNumberAndTime.BlockNumber), input.ReferenceTimestamp)
	if err != nil {
		return nil, err
	}

	return &services.EthereumGetBlockNumberByTimeOutput{
		EthereumBlockNumber: uint64(blockNumberAndTime.BlockNumber),
	}, nil
}

func (s *service) EthereumGetBlockTime(ctx context.Context, input *services.EthereumGetBlockTimeInput) (*services.EthereumGetBlockTimeOutput, error) {
	blockNumberAndTime, err := s.getFinalitySafeBlockNumber(ctx, input.ReferenceTimestamp)
	if err != nil {
		return nil, err
	}

	return &services.EthereumGetBlockTimeOutput{
		EthereumTimestamp: blockNumberAndTime.BlockTimeNano,
	}, nil
}

func (s *service) EthereumGetBlockTimeByNumber(ctx context.Context, input *services.EthereumGetBlockTimeByNumberInput) (*services.EthereumGetBlockTimeByNumberOutput, error) {
	blockNumberAndTime, err := s.blockTimeGetter.GetTimestampForBlockNumber(ctx, big.NewInt(int64(input.EthereumBlockNumber)))
	if err != nil {
		return nil, err
	}

	err = s.verifyBlockNumberIsFinalitySafe(ctx, uint64(blockNumberAndTime.BlockNumber), input.ReferenceTimestamp)
	if err != nil {
		return nil, err
	}

	return &services.EthereumGetBlockTimeByNumberOutput{
		EthereumTimestamp: blockNumberAndTime.BlockTimeNano,
	}, nil
}
