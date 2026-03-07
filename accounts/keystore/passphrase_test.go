// Copyright 2016 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package keystore

import (
	"os"
	"testing"

	"github.com/theQRL/go-qrl/common"
)

const (
	veryLightArgon2idT uint32 = 8
	veryLightArgon2idM uint32 = 2
	veryLightArgon2idP uint8  = 1
)

// Tests that a json key file can be decrypted and encrypted in multiple rounds.
func TestKeyEncryptDecrypt(t *testing.T) {
	t.Parallel()
	keyjson, err := os.ReadFile("testdata/very-light-argon2id.json")
	if err != nil {
		t.Fatal(err)
	}
	password := ""
	address, _ := common.NewAddressFromString("Qcf3a354d32c36db1e9c27602bd3c154f5679401e")

	// Do a few rounds of decryption and encryption
	for i := range 3 {
		// Try a bad password first
		if _, err := DecryptKey(keyjson, password+"bad"); err == nil {
			t.Errorf("test %d: json key decrypted with bad password", i)
		}
		// Decrypt with the correct password
		key, err := DecryptKey(keyjson, password)
		if err != nil {
			t.Fatalf("test %d: json key failed to decrypt: %v", i, err)
		}
		if key.Address != address {
			t.Errorf("test %d: key address mismatch: have %x, want %x", i, key.Address, address)
		}
		// Re-encrypt with a new password and start over
		password += "new data appended" // nolint: gosec
		if keyjson, err = EncryptKey(key, password, veryLightArgon2idT, veryLightArgon2idM, veryLightArgon2idP); err != nil {
			t.Errorf("test %d: failed to re-encrypt key %v", i, err)
		}
	}
}
