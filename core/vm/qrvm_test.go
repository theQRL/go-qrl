// Copyright 2026 The go-qrl Authors
// This file is part of the go-qrl library.
//
// The go-qrl library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-qrl library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-qrl library. If not, see <http://www.gnu.org/licenses/>.

package vm

import (
	"math/big"
	"testing"

	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/core/rawdb"
	"github.com/theQRL/go-qrl/core/state"
	"github.com/theQRL/go-qrl/core/types"
	"github.com/theQRL/go-qrl/params"
)

var codeStoreOutOfGasInitCode = []byte{
	byte(PUSH1), 1,
	byte(PUSH1), 0,
	byte(RETURN),
}

func newCodeStoreOutOfGasTestVM(t *testing.T, creator common.Address) (*QRVM, *state.StateDB) {
	t.Helper()

	statedb, err := state.New(types.EmptyRootHash, state.NewDatabase(rawdb.NewMemoryDatabase()), nil)
	if err != nil {
		t.Fatal(err)
	}
	statedb.CreateAccount(creator)
	statedb.AddBalance(creator, big.NewInt(10))

	blockContext := BlockContext{
		CanTransfer: func(db StateDB, address common.Address, amount *big.Int) bool {
			return db.GetBalance(address).Cmp(amount) >= 0
		},
		Transfer: func(db StateDB, sender, recipient common.Address, amount *big.Int) {
			db.SubBalance(sender, amount)
			db.AddBalance(recipient, amount)
		},
		BlockNumber: big.NewInt(0),
	}
	return NewQRVM(blockContext, TxContext{}, statedb, params.AllBeaconProtocolChanges, Config{}), statedb
}

func assertFailedCreationState(t *testing.T, statedb *state.StateDB, creator, contract common.Address) {
	t.Helper()

	if nonce := statedb.GetNonce(creator); nonce != 1 {
		t.Fatalf("creator nonce mismatch: have %d, want 1", nonce)
	}
	if balance := statedb.GetBalance(creator); balance.Cmp(big.NewInt(10)) != 0 {
		t.Fatalf("creator balance mismatch: have %v, want 10", balance)
	}
	if statedb.Exist(contract) {
		t.Fatal("failed creation left the contract account in state")
	}
}

func TestCreateCodeStoreOutOfGas(t *testing.T) {
	creator := common.BytesToAddress([]byte("creator"))
	qrvm, statedb := newCodeStoreOutOfGasTestVM(t, creator)

	_, address, leftOverGas, err := qrvm.Create(AccountRef(creator), codeStoreOutOfGasInitCode, 199, big.NewInt(7))
	if err != ErrCodeStoreOutOfGas {
		t.Fatalf("error mismatch: have %v, want %v", err, ErrCodeStoreOutOfGas)
	}
	if leftOverGas != 0 {
		t.Fatalf("leftover gas mismatch: have %d, want 0", leftOverGas)
	}
	assertFailedCreationState(t, statedb, creator, address)
}
