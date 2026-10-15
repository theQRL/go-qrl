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

package tracers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/common/hexutil"
	"github.com/theQRL/go-qrl/consensus"
	"github.com/theQRL/go-qrl/consensus/beacon"
	"github.com/theQRL/go-qrl/core"
	"github.com/theQRL/go-qrl/core/rawdb"
	"github.com/theQRL/go-qrl/core/state"
	"github.com/theQRL/go-qrl/core/types"
	"github.com/theQRL/go-qrl/core/vm"
	"github.com/theQRL/go-qrl/crypto/pqcrypto/wallet"
	"github.com/theQRL/go-qrl/internal/qrlapi"
	"github.com/theQRL/go-qrl/params"
	"github.com/theQRL/go-qrl/qrl/tracers/logger"
	"github.com/theQRL/go-qrl/qrldb"
	"github.com/theQRL/go-qrl/rpc"
)

var (
	errStateNotFound = errors.New("state not found")
	errBlockNotFound = errors.New("block not found")
)

type testBackend struct {
	chainConfig *params.ChainConfig
	engine      consensus.Engine
	chaindb     qrldb.Database
	chain       *core.BlockChain

	refHook func() // Hook is invoked when the requested state is referenced
	relHook func() // Hook is invoked when the requested state is released
}

// testBackend creates a new test backend. OBS: After test is done, teardown must be
// invoked in order to release associated resources.
func newTestBackend(t *testing.T, n int, gspec *core.Genesis, generator func(i int, b *core.BlockGen)) *testBackend {
	backend := &testBackend{
		chainConfig: gspec.Config,
		engine:      beacon.NewFaker(),
		chaindb:     rawdb.NewMemoryDatabase(),
	}
	// Generate blocks for testing
	_, blocks, _ := core.GenerateChainWithGenesis(gspec, backend.engine, n, generator)

	// Import the canonical chain
	cacheConfig := &core.CacheConfig{
		TrieCleanLimit:    256,
		TrieDirtyLimit:    256,
		TrieTimeLimit:     5 * time.Minute,
		SnapshotLimit:     0,
		TrieDirtyDisabled: true, // Archive mode
	}
	chain, err := core.NewBlockChain(backend.chaindb, cacheConfig, gspec, backend.engine, vm.Config{}, nil)
	if err != nil {
		t.Fatalf("failed to create tester chain: %v", err)
	}
	if n, err := chain.InsertChain(blocks); err != nil {
		t.Fatalf("block %d: failed to insert into chain: %v", n, err)
	}
	backend.chain = chain
	return backend
}

func (b *testBackend) HeaderByHash(ctx context.Context, hash common.Hash) (*types.Header, error) {
	return b.chain.GetHeaderByHash(hash), nil
}

func (b *testBackend) HeaderByNumber(ctx context.Context, number rpc.BlockNumber) (*types.Header, error) {
	if number == rpc.PendingBlockNumber || number == rpc.LatestBlockNumber {
		return b.chain.CurrentHeader(), nil
	}
	return b.chain.GetHeaderByNumber(uint64(number)), nil
}

func (b *testBackend) BlockByHash(ctx context.Context, hash common.Hash) (*types.Block, error) {
	return b.chain.GetBlockByHash(hash), nil
}

func (b *testBackend) BlockByNumber(ctx context.Context, number rpc.BlockNumber) (*types.Block, error) {
	if number == rpc.PendingBlockNumber || number == rpc.LatestBlockNumber {
		return b.chain.GetBlockByNumber(b.chain.CurrentBlock().Number.Uint64()), nil
	}
	return b.chain.GetBlockByNumber(uint64(number)), nil
}

func (b *testBackend) GetTransaction(ctx context.Context, txHash common.Hash) (*types.Transaction, common.Hash, uint64, uint64, error) {
	tx, hash, blockNumber, index := rawdb.ReadTransaction(b.chaindb, txHash)
	return tx, hash, blockNumber, index, nil
}

func (b *testBackend) RPCGasCap() uint64 {
	return 25000000
}

func (b *testBackend) ChainConfig() *params.ChainConfig {
	return b.chainConfig
}

func (b *testBackend) Engine() consensus.Engine {
	return b.engine
}

func (b *testBackend) ChainDb() qrldb.Database {
	return b.chaindb
}

// teardown releases the associated resources.
func (b *testBackend) teardown() {
	b.chain.Stop()
}

func (b *testBackend) StateAtBlock(ctx context.Context, block *types.Block, reexec uint64, base *state.StateDB, readOnly bool, preferDisk bool) (*state.StateDB, StateReleaseFunc, error) {
	statedb, err := b.chain.StateAt(block.Root())
	if err != nil {
		return nil, nil, errStateNotFound
	}
	if b.refHook != nil {
		b.refHook()
	}
	release := func() {
		if b.relHook != nil {
			b.relHook()
		}
	}
	return statedb, release, nil
}

func (b *testBackend) StateAtTransaction(ctx context.Context, block *types.Block, txIndex int, reexec uint64) (*core.Message, vm.BlockContext, *state.StateDB, StateReleaseFunc, error) {
	parent := b.chain.GetBlock(block.ParentHash(), block.NumberU64()-1)
	if parent == nil {
		return nil, vm.BlockContext{}, nil, nil, errBlockNotFound
	}
	statedb, release, err := b.StateAtBlock(ctx, parent, reexec, nil, true, false)
	if err != nil {
		return nil, vm.BlockContext{}, nil, nil, errStateNotFound
	}
	if txIndex == 0 && len(block.Transactions()) == 0 {
		return nil, vm.BlockContext{}, statedb, release, nil
	}
	// Recompute transactions up to the target index.
	signer := types.NewZondSigner(b.chainConfig.ChainID)
	for idx, tx := range block.Transactions() {
		msg, _ := core.TransactionToMessage(tx, signer, block.BaseFee())
		txContext := core.NewQRVMTxContext(msg)
		context := core.NewQRVMBlockContext(block.Header(), b.chain, nil)
		if idx == txIndex {
			return msg, context, statedb, release, nil
		}
		vmenv := vm.NewQRVM(context, txContext, statedb, b.chainConfig, vm.Config{})
		if _, err := core.ApplyMessage(vmenv, msg, new(core.GasPool).AddGas(tx.Gas())); err != nil {
			return nil, vm.BlockContext{}, nil, nil, fmt.Errorf("transaction %#x failed: %v", tx.Hash(), err)
		}
		statedb.Finalise(true)
	}
	return nil, vm.BlockContext{}, nil, nil, fmt.Errorf("transaction index %d out of range for block %#x", txIndex, block.Hash())
}

func TestTraceCall(t *testing.T) {
	t.Parallel()

	// Initialize test accounts
	accounts := newAccounts(3)
	genesis := &core.Genesis{
		Config: params.TestChainConfig,
		Alloc: core.GenesisAlloc{
			accounts[0].addr: {Balance: big.NewInt(params.Quanta)},
			accounts[1].addr: {Balance: big.NewInt(params.Quanta)},
			accounts[2].addr: {Balance: big.NewInt(params.Quanta)},
		},
	}
	genBlocks := 10
	signer := types.ZondSigner{ChainId: big.NewInt(1)}
	backend := newTestBackend(t, genBlocks, genesis, func(i int, b *core.BlockGen) {
		// Transfer from account[0] to account[1]
		//    value: 1000 planck
		//    fee:   0 planck
		tx := types.NewTx(&types.DynamicFeeTx{
			Nonce:     uint64(i),
			To:        &accounts[1].addr,
			Value:     big.NewInt(1000),
			Gas:       params.TxGas,
			GasFeeCap: b.BaseFee(),
			Data:      nil,
		})
		signedTx, _ := types.SignTx(tx, signer, accounts[0].wallet)
		b.AddTx(signedTx)
	})
	defer backend.teardown()
	api := NewAPI(backend)
	var testSuite = []struct {
		blockNumber rpc.BlockNumber
		call        qrlapi.TransactionArgs
		config      *TraceCallConfig
		expectErr   error
		expect      string
	}{
		// Standard JSON trace upon the genesis, plain transfer.
		{
			blockNumber: rpc.BlockNumber(0),
			call: qrlapi.TransactionArgs{
				From:  &accounts[0].addr,
				To:    &accounts[1].addr,
				Value: (*hexutil.Big)(big.NewInt(1000)),
			},
			config:    nil,
			expectErr: nil,
			expect:    `{"gas":21000,"failed":false,"returnValue":"0x","structLogs":[]}`,
		},
		// Standard JSON trace upon the head, plain transfer.
		{
			blockNumber: rpc.BlockNumber(genBlocks),
			call: qrlapi.TransactionArgs{
				From:  &accounts[0].addr,
				To:    &accounts[1].addr,
				Value: (*hexutil.Big)(big.NewInt(1000)),
			},
			config:    nil,
			expectErr: nil,
			expect:    `{"gas":21000,"failed":false,"returnValue":"0x","structLogs":[]}`,
		},
		// Standard JSON trace upon the non-existent block, error expects
		{
			blockNumber: rpc.BlockNumber(genBlocks + 1),
			call: qrlapi.TransactionArgs{
				From:  &accounts[0].addr,
				To:    &accounts[1].addr,
				Value: (*hexutil.Big)(big.NewInt(1000)),
			},
			config:    nil,
			expectErr: fmt.Errorf("block #%d not found", genBlocks+1),
			//expect:    nil,
		},
		// Standard JSON trace upon the latest block
		{
			blockNumber: rpc.LatestBlockNumber,
			call: qrlapi.TransactionArgs{
				From:  &accounts[0].addr,
				To:    &accounts[1].addr,
				Value: (*hexutil.Big)(big.NewInt(1000)),
			},
			config:    nil,
			expectErr: nil,
			expect:    `{"gas":21000,"failed":false,"returnValue":"0x","structLogs":[]}`,
		},
		// Tracing on 'pending' should fail:
		{
			blockNumber: rpc.PendingBlockNumber,
			call: qrlapi.TransactionArgs{
				From:  &accounts[0].addr,
				To:    &accounts[1].addr,
				Value: (*hexutil.Big)(big.NewInt(1000)),
			},
			config:    nil,
			expectErr: errors.New("tracing on top of pending is not supported"),
		},
		{
			blockNumber: rpc.LatestBlockNumber,
			call: qrlapi.TransactionArgs{
				From:  &accounts[0].addr,
				Input: &hexutil.Bytes{0x43}, // blocknumber
			},
			config: &TraceCallConfig{
				BlockOverrides: &qrlapi.BlockOverrides{Number: (*hexutil.Big)(big.NewInt(0x1337))},
			},
			expectErr: nil,
			expect: ` {"gas":53020,"failed":false,"returnValue":"0x","structLogs":[
				{"pc":0,"op":"NUMBER","gas":24946982,"gasCost":2,"depth":1,"stack":[]},
				{"pc":1,"op":"STOP","gas":24946980,"gasCost":0,"depth":1,"stack":["0x1337"]}]}`,
		},
	}
	for i, testspec := range testSuite {
		result, err := api.TraceCall(t.Context(), testspec.call, rpc.BlockNumberOrHash{BlockNumber: &testspec.blockNumber}, testspec.config)
		if testspec.expectErr != nil {
			if err == nil {
				t.Errorf("test %d: expect error %v, got nothing", i, testspec.expectErr)
				continue
			}
			if !reflect.DeepEqual(err, testspec.expectErr) {
				t.Errorf("test %d: error mismatch, want %v, git %v", i, testspec.expectErr, err)
			}
		} else {
			if err != nil {
				t.Errorf("test %d: expect no error, got %v", i, err)
				continue
			}
			var have *logger.ExecutionResult
			if err := json.Unmarshal(result.(json.RawMessage), &have); err != nil {
				t.Errorf("test %d: failed to unmarshal result %v", i, err)
			}
			var want *logger.ExecutionResult
			if err := json.Unmarshal([]byte(testspec.expect), &want); err != nil {
				t.Errorf("test %d: failed to unmarshal result %v", i, err)
			}
			if !reflect.DeepEqual(have, want) {
				t.Errorf("test %d: result mismatch, want %v, got %v", i, testspec.expect, string(result.(json.RawMessage)))
			}
		}
	}
}

func TestTraceTransaction(t *testing.T) {
	t.Parallel()

	// Initialize test accounts
	accounts := newAccounts(2)
	genesis := &core.Genesis{
		Config: params.TestChainConfig,
		Alloc: core.GenesisAlloc{
			accounts[0].addr: {Balance: big.NewInt(params.Quanta)},
			accounts[1].addr: {Balance: big.NewInt(params.Quanta)},
		},
	}
	target := common.Hash{}
	signer := types.ZondSigner{ChainId: big.NewInt(1)}
	backend := newTestBackend(t, 1, genesis, func(i int, b *core.BlockGen) {
		// Transfer from account[0] to account[1]
		//    value: 1000 planck
		//    fee:   0 planck
		tx := types.NewTx(&types.DynamicFeeTx{
			Nonce:     uint64(i),
			To:        &accounts[1].addr,
			Value:     big.NewInt(1000),
			Gas:       params.TxGas,
			GasFeeCap: b.BaseFee(),
			Data:      nil,
		})
		signedTx, _ := types.SignTx(tx, signer, accounts[0].wallet)
		b.AddTx(signedTx)
		target = signedTx.Hash()
	})
	defer backend.chain.Stop()
	api := NewAPI(backend)
	result, err := api.TraceTransaction(t.Context(), target, nil)
	if err != nil {
		t.Errorf("Failed to trace transaction %v", err)
	}
	var have *logger.ExecutionResult
	if err := json.Unmarshal(result.(json.RawMessage), &have); err != nil {
		t.Errorf("failed to unmarshal result %v", err)
	}
	if !reflect.DeepEqual(have, &logger.ExecutionResult{
		Gas:         params.TxGas,
		Failed:      false,
		ReturnValue: []byte{},
		StructLogs:  []logger.StructLogRes{},
	}) {
		t.Error("Transaction tracing result is different")
	}

	// Test non-existent transaction
	_, err = api.TraceTransaction(t.Context(), common.Hash{42}, nil)
	if !errors.Is(err, errTxNotFound) {
		t.Fatalf("want %v, have %v", errTxNotFound, err)
	}
}

func TestTraceBlock(t *testing.T) {
	t.Parallel()

	// Initialize test accounts
	accounts := newAccounts(3)
	genesis := &core.Genesis{
		Config: params.TestChainConfig,
		Alloc: core.GenesisAlloc{
			accounts[0].addr: {Balance: big.NewInt(params.Quanta)},
			accounts[1].addr: {Balance: big.NewInt(params.Quanta)},
			accounts[2].addr: {Balance: big.NewInt(params.Quanta)},
		},
	}
	genBlocks := 10
	signer := types.ZondSigner{ChainId: big.NewInt(1)}
	var txHash common.Hash
	backend := newTestBackend(t, genBlocks, genesis, func(i int, b *core.BlockGen) {
		// Transfer from account[0] to account[1]
		//    value: 1000 planck
		//    fee:   0 planck
		tx := types.NewTx(&types.DynamicFeeTx{
			Nonce:     uint64(i),
			To:        &accounts[1].addr,
			Value:     big.NewInt(1000),
			Gas:       params.TxGas,
			GasFeeCap: b.BaseFee(),
			Data:      nil,
		})
		signedTx, _ := types.SignTx(tx, signer, accounts[0].wallet)
		b.AddTx(signedTx)
		txHash = signedTx.Hash()
	})
	defer backend.chain.Stop()
	api := NewAPI(backend)

	var testSuite = []struct {
		blockNumber rpc.BlockNumber
		config      *TraceConfig
		want        string
		expectErr   error
	}{
		// Trace genesis block, expect error
		{
			blockNumber: rpc.BlockNumber(0),
			expectErr:   errors.New("genesis is not traceable"),
		},
		// Trace head block
		{
			blockNumber: rpc.BlockNumber(genBlocks),
			want:        fmt.Sprintf(`[{"txHash":"%v","result":{"gas":21000,"failed":false,"returnValue":"0x","structLogs":[]}}]`, txHash),
		},
		// Trace non-existent block
		{
			blockNumber: rpc.BlockNumber(genBlocks + 1),
			expectErr:   fmt.Errorf("block #%d not found", genBlocks+1),
		},
		// Trace latest block
		{
			blockNumber: rpc.LatestBlockNumber,
			want:        fmt.Sprintf(`[{"txHash":"%v","result":{"gas":21000,"failed":false,"returnValue":"0x","structLogs":[]}}]`, txHash),
		},
		// Trace pending block
		{
			blockNumber: rpc.PendingBlockNumber,
			want:        fmt.Sprintf(`[{"txHash":"%v","result":{"gas":21000,"failed":false,"returnValue":"0x","structLogs":[]}}]`, txHash),
		},
	}
	for i, tc := range testSuite {
		result, err := api.TraceBlockByNumber(t.Context(), tc.blockNumber, tc.config)
		if tc.expectErr != nil {
			if err == nil {
				t.Errorf("test %d, want error %v", i, tc.expectErr)
				continue
			}
			if !reflect.DeepEqual(err, tc.expectErr) {
				t.Errorf("test %d: error mismatch, want %v, get %v", i, tc.expectErr, err)
			}
			continue
		}
		if err != nil {
			t.Errorf("test %d, want no error, have %v", i, err)
			continue
		}
		have, _ := json.Marshal(result)
		want := tc.want
		if string(have) != want {
			t.Errorf("test %d, result mismatch, have\n%v\n, want\n%v\n", i, string(have), want)
		}
	}
}

func TestTracingWithOverrides(t *testing.T) {
	t.Parallel()
	// Initialize test accounts
	accounts := newAccounts(3)
	storageAccount := common.Address{0x13, 37}
	genesis := &core.Genesis{
		Config: params.TestChainConfig,
		Alloc: core.GenesisAlloc{
			accounts[0].addr: {Balance: big.NewInt(params.Quanta)},
			accounts[1].addr: {Balance: big.NewInt(params.Quanta)},
			accounts[2].addr: {Balance: big.NewInt(params.Quanta)},
			// An account with existing storage
			storageAccount: {
				Balance: new(big.Int),
				Storage: map[common.Hash]common.StorageValue64{
					common.HexToHash("0x03"): common.HexToStorageValue64("0x33"),
					common.HexToHash("0x04"): common.HexToStorageValue64("0x44"),
				},
			},
		},
	}
	genBlocks := 10
	signer := types.ZondSigner{ChainId: big.NewInt(1)}
	backend := newTestBackend(t, genBlocks, genesis, func(i int, b *core.BlockGen) {
		// Transfer from account[0] to account[1]
		//    value: 1000 planck
		//    fee:   0 planck
		tx := types.NewTx(&types.DynamicFeeTx{
			Nonce:     uint64(i),
			To:        &accounts[1].addr,
			Value:     big.NewInt(1000),
			Gas:       params.TxGas,
			GasFeeCap: b.BaseFee(),
			Data:      nil,
		})
		signedTx, _ := types.SignTx(tx, signer, accounts[0].wallet)
		b.AddTx(signedTx)
	})
	defer backend.chain.Stop()
	api := NewAPI(backend)
	randomAccounts := newAccounts(3)
	type res struct {
		Gas         int
		Failed      bool
		ReturnValue string
	}
	var testSuite = []struct {
		blockNumber rpc.BlockNumber
		call        qrlapi.TransactionArgs
		config      *TraceCallConfig
		expectErr   error
		want        string
	}{
		// Call which can only succeed if state is state overridden
		{
			blockNumber: rpc.LatestBlockNumber,
			call: qrlapi.TransactionArgs{
				From:  &randomAccounts[0].addr,
				To:    &randomAccounts[1].addr,
				Value: (*hexutil.Big)(big.NewInt(1000)),
			},
			config: &TraceCallConfig{
				StateOverrides: &qrlapi.StateOverride{
					randomAccounts[0].addr: qrlapi.OverrideAccount{Balance: newRPCBalance(new(big.Int).Mul(big.NewInt(1), big.NewInt(params.Quanta)))},
				},
			},
			want: `{"gas":21000,"failed":false,"returnValue":"0x"}`,
		},
		// Invalid call without state overriding
		{
			blockNumber: rpc.LatestBlockNumber,
			call: qrlapi.TransactionArgs{
				From:  &randomAccounts[0].addr,
				To:    &randomAccounts[1].addr,
				Value: (*hexutil.Big)(big.NewInt(1000)),
			},
			config:    &TraceCallConfig{},
			expectErr: core.ErrInsufficientFunds,
		},
		// Successful simple contract call. Originally a Solidity Storage
		// contract; replaced with minimal SLOAD(0) + MSTORE(0) +
		// RETURN(32, 32) bytecode. MSTORE writes a 64-byte slot; the
		// numeric value lands in the low 32 bytes at memory[32:64], so
		// RETURN offsets 32 to surface it in the returnValue.
		{
			blockNumber: rpc.LatestBlockNumber,
			call: qrlapi.TransactionArgs{
				From: &randomAccounts[0].addr,
				To:   &randomAccounts[2].addr,
			},
			config: &TraceCallConfig{
				StateOverrides: &qrlapi.StateOverride{
					randomAccounts[2].addr: qrlapi.OverrideAccount{
						Code:      newRPCBytes(common.Hex2Bytes("60005460005260206020f3")),
						StateDiff: newStates([]common.Hash{{}}, []common.Hash{common.BigToHash(big.NewInt(123))}),
					},
				},
			},
			want: `{"gas":23118,"failed":false,"returnValue":"0x000000000000000000000000000000000000000000000000000000000000007b"}`,
		},
		{ // Override blocknumber. BLOCKNUMBER PUSH1 MSTORE; writing a full
			// 64-byte word places the number in the low 32 bytes, so
			// RETURN offsets 32 to pick it up.
			blockNumber: rpc.LatestBlockNumber,
			call: qrlapi.TransactionArgs{
				From:  &accounts[0].addr,
				Input: newRPCBytes(common.Hex2Bytes("4360005260206020f3")),
			},
			config: &TraceCallConfig{
				BlockOverrides: &qrlapi.BlockOverrides{Number: (*hexutil.Big)(big.NewInt(0x1337))},
			},
			want: `{"gas":59551,"failed":false,"returnValue":"0x0000000000000000000000000000000000000000000000000000000000001337"}`,
		},
		{ // Override blocknumber, and query a blockhash
			blockNumber: rpc.LatestBlockNumber,
			call: qrlapi.TransactionArgs{
				From: &accounts[0].addr,
				Input: &hexutil.Bytes{
					0x60, 0x00, 0x40, // BLOCKHASH(0)
					0x60, 0x00, 0x52, // STORE memory offset 0
					0x61, 0x13, 0x36, 0x40, // BLOCKHASH(0x1336)
					0x60, 0x40, 0x52, // STORE memory offset 64
					0x61, 0x13, 0x37, 0x40, // BLOCKHASH(0x1337)
					0x60, 0x80, 0x52, // STORE memory offset 128
					0x60, 0xc0, 0x60, 0x00, 0xf3, // RETURN (0-192)

				}, // blocknumber
			},
			config: &TraceCallConfig{
				BlockOverrides: &qrlapi.BlockOverrides{Number: (*hexutil.Big)(big.NewInt(0x1337))},
			},
			want: `{"gas":91868,"failed":false,"returnValue":"0x000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"}`,
		},
		// Originally a Solidity try/catch contract that returned the
		// post-catch value of storage slot 0 (= 1). Replaced with
		// minimal bytecode that writes 1 into memory and returns it:
		// PUSH1 1 ; PUSH1 0 ; MSTORE ; PUSH1 32 ; PUSH1 32 ; RETURN.
		{ // First with only code override, not storage override
			blockNumber: rpc.LatestBlockNumber,
			call: qrlapi.TransactionArgs{
				From: &randomAccounts[0].addr,
				To:   &randomAccounts[2].addr,
			},
			config: &TraceCallConfig{
				StateOverrides: &qrlapi.StateOverride{
					randomAccounts[2].addr: qrlapi.OverrideAccount{
						Code: newRPCBytes(common.Hex2Bytes("600160005260206020f3")),
					},
				},
			},
			want: `{"gas":21018,"failed":false,"returnValue":"0x0000000000000000000000000000000000000000000000000000000000000001"}`,
		},
		{ // Same again, this time with storage override (slot 0 = 0)
			blockNumber: rpc.LatestBlockNumber,
			call: qrlapi.TransactionArgs{
				From: &randomAccounts[0].addr,
				To:   &randomAccounts[2].addr,
			},
			config: &TraceCallConfig{
				StateOverrides: &qrlapi.StateOverride{
					randomAccounts[2].addr: qrlapi.OverrideAccount{
						Code:  newRPCBytes(common.Hex2Bytes("600160005260206020f3")),
						State: newStates([]common.Hash{{}}, []common.Hash{{}}),
					},
				},
			},
			want: `{"gas":21018,"failed":false,"returnValue":"0x0000000000000000000000000000000000000000000000000000000000000001"}`,
		},
		// For the storage-load test cases below, the contract does
		// SLOAD(4) + SLOAD(3), MSTOREs the sum at memory offset 0, and
		// RETURNs 32 bytes. Because MSTORE writes a full 64-byte slot,
		// the numeric value lands in memory[32:64]; RETURN offsets 32
		// to surface it as the returnValue's big-endian low 32 bytes.
		{ // No state override
			blockNumber: rpc.LatestBlockNumber,
			call: qrlapi.TransactionArgs{
				From: &randomAccounts[0].addr,
				To:   &storageAccount,
			},
			config: &TraceCallConfig{
				StateOverrides: &qrlapi.StateOverride{
					storageAccount: qrlapi.OverrideAccount{
						Code: newRPCBytes([]byte{
							// SLOAD(4) + SLOAD(3) (0x44 + 0x33 = 0x77)
							byte(vm.PUSH1), 0x04,
							byte(vm.SLOAD),
							byte(vm.PUSH1), 0x03,
							byte(vm.SLOAD),
							byte(vm.ADD),
							// MSTORE(0, sum) writes 64 bytes; value in memory[32:64]
							byte(vm.PUSH1), 0x00,
							byte(vm.MSTORE),
							// RETURN (32, 32)
							byte(vm.PUSH1), 32,
							byte(vm.PUSH1), 32,
							byte(vm.RETURN),
						}),
					},
				},
			},
			want: `{"gas":25224,"failed":false,"returnValue":"0x0000000000000000000000000000000000000000000000000000000000000077"}`,
		},
		{ // Full state override
			// The original storage is
			// 3: 0x33
			// 4: 0x44
			// With a full override, where we set 3:0x11, the slot 4 should be
			// removed. So SLOT(3)+SLOT(4) should be 0x11.
			blockNumber: rpc.LatestBlockNumber,
			call: qrlapi.TransactionArgs{
				From: &randomAccounts[0].addr,
				To:   &storageAccount,
			},
			config: &TraceCallConfig{
				StateOverrides: &qrlapi.StateOverride{
					storageAccount: qrlapi.OverrideAccount{
						Code: newRPCBytes([]byte{
							byte(vm.PUSH1), 0x04,
							byte(vm.SLOAD),
							byte(vm.PUSH1), 0x03,
							byte(vm.SLOAD),
							byte(vm.ADD),
							byte(vm.PUSH1), 0x00,
							byte(vm.MSTORE),
							byte(vm.PUSH1), 32,
							byte(vm.PUSH1), 32,
							byte(vm.RETURN),
						}),
						State: newStates(
							[]common.Hash{common.HexToHash("0x03")},
							[]common.Hash{common.HexToHash("0x11")}),
					},
				},
			},
			want: `{"gas":25224,"failed":false,"returnValue":"0x0000000000000000000000000000000000000000000000000000000000000011"}`,
		},
		{ // Partial state override
			// The original storage is
			// 3: 0x33
			// 4: 0x44
			// With a partial override, where we set 3:0x11, the slot 4 as before.
			// So SLOT(3)+SLOT(4) should be 0x55.
			blockNumber: rpc.LatestBlockNumber,
			call: qrlapi.TransactionArgs{
				From: &randomAccounts[0].addr,
				To:   &storageAccount,
			},
			config: &TraceCallConfig{
				StateOverrides: &qrlapi.StateOverride{
					storageAccount: qrlapi.OverrideAccount{
						Code: newRPCBytes([]byte{
							byte(vm.PUSH1), 0x04,
							byte(vm.SLOAD),
							byte(vm.PUSH1), 0x03,
							byte(vm.SLOAD),
							byte(vm.ADD),
							byte(vm.PUSH1), 0x00,
							byte(vm.MSTORE),
							byte(vm.PUSH1), 32,
							byte(vm.PUSH1), 32,
							byte(vm.RETURN),
						}),
						StateDiff: &map[common.Hash]common.StorageValue64{
							common.HexToHash("0x03"): common.HexToStorageValue64("0x11"),
						},
					},
				},
			},
			want: `{"gas":25224,"failed":false,"returnValue":"0x0000000000000000000000000000000000000000000000000000000000000055"}`,
		},
	}
	for i, tc := range testSuite {
		result, err := api.TraceCall(t.Context(), tc.call, rpc.BlockNumberOrHash{BlockNumber: &tc.blockNumber}, tc.config)
		if tc.expectErr != nil {
			if err == nil {
				t.Errorf("test %d: want error %v, have nothing", i, tc.expectErr)
				continue
			}
			if !errors.Is(err, tc.expectErr) {
				t.Errorf("test %d: error mismatch, want %v, have %v", i, tc.expectErr, err)
			}
			continue
		}
		if err != nil {
			t.Errorf("test %d: want no error, have %v", i, err)
			continue
		}
		// Turn result into res-struct
		var (
			have res
			want res
		)
		resBytes, _ := json.Marshal(result)
		json.Unmarshal(resBytes, &have)
		json.Unmarshal([]byte(tc.want), &want)
		if !reflect.DeepEqual(have, want) {
			t.Logf("result: %v\n", string(resBytes))
			t.Errorf("test %d, result mismatch, have\n%v\n, want\n%v\n", i, have, want)
		}
	}
}

type Account struct {
	wallet wallet.Wallet
	addr   common.Address
}

func newAccounts(n int) (accounts []Account) {
	for range n {
		wallet, _ := wallet.Generate(wallet.ML_DSA_87)
		accounts = append(accounts, Account{wallet: wallet, addr: wallet.GetAddress()})
	}
	slices.SortFunc(accounts, func(a, b Account) int { return a.addr.Cmp(b.addr) })
	return accounts
}

func newRPCBalance(balance *big.Int) **hexutil.Big {
	rpcBalance := (*hexutil.Big)(balance)
	return &rpcBalance
}

func newRPCBytes(bytes []byte) *hexutil.Bytes {
	rpcBytes := hexutil.Bytes(bytes)
	return &rpcBytes
}

func newStates(keys []common.Hash, vals []common.Hash) *map[common.Hash]common.StorageValue64 {
	if len(keys) != len(vals) {
		panic("invalid input")
	}
	m := make(map[common.Hash]common.StorageValue64)
	for i := range keys {
		m[keys[i]] = common.BytesToStorageValue64(vals[i].Bytes())
	}
	return &m
}

func TestTraceChain(t *testing.T) {
	// Initialize test accounts
	accounts := newAccounts(3)
	genesis := &core.Genesis{
		Config: params.TestChainConfig,
		Alloc: core.GenesisAlloc{
			accounts[0].addr: {Balance: big.NewInt(params.Quanta)},
			accounts[1].addr: {Balance: big.NewInt(params.Quanta)},
			accounts[2].addr: {Balance: big.NewInt(params.Quanta)},
		},
	}
	genBlocks := 50
	signer := types.ZondSigner{ChainId: big.NewInt(1)}

	var (
		ref   atomic.Uint32 // total refs has made
		rel   atomic.Uint32 // total rels has made
		nonce uint64
	)
	backend := newTestBackend(t, genBlocks, genesis, func(i int, b *core.BlockGen) {
		// Transfer from account[0] to account[1]
		//    value: 1000 planck
		//    fee:   0 planck
		for j := 0; j < i+1; j++ {
			tx := types.NewTx(&types.DynamicFeeTx{
				Nonce:     nonce,
				To:        &accounts[1].addr,
				Value:     big.NewInt(1000),
				Gas:       params.TxGas,
				GasFeeCap: b.BaseFee(),
				Data:      nil,
			})
			signedTx, _ := types.SignTx(tx, signer, accounts[0].wallet)
			b.AddTx(signedTx)
			nonce += 1
		}
	})
	backend.refHook = func() { ref.Add(1) }
	backend.relHook = func() { rel.Add(1) }
	api := NewAPI(backend)

	single := `{"txHash":"0x0000000000000000000000000000000000000000000000000000000000000000","result":{"gas":21000,"failed":false,"returnValue":"0x","structLogs":[]}}`
	var cases = []struct {
		start  uint64
		end    uint64
		config *TraceConfig
	}{
		{0, 50, nil},  // the entire chain range, blocks [1, 50]
		{10, 20, nil}, // the middle chain range, blocks [11, 20]
	}
	for _, c := range cases {
		ref.Store(0)
		rel.Store(0)

		from, _ := api.blockByNumber(t.Context(), rpc.BlockNumber(c.start))
		to, _ := api.blockByNumber(t.Context(), rpc.BlockNumber(c.end))
		resCh := api.traceChain(from, to, c.config, nil)

		next := c.start + 1
		for result := range resCh {
			if have, want := uint64(result.Block), next; have != want {
				t.Fatalf("unexpected tracing block, have %d want %d", have, want)
			}
			if have, want := len(result.Traces), int(next); have != want {
				t.Fatalf("unexpected result length, have %d want %d", have, want)
			}
			for _, trace := range result.Traces {
				trace.TxHash = common.Hash{}
				blob, _ := json.Marshal(trace)
				if have, want := string(blob), single; have != want {
					t.Fatalf("unexpected tracing result, have\n%v\nwant:\n%v", have, want)
				}
			}
			next += 1
		}
		if next != c.end+1 {
			t.Error("Missing tracing block")
		}

		if nref, nrel := ref.Load(), rel.Load(); nref != nrel {
			t.Errorf("Ref and deref actions are not equal, ref %d rel %d", nref, nrel)
		}
	}
}
