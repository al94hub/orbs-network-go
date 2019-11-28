// Copyright 2019 the orbs-network-go authors
// This file is part of the orbs-network-go library in the Orbs project.
//
// This source code is licensed under the MIT license found in the LICENSE file in the root directory of this source tree.
// The above notice should be included in all copies or substantial portions of the software.

package e2e

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"testing"
	"time"
)

func TestContractExperimentalLibraries(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E tests in short mode")
	}

	if !GetConfig().IsExperimental {
		t.Skip("skipping because it's a tagged release")
	}

	runMultipleTimes(t, func(t *testing.T) {
		h := NewAppHarness()
		lt := time.Now()
		PrintTestTime(t, "started", &lt)

		h.WaitUntilTransactionPoolIsReady(t)
		PrintTestTime(t, "first block committed", &lt)

		counterStart := uint64(time.Now().UnixNano())
		contractName := fmt.Sprintf("Experimental%d", counterStart)
		contractSource, err := ioutil.ReadFile("../contracts/_experimental/experimental.go")
		require.NoError(t, err, "failed loading contract source")

		PrintTestTime(t, "send deploy - start", &lt)

		blockHeight := h.DeployContractAndRequireSuccess(t, OwnerOfAllSupply, contractName, contractSource)

		PrintTestTime(t, "send deploy - end", &lt)

		// warmup call
		_, err = h.runQueryAtBlockHeight(5*time.Second, blockHeight, OwnerOfAllSupply.PublicKey(), contractName, "get", uint64(0))
		require.NoError(t, err)

		PrintTestTime(t, "send transaction - start", &lt)
		addResponse, _, err := h.SendTransaction(OwnerOfAllSupply.PublicKey(), OwnerOfAllSupply.PrivateKey(), contractName, "add", "Diamond Dogs")
		PrintTestTime(t, "send transaction - end", &lt)

		require.NoError(t, err, "add transaction should not return error")
		requireSuccessful(t, addResponse)

		queryResponse, err := h.runQueryAtBlockHeight(5*time.Second, addResponse.BlockHeight, OwnerOfAllSupply.PublicKey(), contractName, "get", uint64(0))
		require.NoError(t, err)
		require.EqualValues(t, "Diamond Dogs", queryResponse.OutputArguments[0])

	})
}
