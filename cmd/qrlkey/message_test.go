// Copyright 2018 The go-ethereum Authors
// This file is part of go-ethereum.
//
// go-ethereum is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// go-ethereum is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with go-ethereum. If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/theQRL/go-qrl/accounts/keystore"
	"github.com/theQRL/go-qrl/cmd/utils"
	"github.com/theQRL/go-qrl/common"
)

func TestMessageSignVerify(t *testing.T) {
	t.Parallel()
	tmpdir := t.TempDir()

	keyfile := filepath.Join(tmpdir, "the-keyfile")
	message := "test message"

	// Create the key.
	generate := runQRLkey(t, "generate", "--lightkdf", keyfile)
	generate.Expect(`
!! Unsupported terminal, password will be echoed.
Password: {{.InputLine "foobar"}}
Repeat password: {{.InputLine "foobar"}}
`)
	_, addrMatches := generate.ExpectRegexp(`Address: (Q[0-9a-fA-F]{128})\n`)
	address := addrMatches[1]
	generate.ExpectExit()

	// Sign a message.
	sign := runQRLkey(t, "signmessage", keyfile, message)
	sign.Expect(`
!! Unsupported terminal, password will be echoed.
Password: {{.InputLine "foobar"}}
`)
	_, matches := sign.ExpectRegexp(`Signature: ([0-9a-f]+)\n`)
	signature := matches[1]
	sign.ExpectExit()

	// Read key from file.
	keyjson, err := os.ReadFile(keyfile)
	if err != nil {
		utils.Fatalf("Failed to read the keyfile at '%s': %v", keyfile, err)
	}

	// Decrypt key with passphrase.
	key, err := keystore.DecryptKey(keyjson, "foobar")
	if err != nil {
		utils.Fatalf("Error decrypting key: %v", err)
	}

	// Verify the message; the printed address must be the key's address.
	publicKey := key.Wallet.GetPK()
	verify := runQRLkey(t, "verifymessage", signature, common.Bytes2Hex(publicKey[:]), message)
	_, verifyMatches := verify.ExpectRegexp(`
Signature verification successful!
Address: (Q[0-9a-fA-F]{128})
`)
	verify.ExpectExit()
	if verifyMatches[1] != address {
		t.Fatalf("verifymessage printed address %s, want %s", verifyMatches[1], address)
	}

	// Verify with the matching expected address.
	verify = runQRLkey(t, "verifymessage", "--address", address, signature, common.Bytes2Hex(publicKey[:]), message)
	verify.ExpectRegexp(`
Signature verification successful!
Address: ` + address + `
`)
	verify.ExpectExit()

	// A different expected address must fail even though the signature is valid.
	other := "Q" + "00" + address[3:]
	if other == address {
		other = "Q" + "11" + address[3:]
	}
	verify = runQRLkey(t, "verifymessage", "--address", other, signature, common.Bytes2Hex(publicKey[:]), message)
	verify.ExpectRegexp(`
Signature verification failed!
`)
	verify.ExpectExit()
}
