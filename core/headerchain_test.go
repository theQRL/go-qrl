// Copyright 2020 The go-ethereum Authors
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

package core

import (
	"errors"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/theQRL/go-qrl/consensus/beacon"
	"github.com/theQRL/go-qrl/core/rawdb"
	"github.com/theQRL/go-qrl/core/types"
	"github.com/theQRL/go-qrl/params"
	"github.com/theQRL/go-qrl/trie"
)

func verifyUnbrokenCanonchain(hc *HeaderChain) error {
	h := hc.CurrentHeader()
	for {
		canonHash := rawdb.ReadCanonicalHash(hc.chainDb, h.Number.Uint64())
		if exp := h.Hash(); canonHash != exp {
			return fmt.Errorf("Canon hash chain broken, block %d got %x, expected %x",
				h.Number, canonHash[:8], exp[:8])
		}
		if h.Number.Uint64() == 0 {
			break
		}
		h = hc.GetHeader(h.ParentHash, h.Number.Uint64()-1)
	}
	return nil
}

func testInsert(t *testing.T, hc *HeaderChain, chain []*types.Header, wantStatus WriteStatus, wantErr error) {
	t.Helper()

	status, err := hc.InsertHeaderChain(chain, time.Now())
	if status != wantStatus {
		t.Errorf("wrong write status from InsertHeaderChain: got %v, want %v", status, wantStatus)
	}
	// Always verify that the header chain is unbroken
	if err := verifyUnbrokenCanonchain(hc); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("unexpected error from InsertHeaderChain: %v", err)
	}
}

// This test checks status reporting of InsertHeaderChain.
func TestHeaderInsertion(t *testing.T) {
	var (
		db    = rawdb.NewMemoryDatabase()
		gspec = &Genesis{BaseFee: big.NewInt(params.InitialBaseFee), Config: params.AllBeaconProtocolChanges}
	)
	gspec.Commit(db, trie.NewDatabase(db, nil))
	hc, err := NewHeaderChain(db, gspec.Config, beacon.NewFaker(), func() bool { return false })
	if err != nil {
		t.Fatal(err)
	}
	// chain A: G->A1->A2...A128
	_, chainA := makeHeaderChainWithGenesis(gspec, 128, beacon.NewFaker(), 10)

	// Inserting 64 headers on an empty chain, expecting
	// 1 callbacks, 1 canon-status, 0 sidestatus,
	testInsert(t, hc, chainA[:64], CanonStatTy, nil)

	// Inserting 64 identical headers, expecting
	// 0 callbacks, 0 canon-status, 0 sidestatus,
	testInsert(t, hc, chainA[:64], NonStatTy, nil)

	// Inserting the same some old, some new headers
	// 1 callbacks, 1 canon, 0 side
	testInsert(t, hc, chainA[32:96], CanonStatTy, nil)

	// Inserting more A-headers, taking back the canonicality
	testInsert(t, hc, chainA[90:100], CanonStatTy, nil)
}
