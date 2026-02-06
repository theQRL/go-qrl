// Copyright 2016 The go-ethereum Authors
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
	"runtime"
	"testing"

	"github.com/cespare/cp"
)

// These tests are 'smoke tests' for the account related
// subcommands and flags.
//
// For most tests, the test files from package accounts
// are copied into a temporary keystore directory.

func tmpDatadirWithKeystore(t *testing.T) string {
	datadir := t.TempDir()
	keystore := filepath.Join(datadir, "keystore")
	source := filepath.Join("..", "..", "accounts", "keystore", "testdata", "keystore")
	if err := cp.CopyAll(keystore, source); err != nil {
		t.Fatal(err)
	}
	return datadir
}

func TestAccountListEmpty(t *testing.T) {
	t.Parallel()
	gzond := runGzond(t, "account", "list")
	gzond.ExpectExit()
}

func TestAccountList(t *testing.T) {
	t.Parallel()
	datadir := tmpDatadirWithKeystore(t)
	var want = `
Account #0: {Q31fec69ece96b8cdac5814ff9dd92759e7c6018b} keystore://{{.Datadir}}/keystore/UTC--2025-11-06T07-34-54.273240000Z--Q31fec69ece96b8cdac5814ff9dd92759e7c6018b
Account #1: {Q4cce0507b955d0c7e6b79269b66ed498c670bb0a} keystore://{{.Datadir}}/keystore/aaa
Account #2: {Q2d9b972ef8219246c73363fd7c048cef81456f9d} keystore://{{.Datadir}}/keystore/zzz
`
	if runtime.GOOS == "windows" {
		want = `
Account #0: {Q31fec69ece96b8cdac5814ff9dd92759e7c6018b} keystore://{{.Datadir}}\keystore\UTC--2025-11-06T07-34-54.273240000Z--Q31fec69ece96b8cdac5814ff9dd92759e7c6018b
Account #1: {Q4cce0507b955d0c7e6b79269b66ed498c670bb0a} keystore://{{.Datadir}}\keystore\aaa
Account #2: {Q2d9b972ef8219246c73363fd7c048cef81456f9d} keystore://{{.Datadir}}\keystore\zzz
`
	}
	{
		gzond := runGzond(t, "account", "list", "--datadir", datadir)
		gzond.Expect(want)
		gzond.ExpectExit()
	}
	{
		gzond := runGzond(t, "--datadir", datadir, "account", "list")
		gzond.Expect(want)
		gzond.ExpectExit()
	}
}

func TestAccountNew(t *testing.T) {
	t.Parallel()
	gzond := runGzond(t, "account", "new", "--lightkdf")
	defer gzond.ExpectExit()
	gzond.Expect(`
Your new account is locked with a password. Please give a password. Do not forget this password.
!! Unsupported terminal, password will be echoed.
Password: {{.InputLine "foobar"}}
Repeat password: {{.InputLine "foobar"}}

Your new key was generated
`)
	gzond.ExpectRegexp(`
Public address of the key:   Q[0-9a-fA-F]{40}
Path of the secret key file: .*UTC--.+--Q[0-9a-f]{40}

- You can share your public address with anyone. Others need it to interact with you.
- You must NEVER share the secret key with anyone! The key controls access to your funds!
- You must BACKUP your key file! Without the key, it's impossible to access account funds!
- You must REMEMBER your password! Without the password, it's impossible to decrypt the key!
`)
}

func TestAccountImport(t *testing.T) {
	t.Parallel()
	tests := []struct{ name, seed, output string }{
		{
			name:   "correct account",
			seed:   "0100000123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdeffcad0b19bb29d4674531d6f115237e16",
			output: "Address: {Q958d36976b91586a10341cf20c7dfbcb122a1065}\n",
		},
		{
			name:   "invalid character",
			seed:   "0100000123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdeffcad0b19bb29d4674531d6f115237e161",
			output: "Fatal: Failed to restore wallet from file: invalid character '1' at end of key file\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			importAccountWithExpect(t, test.seed, test.output)
		})
	}
}

func TestAccountHelp(t *testing.T) {
	t.Parallel()
	gzond := runGzond(t, "account", "-h")
	gzond.WaitExit()
	if have, want := gzond.ExitStatus(), 0; have != want {
		t.Errorf("exit error, have %d want %d", have, want)
	}

	gzond = runGzond(t, "account", "import", "-h")
	gzond.WaitExit()
	if have, want := gzond.ExitStatus(), 0; have != want {
		t.Errorf("exit error, have %d want %d", have, want)
	}
}

func importAccountWithExpect(t *testing.T, seed string, expected string) {
	dir := t.TempDir()
	seedfile := filepath.Join(dir, "seed.txt")
	if err := os.WriteFile(seedfile, []byte(seed), 0600); err != nil {
		t.Error(err)
	}
	passwordFile := filepath.Join(dir, "password.txt")
	if err := os.WriteFile(passwordFile, []byte("foobar"), 0600); err != nil {
		t.Error(err)
	}
	gzond := runGzond(t, "--lightkdf", "account", "import", "-password", passwordFile, seedfile)
	defer gzond.ExpectExit()
	gzond.Expect(expected)
}

func TestAccountNewBadRepeat(t *testing.T) {
	t.Parallel()
	gzond := runGzond(t, "account", "new", "--lightkdf")
	defer gzond.ExpectExit()
	gzond.Expect(`
Your new account is locked with a password. Please give a password. Do not forget this password.
!! Unsupported terminal, password will be echoed.
Password: {{.InputLine "something"}}
Repeat password: {{.InputLine "something else"}}
Fatal: Passwords do not match
`)
}

func TestAccountUpdate(t *testing.T) {
	t.Parallel()
	datadir := tmpDatadirWithKeystore(t)
	gzond := runGzond(t, "account", "update",
		"--datadir", datadir, "--lightkdf",
		"Q2d9b972ef8219246c73363fd7c048cef81456f9d")
	defer gzond.ExpectExit()
	gzond.Expect(`
Please give a NEW password. Do not forget this password.
!! Unsupported terminal, password will be echoed.
Password: {{.InputLine "foobar"}}
Repeat password: {{.InputLine "foobar"}}
Please provide the OLD password for account Q2d9B972ef8219246C73363fD7c048ceF81456F9d | Attempt 1/3
Password: {{.InputLine "1234567890"}}
`)
}
