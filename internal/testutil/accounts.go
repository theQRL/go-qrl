// Copyright 2026 The go-qrl Authors
// This file is part of go-qrl.
//
// go-qrl is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// go-qrl is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.

// Package testutil exposes helpers that load checked-in fixture accounts.
// Tests should use these instead of hard-coding seeds / addresses inline.
package testutil

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/crypto/pqcrypto/wallet"
)

// Account is the decoded form of one entry in testdata/addresses.json. Wallet
// is derived lazily on first access via the Wallet() method so tests that
// only need the address do not pay the ML-DSA-87 restore cost.
type Account struct {
	Label   string `json:"label"`
	Seed    string `json:"seed"`
	Address string `json:"address"`
}

// AddressBytes parses the Q-prefixed hex address into a fixed common.Address.
func (a Account) AddressBytes(t testing.TB) common.Address {
	t.Helper()
	addr, err := common.NewAddressFromString(a.Address)
	if err != nil {
		t.Fatalf("testutil: account %q has invalid address %q: %v", a.Label, a.Address, err)
	}
	return addr
}

// Wallet restores and returns the ML-DSA-87 wallet backing the account.
func (a Account) Wallet(t testing.TB) wallet.Wallet {
	t.Helper()
	w, err := wallet.RestoreFromSeedHex(a.Seed)
	if err != nil {
		t.Fatalf("testutil: account %q: restore wallet from seed: %v", a.Label, err)
	}
	return w
}

// DeterministicWallet returns a fixture wallet that uses deterministic ML-DSA
// signing. Use this only in tests that need stable signed transaction hashes.
func (a Account) DeterministicWallet(t testing.TB) wallet.Wallet {
	t.Helper()
	w, err := NewDeterministicWallet(a.Wallet(t))
	if err != nil {
		t.Fatalf("testutil: account %q: deterministic wallet: %v", a.Label, err)
	}
	return w
}

type deterministicWallet struct {
	*wallet.MLDSA87Wallet
}

// NewDeterministicWallet wraps an ML-DSA-87 wallet for reproducible fixture
// signing. Runtime transaction signing must use the wallet's regular Sign path.
func NewDeterministicWallet(w wallet.Wallet) (wallet.Wallet, error) {
	signer, ok := w.(*wallet.MLDSA87Wallet)
	if !ok {
		return nil, fmt.Errorf("deterministic signing is only supported for ML-DSA-87 wallets, got %T", w)
	}
	return deterministicWallet{MLDSA87Wallet: signer}, nil
}

func (w deterministicWallet) Sign(message []uint8) ([]byte, error) {
	sig, err := w.Wallet.SignDeterministic(message)
	if err != nil {
		return nil, err
	}
	return sig[:], nil
}

var (
	accountsOnce sync.Once
	accountsMap  map[string]Account
	accountsErr  error
)

// loadAccounts reads testdata/addresses.json once and caches the label→Account
// map. Callers get a fresh error if the file is missing or malformed.
func loadAccounts() (map[string]Account, error) {
	accountsOnce.Do(func() {
		path, err := locateFixture("testdata/addresses.json")
		if err != nil {
			accountsErr = err
			return
		}
		data, err := os.ReadFile(path)
		if err != nil {
			accountsErr = fmt.Errorf("testutil: read %s: %w", path, err)
			return
		}
		var list []Account
		if err := json.Unmarshal(data, &list); err != nil {
			accountsErr = fmt.Errorf("testutil: decode %s: %w", path, err)
			return
		}
		m := make(map[string]Account, len(list))
		for _, acc := range list {
			m[acc.Label] = acc
		}
		accountsMap = m
	})
	return accountsMap, accountsErr
}

// LoadAccount returns the fixture account registered under label. The test
// fails if the file is missing or the label is unknown — there is no silent
// fallback to hard-coded data.
func LoadAccount(t testing.TB, label string) Account {
	t.Helper()
	accs, err := loadAccounts()
	if err != nil {
		t.Fatalf("testutil: %v (update testdata/addresses.json)", err)
	}
	acc, ok := accs[label]
	if !ok {
		t.Fatalf("testutil: unknown account label %q (add it to testdata/addresses.json)", label)
	}
	return acc
}

// MustLoadAccount is the package-init / example variant of LoadAccount. It
// panics on missing fixtures or unknown labels, which is the only way to
// surface failure outside a *testing.T context. Prefer LoadAccount inside
// test bodies.
func MustLoadAccount(label string) Account {
	accs, err := loadAccounts()
	if err != nil {
		panic(fmt.Sprintf("testutil: %v (update testdata/addresses.json)", err))
	}
	acc, ok := accs[label]
	if !ok {
		panic(fmt.Sprintf("testutil: unknown account label %q (add it to testdata/addresses.json)", label))
	}
	return acc
}

// MustWallet is the counterpart of Account.Wallet for callers without a
// testing handle. Panics if the seed cannot be restored.
func (a Account) MustWallet() wallet.Wallet {
	w, err := wallet.RestoreFromSeedHex(a.Seed)
	if err != nil {
		panic(fmt.Sprintf("testutil: account %q: restore wallet from seed: %v", a.Label, err))
	}
	return w
}

// MustAddressBytes is the counterpart of Account.AddressBytes for callers
// without a testing handle. Panics if the stored address string is invalid.
func (a Account) MustAddressBytes() common.Address {
	addr, err := common.NewAddressFromString(a.Address)
	if err != nil {
		panic(fmt.Sprintf("testutil: account %q has invalid address %q: %v", a.Label, a.Address, err))
	}
	return addr
}

// locateFixture walks up from the working directory until it finds a go.mod
// and returns the absolute path to the requested fixture file. Works from any
// test package in the module without hard-coding the repo layout.
func locateFixture(rel string) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := cwd
	for {
		candidate := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(candidate); err == nil {
			return filepath.Join(dir, rel), nil
		} else if !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("testutil: module root not found walking up from %s", cwd)
		}
		dir = parent
	}
}
