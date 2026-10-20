// Copyright 2017 The go-ethereum Authors
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

package vm

import (
	"bytes"
	"math/big"
	"sort"
	"testing"

	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/common/hexutil"
	"github.com/theQRL/go-qrl/core/rawdb"
	"github.com/theQRL/go-qrl/core/state"
	"github.com/theQRL/go-qrl/core/types"
	"github.com/theQRL/go-qrl/params"
)

func TestMemoryGasCost(t *testing.T) {
	tests := []struct {
		size     uint64
		cost     uint64
		overflow bool
	}{
		// One more byte would round up to a word count whose square overflows uint64.
		{maxMemoryExpansionBytes, 36028809887088637, false},
		{maxMemoryExpansionBytes + 1, 0, true},
	}
	for i, tt := range tests {
		v, err := memoryGasCost(&Memory{}, tt.size)
		if (err == ErrGasUintOverflow) != tt.overflow {
			t.Errorf("test %d: overflow mismatch: have %v, want %v", i, err == ErrGasUintOverflow, tt.overflow)
		}
		if v != tt.cost {
			t.Errorf("test %d: gas cost mismatch: have %v, want %v", i, v, tt.cost)
		}
	}
}

// Gas expectations reflect the 64-byte VM word: CREATE / CREATE2 initcode
// charges use half as many words as the original 32-byte-word values.
var createGasTests = []struct {
	code       string
	gasUsed    uint64
	minimumGas uint64
}{
	// legacy create(0, 0, 0xc000) _with_ 3860
	{"0x61C00060006000f0" + "600052" + "60406000F3", 37013, 37013},
	// create2(0, 0, 0xc001, 0) (too large), with 3860
	{"0x600061C00160006000f5" + "600052" + "60406000F3", 32012, 100_000},
	// create2(0, 0, 0xc000, 0)
	// This case is trying to deploy code at (within) the limit
	{"0x600061C00060006000f5" + "600052" + "60406000F3", 41624, 41624},
	// create2(0, 0, 0xc001, 0)
	// This case is trying to deploy code exceeding the limit
	{"0x600061C00160006000f5" + "600052" + "60406000F3", 32024, 100000},
}

func TestCreateGas(t *testing.T) {
	for i, tt := range createGasTests {
		var gasUsed = uint64(0)
		doCheck := func(testGas int) bool {
			address := common.BytesToAddress([]byte("contract"))
			statedb, _ := state.New(types.EmptyRootHash, state.NewDatabase(rawdb.NewMemoryDatabase()), nil)
			statedb.CreateAccount(address)
			statedb.SetCode(address, hexutil.MustDecode(tt.code))
			statedb.Finalise(true)
			vmctx := BlockContext{
				CanTransfer: func(StateDB, common.Address, *big.Int) bool { return true },
				Transfer:    func(StateDB, common.Address, common.Address, *big.Int) {},
				BlockNumber: big.NewInt(0),
			}
			config := Config{}

			vmenv := NewQRVM(vmctx, TxContext{}, statedb, params.AllBeaconProtocolChanges, config)
			var startGas = uint64(testGas)
			ret, gas, err := vmenv.Call(AccountRef(common.Address{}), address, nil, startGas, new(big.Int))
			if err != nil {
				return false
			}
			gasUsed = startGas - gas
			if len(ret) != WordBytes {
				t.Fatalf("test %d: expected %d bytes returned, have %d", i, WordBytes, len(ret))
			}
			if bytes.Equal(ret, make([]byte, WordBytes)) {
				// Failure
				return false
			}
			return true
		}
		minGas := sort.Search(100_000, doCheck)
		if uint64(minGas) != tt.minimumGas {
			t.Fatalf("test %d: min gas error, want %d, have %d", i, tt.minimumGas, minGas)
		}
		// If the deployment succeeded, we also check the gas used
		if minGas < 100_000 {
			if gasUsed != tt.gasUsed {
				t.Errorf("test %d: gas used mismatch: have %v, want %v", i, gasUsed, tt.gasUsed)
			}
		}
	}
}
