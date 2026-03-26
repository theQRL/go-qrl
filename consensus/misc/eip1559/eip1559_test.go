// Copyright 2021 The go-ethereum Authors
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

package eip1559

import (
	"math/big"
	"testing"

	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/core/types"
	"github.com/theQRL/go-qrl/params"
)

// copyConfig does a _shallow_ copy of a given config. Safe to set new values, but
// do not use e.g. SetInt() on the numbers. For testing only
func copyConfig(original *params.ChainConfig) *params.ChainConfig {
	return &params.ChainConfig{
		ChainID: original.ChainID,
	}
}

func config() *params.ChainConfig {
	config := copyConfig(params.TestChainConfig)
	return config
}

// TestBlockGasLimits tests the gasLimit checks for blocks both across
// the EIP-1559 boundary and post-1559 blocks
func TestBlockGasLimits(t *testing.T) {
	initial := new(big.Int).SetUint64(params.InitialBaseFee)

	for i, tc := range []struct {
		pGasLimit uint64
		pNum      int64
		gasLimit  uint64
		ok        bool
	}{
		{20000000, 5, 20000000, true},  // within min and max gas limit
		{20000000, 5, 5000, true},      // within min and max gas limit
		{20000000, 5, 4999, false},     // within min and max gas limit
		{20000000, 5, 20019530, false}, // Upper limit
		{20000000, 5, 20019531, false}, // Beyond max gas limit
		{20000000, 5, 19980470, true},  // within min and max gas limit
		{20000000, 5, 19980469, true},  // within min and max gas limit
		{40000000, 5, 40039061, false}, // Beyond max gas limit
		{40000000, 5, 40039062, false}, // Beyond max gas limit
		{40000000, 5, 39960939, false}, // Beyond max gas limit
		{40000000, 5, 39960938, false}, // Beyond max gas limit
	} {
		parent := &types.Header{
			GasUsed:  tc.pGasLimit / 2,
			GasLimit: tc.pGasLimit,
			BaseFee:  initial,
			Number:   big.NewInt(tc.pNum),
		}
		header := &types.Header{
			GasUsed:  tc.gasLimit / 2,
			GasLimit: tc.gasLimit,
			BaseFee:  initial,
			Number:   big.NewInt(tc.pNum + 1),
		}
		err := VerifyEIP1559Header(config(), parent, header)
		if tc.ok && err != nil {
			t.Errorf("test %d: Expected valid header: %s", i, err)
		}
		if !tc.ok && err == nil {
			t.Errorf("test %d: Expected invalid header", i)
		}
	}
}

// TestGasCostUnderSpam simulates how the base fee (and therefore gas cost)
// increases when an attacker fills every block to the 20M gas limit.
func TestGasCostUnderSpam(t *testing.T) {
	gasLimit := params.MaxGasLimit
	numTxsPerBlock := gasLimit / params.TxGas // 952 txs per block
	gasUsedPerBlock := numTxsPerBlock * params.TxGas

	initialBaseFee := big.NewInt(int64(params.InitialBaseFee))
	numBlocks := 100

	baseFee := new(big.Int).Set(initialBaseFee)

	t.Logf("%-8s %-25s %-25s %-25s", "Block", "BaseFee (planck)", "Tx Cost (planck)", "Block Cost (planck)")
	t.Logf("%-8s %-25s %-25s %-25s", "-----", "----------------", "----------------", "-------------------")

	for i := 0; i < numBlocks; i++ {
		// Cost for one transfer at this base fee
		txCost := new(big.Int).Mul(big.NewInt(int64(params.TxGas)), baseFee)
		// Total cost for the entire block
		blockCost := new(big.Int).Mul(big.NewInt(int64(gasUsedPerBlock)), baseFee)

		// Log every 10 blocks and the first/last
		if i%10 == 0 || i == numBlocks-1 {
			quantaFloat := new(big.Float).Quo(
				new(big.Float).SetInt(blockCost),
				new(big.Float).SetUint64(params.Quanta),
			)
			t.Logf("%-8d %-25s %-25s %-25s (%.4f Quanta)",
				i, baseFee, txCost, blockCost, quantaFloat)
		}

		// Compute next base fee using CalcBaseFee
		parent := &types.Header{
			GasLimit: gasLimit,
			GasUsed:  gasUsedPerBlock,
			BaseFee:  baseFee,
		}
		baseFee = CalcBaseFee(config(), parent)
	}

	// After 100 full blocks the base fee should have increased dramatically
	ratio := new(big.Float).Quo(
		new(big.Float).SetInt(baseFee),
		new(big.Float).SetInt(initialBaseFee),
	)
	ratioF, _ := ratio.Float64()
	t.Logf("\nAfter %d full blocks:", numBlocks)
	t.Logf("  Initial base fee: %s planck", initialBaseFee)
	t.Logf("  Final base fee:   %s planck", baseFee)
	t.Logf("  Increase factor:  %.2fx", ratioF)

	if baseFee.Cmp(initialBaseFee) <= 0 {
		t.Error("base fee did not increase after sustained full blocks")
	}
}

// TestCalcBaseFee assumes all blocks are 1559-blocks
func TestCalcBaseFee(t *testing.T) {
	tests := []struct {
		parentBaseFee   int64
		parentGasLimit  uint64
		parentGasUsed   uint64
		expectedBaseFee int64
	}{
		{params.InitialBaseFee, 20000000, 10000000, params.InitialBaseFee}, // usage == target
		{params.InitialBaseFee, 20000000, 9000000, 98750000000},            // usage below target
		{params.InitialBaseFee, 20000000, 11000000, 101250000000},          // usage above target
	}
	for i, test := range tests {
		parent := &types.Header{
			Number:   common.Big32,
			GasLimit: test.parentGasLimit,
			GasUsed:  test.parentGasUsed,
			BaseFee:  big.NewInt(test.parentBaseFee),
		}
		if have, want := CalcBaseFee(config(), parent), big.NewInt(test.expectedBaseFee); have.Cmp(want) != 0 {
			t.Errorf("test %d: have %d  want %d, ", i, have, want)
		}
	}
}
