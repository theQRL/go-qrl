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

package bind_test

import (
	"context"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/theQRL/go-qrl/accounts/abi/bind"
	"github.com/theQRL/go-qrl/accounts/abi/bind/backends"
	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/core"
	"github.com/theQRL/go-qrl/core/types"
	"github.com/theQRL/go-qrl/crypto"
	"github.com/theQRL/go-qrl/internal/testutil"
)

var testWallet = testutil.MustLoadAccount("dave").MustWallet()

// wantedAddr is CreateAddress(dave, nonce=0) — the address of the first
// contract dave deploys. Recomputed at init time so it stays in sync with
// whatever fixture dave uses.
var wantedAddr = crypto.CreateAddress(testWallet.GetAddress(), 0)

// The fixture bytecode is a minimal constructor that CODECOPYs a single
// STOP (0x00) from the deploy code into memory and RETURNs it as runtime
// code; every opcode used (PUSH1, CODECOPY, RETURN, STOP) is stable across
// the 512-bit VM opcode shift.
var waitDeployedTests = map[string]struct {
	code        string
	gas         uint64
	wantAddress common.Address
	wantErr     error
}{
	"successful deploy": {
		code:        "6001600c60003960016000f300",
		gas:         3000000,
		wantAddress: wantedAddr,
	},
	"empty code": {
		code:        ``,
		gas:         300000,
		wantErr:     bind.ErrNoCodeAfterDeploy,
		wantAddress: wantedAddr,
	},
}

func TestWaitDeployed(t *testing.T) {
	for name, test := range waitDeployedTests {
		backend := backends.NewSimulatedBackend(
			core.GenesisAlloc{
				testWallet.GetAddress(): {Balance: big.NewInt(9000000000000000000)},
			},
			10000000,
		)
		defer backend.Close()

		// Create the transaction
		head, _ := backend.HeaderByNumber(t.Context(), nil) // Should be child's, good enough
		gasFeeCap := new(big.Int).Add(head.BaseFee, big.NewInt(1))

		tx := types.NewTx(&types.DynamicFeeTx{
			Nonce:     0,
			Value:     big.NewInt(0),
			Gas:       test.gas,
			Data:      common.FromHex(test.code),
			GasFeeCap: gasFeeCap,
		})
		tx, _ = types.SignTx(tx, types.ZondSigner{ChainId: big.NewInt(1337)}, testWallet)

		// Wait for it to get mined in the background.
		var (
			err     error
			address common.Address
			mined   = make(chan struct{})
		)
		go func() {
			address, err = bind.WaitDeployed(t.Context(), backend, tx)
			close(mined)
		}()

		// Send and mine the transaction.
		backend.SendTransaction(t.Context(), tx)
		backend.Commit()

		select {
		case <-mined:
			if err != test.wantErr {
				t.Errorf("test %q: error mismatch: want %q, got %q", name, test.wantErr, err)
			}
			if address != test.wantAddress {
				t.Errorf("test %q: unexpected contract address %s %s", name, address.Hex(), test.wantAddress)
			}
		case <-time.After(2 * time.Second):
			t.Errorf("test %q: timeout", name)
		}
	}
}

func TestWaitDeployedCornerCases(t *testing.T) {
	backend := backends.NewSimulatedBackend(
		core.GenesisAlloc{
			testWallet.GetAddress(): {Balance: big.NewInt(9000000000000000000)},
		},
		10000000,
	)
	defer backend.Close()

	head, _ := backend.HeaderByNumber(t.Context(), nil) // Should be child's, good enough
	gasFeeCap := new(big.Int).Add(head.BaseFee, big.NewInt(1))

	// Create a transaction to an account.
	code := "6060604052600a8060106000396000f360606040526008565b00"
	to := common.MustParseAddress("Q00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000001")
	tx := types.NewTx(&types.DynamicFeeTx{
		Nonce:     0,
		To:        &to,
		Value:     big.NewInt(0),
		Gas:       3000000,
		GasFeeCap: gasFeeCap,
		Data:      common.FromHex(code),
	})
	tx, _ = types.SignTx(tx, types.ZondSigner{ChainId: big.NewInt(0)}, testWallet)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	backend.SendTransaction(ctx, tx)
	backend.Commit()
	notContentCreation := errors.New("tx is not contract creation")
	if _, err := bind.WaitDeployed(ctx, backend, tx); err.Error() != notContentCreation.Error() {
		t.Errorf("error missmatch: want %q, got %q, ", notContentCreation, err)
	}

	// Create a transaction that is not mined.
	tx = types.NewTx(&types.DynamicFeeTx{
		Nonce:     1,
		Value:     big.NewInt(0),
		Gas:       3000000,
		GasFeeCap: gasFeeCap,
		Data:      common.FromHex(code),
	})
	tx, _ = types.SignTx(tx, types.ZondSigner{ChainId: big.NewInt(0)}, testWallet)

	go func() {
		contextCanceled := errors.New("context canceled")
		if _, err := bind.WaitDeployed(ctx, backend, tx); err.Error() != contextCanceled.Error() {
			t.Errorf("error missmatch: want %q, got %q, ", contextCanceled, err)
		}
	}()

	backend.SendTransaction(ctx, tx)
	cancel()
}
