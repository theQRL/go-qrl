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

package gasprice

import (
	"context"
	"math"
	"math/big"
	"testing"

	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/consensus/beacon"
	"github.com/theQRL/go-qrl/core"
	"github.com/theQRL/go-qrl/core/state"
	"github.com/theQRL/go-qrl/core/types"
	"github.com/theQRL/go-qrl/core/vm"
	"github.com/theQRL/go-qrl/crypto/pqcrypto/wallet"
	"github.com/theQRL/go-qrl/event"
	"github.com/theQRL/go-qrl/params"
	"github.com/theQRL/go-qrl/rpc"
)

const testHead = 32

type testBackend struct {
	chain   *core.BlockChain
	pending bool // pending block available
}

func (b *testBackend) HeaderByNumber(ctx context.Context, number rpc.BlockNumber) (*types.Header, error) {
	if number > testHead {
		return nil, nil
	}
	if number == rpc.EarliestBlockNumber {
		number = 0
	}
	if number == rpc.FinalizedBlockNumber {
		return b.chain.CurrentFinalBlock(), nil
	}
	if number == rpc.SafeBlockNumber {
		return b.chain.CurrentSafeBlock(), nil
	}
	if number == rpc.LatestBlockNumber {
		number = testHead
	}
	if number == rpc.PendingBlockNumber {
		if b.pending {
			number = testHead + 1
		} else {
			return nil, nil
		}
	}
	return b.chain.GetHeaderByNumber(uint64(number)), nil
}

func (b *testBackend) BlockByNumber(ctx context.Context, number rpc.BlockNumber) (*types.Block, error) {
	if number > testHead {
		return nil, nil
	}
	if number == rpc.EarliestBlockNumber {
		number = 0
	}
	if number == rpc.FinalizedBlockNumber {
		number = rpc.BlockNumber(b.chain.CurrentFinalBlock().Number.Uint64())
	}
	if number == rpc.SafeBlockNumber {
		number = rpc.BlockNumber(b.chain.CurrentSafeBlock().Number.Uint64())
	}
	if number == rpc.LatestBlockNumber {
		number = testHead
	}
	if number == rpc.PendingBlockNumber {
		if b.pending {
			number = testHead + 1
		} else {
			return nil, nil
		}
	}
	return b.chain.GetBlockByNumber(uint64(number)), nil
}

func (b *testBackend) GetReceipts(ctx context.Context, hash common.Hash) (types.Receipts, error) {
	return b.chain.GetReceiptsByHash(hash), nil
}

func (b *testBackend) Pending() (*types.Block, types.Receipts, *state.StateDB) {
	if b.pending {
		block := b.chain.GetBlockByNumber(testHead + 1)
		state, _ := b.chain.StateAt(block.Root())
		return block, b.chain.GetReceiptsByHash(block.Hash()), state
	}
	return nil, nil, nil
}

func (b *testBackend) ChainConfig() *params.ChainConfig {
	return b.chain.Config()
}

func (b *testBackend) SubscribeChainHeadEvent(ch chan<- core.ChainHeadEvent) event.Subscription {
	return nil
}

func (b *testBackend) teardown() {
	b.chain.Stop()
}

// newTestBackend creates a test backend. OBS: don't forget to invoke tearDown
// after use, otherwise the blockchain instance will mem-leak via goroutines.
func newTestBackend(t *testing.T, pending bool) *testBackend {
	var (
		wallet, _ = wallet.RestoreFromSeedHex("010000b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f29100000000000000000000000000000000")
		addr      = wallet.GetAddress()
		config    = *params.TestChainConfig // needs copy because it is modified below
		gspec     = &core.Genesis{
			Config: &config,
			Alloc:  core.GenesisAlloc{addr: {Balance: big.NewInt(math.MaxInt64)}},
		}
		signer = types.LatestSigner(gspec.Config)
	)

	engine := beacon.NewFaker()

	// Generate testing blocks
	db, blocks, _ := core.GenerateChainWithGenesis(gspec, engine, testHead+1, func(i int, b *core.BlockGen) {
		b.SetCoinbase(common.Address{1})

		txdata := &types.DynamicFeeTx{
			ChainID:   gspec.Config.ChainID,
			Nonce:     b.TxNonce(addr),
			To:        &common.Address{},
			Gas:       30000,
			GasFeeCap: big.NewInt(100 * params.Shor),
			GasTipCap: big.NewInt(int64(i+1) * params.Shor),
			Data:      []byte{},
		}
		b.AddTx(types.MustSignNewTx(wallet, signer, txdata))
	})
	// Construct testing chain
	chain, err := core.NewBlockChain(db, &core.CacheConfig{TrieCleanNoPrefetch: true}, gspec, engine, vm.Config{}, nil)
	if err != nil {
		t.Fatalf("Failed to create local chain, %v", err)
	}
	if i, err := chain.InsertChain(blocks); err != nil {
		t.Fatalf("Failed to insert block %d: %v", i, err)
	}
	chain.SetFinalized(chain.GetBlockByNumber(25).Header())
	chain.SetSafe(chain.GetBlockByNumber(25).Header())
	return &testBackend{chain: chain, pending: pending}
}

func (b *testBackend) CurrentHeader() *types.Header {
	return b.chain.CurrentHeader()
}

func (b *testBackend) GetBlockByNumber(number uint64) *types.Block {
	return b.chain.GetBlockByNumber(number)
}

func TestSuggestTipCap(t *testing.T) {
	config := Config{
		Blocks:     3,
		Percentile: 60,
		Default:    big.NewInt(params.Shor),
	}
	var cases = []struct {
		fork   *big.Int // London fork number
		expect *big.Int // Expected gasprice suggestion
	}{
		{nil, big.NewInt(params.Shor * int64(30))},
		{big.NewInt(0), big.NewInt(params.Shor * int64(30))},  // Fork point in genesis
		{big.NewInt(1), big.NewInt(params.Shor * int64(30))},  // Fork point in first block
		{big.NewInt(32), big.NewInt(params.Shor * int64(30))}, // Fork point in last block
		{big.NewInt(33), big.NewInt(params.Shor * int64(30))}, // Fork point in the future
	}
	for _, c := range cases {
		backend := newTestBackend(t, false)
		oracle := NewOracle(backend, config)

		// The gas price sampled is: 32G, 31G, 30G, 29G, 28G, 27G
		got, err := oracle.SuggestTipCap(t.Context())
		backend.teardown()
		if err != nil {
			t.Fatalf("Failed to retrieve recommended gas price: %v", err)
		}
		if got.Cmp(c.expect) != 0 {
			t.Fatalf("Gas price mismatch, want %d, got %d", c.expect, got)
		}
	}
}
