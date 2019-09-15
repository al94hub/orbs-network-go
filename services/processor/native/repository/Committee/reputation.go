// Copyright 2019 the orbs-network-go authors
// This file is part of the orbs-network-go library in the Orbs project.
//
// This source code is licensed under the MIT license found in the LICENSE file in the root directory of this source tree.
// The above notice should be included in all copies or substantial portions of the software.

package committee_systemcontract

import (
	"encoding/hex"
	"fmt"
	"github.com/orbs-network/orbs-contract-sdk/go/sdk/v1/safemath/safeuint32"
	"github.com/orbs-network/orbs-contract-sdk/go/sdk/v1/state"
)

const ToleranceLevel = uint32(4)
const ReputationBottomCap = uint32(10)

func _formatMisses(addr []byte) []byte {
	return []byte(fmt.Sprintf("Address_%s_Misses", hex.EncodeToString(addr)))
}

func getMisses(addr []byte) uint32 {
	return state.ReadUint32(_formatMisses(addr))
}

func _addMiss(addr []byte) {
	currMiss := getMisses(addr)
	state.WriteUint32(_formatMisses(addr), safeuint32.Add(currMiss, 1))
}

func getReputation(addr []byte) uint32 {
	currMiss := getMisses(addr)
	if currMiss < ToleranceLevel {
		return 0
	}
	if currMiss < ReputationBottomCap {
		return currMiss
	}
	return ReputationBottomCap
}

func _clearMiss(addr []byte) {
	state.Clear(_formatMisses(addr))
}