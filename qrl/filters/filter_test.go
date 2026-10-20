// Copyright 2015 The go-ethereum Authors
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

package filters

import (
	"context"
	"encoding/json"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/theQRL/go-qrl/accounts/abi"
	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/consensus/beacon"
	"github.com/theQRL/go-qrl/core"
	"github.com/theQRL/go-qrl/core/rawdb"
	"github.com/theQRL/go-qrl/core/types"
	"github.com/theQRL/go-qrl/core/vm"
	"github.com/theQRL/go-qrl/internal/testutil"
	"github.com/theQRL/go-qrl/params"
	"github.com/theQRL/go-qrl/rpc"
	"github.com/theQRL/go-qrl/trie"
)

func makeReceipt(addr common.Address) *types.Receipt {
	receipt := &types.Receipt{
		Type:              types.DynamicFeeTxType,
		CumulativeGasUsed: 0,
		Status:            types.ReceiptStatusSuccessful,
	}
	receipt.Logs = []*types.Log{
		{Address: addr},
	}
	receipt.Bloom = types.CreateBloom(types.Receipts{receipt})
	return receipt
}

func BenchmarkFilters(b *testing.B) {
	var (
		db, _   = rawdb.NewLevelDBDatabase(b.TempDir(), 0, 0, "", false)
		_, sys  = newTestFilterSystem(b, db, Config{})
		wallet1 = testutil.MustLoadAccount("alice").MustWallet()
		addr1   = wallet1.GetAddress()
		addr2   = common.BytesToAddress([]byte("jeff"))
		addr3   = common.BytesToAddress([]byte("ethereum"))
		addr4   = common.BytesToAddress([]byte("random addresses please"))
		to      = common.MustParseAddress("Q00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000999")

		gspec = &core.Genesis{
			Alloc:   core.GenesisAlloc{addr1: {Balance: big.NewInt(1000000)}},
			BaseFee: big.NewInt(params.InitialBaseFee),
			Config:  params.TestChainConfig,
		}
	)
	defer db.Close()
	_, chain, receipts := core.GenerateChainWithGenesis(gspec, beacon.NewFaker(), 100010, func(i int, gen *core.BlockGen) {
		switch i {
		case 2403:
			receipt := makeReceipt(addr1)
			gen.AddUncheckedReceipt(receipt)
			gen.AddUncheckedTx(types.NewTx(&types.DynamicFeeTx{Nonce: 999, To: &to, Value: big.NewInt(999), Gas: 999, GasFeeCap: gen.BaseFee(), Data: nil}))
		case 1034:
			receipt := makeReceipt(addr2)
			gen.AddUncheckedReceipt(receipt)
			gen.AddUncheckedTx(types.NewTx(&types.DynamicFeeTx{Nonce: 999, To: &to, Value: big.NewInt(999), Gas: 999, GasFeeCap: gen.BaseFee(), Data: nil}))
		case 34:
			receipt := makeReceipt(addr3)
			gen.AddUncheckedReceipt(receipt)
			gen.AddUncheckedTx(types.NewTx(&types.DynamicFeeTx{Nonce: 999, To: &to, Value: big.NewInt(999), Gas: 999, GasFeeCap: gen.BaseFee(), Data: nil}))
		case 99999:
			receipt := makeReceipt(addr4)
			gen.AddUncheckedReceipt(receipt)
			gen.AddUncheckedTx(types.NewTx(&types.DynamicFeeTx{Nonce: 999, To: &to, Value: big.NewInt(999), Gas: 999, GasFeeCap: gen.BaseFee(), Data: nil}))
		}
	})
	// The test txs are not properly signed, can't simply create a chain
	// and then import blocks. TODO(rjl493456442) try to get rid of the
	// manual database writes.
	gspec.MustCommit(db, trie.NewDatabase(db, trie.HashDefaults))

	for i, block := range chain {
		rawdb.WriteBlock(db, block)
		rawdb.WriteCanonicalHash(db, block.Hash(), block.NumberU64())
		rawdb.WriteHeadBlockHash(db, block.Hash())
		rawdb.WriteReceipts(db, block.Hash(), block.NumberU64(), receipts[i])
	}

	filter := sys.NewRangeFilter(0, -1, []common.Address{addr1, addr2, addr3, addr4}, nil)

	for b.Loop() {
		logs, _ := filter.Logs(b.Context())
		if len(logs) != 4 {
			b.Fatal("expected 4 logs, got", len(logs))
		}
	}
}

func TestFilters(t *testing.T) {
	var (
		db           = rawdb.NewMemoryDatabase()
		backend, sys = newTestFilterSystem(t, db, Config{})
		// Sender account
		wallet1 = testutil.MustLoadAccount("alice").MustWallet()
		addr    = wallet1.GetAddress()
		signer  = types.NewZondSigner(big.NewInt(1))
		// Logging contract
		contract  = common.Address{0xfe}
		contract2 = common.Address{0xff}
		abiStr    = `[{"inputs":[],"name":"log0","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"uint256","name":"t1","type":"uint256"}],"name":"log1","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"uint256","name":"t1","type":"uint256"},{"internalType":"uint256","name":"t2","type":"uint256"}],"name":"log2","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"uint256","name":"t1","type":"uint256"},{"internalType":"uint256","name":"t2","type":"uint256"},{"internalType":"uint256","name":"t3","type":"uint256"}],"name":"log3","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"uint256","name":"t1","type":"uint256"},{"internalType":"uint256","name":"t2","type":"uint256"},{"internalType":"uint256","name":"t3","type":"uint256"},{"internalType":"uint256","name":"t4","type":"uint256"}],"name":"log4","outputs":[],"stateMutability":"nonpayable","type":"function"}]`

		// Hand-rolled replacement for the Solidity Logger fixture. The
		// original contract relied on LOGn/DUP/SWAP opcodes that shifted
		// when the VM widened to 512-bit words. Under the 64-byte ABI
		// slot layout, log1(uint256) packs to 4+64=68 bytes of calldata
		// and log2(uint256,uint256) packs to 4+64+64=132 bytes. This
		// minimal bytecode branches on calldata size:
		//
		//   size == 132: emit LOG2 with topics t1=CALLDATALOAD(4),
		//                                       t2=CALLDATALOAD(68)
		//   otherwise:  emit LOG1 with topic  t1=CALLDATALOAD(4)
		//
		// The test never invokes log0/log3/log4, so the fall-through is
		// fine.
		bytecode = common.FromHex("60043536610084146100125760006000c1005b604435b060006000c200")

		// Arbitrary 32-byte values passed to the contract as uint256
		// indexed args, so they surface on chain as right-aligned numeric
		// topics — not left-aligned hash topics, despite the Hash type.
		hash1 = common.BytesToHash([]byte("topic1"))
		hash2 = common.BytesToHash([]byte("topic2"))
		hash3 = common.BytesToHash([]byte("topic3"))
		hash4 = common.BytesToHash([]byte("topic4"))
		hash5 = common.BytesToHash([]byte("topic5"))

		gspec = &core.Genesis{
			Config: params.TestChainConfig,
			Alloc: core.GenesisAlloc{
				addr:      {Balance: big.NewInt(0).Mul(big.NewInt(100), big.NewInt(params.Quanta))},
				contract:  {Balance: big.NewInt(0), Code: bytecode},
				contract2: {Balance: big.NewInt(0), Code: bytecode},
			},
			BaseFee: big.NewInt(params.InitialBaseFee),
		}
	)

	contractABI, err := abi.JSON(strings.NewReader(abiStr))
	if err != nil {
		t.Fatal(err)
	}

	// Hack: GenerateChainWithGenesis creates a new db.
	// Commit the genesis manually and use GenerateChain.
	_, err = gspec.Commit(db, trie.NewDatabase(db, nil))
	if err != nil {
		t.Fatal(err)
	}
	chain, _ := core.GenerateChain(gspec.Config, gspec.ToBlock(), beacon.NewFaker(), db, 1000, func(i int, gen *core.BlockGen) {
		switch i {
		case 1:
			data, err := contractABI.Pack("log1", hash1.Big())
			if err != nil {
				t.Fatal(err)
			}
			tx, _ := types.SignTx(types.NewTx(&types.DynamicFeeTx{
				Nonce:     0,
				GasFeeCap: gen.BaseFee(),
				Gas:       30000,
				To:        &contract,
				Data:      data,
			}), signer, wallet1)
			gen.AddTx(tx)
			tx2, _ := types.SignTx(types.NewTx(&types.DynamicFeeTx{
				Nonce:     1,
				GasFeeCap: gen.BaseFee(),
				Gas:       30000,
				To:        &contract2,
				Data:      data,
			}), signer, wallet1)
			gen.AddTx(tx2)
		case 2:
			data, err := contractABI.Pack("log2", hash2.Big(), hash1.Big())
			if err != nil {
				t.Fatal(err)
			}
			tx, _ := types.SignTx(types.NewTx(&types.DynamicFeeTx{
				Nonce:     2,
				GasFeeCap: gen.BaseFee(),
				Gas:       30000,
				To:        &contract,
				Data:      data,
			}), signer, wallet1)
			gen.AddTx(tx)
		case 998:
			data, err := contractABI.Pack("log1", hash3.Big())
			if err != nil {
				t.Fatal(err)
			}
			tx, _ := types.SignTx(types.NewTx(&types.DynamicFeeTx{
				Nonce:     3,
				GasFeeCap: gen.BaseFee(),
				Gas:       30000,
				To:        &contract2,
				Data:      data,
			}), signer, wallet1)
			gen.AddTx(tx)
		case 999:
			data, err := contractABI.Pack("log1", hash4.Big())
			if err != nil {
				t.Fatal(err)
			}
			tx, _ := types.SignTx(types.NewTx(&types.DynamicFeeTx{
				Nonce:     4,
				GasFeeCap: gen.BaseFee(),
				Gas:       30000,
				To:        &contract,
				Data:      data,
			}), signer, wallet1)
			gen.AddTx(tx)
		}
	})
	var l uint64
	bc, err := core.NewBlockChain(db, nil, gspec, beacon.NewFaker(), vm.Config{}, &l)
	if err != nil {
		t.Fatal(err)
	}
	_, err = bc.InsertChain(chain)
	if err != nil {
		t.Fatal(err)
	}

	// Set block 998 as Finalized (-3)
	bc.SetFinalized(chain[998].Header())

	// Generate pending block
	pchain, preceipts := core.GenerateChain(gspec.Config, chain[len(chain)-1], beacon.NewFaker(), db, 1, func(i int, gen *core.BlockGen) {
		data, err := contractABI.Pack("log1", hash5.Big())
		if err != nil {
			t.Fatal(err)
		}
		tx, err := types.SignTx(types.NewTx(&types.DynamicFeeTx{
			Nonce:     5,
			GasFeeCap: gen.BaseFee(),
			Gas:       30000,
			To:        &contract,
			Data:      data,
		}), signer, wallet1)
		if err != nil {
			t.Fatal(err)
		}
		gen.AddTx(tx)
	})
	backend.setPending(pchain[0], preceipts[0])

	// Expected logs are rebuilt from the actual chain/tx hashes produced
	// above so the test stays insulated from contract-bytecode changes.
	mkLog := func(blockIdx, txIdx, logIdx int, addr common.Address, topicBytes ...[]byte) *types.Log {
		block := chain[blockIdx]
		tx := block.Transactions()[txIdx]
		topics := make([]common.LogTopic, len(topicBytes))
		for i, tb := range topicBytes {
			topics[i] = common.BytesToRightAlignedLogTopic(tb)
		}
		return &types.Log{
			Address:     addr,
			Topics:      topics,
			Data:        []byte{},
			BlockNumber: block.NumberU64(),
			TxHash:      tx.Hash(),
			TxIndex:     uint(txIdx),
			BlockHash:   block.Hash(),
			Index:       uint(logIdx),
		}
	}
	mustJSON := func(logs ...*types.Log) string {
		b, err := json.Marshal(logs)
		if err != nil {
			t.Fatalf("marshal logs: %v", err)
		}
		return string(b)
	}
	for i, tc := range []struct {
		f    *Filter
		want string
		err  string
	}{
		{
			f:    sys.NewBlockFilter(chain[2].Hash(), []common.Address{contract}, nil),
			want: mustJSON(mkLog(2, 0, 0, contract, hash2.Bytes(), hash1.Bytes())),
		},
		{
			f: sys.NewRangeFilter(0, int64(rpc.LatestBlockNumber), []common.Address{contract}, [][]common.LogTopic{{common.BytesToRightAlignedLogTopic(hash1.Bytes()), common.BytesToRightAlignedLogTopic(hash2.Bytes()), common.BytesToRightAlignedLogTopic(hash3.Bytes()), common.BytesToRightAlignedLogTopic(hash4.Bytes())}}),
			want: mustJSON(
				mkLog(1, 0, 0, contract, hash1.Bytes()),
				mkLog(2, 0, 0, contract, hash2.Bytes(), hash1.Bytes()),
				mkLog(999, 0, 0, contract, hash4.Bytes()),
			),
		},
		{
			f: sys.NewRangeFilter(900, 999, []common.Address{contract}, [][]common.LogTopic{{common.BytesToRightAlignedLogTopic(hash3.Bytes())}}),
		},
		{
			f:    sys.NewRangeFilter(990, int64(rpc.LatestBlockNumber), []common.Address{contract2}, [][]common.LogTopic{{common.BytesToRightAlignedLogTopic(hash3.Bytes())}}),
			want: mustJSON(mkLog(998, 0, 0, contract2, hash3.Bytes())),
		},
		{
			f: sys.NewRangeFilter(1, 10, nil, [][]common.LogTopic{{common.BytesToRightAlignedLogTopic(hash1.Bytes()), common.BytesToRightAlignedLogTopic(hash2.Bytes())}}),
			want: mustJSON(
				mkLog(1, 0, 0, contract, hash1.Bytes()),
				mkLog(1, 1, 1, contract2, hash1.Bytes()),
				mkLog(2, 0, 0, contract, hash2.Bytes(), hash1.Bytes()),
			),
		},
		{
			f: sys.NewRangeFilter(0, int64(rpc.LatestBlockNumber), nil, [][]common.LogTopic{{common.BytesToRightAlignedLogTopic([]byte("fail"))}}),
		},
		{
			f: sys.NewRangeFilter(0, int64(rpc.LatestBlockNumber), []common.Address{common.BytesToAddress([]byte("failmenow"))}, nil),
		},
		{
			f: sys.NewRangeFilter(0, int64(rpc.LatestBlockNumber), nil, [][]common.LogTopic{{common.BytesToRightAlignedLogTopic([]byte("fail"))}, {common.BytesToRightAlignedLogTopic(hash1.Bytes())}}),
		},
		{
			f:    sys.NewRangeFilter(int64(rpc.LatestBlockNumber), int64(rpc.LatestBlockNumber), nil, nil),
			want: mustJSON(mkLog(999, 0, 0, contract, hash4.Bytes())),
		},
		{
			f: sys.NewRangeFilter(int64(rpc.FinalizedBlockNumber), int64(rpc.LatestBlockNumber), nil, nil),
			want: mustJSON(
				mkLog(998, 0, 0, contract2, hash3.Bytes()),
				mkLog(999, 0, 0, contract, hash4.Bytes()),
			),
		},
		{
			f:    sys.NewRangeFilter(int64(rpc.FinalizedBlockNumber), int64(rpc.FinalizedBlockNumber), nil, nil),
			want: mustJSON(mkLog(998, 0, 0, contract2, hash3.Bytes())),
		},
		{
			f: sys.NewRangeFilter(int64(rpc.LatestBlockNumber), int64(rpc.FinalizedBlockNumber), nil, nil),
		},
		{
			f:   sys.NewRangeFilter(int64(rpc.SafeBlockNumber), int64(rpc.LatestBlockNumber), nil, nil),
			err: "safe header not found",
		},
		{
			f:   sys.NewRangeFilter(int64(rpc.SafeBlockNumber), int64(rpc.SafeBlockNumber), nil, nil),
			err: "safe header not found",
		},
		{
			f:   sys.NewRangeFilter(int64(rpc.LatestBlockNumber), int64(rpc.SafeBlockNumber), nil, nil),
			err: "safe header not found",
		},
		{
			f:   sys.NewRangeFilter(int64(rpc.PendingBlockNumber), int64(rpc.PendingBlockNumber), nil, nil),
			err: errPendingLogsUnsupported.Error(),
		},
		{
			f:   sys.NewRangeFilter(int64(rpc.LatestBlockNumber), int64(rpc.PendingBlockNumber), nil, nil),
			err: errPendingLogsUnsupported.Error(),
		},
		{
			f:   sys.NewRangeFilter(int64(rpc.PendingBlockNumber), int64(rpc.LatestBlockNumber), nil, nil),
			err: errPendingLogsUnsupported.Error(),
		},
	} {
		logs, err := tc.f.Logs(t.Context())
		if err == nil && tc.err != "" {
			t.Fatalf("test %d, expected error %q, got nil", i, tc.err)
		} else if err != nil && err.Error() != tc.err {
			t.Fatalf("test %d, expected error %q, got %q", i, tc.err, err.Error())
		}
		if tc.want == "" && len(logs) == 0 {
			continue
		}
		have, err := json.Marshal(logs)
		if err != nil {
			t.Fatal(err)
		}
		if string(have) != tc.want {
			t.Fatalf("test %d, have:\n%s\nwant:\n%s", i, have, tc.want)
		}
	}

	t.Run("timeout", func(t *testing.T) {
		f := sys.NewRangeFilter(0, rpc.LatestBlockNumber.Int64(), nil, nil)
		ctx, cancel := context.WithDeadline(t.Context(), time.Now().Add(-time.Hour))
		defer cancel()
		_, err := f.Logs(ctx)
		if err == nil {
			t.Fatal("expected error")
		}
		if err != context.DeadlineExceeded {
			t.Fatalf("expected context.DeadlineExceeded, got %v", err)
		}
	})
}
