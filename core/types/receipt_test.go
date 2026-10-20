// Copyright 2019 The go-ethereum Authors
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
	"encoding/json"
	"math"
	"math/big"
	"reflect"
	"testing"

	"github.com/kylelemons/godebug/diff"
	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/params"
	"github.com/theQRL/go-qrl/rlp"
)

var (
	eip1559Receipt = &Receipt{
		Status:            ReceiptStatusFailed,
		CumulativeGasUsed: 1,
		Logs: []*Log{
			{
				Address: common.BytesToAddress([]byte{0x11}),
				Topics:  []common.LogTopic{common.BytesToRightAlignedLogTopic(common.FromHex("dead")), common.BytesToRightAlignedLogTopic(common.FromHex("beef"))},
				Data:    []byte{0x01, 0x00, 0xff},
			},
			{
				Address: common.BytesToAddress([]byte{0x01, 0x11}),
				Topics:  []common.LogTopic{common.BytesToRightAlignedLogTopic(common.FromHex("dead")), common.BytesToRightAlignedLogTopic(common.FromHex("beef"))},
				Data:    []byte{0x01, 0x00, 0xff},
			},
		},
		Type: DynamicFeeTxType,
	}

	// Create a few transactions to have receipts for (64-byte addresses).
	to2 = common.BytesToAddress([]byte{0x02})
	to3 = common.BytesToAddress([]byte{0x03})
	to4 = common.BytesToAddress([]byte{0x04})
	to5 = common.BytesToAddress([]byte{0x05})
	txs = Transactions{
		NewTx(&DynamicFeeTx{
			Nonce:     1,
			Value:     big.NewInt(1),
			Gas:       1,
			GasFeeCap: big.NewInt(11),
		}),
		NewTx(&DynamicFeeTx{
			To:        &to2,
			Nonce:     2,
			Value:     big.NewInt(2),
			Gas:       2,
			GasFeeCap: big.NewInt(22),
		}),
		NewTx(&DynamicFeeTx{
			To:        &to3,
			Nonce:     3,
			Value:     big.NewInt(3),
			Gas:       3,
			GasFeeCap: big.NewInt(33),
		}),
		// EIP-1559 transactions.
		NewTx(&DynamicFeeTx{
			To:        &to4,
			Nonce:     4,
			Value:     big.NewInt(4),
			Gas:       4,
			GasTipCap: big.NewInt(44),
			GasFeeCap: big.NewInt(1044),
		}),
		NewTx(&DynamicFeeTx{
			To:        &to5,
			Nonce:     5,
			Value:     big.NewInt(5),
			Gas:       5,
			GasTipCap: big.NewInt(55),
			GasFeeCap: big.NewInt(1055),
		}),
	}

	blockNumber = big.NewInt(1)
	blockTime   = uint64(2)
	blockHash   = common.BytesToHash([]byte{0x03, 0x14})
	// contractAddr is CreateAddress(zeroSender, nonce=1) — derived from the
	// first (unsigned) transaction in txs. Recomputed for 64-byte QRL
	// addresses from the full Keccak-512 output.
	contractAddr = common.BytesToAddress(common.FromHex("527695ddb29b7b441078ceb6da24bba83b71b3657ec892e32cabca55ec3760eb72f7786e44b162ca0b2ffa620f3a5aa297dd554dee4d7bf19e5ea420c0ae6ed9"))

	// Create the corresponding receipts
	receipts = Receipts{
		&Receipt{
			Type:              DynamicFeeTxType,
			Status:            ReceiptStatusFailed,
			CumulativeGasUsed: 1,
			Logs: []*Log{
				{
					Address: common.BytesToAddress([]byte{0x11}),
					Topics:  []common.LogTopic{common.BytesToRightAlignedLogTopic(common.FromHex("dead")), common.BytesToRightAlignedLogTopic(common.FromHex("beef"))},
					// derived fields:
					BlockNumber: blockNumber.Uint64(),
					TxHash:      txs[0].Hash(),
					TxIndex:     0,
					BlockHash:   blockHash,
					Index:       0,
				},
				{
					Address: common.BytesToAddress([]byte{0x01, 0x11}),
					Topics:  []common.LogTopic{common.BytesToRightAlignedLogTopic(common.FromHex("dead")), common.BytesToRightAlignedLogTopic(common.FromHex("beef"))},
					// derived fields:
					BlockNumber: blockNumber.Uint64(),
					TxHash:      txs[0].Hash(),
					TxIndex:     0,
					BlockHash:   blockHash,
					Index:       1,
				},
			},
			// derived fields:
			TxHash:            txs[0].Hash(),
			ContractAddress:   contractAddr,
			GasUsed:           1,
			EffectiveGasPrice: big.NewInt(11),
			BlockHash:         blockHash,
			BlockNumber:       blockNumber,
			TransactionIndex:  0,
		},
		&Receipt{
			Type:              DynamicFeeTxType,
			Status:            ReceiptStatusSuccessful,
			CumulativeGasUsed: 3,
			Logs: []*Log{
				{
					Address: common.BytesToAddress([]byte{0x22}),
					Topics:  []common.LogTopic{common.BytesToRightAlignedLogTopic(common.FromHex("dead")), common.BytesToRightAlignedLogTopic(common.FromHex("beef"))},
					// derived fields:
					BlockNumber: blockNumber.Uint64(),
					TxHash:      txs[1].Hash(),
					TxIndex:     1,
					BlockHash:   blockHash,
					Index:       2,
				},
				{
					Address: common.BytesToAddress([]byte{0x02, 0x22}),
					Topics:  []common.LogTopic{common.BytesToRightAlignedLogTopic(common.FromHex("dead")), common.BytesToRightAlignedLogTopic(common.FromHex("beef"))},
					// derived fields:
					BlockNumber: blockNumber.Uint64(),
					TxHash:      txs[1].Hash(),
					TxIndex:     1,
					BlockHash:   blockHash,
					Index:       3,
				},
			},
			// derived fields:
			TxHash:            txs[1].Hash(),
			GasUsed:           2,
			EffectiveGasPrice: big.NewInt(22),
			BlockHash:         blockHash,
			BlockNumber:       blockNumber,
			TransactionIndex:  1,
		},
		&Receipt{
			Type:              DynamicFeeTxType,
			Status:            ReceiptStatusSuccessful,
			CumulativeGasUsed: 6,
			Logs:              []*Log{},
			// derived fields:
			TxHash:            txs[2].Hash(),
			GasUsed:           3,
			EffectiveGasPrice: big.NewInt(33),
			BlockHash:         blockHash,
			BlockNumber:       blockNumber,
			TransactionIndex:  2,
		},
		&Receipt{
			Type:              DynamicFeeTxType,
			Status:            ReceiptStatusSuccessful,
			CumulativeGasUsed: 10,
			Logs:              []*Log{},
			// derived fields:
			TxHash:            txs[3].Hash(),
			GasUsed:           4,
			EffectiveGasPrice: big.NewInt(1044),
			BlockHash:         blockHash,
			BlockNumber:       blockNumber,
			TransactionIndex:  3,
		},
		&Receipt{
			Type:              DynamicFeeTxType,
			Status:            ReceiptStatusSuccessful,
			CumulativeGasUsed: 15,
			Logs:              []*Log{},
			// derived fields:
			TxHash:            txs[4].Hash(),
			GasUsed:           5,
			EffectiveGasPrice: big.NewInt(1055),
			BlockHash:         blockHash,
			BlockNumber:       blockNumber,
			TransactionIndex:  4,
		},
	}
)

func TestRejectPostStateReceipt(t *testing.T) {
	type encodedReceipt struct {
		Status            []byte
		CumulativeGasUsed uint64
		Bloom             Bloom
		Logs              []*Log
	}
	payload, err := rlp.EncodeToBytes(&encodedReceipt{
		Status:            common.Hash{2}.Bytes(),
		CumulativeGasUsed: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	raw := append([]byte{DynamicFeeTxType}, payload...)
	if err := new(Receipt).UnmarshalBinary(raw); err == nil {
		t.Fatal("expected error decoding a pre-Byzantium post-state receipt")
	}
}

func TestDecodeEmptyTypedReceipt(t *testing.T) {
	input := []byte{0x80}
	var r Receipt
	err := rlp.DecodeBytes(input, &r)
	if err != errShortTypedReceipt {
		t.Fatal("wrong error:", err)
	}
}

func TestReceiptSizeCountsLogTopicLength(t *testing.T) {
	withoutTopic := (&Receipt{
		Logs: []*Log{{
			Data: []byte{0x01, 0x02, 0x03},
		}},
	}).Size()

	withTopic := (&Receipt{
		Logs: []*Log{{
			Topics: []common.LogTopic{{}},
			Data:   []byte{0x01, 0x02, 0x03},
		}},
	}).Size()

	if diff := withTopic - withoutTopic; diff != common.StorageSize(common.LogTopicLength) {
		t.Fatalf("receipt topic size mismatch: have %d, want %d", uint64(diff), common.LogTopicLength)
	}
}

// Tests that receipt data can be correctly derived from the contextual infos
func TestDeriveFields(t *testing.T) {
	// Re-derive receipts.
	basefee := big.NewInt(1000)
	derivedReceipts := clearComputedFieldsOnReceipts(receipts)
	err := Receipts(derivedReceipts).DeriveFields(params.TestChainConfig, blockHash, blockNumber.Uint64(), blockTime, basefee, txs)
	if err != nil {
		t.Fatalf("DeriveFields(...) = %v, want <nil>", err)
	}

	// Check diff of receipts against derivedReceipts.
	r1, err := json.MarshalIndent(receipts, "", "  ")
	if err != nil {
		t.Fatal("error marshaling input receipts:", err)
	}

	r2, err := json.MarshalIndent(derivedReceipts, "", "  ")
	if err != nil {
		t.Fatal("error marshaling derived receipts:", err)
	}
	d := diff.Diff(string(r1), string(r2))
	if d != "" {
		t.Fatal("receipts differ:", d)
	}
}

// Test that we can marshal/unmarshal receipts to/from json without errors.
// This also confirms that our test receipts contain all the required fields.
func TestReceiptJSON(t *testing.T) {
	for i := range receipts {
		b, err := receipts[i].MarshalJSON()
		if err != nil {
			t.Fatal("error marshaling receipt to json:", err)
		}
		r := Receipt{}
		err = r.UnmarshalJSON(b)
		if err != nil {
			t.Fatal("error unmarshaling receipt from json:", err)
		}
	}
}

// Test we can still parse receipt without EffectiveGasPrice for backwards compatibility, even
// though it is required per the spec.
func TestEffectiveGasPriceNotRequired(t *testing.T) {
	r := *receipts[0]
	r.EffectiveGasPrice = nil
	b, err := r.MarshalJSON()
	if err != nil {
		t.Fatal("error marshaling receipt to json:", err)
	}
	r2 := Receipt{}
	err = r2.UnmarshalJSON(b)
	if err != nil {
		t.Fatal("error unmarshaling receipt from json:", err)
	}
}

// TestTypedReceiptEncodingDecoding reproduces a flaw that existed in the receipt
// rlp decoder, which failed due to a shadowing error.
func TestTypedReceiptEncodingDecoding(t *testing.T) {
	// Bundle of two EIP-1559 receipts with 64-byte log addresses and topics.
	var payload bytes.Buffer
	if err := rlp.Encode(&payload, Receipts{receipts[0], receipts[0]}); err != nil {
		t.Fatal(err)
	}
	check := func(bundle []*Receipt) {
		t.Helper()
		for i, receipt := range bundle {
			if got, want := receipt.Type, uint8(2); got != want {
				t.Fatalf("bundle %d: got %x, want %x", i, got, want)
			}
		}
	}
	{
		var bundle []*Receipt
		rlp.DecodeBytes(payload.Bytes(), &bundle)
		check(bundle)
	}
	{
		var bundle []*Receipt
		r := bytes.NewReader(payload.Bytes())
		s := rlp.NewStream(r, uint64(payload.Len()))
		if err := s.Decode(&bundle); err != nil {
			t.Fatal(err)
		}
		check(bundle)
	}
}

func TestReceiptMarshalBinary(t *testing.T) {
	buf := new(bytes.Buffer)

	// 1559 Receipt
	buf.Reset()
	eip1559Receipt.Bloom = CreateBloom(Receipts{eip1559Receipt})
	have, err := eip1559Receipt.MarshalBinary()
	if err != nil {
		t.Fatalf("marshal binary error: %v", err)
	}
	eip1559Receipts := Receipts{eip1559Receipt}
	eip1559Receipts.EncodeIndex(0, buf)
	haveEncodeIndex := buf.Bytes()
	if !bytes.Equal(have, haveEncodeIndex) {
		t.Errorf("BinaryMarshal and EncodeIndex mismatch, got %x want %x", have, haveEncodeIndex)
	}
}

func TestReceiptUnmarshalBinary(t *testing.T) {
	eip1559Receipt.Bloom = CreateBloom(Receipts{eip1559Receipt})
	eip1559RctBinary, err := eip1559Receipt.MarshalBinary()
	if err != nil {
		t.Fatalf("marshal binary error: %v", err)
	}
	got1559Receipt := new(Receipt)
	if err := got1559Receipt.UnmarshalBinary(eip1559RctBinary); err != nil {
		t.Fatalf("unmarshal binary error: %v", err)
	}
	if !reflect.DeepEqual(got1559Receipt, eip1559Receipt) {
		t.Errorf("receipt unmarshalled from binary mismatch, got %v want %v", got1559Receipt, eip1559Receipt)
	}
}

func clearComputedFieldsOnReceipts(receipts []*Receipt) []*Receipt {
	r := make([]*Receipt, len(receipts))
	for i, receipt := range receipts {
		r[i] = clearComputedFieldsOnReceipt(receipt)
	}
	return r
}

func clearComputedFieldsOnReceipt(receipt *Receipt) *Receipt {
	cpy := *receipt
	cpy.TxHash = common.Hash{0xff, 0xff, 0x11}
	cpy.BlockHash = common.Hash{0xff, 0xff, 0x22}
	cpy.BlockNumber = big.NewInt(math.MaxUint32)
	cpy.TransactionIndex = math.MaxUint32
	cpy.ContractAddress = common.Address{0xff, 0xff, 0x33}
	cpy.GasUsed = 0xffffffff
	cpy.Logs = clearComputedFieldsOnLogs(receipt.Logs)
	cpy.EffectiveGasPrice = big.NewInt(0)
	return &cpy
}

func clearComputedFieldsOnLogs(logs []*Log) []*Log {
	l := make([]*Log, len(logs))
	for i, log := range logs {
		cpy := *log
		cpy.BlockNumber = math.MaxUint32
		cpy.BlockHash = common.Hash{}
		cpy.TxHash = common.Hash{}
		cpy.TxIndex = math.MaxUint32
		cpy.Index = math.MaxUint32
		l[i] = &cpy
	}
	return l
}
