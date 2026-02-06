// Copyright 2022 The go-ethereum Authors
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
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestImportRaw tests clef --importraw
func TestImportRaw(t *testing.T) {
	t.Parallel()
	keyPath := filepath.Join(t.TempDir(), fmt.Sprintf("%v-tempkey.test", t.Name()))
	os.WriteFile(keyPath, []byte("0100003bf5fbadaa64ce2974d2464dc17fdac669557f05bd5329c35aa84e85636d0565f5cb4d3427d6033ccabf0b6c3f157bb5"), 0777)

	t.Run("happy-path", func(t *testing.T) {
		t.Parallel()
		// Run clef importraw
		clef := runClef(t, "--suppress-bootwarn", "--lightkdf", "importraw", keyPath)
		clef.input("myverylongpassword").input("myverylongpassword")
		if out := string(clef.Output()); !strings.Contains(out,
			"Key imported:\n  Address QBb81a0496aa34a64F96C2BCd28793165E1E6c08a") {
			t.Logf("Output\n%v", out)
			t.Error("Failure")
		}
	})
	// tests clef --importraw with mismatched passwords.
	t.Run("pw-mismatch", func(t *testing.T) {
		t.Parallel()
		// Run clef importraw
		clef := runClef(t, "--suppress-bootwarn", "--lightkdf", "importraw", keyPath)
		clef.input("myverylongpassword1").input("myverylongpassword2").WaitExit()
		if have, want := clef.StderrText(), "Passwords do not match\n"; have != want {
			t.Errorf("have %q, want %q", have, want)
		}
	})
	// tests clef --importraw with a too short password.
	t.Run("short-pw", func(t *testing.T) {
		t.Parallel()
		// Run clef importraw
		clef := runClef(t, "--suppress-bootwarn", "--lightkdf", "importraw", keyPath)
		clef.input("shorty").input("shorty").WaitExit()
		if have, want := clef.StderrText(),
			"password requirements not met: password too short (<10 characters)\n"; have != want {
			t.Errorf("have %q, want %q", have, want)
		}
	})
}

// TestListAccounts tests clef --list-accounts
func TestListAccounts(t *testing.T) {
	t.Parallel()
	keyPath := filepath.Join(t.TempDir(), fmt.Sprintf("%v-tempkey.test", t.Name()))
	os.WriteFile(keyPath, []byte("0100003bf5fbadaa64ce2974d2464dc17fdac669557f05bd5329c35aa84e85636d0565f5cb4d3427d6033ccabf0b6c3f157bb5"), 0777)

	t.Run("no-accounts", func(t *testing.T) {
		t.Parallel()
		clef := runClef(t, "--suppress-bootwarn", "--lightkdf", "list-accounts")
		if out := string(clef.Output()); !strings.Contains(out, "The keystore is empty.") {
			t.Logf("Output\n%v", out)
			t.Error("Failure")
		}
	})
	t.Run("one-account", func(t *testing.T) {
		t.Parallel()
		// First, we need to import
		clef := runClef(t, "--suppress-bootwarn", "--lightkdf", "importraw", keyPath)
		clef.input("myverylongpassword").input("myverylongpassword").WaitExit()
		// Secondly, do a listing, using the same datadir
		clef = runWithKeystore(t, clef.Datadir, "--suppress-bootwarn", "--lightkdf", "list-accounts")
		if out := string(clef.Output()); !strings.Contains(out, "QBb81a0496aa34a64F96C2BCd28793165E1E6c08a (keystore:") {
			t.Logf("Output\n%v", out)
			t.Error("Failure")
		}
	})
}

// TestListWallets tests clef --list-wallets
func TestListWallets(t *testing.T) {
	t.Parallel()
	keyPath := filepath.Join(t.TempDir(), fmt.Sprintf("%v-tempkey.test", t.Name()))
	os.WriteFile(keyPath, []byte("0100003bf5fbadaa64ce2974d2464dc17fdac669557f05bd5329c35aa84e85636d0565f5cb4d3427d6033ccabf0b6c3f157bb5"), 0777)

	t.Run("no-accounts", func(t *testing.T) {
		t.Parallel()
		clef := runClef(t, "--suppress-bootwarn", "--lightkdf", "list-wallets")
		if out := string(clef.Output()); !strings.Contains(out, "There are no wallets.") {
			t.Logf("Output\n%v", out)
			t.Error("Failure")
		}
	})
	t.Run("one-account", func(t *testing.T) {
		t.Parallel()
		// First, we need to import
		clef := runClef(t, "--suppress-bootwarn", "--lightkdf", "importraw", keyPath)
		clef.input("myverylongpassword").input("myverylongpassword").WaitExit()
		// Secondly, do a listing, using the same datadir
		clef = runWithKeystore(t, clef.Datadir, "--suppress-bootwarn", "--lightkdf", "list-wallets")
		if out := string(clef.Output()); !strings.Contains(out, "Account 0: QBb81a0496aa34a64F96C2BCd28793165E1E6c08a") {
			t.Logf("Output\n%v", out)
			t.Error("Failure")
		}
	})
}
