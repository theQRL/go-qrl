// Copyright 2026 The go-qrl Authors
// This file is part of the go-qrl library.
//
// The go-qrl library is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInspectPrivateSeedRoundTrip(t *testing.T) {
	t.Parallel()

	const seed = "010000f29f58aff0b00de2844f7e20bd9eeaacc379150043beeb328335817512b29fbb7184da84a092f842b2a06d72a24a5d28"
	tmpdir := t.TempDir()
	seedFile := filepath.Join(tmpdir, "seed.hex")
	passwordFile := filepath.Join(tmpdir, "password.txt")
	keyFile := filepath.Join(tmpdir, "key.json")
	if err := os.WriteFile(seedFile, []byte(seed+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(passwordFile, []byte("test-password\n"), 0600); err != nil {
		t.Fatal(err)
	}

	generate := runQRLkey(t,
		"generate",
		"--lightkdf",
		"--seed", seedFile,
		"--passwordfile", passwordFile,
		keyFile,
	)
	generate.ExpectRegexp(`Address: Q[0-9a-fA-F]{128}\n`)
	generate.ExpectExit()

	inspect := runQRLkey(t,
		"inspect",
		"--private",
		"--json",
		"--passwordfile", passwordFile,
		keyFile,
	)
	inspect.ExpectRegexp(`"Address": "Q[0-9a-fA-F]{128}"`)
	inspect.ExpectRegexp(`"PublicKey": "[0-9a-f]+"`)
	inspect.ExpectRegexp(`"Seed": "` + seed + `"`)
	inspect.ExpectExit()
}
