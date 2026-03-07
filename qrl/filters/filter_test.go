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
	"github.com/theQRL/go-qrl/crypto/pqcrypto/wallet"
	"github.com/theQRL/go-qrl/params"
	"github.com/theQRL/go-qrl/rpc"
	"github.com/theQRL/go-qrl/trie"
)

func makeReceipt(addr common.Address) *types.Receipt {
	receipt := &types.Receipt{
		Type:              types.DynamicFeeTxType,
		PostState:         common.CopyBytes(nil),
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
		db, _      = rawdb.NewLevelDBDatabase(b.TempDir(), 0, 0, "", false)
		_, sys     = newTestFilterSystem(b, db, Config{})
		wallet1, _ = wallet.RestoreFromSeedHex("0x010000b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f29100000000000000000000000000000000")
		addr1      = wallet1.GetAddress()
		addr2      = common.BytesToAddress([]byte("jeff"))
		addr3      = common.BytesToAddress([]byte("ethereum"))
		addr4      = common.BytesToAddress([]byte("random addresses please"))
		to, _      = common.NewAddressFromString("Q0000000000000000000000000000000000000999")

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
		wallet1, _ = wallet.RestoreFromSeedHex("0x010000b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f29100000000000000000000000000000000")
		addr       = wallet1.GetAddress()
		signer     = types.NewShanghaiSigner(big.NewInt(1))
		// Logging contract
		contract  = common.Address{0xfe}
		contract2 = common.Address{0xff}
		abiStr    = `[{"inputs":[],"name":"log0","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"uint256","name":"t1","type":"uint256"}],"name":"log1","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"uint256","name":"t1","type":"uint256"},{"internalType":"uint256","name":"t2","type":"uint256"}],"name":"log2","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"uint256","name":"t1","type":"uint256"},{"internalType":"uint256","name":"t2","type":"uint256"},{"internalType":"uint256","name":"t3","type":"uint256"}],"name":"log3","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"uint256","name":"t1","type":"uint256"},{"internalType":"uint256","name":"t2","type":"uint256"},{"internalType":"uint256","name":"t3","type":"uint256"},{"internalType":"uint256","name":"t4","type":"uint256"}],"name":"log4","outputs":[],"stateMutability":"nonpayable","type":"function"}]`

		/*
			// SPDX-License-Identifier: GPL-3.0
			// TODO(now.youtrack.cloud/issue/TGZ-30)
			pragma hyperion >=0.7.0 <0.9.0;

			contract Logger {
			    function log0() external {
			        assembly {
			            log0(0, 0)
			        }
			    }

			    function log1(uint t1) external {
			        assembly {
			            log1(0, 0, t1)
			        }
			    }

			    function log2(uint t1, uint t2) external {
			        assembly {
			            log2(0, 0, t1, t2)
			        }
			    }

			    function log3(uint t1, uint t2, uint t3) external {
			        assembly {
			            log3(0, 0, t1, t2, t3)
			        }
			    }

			    function log4(uint t1, uint t2, uint t3, uint t4) external {
			        assembly {
			            log4(0, 0, t1, t2, t3, t4)
			        }
			    }
			}
		*/

		bytecode = common.FromHex("608060405234801561001057600080fd5b50600436106100575760003560e01c80630aa731851461005c5780632a4c08961461006657806378b9a1f314610082578063c670f8641461009e578063c683d6a3146100ba575b600080fd5b6100646100d6565b005b610080600480360381019061007b9190610143565b6100dc565b005b61009c60048036038101906100979190610196565b6100e8565b005b6100b860048036038101906100b391906101d6565b6100f2565b005b6100d460048036038101906100cf9190610203565b6100fa565b005b600080a0565b808284600080a3505050565b8082600080a25050565b80600080a150565b80828486600080a450505050565b600080fd5b6000819050919050565b6101208161010d565b811461012b57600080fd5b50565b60008135905061013d81610117565b92915050565b60008060006060848603121561015c5761015b610108565b5b600061016a8682870161012e565b935050602061017b8682870161012e565b925050604061018c8682870161012e565b9150509250925092565b600080604083850312156101ad576101ac610108565b5b60006101bb8582860161012e565b92505060206101cc8582860161012e565b9150509250929050565b6000602082840312156101ec576101eb610108565b5b60006101fa8482850161012e565b91505092915050565b6000806000806080858703121561021d5761021c610108565b5b600061022b8782880161012e565b945050602061023c8782880161012e565b935050604061024d8782880161012e565b925050606061025e8782880161012e565b9150509295919450925056fea264697066735822122073a4b156f487e59970dc1ef449cc0d51467268f676033a17188edafcee861f9864736f6c63430008110033")

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

	for i, tc := range []struct {
		f    *Filter
		want string
		err  string
	}{
		{
			f:    sys.NewBlockFilter(chain[2].Hash(), []common.Address{contract}, nil),
			want: `[{"address":"Qfe00000000000000000000000000000000000000","topics":["0x0000000000000000000000000000000000000000000000000000746f70696332","0x0000000000000000000000000000000000000000000000000000746f70696331"],"data":"0x","blockNumber":"0x3","transactionHash":"0x0da87fefab92b6580390709d07d170b5a5efa3cad3d6628142c5c8d6e6533279","transactionIndex":"0x0","blockHash":"0x130442bd6f558c8e816bc3d1c5bc506b4c31efc18355eb7f3b6cd9393a272c7e","logIndex":"0x0","removed":false}]`,
		},
		{
			f:    sys.NewRangeFilter(0, int64(rpc.LatestBlockNumber), []common.Address{contract}, [][]common.Hash{{hash1, hash2, hash3, hash4}}),
			want: `[{"address":"Qfe00000000000000000000000000000000000000","topics":["0x0000000000000000000000000000000000000000000000000000746f70696331"],"data":"0x","blockNumber":"0x2","transactionHash":"0xd3facb859daa48326c0e4612feb94115c7b7128ebf614cbec2c34b2280455597","transactionIndex":"0x0","blockHash":"0xcd6b79eabf98c16bf545cb7189b233274f2699d25fcc089dac507cb88f703253","logIndex":"0x0","removed":false},{"address":"Qfe00000000000000000000000000000000000000","topics":["0x0000000000000000000000000000000000000000000000000000746f70696332","0x0000000000000000000000000000000000000000000000000000746f70696331"],"data":"0x","blockNumber":"0x3","transactionHash":"0x0da87fefab92b6580390709d07d170b5a5efa3cad3d6628142c5c8d6e6533279","transactionIndex":"0x0","blockHash":"0x130442bd6f558c8e816bc3d1c5bc506b4c31efc18355eb7f3b6cd9393a272c7e","logIndex":"0x0","removed":false},{"address":"Qfe00000000000000000000000000000000000000","topics":["0x0000000000000000000000000000000000000000000000000000746f70696334"],"data":"0x","blockNumber":"0x3e8","transactionHash":"0xdd1b17bb23012b0f8b3e9fe465f93452069c72aff647ce3a7a997d3710f06d75","transactionIndex":"0x0","blockHash":"0x267d8959e1950881c67a0a4b95d0aa0d3f5961be5800034add8bb9db103da61f","logIndex":"0x0","removed":false}]`,
		},
		{
			f: sys.NewRangeFilter(900, 999, []common.Address{contract}, [][]common.Hash{{hash3}}),
		},
		{
			f:    sys.NewRangeFilter(990, int64(rpc.LatestBlockNumber), []common.Address{contract2}, [][]common.Hash{{hash3}}),
			want: `[{"address":"Qff00000000000000000000000000000000000000","topics":["0x0000000000000000000000000000000000000000000000000000746f70696333"],"data":"0x","blockNumber":"0x3e7","transactionHash":"0xcef6909852c7317a800a13bc8ede04622a0aa15d62807b12122bf8fad6ac0b5f","transactionIndex":"0x0","blockHash":"0xd9886933bb0d1f514f5bdca8e08a73ef3c6014d40474d063e8e8e3aa5a5c7265","logIndex":"0x0","removed":false}]`,
		},
		{
			f:    sys.NewRangeFilter(1, 10, nil, [][]common.Hash{{hash1, hash2}}),
			want: `[{"address":"Qfe00000000000000000000000000000000000000","topics":["0x0000000000000000000000000000000000000000000000000000746f70696331"],"data":"0x","blockNumber":"0x2","transactionHash":"0xd3facb859daa48326c0e4612feb94115c7b7128ebf614cbec2c34b2280455597","transactionIndex":"0x0","blockHash":"0xcd6b79eabf98c16bf545cb7189b233274f2699d25fcc089dac507cb88f703253","logIndex":"0x0","removed":false},{"address":"Qff00000000000000000000000000000000000000","topics":["0x0000000000000000000000000000000000000000000000000000746f70696331"],"data":"0x","blockNumber":"0x2","transactionHash":"0x3ff357685cbd2c0aadc1d614a4315d050115cd2d25e88329654a2108052ae976","transactionIndex":"0x1","blockHash":"0xcd6b79eabf98c16bf545cb7189b233274f2699d25fcc089dac507cb88f703253","logIndex":"0x1","removed":false},{"address":"Qfe00000000000000000000000000000000000000","topics":["0x0000000000000000000000000000000000000000000000000000746f70696332","0x0000000000000000000000000000000000000000000000000000746f70696331"],"data":"0x","blockNumber":"0x3","transactionHash":"0x0da87fefab92b6580390709d07d170b5a5efa3cad3d6628142c5c8d6e6533279","transactionIndex":"0x0","blockHash":"0x130442bd6f558c8e816bc3d1c5bc506b4c31efc18355eb7f3b6cd9393a272c7e","logIndex":"0x0","removed":false}]`,
		},
		{
			f: sys.NewRangeFilter(0, int64(rpc.LatestBlockNumber), nil, [][]common.Hash{{common.BytesToHash([]byte("fail"))}}),
		},
		{
			f: sys.NewRangeFilter(0, int64(rpc.LatestBlockNumber), []common.Address{common.BytesToAddress([]byte("failmenow"))}, nil),
		},
		{
			f: sys.NewRangeFilter(0, int64(rpc.LatestBlockNumber), nil, [][]common.Hash{{common.BytesToHash([]byte("fail"))}, {hash1}}),
		},
		{
			f:    sys.NewRangeFilter(int64(rpc.LatestBlockNumber), int64(rpc.LatestBlockNumber), nil, nil),
			want: `[{"address":"Qfe00000000000000000000000000000000000000","topics":["0x0000000000000000000000000000000000000000000000000000746f70696334"],"data":"0x","blockNumber":"0x3e8","transactionHash":"0xdd1b17bb23012b0f8b3e9fe465f93452069c72aff647ce3a7a997d3710f06d75","transactionIndex":"0x0","blockHash":"0x267d8959e1950881c67a0a4b95d0aa0d3f5961be5800034add8bb9db103da61f","logIndex":"0x0","removed":false}]`,
		},
		{
			f:    sys.NewRangeFilter(int64(rpc.FinalizedBlockNumber), int64(rpc.LatestBlockNumber), nil, nil),
			want: `[{"address":"Qff00000000000000000000000000000000000000","topics":["0x0000000000000000000000000000000000000000000000000000746f70696333"],"data":"0x","blockNumber":"0x3e7","transactionHash":"0xcef6909852c7317a800a13bc8ede04622a0aa15d62807b12122bf8fad6ac0b5f","transactionIndex":"0x0","blockHash":"0xd9886933bb0d1f514f5bdca8e08a73ef3c6014d40474d063e8e8e3aa5a5c7265","logIndex":"0x0","removed":false},{"address":"Qfe00000000000000000000000000000000000000","topics":["0x0000000000000000000000000000000000000000000000000000746f70696334"],"data":"0x","blockNumber":"0x3e8","transactionHash":"0xdd1b17bb23012b0f8b3e9fe465f93452069c72aff647ce3a7a997d3710f06d75","transactionIndex":"0x0","blockHash":"0x267d8959e1950881c67a0a4b95d0aa0d3f5961be5800034add8bb9db103da61f","logIndex":"0x0","removed":false}]`,
		},
		{
			f:    sys.NewRangeFilter(int64(rpc.FinalizedBlockNumber), int64(rpc.FinalizedBlockNumber), nil, nil),
			want: `[{"address":"Qff00000000000000000000000000000000000000","topics":["0x0000000000000000000000000000000000000000000000000000746f70696333"],"data":"0x","blockNumber":"0x3e7","transactionHash":"0xcef6909852c7317a800a13bc8ede04622a0aa15d62807b12122bf8fad6ac0b5f","transactionIndex":"0x0","blockHash":"0xd9886933bb0d1f514f5bdca8e08a73ef3c6014d40474d063e8e8e3aa5a5c7265","logIndex":"0x0","removed":false}]`,
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
