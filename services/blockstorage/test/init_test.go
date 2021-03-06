// Copyright 2019 the orbs-network-go authors
// This file is part of the orbs-network-go library in the Orbs project.
//
// This source code is licensed under the MIT license found in the LICENSE file in the root directory of this source tree.
// The above notice should be included in all copies or substantial portions of the software.

package test

import (
	"context"
	"github.com/orbs-network/orbs-network-go/test/with"
	"github.com/orbs-network/orbs-spec/types/go/services"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestInitSetsLastCommittedBlockHeightToZero(t *testing.T) {
	with.Concurrency(t, func(ctx context.Context, parent *with.ConcurrencyHarness) {
		harness := newBlockStorageHarness(parent).
			withSyncBroadcast(1).
			expectValidateConsensusAlgos().
			start(ctx)

		val, err := harness.blockStorage.GetLastCommittedBlockHeight(ctx, &services.GetLastCommittedBlockHeightInput{})
		require.NoError(t, err)

		require.EqualValues(t, 0, val.LastCommittedBlockHeight)
		require.EqualValues(t, 0, val.LastCommittedBlockTimestamp)

		harness.verifyMocks(t, 1)
	})
}

func TestInitSetsLastCommittedBlockHeightFromPersistence(t *testing.T) {
	with.Concurrency(t, func(ctx context.Context, parent *with.ConcurrencyHarness) {
		harness := newBlockStorageHarness(parent).withSyncBroadcast(1).expectValidateConsensusAlgos()
		now := harness.setupCustomBlocksForInit()
		harness.start(ctx)

		val, err := harness.blockStorage.GetLastCommittedBlockHeight(ctx, &services.GetLastCommittedBlockHeightInput{})
		require.NoError(t, err)

		require.EqualValues(t, 10, val.LastCommittedBlockHeight)
		require.EqualValues(t, now.UnixNano(), val.LastCommittedBlockTimestamp)

		harness.verifyMocks(t, 1)
	})
}
