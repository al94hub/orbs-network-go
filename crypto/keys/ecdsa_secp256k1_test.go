// Copyright 2019 the orbs-network-go authors
// This file is part of the orbs-network-go library in the Orbs project.
//
// This source code is licensed under the MIT license found in the LICENSE file in the root directory of this source tree.
// The above notice should be included in all copies or substantial portions of the software.

package keys

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestGenerateEcdsaSecp256K1Key(t *testing.T) {
	keyPair, err := GenerateEcdsaSecp256K1Key()
	require.NoError(t, err, "should not fail")

	t.Logf("Public: %s", keyPair.PublicKeyHex())
	t.Logf("Private: %s", keyPair.PrivateKeyHex())
	require.Equal(t, ECDSA_SECP256K1_PUBLIC_KEY_SIZE_BYTES, len(keyPair.publicKey), "public key length should match")
	require.Equal(t, ECDSA_SECP256K1_PRIVATE_KEY_SIZE_BYTES, len(keyPair.privateKey), "private key length should match")
}

func TestGenerate10KeysForTests(t *testing.T) {
	for i := 0; i < 10; i++ {
		keyPair, err := GenerateEcdsaSecp256K1Key()
		require.NoError(t, err)
		fmt.Printf("{\"%s\", \"%s\"},\n", keyPair.PublicKeyHex(), keyPair.PrivateKeyHex())
	}
}
