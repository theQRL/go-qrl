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
	"math"
	"math/big"
	"testing"

	walletmldsa87 "github.com/theQRL/go-qrllib/wallet/ml_dsa_87"
	"github.com/theQRL/go-zond/common"
	"github.com/theQRL/go-zond/consensus"
	"github.com/theQRL/go-zond/consensus/beacon"
	"github.com/theQRL/go-zond/consensus/misc/eip1559"
	"github.com/theQRL/go-zond/core/rawdb"
	"github.com/theQRL/go-zond/core/types"
	"github.com/theQRL/go-zond/core/vm"
	"github.com/theQRL/go-zond/crypto/pqcrypto"
	"github.com/theQRL/go-zond/params"
	"github.com/theQRL/go-zond/trie"
	"golang.org/x/crypto/sha3"
)

// TestStateProcessorErrors tests the output from the 'core' errors
// as defined in core/error.go. These errors are generated when the
// blockchain imports bad blocks, meaning blocks which have valid headers but
// contain invalid transactions
func TestStateProcessorErrors(t *testing.T) {
	var (
		config = &params.ChainConfig{
			ChainID: big.NewInt(1),
		}
		signer  = types.LatestSigner(config)
		key1, _ = pqcrypto.HexToWallet("f29f58aff0b00de2844f7e20bd9eeaacc379150043beeb328335817512b29fbb7184da84a092f842b2a06d72a24a5d28")
		key2, _ = pqcrypto.HexToWallet("a7b1a3005d9e110009c48d45deb43f0a0e31846ed2c5aaefb6d4238040ad4c08794ffe65585c13eb6948c2faf6db90c2")
	)

	var mkDynamicTx = func(key *walletmldsa87.Wallet, nonce uint64, to common.Address, value *big.Int, gasLimit uint64, gasTipCap, gasFeeCap *big.Int) *types.Transaction {
		tx, _ := types.SignTx(types.NewTx(&types.DynamicFeeTx{
			Nonce:     nonce,
			GasTipCap: gasTipCap,
			GasFeeCap: gasFeeCap,
			Gas:       gasLimit,
			To:        &to,
			Value:     value,
		}), signer, key)
		return tx
	}
	var mkDynamicCreationTx = func(nonce uint64, gasLimit uint64, gasTipCap, gasFeeCap *big.Int, data []byte) *types.Transaction {
		tx, _ := types.SignTx(types.NewTx(&types.DynamicFeeTx{
			Nonce:     nonce,
			GasTipCap: gasTipCap,
			GasFeeCap: gasFeeCap,
			Gas:       gasLimit,
			Value:     big.NewInt(0),
			Data:      data,
		}), signer, key1)
		return tx
	}

	{ // Tests against a 'recent' chain definition
		var (
			address0, _ = common.NewAddressFromString("QD5812F6cf4a0f645aA620CD57319a0Ed649Dd8f5")
			address1, _ = common.NewAddressFromString("Qbe95a82D87a6Cb9c7fF4C64e0C15bB1dfF20b1d7")
			db          = rawdb.NewMemoryDatabase()
			gspec       = &Genesis{
				Config: config,
				Alloc: GenesisAlloc{
					address0: GenesisAccount{
						Balance: big.NewInt(1000000000000000000), // 1 quanta
						Nonce:   0,
					},
					address1: GenesisAccount{
						Balance: big.NewInt(1000000000000000000), // 1 quanta
						Nonce:   math.MaxUint64,
					},
				},
			}
			blockchain, _  = NewBlockChain(db, nil, gspec, beacon.New(), vm.Config{}, nil)
			tooBigInitCode = [params.MaxInitCodeSize + 1]byte{}
		)

		defer blockchain.Stop()
		bigNumber := new(big.Int).SetBytes(common.MaxHash.Bytes())
		tooBigNumber := new(big.Int).Set(bigNumber)
		tooBigNumber.Add(tooBigNumber, common.Big1)
		for i, tt := range []struct {
			txs  []*types.Transaction
			want string
		}{

			{ // ErrNonceTooLow
				txs: []*types.Transaction{
					mkDynamicTx(key1, 0, common.Address{}, big.NewInt(0), params.TxGas, big.NewInt(0), big.NewInt(875000000)),
					mkDynamicTx(key1, 0, common.Address{}, big.NewInt(0), params.TxGas, big.NewInt(0), big.NewInt(875000000)),
				},
				want: "could not apply tx 1 [0x18680fbd1b2d4621be7be75a21c40e0ddc5d81d8c5dabf76ed8628db5e6b2360]: nonce too low: address QD5812F6cf4a0f645aA620CD57319a0Ed649Dd8f5, tx: 0 state: 1",
			},
			{ // ErrNonceTooHigh
				txs: []*types.Transaction{
					mkDynamicTx(key1, 100, common.Address{}, big.NewInt(0), params.TxGas, big.NewInt(0), big.NewInt(875000000)),
				},
				want: "could not apply tx 0 [0x1b6ab310ac27eff2a5d3d21bd2753aace7f6d43b15d67fb9e8940d746c65c61a]: nonce too high: address QD5812F6cf4a0f645aA620CD57319a0Ed649Dd8f5, tx: 100 state: 0",
			},
			{ // ErrNonceMax
				txs: []*types.Transaction{
					mkDynamicTx(key2, math.MaxUint64, common.Address{}, big.NewInt(0), params.TxGas, big.NewInt(0), big.NewInt(875000000)),
				},
				want: "could not apply tx 0 [0x231a00dd7e841ceb2833f62c0b2b6e5adbfd8ea815a765e0e9949c6219f662a4]: nonce has max value: address Qbe95a82D87a6Cb9c7fF4C64e0C15bB1dfF20b1d7, nonce: 18446744073709551615",
			},
			{ // ErrGasLimitReached
				txs: []*types.Transaction{
					mkDynamicTx(key1, 0, common.Address{}, big.NewInt(0), 21000000, big.NewInt(0), big.NewInt(875000000)),
				},
				want: "could not apply tx 0 [0xb9b495c9bca395f4927bdb10a33c84db602d48bffc7a9412e64dc09ec41d0628]: gas limit reached",
			},
			{ // ErrInsufficientFundsForTransfer
				txs: []*types.Transaction{
					mkDynamicTx(key1, 0, common.Address{}, big.NewInt(1000000000000000000), params.TxGas, big.NewInt(0), big.NewInt(875000000)),
				},
				want: "could not apply tx 0 [0x98fd012981b157672b3a43feae6079c0ebf209b8802ee7fc3d6413febdbb91f3]: insufficient funds for gas * price + value: address QD5812F6cf4a0f645aA620CD57319a0Ed649Dd8f5 have 1000000000000000000 want 1000018375000000000",
			},
			{ // ErrInsufficientFunds
				txs: []*types.Transaction{
					mkDynamicTx(key1, 0, common.Address{}, big.NewInt(0), params.TxGas, big.NewInt(0), big.NewInt(900000000000000000)),
				},
				want: "could not apply tx 0 [0x9de32d33a85020efc7272bf2420f9104957faa2941d983b16982f39adbc9e47a]: insufficient funds for gas * price + value: address QD5812F6cf4a0f645aA620CD57319a0Ed649Dd8f5 have 1000000000000000000 want 18900000000000000000000",
			},
			// ErrGasUintOverflow
			// One missing 'core' error is ErrGasUintOverflow: "gas uint64 overflow",
			// In order to trigger that one, we'd have to allocate a _huge_ chunk of data, such that the
			// multiplication len(data) +gas_per_byte overflows uint64. Not testable at the moment
			{ // ErrIntrinsicGas
				txs: []*types.Transaction{
					mkDynamicTx(key1, 0, common.Address{}, big.NewInt(0), params.TxGas-1000, big.NewInt(0), big.NewInt(875000000)),
				},
				want: "could not apply tx 0 [0x8c9f174936d64bfb8c27d529d23654ad180e4c937cca87f42fb90394d8476a38]: intrinsic gas too low: have 20000, want 21000",
			},
			{ // ErrGasLimitReached
				txs: []*types.Transaction{
					mkDynamicTx(key1, 0, common.Address{}, big.NewInt(0), params.TxGas*1000, big.NewInt(0), big.NewInt(875000000)),
				},
				want: "could not apply tx 0 [0xb9b495c9bca395f4927bdb10a33c84db602d48bffc7a9412e64dc09ec41d0628]: gas limit reached",
			},
			{ // ErrFeeCapTooLow
				txs: []*types.Transaction{
					mkDynamicTx(key1, 0, common.Address{}, big.NewInt(0), params.TxGas, big.NewInt(0), big.NewInt(0)),
				},
				want: "could not apply tx 0 [0x531be3e0e884adc5c46760cbb9ea261484ff665cfd62f759984ea4aee52d3dd6]: max fee per gas less than block base fee: address QD5812F6cf4a0f645aA620CD57319a0Ed649Dd8f5, maxFeePerGas: 0 baseFee: 875000000",
			},
			{ // ErrTipVeryHigh
				txs: []*types.Transaction{
					mkDynamicTx(key1, 0, common.Address{}, big.NewInt(0), params.TxGas, tooBigNumber, big.NewInt(1)),
				},
				want: "could not apply tx 0 [0xc64e235203efbb67b86c9f8575fa077266912afa5e1c0464a1ad8fb016d060db]: max priority fee per gas higher than 2^256-1: address QD5812F6cf4a0f645aA620CD57319a0Ed649Dd8f5, maxPriorityFeePerGas bit length: 257",
			},
			{ // ErrFeeCapVeryHigh
				txs: []*types.Transaction{
					mkDynamicTx(key1, 0, common.Address{}, big.NewInt(0), params.TxGas, big.NewInt(1), tooBigNumber),
				},
				want: "could not apply tx 0 [0x7a6daa7035f3afc56058e48f9bd7fc0e0acc749eda515fbd534a40af246057b9]: max fee per gas higher than 2^256-1: address QD5812F6cf4a0f645aA620CD57319a0Ed649Dd8f5, maxFeePerGas bit length: 257",
			},
			{ // ErrTipAboveFeeCap
				txs: []*types.Transaction{
					mkDynamicTx(key1, 0, common.Address{}, big.NewInt(0), params.TxGas, big.NewInt(2), big.NewInt(1)),
				},
				want: "could not apply tx 0 [0xa0be5a75283012c100f1dc45c9c7166b72193a17e188d2488ddc5d90e37cba32]: max priority fee per gas higher than max fee per gas: address QD5812F6cf4a0f645aA620CD57319a0Ed649Dd8f5, maxPriorityFeePerGas: 2, maxFeePerGas: 1",
			},
			{ // ErrInsufficientFunds
				// Available balance:           1000000000000000000
				// Effective cost:                   18375000021000
				// FeeCap * gas:                1050000000000000000
				// This test is designed to have the effective cost be covered by the balance, but
				// the extended requirement on FeeCap*gas < balance to fail
				txs: []*types.Transaction{
					mkDynamicTx(key1, 0, common.Address{}, big.NewInt(0), params.TxGas, big.NewInt(1), big.NewInt(50000000000000)),
				},
				want: "could not apply tx 0 [0x048e88c4155bb31674e61c5636802efe80d1ad556cb7c6dec86ff6f138819afd]: insufficient funds for gas * price + value: address QD5812F6cf4a0f645aA620CD57319a0Ed649Dd8f5 have 1000000000000000000 want 1050000000000000000",
			},
			{ // Another ErrInsufficientFunds, this one to ensure that feecap/tip of max u256 is allowed
				txs: []*types.Transaction{
					mkDynamicTx(key1, 0, common.Address{}, big.NewInt(0), params.TxGas, bigNumber, bigNumber),
				},
				want: "could not apply tx 0 [0xf1134538210807db830132ef4ad6d17c63f50be0934bc3eaebe07140da64d3d6]: insufficient funds for gas * price + value: address QD5812F6cf4a0f645aA620CD57319a0Ed649Dd8f5 have 1000000000000000000 want 2431633873983640103894990685182446064918669677978451844828609264166175722438635000",
			},
			{ // ErrMaxInitCodeSizeExceeded
				txs: []*types.Transaction{
					mkDynamicCreationTx(0, 500000, common.Big0, big.NewInt(params.InitialBaseFee), tooBigInitCode[:]),
				},
				want: "could not apply tx 0 [0xd5bf3e553d301fe2142984a2bfebbed24d0db12c4c9d93a743fa8dbce30e6a29]: max initcode size exceeded: code size 49153 limit 49152",
			},
			{ // ErrIntrinsicGas: Not enough gas to cover init code
				txs: []*types.Transaction{
					mkDynamicCreationTx(0, 54299, common.Big0, big.NewInt(params.InitialBaseFee), make([]byte, 320)),
				},
				want: "could not apply tx 0 [0x2f9df3e31ffca243a5a34d52cb6b0d93cc6e979e0bd5e68cbc67c724eaa3a5a0]: intrinsic gas too low: have 54299, want 54300",
			},
		} {
			block := GenerateBadBlock(gspec.ToBlock(), beacon.New(), tt.txs, gspec.Config)
			_, err := blockchain.InsertChain(types.Blocks{block})
			if err == nil {
				t.Fatal("block imported without errors")
			}
			if have, want := err.Error(), tt.want; have != want {
				t.Errorf("test %d:\nhave \"%v\"\nwant \"%v\"\n", i, have, want)
			}
		}
	}

	// NOTE(rgeraldes24): test not valid for now
	/*
		// ErrTxTypeNotSupported, For this, we need an older chain
		{
			var (
				db    = rawdb.NewMemoryDatabase()
				gspec = &Genesis{
					Config: &params.ChainConfig{
						ChainID: big.NewInt(1),
					},
					Alloc: GenesisAlloc{
						common.HexToAddress("Q71562b71999873DB5b286dF957af199Ec94617F7"): GenesisAccount{
							Balance: big.NewInt(1000000000000000000), // 1 quanta
							Nonce:   0,
						},
					},
				}
				blockchain, _ = NewBlockChain(db, nil, gspec, beacon.NewFaker(), vm.Config{}, nil)
			)
			defer blockchain.Stop()
			for i, tt := range []struct {
				txs  []*types.Transaction
				want string
			}{
				{ // ErrTxTypeNotSupported
					txs: []*types.Transaction{
						mkDynamicTx(0, common.Address{}, params.TxGas-1000, big.NewInt(0), big.NewInt(0)),
					},
					want: "could not apply tx 0 [0x88626ac0d53cb65308f2416103c62bb1f18b805573d4f96a3640bbbfff13c14f]: transaction type not supported",
				},
			} {
				block := GenerateBadBlock(gspec.ToBlock(), beacon.NewFaker(), tt.txs, gspec.Config)
				_, err := blockchain.InsertChain(types.Blocks{block})
				if err == nil {
					t.Fatal("block imported without errors")
				}
				if have, want := err.Error(), tt.want; have != want {
					t.Errorf("test %d:\nhave \"%v\"\nwant \"%v\"\n", i, have, want)
				}
			}
		}
	*/

	// ErrSenderNoEOA, for this we need the sender to have contract code
	{
		var (
			address, _ = common.NewAddressFromString("QD5812F6cf4a0f645aA620CD57319a0Ed649Dd8f5")
			db         = rawdb.NewMemoryDatabase()
			gspec      = &Genesis{
				Config: config,
				Alloc: GenesisAlloc{
					address: GenesisAccount{
						Balance: big.NewInt(1000000000000000000), // 1 quanta
						Nonce:   0,
						Code:    common.FromHex("0xB0B0FACE"),
					},
				},
			}
			blockchain, _ = NewBlockChain(db, nil, gspec, beacon.New(), vm.Config{}, nil)
		)
		defer blockchain.Stop()
		for i, tt := range []struct {
			txs  []*types.Transaction
			want string
		}{
			{ // ErrSenderNoEOA
				txs: []*types.Transaction{
					mkDynamicTx(key1, 0, common.Address{}, big.NewInt(0), params.TxGas-1000, big.NewInt(875000000), big.NewInt(0)),
				},
				want: "could not apply tx 0 [0xc7bbe0cb9f0a9b3dde352739b0292b3a986340862e9236b8f05254a4388e2c20]: sender not an eoa: address QD5812F6cf4a0f645aA620CD57319a0Ed649Dd8f5, codehash: 0x9280914443471259d4570a8661015ae4a5b80186dbc619658fb494bebc3da3d1",
			},
		} {
			block := GenerateBadBlock(gspec.ToBlock(), beacon.New(), tt.txs, gspec.Config)
			_, err := blockchain.InsertChain(types.Blocks{block})
			if err == nil {
				t.Fatal("block imported without errors")
			}
			if have, want := err.Error(), tt.want; have != want {
				t.Errorf("test %d:\nhave \"%v\"\nwant \"%v\"\n", i, have, want)
			}
		}
	}
}

// GenerateBadBlock constructs a "block" which contains the transactions. The transactions are not expected to be
// valid, and no proper post-state can be made. But from the perspective of the blockchain, the block is sufficiently
// valid to be considered for import:
// - valid pow (fake), ancestry, difficulty, gaslimit etc
func GenerateBadBlock(parent *types.Block, engine consensus.Engine, txs types.Transactions, config *params.ChainConfig) *types.Block {
	header := &types.Header{
		ParentHash: parent.Hash(),
		Coinbase:   parent.Coinbase(),
		GasLimit:   parent.GasLimit(),
		Number:     new(big.Int).Add(parent.Number(), common.Big1),
		Time:       parent.Time() + 10,
	}
	header.BaseFee = eip1559.CalcBaseFee(config, parent.Header())
	header.WithdrawalsHash = &types.EmptyWithdrawalsHash
	var receipts []*types.Receipt
	// The post-state result doesn't need to be correct (this is a bad block), but we do need something there
	// Preferably something unique. So let's use a combo of blocknum + txhash
	hasher := sha3.NewLegacyKeccak256()
	hasher.Write(header.Number.Bytes())
	var cumulativeGas uint64
	for _, tx := range txs {
		txh := tx.Hash()
		hasher.Write(txh[:])
		receipt := &types.Receipt{
			Type:              types.DynamicFeeTxType,
			PostState:         common.CopyBytes(nil),
			CumulativeGasUsed: cumulativeGas + tx.Gas(),
			Status:            types.ReceiptStatusSuccessful,
		}
		receipt.TxHash = tx.Hash()
		receipt.GasUsed = tx.Gas()
		receipts = append(receipts, receipt)
		cumulativeGas += tx.Gas()
	}
	header.Root = common.BytesToHash(hasher.Sum(nil))

	// Assemble and return the final block for sealing
	body := &types.Body{Transactions: txs, Withdrawals: []*types.Withdrawal{}}
	return types.NewBlock(header, body, receipts, trie.NewStackTrie(nil))
}
