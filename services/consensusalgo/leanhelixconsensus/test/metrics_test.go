// +build !race

package test

import (
	"context"
	"github.com/orbs-network/lean-helix-go/services/blockproof"
	"github.com/orbs-network/lean-helix-go/services/interfaces"
	"github.com/orbs-network/lean-helix-go/services/randomseed"
	lhprimitives "github.com/orbs-network/lean-helix-go/spec/types/go/primitives"
	"github.com/orbs-network/lean-helix-go/spec/types/go/protocol"
	"github.com/orbs-network/orbs-network-go/test"
	"github.com/orbs-network/orbs-network-go/test/builders"
	"github.com/orbs-network/orbs-network-go/test/crypto/keys"
	"github.com/orbs-network/orbs-network-go/test/with"
	"github.com/orbs-network/orbs-spec/types/go/primitives"
	"github.com/stretchr/testify/require"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestMetricsAreUpdatedOnElectionTrigger(t *testing.T) {

	with.Concurrency(t, func(ctx context.Context, parent *with.ConcurrencyHarness) {
		h := newSingleLhcNodeHarness().
			withBaseConsensusRoundTimeout(10*time.Millisecond).
			start(parent, ctx)

		h.beFirstInCommittee() // just so RequestOrderedCommittee will succeed
		h.expectGossipSendLeanHelixMessage()

		h.handleBlockSync(ctx, 1)

		// Election trigger should fire and metrics should be updated

		metrics := h.getMetrics()

		require.True(t, test.Eventually(1*time.Second, func() bool {
			return metrics.currentElectionCount.Value() > 0
		}), "expected currentElectionCount metric to update")

		require.True(t, test.Eventually(1*time.Second, func() bool {
			return metrics.currentLeaderMemberId.Value() != ""
		}), "expected currentLeaderMemberId metric to update")

		require.True(t, test.Eventually(1*time.Second, func() bool {
			matched, err := regexp.MatchString("samples=[1-9]", metrics.timeSinceLastElectionMillis.String())
			if err != nil {
				panic(err)
			}
			return matched
		}), "expected timeSinceLastElectionMillis metric to update")
	})

}

func TestElectionCountMetricIsResetOnNewTerm(t *testing.T) {

	with.Concurrency(t, func(ctx context.Context, parent *with.ConcurrencyHarness) {
		h := newSingleLhcNodeHarness().
			start(parent, ctx)

		h.dontBeFirstInCommitee()
		h.expectGossipSendLeanHelixMessage()
		h.expectValidateTransactionBlock()
		h.expectValidateResultsBlock()
		h.expectCommitBlock()

		const baseHeight = 1
		const view = 0

		h.handleBlockSync(ctx, baseHeight)

		// Election trigger should fire and metrics should be updated
		metrics := h.getMetrics()

		// By block sync
		metrics.currentElectionCount.Update(-1) // So updating to 0 is meaningful
		h.handleBlockSync(ctx, baseHeight+1)
		require.True(t, test.Eventually(1*time.Second, func() bool {
			return metrics.currentElectionCount.Value() == view
		}), "expected currentElectionCount metric to update")

		// And by commit

		randomSeed := getInitialRandomSeed()
		advanceConsesnsusToNextBlock(t, ctx, h, randomSeed, baseHeight+2, view)
		metrics.currentElectionCount.Update(-1) // So updating it to 0 is meaningful
		advanceConsesnsusToNextBlock(t, ctx, h, randomSeed, baseHeight+3, view)
		require.True(t, test.Eventually(1*time.Second, func() bool {
			return metrics.currentElectionCount.Value() == view
		}), "expected currentElectionCount metric to update")

	})

}

func TestMetricsAreUpdatedOnCommit(t *testing.T) {

	with.Concurrency(t, func(ctx context.Context, parent *with.ConcurrencyHarness) {
		h := newSingleLhcNodeHarness().start(parent, ctx)

		h.expectGossipSendLeanHelixMessage()
		h.expectValidateTransactionBlock()
		h.expectValidateResultsBlock()
		h.expectCommitBlock()

		h.dontBeFirstInCommitee()

		const syncedBlockHeight = 1
		h.handleBlockSync(ctx, syncedBlockHeight)

		metrics := h.getMetrics()

		const firstTermHeight = syncedBlockHeight + 1
		const view = 0

		randomSeed := getInitialRandomSeed()

		metrics.viewAtLastCommit.Update(-1) // So updating it to 0 is meaningful
		randomSeed = advanceConsesnsusToNextBlock(t, ctx, h, randomSeed, firstTermHeight, view)

		// At this point the first block should be committed, lastCommitTime should update
		now := time.Now()
		require.True(t, test.Eventually(1*time.Second, func() bool {
			return abs(now.UnixNano()-metrics.lastCommittedTime.Value()) < int64(time.Minute)
		}), "expected lastCommittedTime to update")

		require.True(t, test.Eventually(1*time.Second, func() bool {
			return metrics.viewAtLastCommit.Value() == view
		}), "expected viewAtLastCommit to update")

		// timeSinceLastCommitMillis will NOT update because this is the first commit
		require.True(t, strings.Contains(metrics.timeSinceLastCommitMillis.String(), "samples=0"), "expected timeSinceLastCommitMillis not to update on first commit")

		firstTermCommitTime := metrics.lastCommittedTime.Value()

		// Starting the second term

		const secondTermHeight = firstTermHeight + 1

		metrics.viewAtLastCommit.Update(-1) // So updating it to 0 is meaningful
		advanceConsesnsusToNextBlock(t, ctx, h, randomSeed, secondTermHeight, view)

		// A second commit should take place now and this time timeSinceLastCommitMillis should update

		require.True(t, test.Eventually(1*time.Second, func() bool {
			matched, err := regexp.MatchString("samples=[1-9]", metrics.timeSinceLastCommitMillis.String())
			if err != nil {
				panic(err)
			}
			return matched
		}), "expected timeSinceLastCommitMillis metric to update")

		require.True(t, test.Eventually(1*time.Second, func() bool {
			return metrics.lastCommittedTime.Value() > firstTermCommitTime
		}), "expected lastCommittedTime to increase after the second commit")

		require.True(t, test.Eventually(1*time.Second, func() bool {
			return metrics.viewAtLastCommit.Value() == view
		}), "expected viewAtLastCommit to update")

	})

}

func getInitialRandomSeed() uint64 {
	prevBlockProof := protocol.BlockProofReader(nil) // the previous block (from sync) had no proof
	return randomseed.CalculateRandomSeed(prevBlockProof.RandomSeedSignature())
}

func advanceConsesnsusToNextBlock(t *testing.T, ctx context.Context, h *singleLhcNodeHarness, randomSeed uint64, height primitives.BlockHeight, view lhprimitives.View) uint64 {
	orderedCommittee := h.requestOrderingCommittee(ctx)
	orderedComitteeNodeIndicies := keys.NodeAddressesForTestsToIndexes(orderedCommittee.NodeAddresses)
	leaderNodeIndex := orderedComitteeNodeIndicies[int(view)%len(orderedComitteeNodeIndicies)] // leader for view 0 is always the first member in the ordered committee

	blockPair := builders.BlockPair().WithHeight(height).WithTransactions(1).WithEmptyLeanHelixBlockProof().Build()

	h.handlePreprepareMessage(ctx, blockPair, height, view, leaderNodeIndex)

	commitMessages := []*interfaces.CommitMessage{}
	for i := 0; i < h.networkSize(); i++ {
		cmsg := h.handleCommitMessage(ctx, blockPair, height, view, randomSeed, i)
		commitMessages = append(commitMessages, cmsg)
	}

	// Use commits from previous term to calculate the random seed for the next term
	proof := blockproof.GenerateLeanHelixBlockProof(h.keyManagerForNode(h.nodeIndex()), commitMessages)
	prevBlockProof := protocol.BlockProofReader(proof.Raw())
	randomSeedForNextTerm := randomseed.CalculateRandomSeed(prevBlockProof.RandomSeedSignature())

	return randomSeedForNextTerm
}

func abs(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}
