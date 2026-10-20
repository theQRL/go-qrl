// Copyright 2014 The go-ethereum Authors
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

package types

import (
	"bytes"
	gomath "math"
	"math/big"
	"testing"

	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/common/math"
	"github.com/theQRL/go-qrl/crypto/pqcrypto"
	"github.com/theQRL/go-qrl/crypto/pqcrypto/wallet"
	"github.com/theQRL/go-qrl/internal/blocktest"
	"github.com/theQRL/go-qrl/internal/testutil"
	"github.com/theQRL/go-qrl/params"
	"github.com/theQRL/go-qrl/rlp"
)

// TestBlockEncoding builds a canonical one-tx block, RLP-encodes it,
// decodes the bytes back, and re-encodes. It asserts the round-trip is a
// fixed point. A previous version compared against a hard-coded hex blob
// produced pre-migration; with 64-byte addresses and the wider VM word the
// blob is no longer valid, but canonicality of the encoding still is.
func TestBlockEncoding(t *testing.T) {
	roundTripBlockEncoding(t, func(signer Signer, w wallet.Wallet) *Transaction {
		to := common.MustParseAddress("Q000000000000000000000000000000000000000000000000000000009a9070028361F7AAbeB3f2F2Dc07F82C4a98A02a99aabbccddeeff001122334455667788")
		tx, err := SignNewTx(w, signer, &DynamicFeeTx{
			ChainID:   signer.ChainID(),
			Nonce:     9,
			To:        &to,
			Value:     big.NewInt(1),
			Gas:       params.TxGas,
			GasFeeCap: big.NewInt(875000000),
			GasTipCap: big.NewInt(params.Shor / 1000),
			Data:      nil,
		})
		if err != nil {
			t.Fatalf("sign tx: %v", err)
		}
		return tx
	})
}

// TestEIP1559BlockEncoding is TestBlockEncoding with non-zero gas-tip/fee-cap
// to exercise the EIP-1559 encoding path.
func TestEIP1559BlockEncoding(t *testing.T) {
	roundTripBlockEncoding(t, func(signer Signer, w wallet.Wallet) *Transaction {
		to := common.MustParseAddress("Q000000000000000000000000000000000000000000000000000000009a9070028361F7AAbeB3f2F2Dc07F82C4a98A02a99aabbccddeeff001122334455667788")
		tx, err := SignNewTx(w, signer, &DynamicFeeTx{
			ChainID:   signer.ChainID(),
			Nonce:     18,
			To:        &to,
			Value:     big.NewInt(1),
			Gas:       25300,
			GasFeeCap: big.NewInt(875000000),
			GasTipCap: big.NewInt(params.Shor / 1000),
			AccessList: AccessList{
				AccessTuple{
					Address:     to,
					StorageKeys: []common.Hash{{}},
				},
			},
		})
		if err != nil {
			t.Fatalf("sign tx: %v", err)
		}
		return tx
	})
}

// TestEIP2718BlockEncoding is the typed-transaction variant of the round-trip
// — same invariants, a different transaction shape.
func TestEIP2718BlockEncoding(t *testing.T) {
	roundTripBlockEncoding(t, func(signer Signer, w wallet.Wallet) *Transaction {
		to := common.MustParseAddress("Q000000000000000000000000000000000000000000000000000000009a9070028361F7AAbeB3f2F2Dc07F82C4a98A02a99aabbccddeeff001122334455667788")
		tx, err := SignNewTx(w, signer, &DynamicFeeTx{
			ChainID:   signer.ChainID(),
			Nonce:     42,
			To:        &to,
			Value:     big.NewInt(1234),
			Gas:       21300,
			GasFeeCap: big.NewInt(875000000),
			GasTipCap: big.NewInt(params.Shor / 2000),
			Data:      []byte{0xde, 0xad, 0xbe, 0xef},
		})
		if err != nil {
			t.Fatalf("sign tx: %v", err)
		}
		return tx
	})
}

// roundTripBlockEncoding drives the encode→decode→encode fixed-point check.
// Callers supply a factory that produces the signed transaction included in
// the block so each test can exercise its own transaction shape without
// duplicating the block scaffolding.
func roundTripBlockEncoding(t *testing.T, makeTx func(Signer, wallet.Wallet) *Transaction) {
	t.Helper()
	signer := ZondSigner{ChainId: big.NewInt(1337)}
	w := testutil.LoadAccount(t, "alice").Wallet(t)
	tx := makeTx(signer, w)

	header := &Header{
		ParentHash:      common.Hash{},
		Coinbase:        common.Address{},
		Root:            common.HexToHash("0xe0d841b3f9f2ca592e4075480aeb7d434a94dc45270d855afe432764d83eaec9"),
		TxHash:          EmptyTxsHash,
		ReceiptHash:     EmptyReceiptsHash,
		Number:          big.NewInt(1),
		GasLimit:        4712388,
		GasUsed:         21000,
		Time:            9150,
		Extra:           []byte{},
		BaseFee:         big.NewInt(0),
		WithdrawalsHash: &EmptyWithdrawalsHash,
	}

	// Pass an empty Withdrawals slice (not nil) so NewBlock keeps
	// header.WithdrawalsHash populated — the reflection-based Header decoder
	// requires the field, otherwise it fails with "too few elements".
	block := NewBlock(header, &Body{Transactions: Transactions{tx}, Withdrawals: Withdrawals{}}, nil, blocktest.NewHasher())

	enc, err := rlp.EncodeToBytes(block)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	var decoded Block
	if err := rlp.DecodeBytes(enc, &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// Spot-check the decoded fields survived the round-trip.
	if got, want := decoded.GasLimit(), block.GasLimit(); got != want {
		t.Errorf("GasLimit mismatch: got %d, want %d", got, want)
	}
	if got, want := decoded.GasUsed(), block.GasUsed(); got != want {
		t.Errorf("GasUsed mismatch: got %d, want %d", got, want)
	}
	if got, want := len(decoded.Transactions()), 1; got != want {
		t.Fatalf("Transactions len mismatch: got %d, want %d", got, want)
	}
	if got, want := decoded.Transactions()[0].Hash(), tx.Hash(); got != want {
		t.Errorf("Transactions[0].Hash mismatch: got %x, want %x", got, want)
	}

	// Canonicality: re-encoding the decoded block must produce the same bytes.
	reenc, err := rlp.EncodeToBytes(&decoded)
	if err != nil {
		t.Fatalf("re-encode: %v", err)
	}
	if !bytes.Equal(enc, reenc) {
		t.Errorf("re-encoded block differs:\nfirst:  %x\nsecond: %x", enc, reenc)
	}
}

var benchBuffer = bytes.NewBuffer(make([]byte, 0, 32000))

func BenchmarkEncodeBlock(b *testing.B) {
	block := makeBenchBlock()

	for b.Loop() {
		benchBuffer.Reset()
		if err := rlp.Encode(benchBuffer, block); err != nil {
			b.Fatal(err)
		}
	}
}

func makeBenchBlock() *Block {
	var (
		w, _     = wallet.Generate(wallet.ML_DSA_87)
		txs      = make([]*Transaction, 70)
		receipts = make([]*Receipt, len(txs))
		signer   = NewZondSigner(params.TestChainConfig.ChainID)
	)
	header := &Header{
		Number:   math.BigPow(2, 9),
		GasLimit: 12345678,
		GasUsed:  1476322,
		Time:     9876543,
		Extra:    []byte("coolest block on chain"),
	}
	for i := range txs {
		amount := math.BigPow(2, int64(i))
		gasFeeCap := big.NewInt(300000)
		data := make([]byte, 100)
		tx := NewTx(&DynamicFeeTx{
			Nonce:     uint64(i),
			To:        &common.Address{},
			Value:     amount,
			Gas:       123457,
			GasFeeCap: gasFeeCap,
			Data:      data,
		})
		signedTx, err := SignTx(tx, signer, w)
		if err != nil {
			panic(err)
		}
		txs[i] = signedTx
		receipts[i] = &Receipt{
			Type:              DynamicFeeTxType,
			CumulativeGasUsed: tx.Gas(),
			Status:            ReceiptStatusSuccessful,
		}
	}

	return NewBlock(header, &Body{Transactions: txs}, receipts, blocktest.NewHasher())
}

func TestRlpDecodeParentHash(t *testing.T) {
	// A minimum one
	want := common.HexToHash("0x112233445566778899001122334455667788990011223344556677889900aabb")
	if rlpData, err := rlp.EncodeToBytes(&Header{ParentHash: want}); err != nil {
		t.Fatal(err)
	} else {
		if have := HeaderParentHashFromRLP(rlpData); have != want {
			t.Fatalf("have %x, want %x", have, want)
		}
	}
	// And a maximum one
	// | Number      | dynamic| *big.Int       | 64 bits               |
	// | Extra       | dynamic| []byte         | 65+32 byte (clique)   |
	// | BaseFee     | dynamic| *big.Int       | 64 bits               |
	if rlpData, err := rlp.EncodeToBytes(&Header{
		ParentHash: want,
		Number:     new(big.Int).SetUint64(gomath.MaxUint64),
		Extra:      make([]byte, 65+32),
		BaseFee:    new(big.Int).SetUint64(gomath.MaxUint64),
	}); err != nil {
		t.Fatal(err)
	} else {
		if have := HeaderParentHashFromRLP(rlpData); have != want {
			t.Fatalf("have %x, want %x", have, want)
		}
	}
	// Also test a very very large header.
	{
		// The rlp-encoding of the header belowCauses _total_ length of 65540,
		// which is the first to blow the fast-path.
		h := &Header{
			ParentHash: want,
			Extra:      make([]byte, 65041),
		}
		if rlpData, err := rlp.EncodeToBytes(h); err != nil {
			t.Fatal(err)
		} else {
			if have := HeaderParentHashFromRLP(rlpData); have != want {
				t.Fatalf("have %x, want %x", have, want)
			}
		}
	}
	{
		// Test some invalid erroneous stuff
		for i, rlpData := range [][]byte{
			nil,
			common.FromHex("0x"),
			common.FromHex("0x01"),
			common.FromHex("0x3031323334"),
		} {
			if have, want := HeaderParentHashFromRLP(rlpData), (common.Hash{}); have != want {
				t.Fatalf("invalid %d: have %x, want %x", i, have, want)
			}
		}
	}
}

// TestMaxBlockSize computes the maximum possible block size when the block is
// full of simple transfer transactions that consume the entire gas limit.
func TestMaxBlockSize(t *testing.T) {
	gasLimit := params.MaxGasLimit
	numTxs := gasLimit / params.TxGas

	to := common.MustParseAddress("Q000000000000000000000000000000000000000000000000000000009a9070028361F7AAbeB3f2F2Dc07F82C4a98A02a99aabbccddeeff001122334455667788")

	txs := make([]*Transaction, numTxs)
	for i := uint64(0); i < numTxs; i++ {
		tx := NewTx(&DynamicFeeTx{
			ChainID:     big.NewInt(1),
			Nonce:       i,
			GasTipCap:   big.NewInt(1),
			GasFeeCap:   big.NewInt(1000000000),
			Gas:         params.TxGas,
			To:          &to,
			Value:       big.NewInt(1),
			Descriptor:  [3]byte{},
			ExtraParams: []byte{},
			Signature:   make([]byte, pqcrypto.MLDSA87SignatureLength),
			PublicKey:   make([]byte, pqcrypto.MLDSA87PublicKeyLength),
		})
		txs[i] = tx
	}

	header := &Header{
		ParentHash: common.Hash{},
		Number:     big.NewInt(1),
		GasLimit:   gasLimit,
		GasUsed:    numTxs * params.TxGas,
		BaseFee:    big.NewInt(1000000000),
		Extra:      make([]byte, params.MaximumExtraDataSize),
	}

	block := NewBlock(header, &Body{Transactions: txs}, nil, blocktest.NewHasher())

	txSize := txs[0].Size()
	blockSize := block.Size()

	t.Logf("Gas limit:          %d", gasLimit)
	t.Logf("Tx gas:             %d", params.TxGas)
	t.Logf("Number of txs:      %d", numTxs)
	t.Logf("Single tx size:     %d bytes", txSize)
	t.Logf("Block size:         %d bytes (%.2f MB)", blockSize, float64(blockSize)/(1024*1024))

	if block.GasLimit() != 20000000 {
		t.Errorf("gas limit mismatch: got %d, want %d", block.GasLimit(), 20000000)
	}
	if block.GasUsed() != 19992000 {
		t.Errorf("gas used mismatch: got %d, want %d", block.GasUsed(), 19992000)
	}
	// Block layout widened for 64-byte addresses (Coinbase, tx To fields,
	// bloom topic words). Re-run the test and print if that ever changes again.
	const expectedBlockSize = uint64(6967855)
	if blockSize != expectedBlockSize {
		t.Errorf("block size mismatch: got %d, want %d", blockSize, expectedBlockSize)
	}
}
