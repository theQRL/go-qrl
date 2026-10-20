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

package tracetest

import (
	"encoding/json"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/common/hexutil"
	"github.com/theQRL/go-qrl/common/math"
	"github.com/theQRL/go-qrl/core"
	"github.com/theQRL/go-qrl/core/rawdb"
	"github.com/theQRL/go-qrl/core/types"
	"github.com/theQRL/go-qrl/core/vm"
	"github.com/theQRL/go-qrl/params"
	"github.com/theQRL/go-qrl/qrl/tracers"
	"github.com/theQRL/go-qrl/rlp"
	"github.com/theQRL/go-qrl/tests"
)

type callContext struct {
	Number   math.HexOrDecimal64   `json:"number"`
	Time     math.HexOrDecimal64   `json:"timestamp"`
	GasLimit math.HexOrDecimal64   `json:"gasLimit"`
	Miner    common.Address        `json:"miner"`
	BaseFee  *math.HexOrDecimal256 `json:"baseFeePerGas"`
}

// callLog is the result of LOG opCode
type callLog struct {
	Address common.Address    `json:"address"`
	Topics  []common.LogTopic `json:"topics"`
	Data    hexutil.Bytes     `json:"data"`
}

// callTrace is the result of a callTracer run.
type callTrace struct {
	From         common.Address  `json:"from"`
	Gas          *hexutil.Uint64 `json:"gas"`
	GasUsed      *hexutil.Uint64 `json:"gasUsed"`
	To           *common.Address `json:"to,omitempty"`
	Input        hexutil.Bytes   `json:"input"`
	Output       hexutil.Bytes   `json:"output,omitempty"`
	Error        string          `json:"error,omitempty"`
	RevertReason string          `json:"revertReason,omitempty"`
	Calls        []callTrace     `json:"calls,omitempty"`
	Logs         []callLog       `json:"logs,omitempty"`
	Value        *hexutil.Big    `json:"value,omitempty"`
	// Gencodec adds overridden fields at the end
	Type string `json:"type"`
}

// callTracerTest defines a single test to check the call tracer against.
type callTracerTest struct {
	Genesis      *core.Genesis   `json:"genesis"`
	Context      *callContext    `json:"context"`
	Input        string          `json:"input"`
	TracerConfig json.RawMessage `json:"tracerConfig"`
	Result       *callTrace      `json:"result"`
}

// TestCallTracerNative scans testdata/call_tracer/*.json and runs the
// callTracer against each regenerated 64-byte-address fixture.
func TestCallTracerNative(t *testing.T) {
	testCallTracer("callTracer", "call_tracer", t)
}

// TODO(now.youtrack.cloud/issue/TGZ-13)
func TestCallTracerNativeWithLog(t *testing.T) {
	testCallTracer("callTracer", "call_tracer_withLog", t)
}

func testCallTracer(tracerName string, dirPath string, t *testing.T) {
	files, err := os.ReadDir(filepath.Join("testdata", dirPath))
	if err != nil {
		t.Fatalf("failed to retrieve tracer test suite: %v", err)
	}
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".json") {
			continue
		}
		t.Run(camel(strings.TrimSuffix(file.Name(), ".json")), func(t *testing.T) {
			t.Parallel()

			var (
				test = new(callTracerTest)
				tx   = new(types.Transaction)
			)
			// Call tracer test found, read if from disk
			if blob, err := os.ReadFile(filepath.Join("testdata", dirPath, file.Name())); err != nil {
				t.Fatalf("failed to read testcase: %v", err)
			} else if err := json.Unmarshal(blob, test); err != nil {
				t.Fatalf("failed to parse testcase: %v", err)
			}
			if err := tx.UnmarshalBinary(common.FromHex(test.Input)); err != nil {
				t.Fatalf("failed to parse testcase input: %v", err)
			}
			// Configure a blockchain with the given prestate
			var (
				signer    = types.NewZondSigner(test.Genesis.Config.ChainID)
				origin, _ = signer.Sender(tx)
				txContext = vm.TxContext{
					Origin:   origin,
					GasPrice: tx.GasPrice(),
				}
				context = vm.BlockContext{
					CanTransfer: core.CanTransfer,
					Transfer:    core.Transfer,
					Coinbase:    test.Context.Miner,
					BlockNumber: new(big.Int).SetUint64(uint64(test.Context.Number)),
					Time:        uint64(test.Context.Time),
					GasLimit:    uint64(test.Context.GasLimit),
					BaseFee:     test.Genesis.BaseFee,
				}
				triedb, _, statedb = tests.MakePreState(rawdb.NewMemoryDatabase(), test.Genesis.Alloc, false, rawdb.HashScheme)
			)
			triedb.Close()

			tracer, err := tracers.DefaultDirectory.New(tracerName, new(tracers.Context), test.TracerConfig)
			if err != nil {
				t.Fatalf("failed to create call tracer: %v", err)
			}
			qrvm := vm.NewQRVM(context, txContext, statedb, test.Genesis.Config, vm.Config{Tracer: tracer})
			msg, err := core.TransactionToMessage(tx, signer, nil)
			if err != nil {
				t.Fatalf("failed to prepare transaction for tracing: %v", err)
			}
			vmRet, err := core.ApplyMessage(qrvm, msg, new(core.GasPool).AddGas(tx.Gas()))
			if err != nil {
				t.Fatalf("failed to execute transaction: %v", err)
			}
			// Retrieve the trace result and compare against the expected.
			res, err := tracer.GetResult()
			if err != nil {
				t.Fatalf("failed to retrieve trace result: %v", err)
			}
			want, err := json.Marshal(test.Result)
			if err != nil {
				t.Fatalf("failed to marshal test: %v", err)
			}
			if !sameTraceJSON(want, res) {
				t.Fatalf("trace mismatch\n have: %v\n want: %v\n", string(res), string(want))
			}
			// Sanity check: compare top call's gas used against vm result
			type simpleResult struct {
				GasUsed hexutil.Uint64
			}
			var topCall simpleResult
			if err := json.Unmarshal(res, &topCall); err != nil {
				t.Fatalf("failed to unmarshal top calls gasUsed: %v", err)
			}
			if uint64(topCall.GasUsed) != vmRet.UsedGas {
				t.Fatalf("top call has invalid gasUsed. have: %d want: %d", topCall.GasUsed, vmRet.UsedGas)
			}
		})
	}
}

func BenchmarkTracers(b *testing.B) {
	files, err := os.ReadDir(filepath.Join("testdata", "call_tracer"))
	if err != nil {
		b.Fatalf("failed to retrieve tracer test suite: %v", err)
	}
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".json") {
			continue
		}
		b.Run(camel(strings.TrimSuffix(file.Name(), ".json")), func(b *testing.B) {
			blob, err := os.ReadFile(filepath.Join("testdata", "call_tracer", file.Name()))
			if err != nil {
				b.Fatalf("failed to read testcase: %v", err)
			}
			test := new(callTracerTest)
			if err := json.Unmarshal(blob, test); err != nil {
				b.Fatalf("failed to parse testcase: %v", err)
			}
			benchTracer("callTracer", test, b)
		})
	}
}

func benchTracer(tracerName string, test *callTracerTest, b *testing.B) {
	// Configure a blockchain with the given prestate
	tx := new(types.Transaction)
	if err := rlp.DecodeBytes(common.FromHex(test.Input), tx); err != nil {
		b.Fatalf("failed to parse testcase input: %v", err)
	}
	signer := types.NewZondSigner(test.Genesis.Config.ChainID)
	msg, err := core.TransactionToMessage(tx, signer, nil)
	if err != nil {
		b.Fatalf("failed to prepare transaction for tracing: %v", err)
	}
	origin, _ := signer.Sender(tx)
	txContext := vm.TxContext{
		Origin:   origin,
		GasPrice: tx.GasPrice(),
	}
	context := vm.BlockContext{
		CanTransfer: core.CanTransfer,
		Transfer:    core.Transfer,
		Coinbase:    test.Context.Miner,
		BlockNumber: new(big.Int).SetUint64(uint64(test.Context.Number)),
		Time:        uint64(test.Context.Time),
		GasLimit:    uint64(test.Context.GasLimit),
	}
	triedb, _, statedb := tests.MakePreState(rawdb.NewMemoryDatabase(), test.Genesis.Alloc, false, rawdb.HashScheme)
	defer triedb.Close()

	b.ReportAllocs()

	for b.Loop() {
		tracer, err := tracers.DefaultDirectory.New(tracerName, new(tracers.Context), nil)
		if err != nil {
			b.Fatalf("failed to create call tracer: %v", err)
		}
		qrvm := vm.NewQRVM(context, txContext, statedb, test.Genesis.Config, vm.Config{Tracer: tracer})
		snap := statedb.Snapshot()
		st := core.NewStateTransition(qrvm, msg, new(core.GasPool).AddGas(tx.Gas()))
		if _, err = st.TransitionDb(); err != nil {
			b.Fatalf("failed to execute transaction: %v", err)
		}
		if _, err = tracer.GetResult(); err != nil {
			b.Fatal(err)
		}
		statedb.RevertToSnapshot(snap)
	}
}

func TestInternals(t *testing.T) {
	var (
		to        = common.BytesToAddress(common.FromHex("deadbeef00000000000000000000000000000000deadbeef000000000000000000000000000000000000000000000000"))
		origin    = common.BytesToAddress(common.FromHex("feed000000000000000000000000000000000000feed0000000000000000000000000000000000000000000000000000"))
		txContext = vm.TxContext{
			Origin:   origin,
			GasPrice: big.NewInt(1),
		}
		context = vm.BlockContext{
			CanTransfer: core.CanTransfer,
			Transfer:    core.Transfer,
			Coinbase:    common.Address{},
			BlockNumber: new(big.Int).SetUint64(8000000),
			Time:        5,
			GasLimit:    uint64(6000000),
			BaseFee:     new(big.Int),
		}
	)
	mkTracer := func(name string, cfg json.RawMessage) tracers.Tracer {
		tr, err := tracers.DefaultDirectory.New(name, nil, cfg)
		if err != nil {
			t.Fatalf("failed to create call tracer: %v", err)
		}
		return tr
	}

	for _, tc := range []struct {
		name   string
		code   []byte
		tracer tracers.Tracer
		want   string
	}{
		{
			// TestZeroValueToNotExitCall tests the calltracer(s) on the following:
			// Tx to A, A calls B with zero value. B does not already exist.
			// Expected: that enter/exit is invoked and the inner call is shown in the result
			name: "ZeroValueToNotExitCall",
			code: []byte{
				byte(vm.PUSH1), 0x0, byte(vm.DUP1), byte(vm.DUP1), byte(vm.DUP1), // in and outs zero
				byte(vm.DUP1), byte(vm.PUSH1), 0xff, byte(vm.GAS), // value=0,address=0xff, gas=GAS
				byte(vm.CALL),
			},
			tracer: mkTracer("callTracer", nil),
			want:   `{"from":"Q00000000000000000000000000000000feed000000000000000000000000000000000000feed0000000000000000000000000000000000000000000000000000","gas":"0x13880","gasUsed":"0x5c44","to":"Q00000000000000000000000000000000deadbeef00000000000000000000000000000000deadbeef000000000000000000000000000000000000000000000000","input":"0x","calls":[{"from":"Q00000000000000000000000000000000deadbeef00000000000000000000000000000000deadbeef000000000000000000000000000000000000000000000000","gas":"0xd8cc","gasUsed":"0x0","to":"Q000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000ff","input":"0x","value":"0x0","type":"CALL"}],"value":"0x0","type":"CALL"}`,
		},
		{
			name:   "Stack depletion in LOG0",
			code:   []byte{byte(vm.LOG3)},
			tracer: mkTracer("callTracer", json.RawMessage(`{ "withLog": true }`)),
			want:   `{"from":"Q00000000000000000000000000000000feed000000000000000000000000000000000000feed0000000000000000000000000000000000000000000000000000","gas":"0x13880","gasUsed":"0x13880","to":"Q00000000000000000000000000000000deadbeef00000000000000000000000000000000deadbeef000000000000000000000000000000000000000000000000","input":"0x","error":"stack underflow (0 \u003c=\u003e 5)","value":"0x0","type":"CALL"}`,
		},
		{
			name: "Mem expansion in LOG0",
			code: []byte{
				byte(vm.PUSH1), 0x1,
				byte(vm.PUSH1), 0x0,
				byte(vm.MSTORE),
				byte(vm.PUSH1), 0xff,
				byte(vm.PUSH1), 0x0,
				byte(vm.LOG0),
			},
			tracer: mkTracer("callTracer", json.RawMessage(`{ "withLog": true }`)),
			want:   `{"from":"Q00000000000000000000000000000000feed000000000000000000000000000000000000feed0000000000000000000000000000000000000000000000000000","gas":"0x13880","gasUsed":"0x5b92","to":"Q00000000000000000000000000000000deadbeef00000000000000000000000000000000deadbeef000000000000000000000000000000000000000000000000","input":"0x","logs":[{"address":"Q00000000000000000000000000000000deadbeef00000000000000000000000000000000deadbeef000000000000000000000000000000000000000000000000","topics":[],"data":"0x000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"}],"value":"0x0","type":"CALL"}`,
		},
		{
			// Leads to OOM on the prestate tracer
			name: "Prestate-tracer - CREATE2 OOM",
			code: []byte{
				byte(vm.PUSH1), 0x1,
				byte(vm.PUSH1), 0x0,
				byte(vm.MSTORE),
				byte(vm.PUSH1), 0x1,
				byte(vm.PUSH5), 0xff, 0xff, 0xff, 0xff, 0xff,
				byte(vm.PUSH1), 0x1,
				byte(vm.PUSH1), 0x0,
				byte(vm.CREATE2),
				byte(vm.PUSH1), 0xff,
				byte(vm.PUSH1), 0x0,
				byte(vm.LOG0),
			},
			tracer: mkTracer("prestateTracer", nil),
			want:   `{"Q00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000":{"balance":"0x0"},"Q00000000000000000000000000000000deadbeef00000000000000000000000000000000deadbeef000000000000000000000000000000000000000000000000":{"balance":"0x0","code":"0x6001600052600164ffffffffff60016000f560ff6000c0"},"Q00000000000000000000000000000000feed000000000000000000000000000000000000feed0000000000000000000000000000000000000000000000000000":{"balance":"0x1c6bf52647880"}}`,
		},
		{
			// CREATE2 which requires padding memory by prestate tracer
			name: "Prestate-tracer - CREATE2 Memory padding",
			code: []byte{
				byte(vm.PUSH1), 0x1,
				byte(vm.PUSH1), 0x0,
				byte(vm.MSTORE),
				byte(vm.PUSH1), 0x1,
				byte(vm.PUSH1), 0xff,
				byte(vm.PUSH1), 0x1,
				byte(vm.PUSH1), 0x0,
				byte(vm.CREATE2),
				byte(vm.PUSH1), 0xff,
				byte(vm.PUSH1), 0x0,
				byte(vm.LOG0),
			},
			tracer: mkTracer("prestateTracer", nil),
			want:   `{"Q00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000":{"balance":"0x0"},"Q00000000000000000000000000000000deadbeef00000000000000000000000000000000deadbeef000000000000000000000000000000000000000000000000":{"balance":"0x0","code":"0x6001600052600160ff60016000f560ff6000c0"},"Q00000000000000000000000000000000feed000000000000000000000000000000000000feed0000000000000000000000000000000000000000000000000000":{"balance":"0x1c6bf52647880"},"Q15f1dff47eecfaa918fb221a04584ecdd31251b2737837bd59ea462f1840e150b1434a2eadd56fdc3255910047770e8cf2d630f16b67d4ba9b10f6b7ff17c6d7":{"balance":"0x0"}}`,
		},
		{
			// callTracer: contract reverts with an Error(string) payload; the
			// tracer surfaces the raw output plus the decoded revertReason.
			name: "Revert with reason",
			code: []byte{
				// Build Error("nope") payload and REVERT it.
				byte(vm.PUSH4), 0x08, 0xc3, 0x79, 0xa0, // selector
				byte(vm.PUSH2), 0x01, 0xe0, // 480
				byte(vm.SHL),
				byte(vm.PUSH1), 0x00,
				byte(vm.MSTORE), // memory[0:64] = selector<<480
				byte(vm.PUSH1), 0x40,
				byte(vm.PUSH1), 0x04,
				byte(vm.MSTORE), // memory[4:68] = offset 0x40
				byte(vm.PUSH1), 0x04,
				byte(vm.PUSH1), 0x44,
				byte(vm.MSTORE),                        // memory[68:132] = length 4
				byte(vm.PUSH4), 0x6e, 0x6f, 0x70, 0x65, // "nope"
				byte(vm.PUSH2), 0x01, 0xe0, // 480 = (64-4)*8
				byte(vm.SHL),
				byte(vm.PUSH1), 0x84,
				byte(vm.MSTORE), // memory[132:196] = "nope"<<480
				byte(vm.PUSH1), 0xc4,
				byte(vm.PUSH1), 0x00,
				byte(vm.REVERT),
			},
			tracer: mkTracer("callTracer", nil),
			want:   `{"from":"Q00000000000000000000000000000000feed000000000000000000000000000000000000feed0000000000000000000000000000000000000000000000000000","gas":"0x13880","gasUsed":"0x524a","to":"Q00000000000000000000000000000000deadbeef00000000000000000000000000000000deadbeef000000000000000000000000000000000000000000000000","input":"0x","output":"0x08c379a000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000040000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000046e6f7065000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000","error":"execution reverted","revertReason":"nope","value":"0x0","type":"CALL"}`,
		},
		{
			// flatCallTracer: a top-level CALL to a STOP contract produces a
			// single Parity-style "call"-type entry; the outer block/tx hash
			// are null since this runs outside of chain context.
			name:   "FlatCallTracer - top-level CALL",
			code:   []byte{byte(vm.STOP)},
			tracer: mkTracer("flatCallTracer", nil),
			want:   `[{"action":{"callType":"call","from":"Q00000000000000000000000000000000feed000000000000000000000000000000000000feed0000000000000000000000000000000000000000000000000000","gas":"0x13880","input":"0x","to":"Q00000000000000000000000000000000deadbeef00000000000000000000000000000000deadbeef000000000000000000000000000000000000000000000000","value":"0x0"},"blockHash":null,"blockNumber":0,"result":{"gasUsed":"0x5208","output":"0x"},"subtraces":0,"traceAddress":[],"transactionHash":null,"transactionPosition":0,"type":"call"}]`,
		},
		{
			// prestateTracer diffMode: SSTORE into a fresh slot produces a
			// `post` entry for the touched storage and an updated sender
			// balance; `pre` records the original empty storage and the
			// sender's original balance. The 64-byte StorageValue64 encoding
			// left-pads the stored scalar to the full slot width.
			name: "Prestate-tracer diffMode - SSTORE",
			code: []byte{
				byte(vm.PUSH1), 0x2a, // value 42
				byte(vm.PUSH1), 0x01, // slot 1
				byte(vm.SSTORE),
			},
			tracer: mkTracer("prestateTracer", json.RawMessage(`{"diffMode":true}`)),
			want:   `{"post":{"Q00000000000000000000000000000000deadbeef00000000000000000000000000000000deadbeef000000000000000000000000000000000000000000000000":{"storage":{"0x0000000000000000000000000000000000000000000000000000000000000001":"0x0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000002a"}},"Q00000000000000000000000000000000feed000000000000000000000000000000000000feed0000000000000000000000000000000000000000000000000000":{"balance":"0x1c6bf52634000","nonce":1}},"pre":{"Q00000000000000000000000000000000deadbeef00000000000000000000000000000000deadbeef000000000000000000000000000000000000000000000000":{"balance":"0x0","code":"0x602a600155"},"Q00000000000000000000000000000000feed000000000000000000000000000000000000feed0000000000000000000000000000000000000000000000000000":{"balance":"0x1c6bf52647880"}}}`,
		},
		{
			// callTracer: a CALL that reverts without any output. The tracer
			// should emit the outer `error:"execution reverted"` without a
			// revertReason, and the `output` field is omitted.
			name: "CallTracer - plain revert",
			code: []byte{
				byte(vm.PUSH1), 0x00,
				byte(vm.PUSH1), 0x00,
				byte(vm.REVERT),
			},
			tracer: mkTracer("callTracer", nil),
			want:   `{"from":"Q00000000000000000000000000000000feed000000000000000000000000000000000000feed0000000000000000000000000000000000000000000000000000","gas":"0x13880","gasUsed":"0x520e","to":"Q00000000000000000000000000000000deadbeef00000000000000000000000000000000deadbeef000000000000000000000000000000000000000000000000","input":"0x","error":"execution reverted","value":"0x0","type":"CALL"}`,
		},
		{
			// 4byteTracer: the tracer key is `selector-<callData-len>` and
			// value is the hit count. An outer call with no calldata yields
			// no hit, so result is an empty object.
			name:   "FourByteTracer - empty calldata",
			code:   []byte{byte(vm.STOP)},
			tracer: mkTracer("4byteTracer", nil),
			want:   `{}`,
		},
		{
			// callTracer: a STATICCALL to a non-existent address. The outer
			// CALL succeeds with status 0 on the inner (cold) account, and
			// the tracer records the inner entry as a STATICCALL with an
			// empty input and zero gasUsed.
			name: "CallTracer - STATICCALL to empty account",
			code: []byte{
				byte(vm.PUSH1), 0x00, // retSize
				byte(vm.PUSH1), 0x00, // retOffset
				byte(vm.PUSH1), 0x00, // argSize
				byte(vm.PUSH1), 0x00, // argOffset
				byte(vm.PUSH1), 0xbb, // address 0xbb
				byte(vm.GAS),
				byte(vm.STATICCALL),
			},
			tracer: mkTracer("callTracer", nil),
			want:   `{"from":"Q00000000000000000000000000000000feed000000000000000000000000000000000000feed0000000000000000000000000000000000000000000000000000","gas":"0x13880","gasUsed":"0x5c41","to":"Q00000000000000000000000000000000deadbeef00000000000000000000000000000000deadbeef000000000000000000000000000000000000000000000000","input":"0x","calls":[{"from":"Q00000000000000000000000000000000deadbeef00000000000000000000000000000000deadbeef000000000000000000000000000000000000000000000000","gas":"0xd8cf","gasUsed":"0x0","to":"Q000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000bb","input":"0x","type":"STATICCALL"}],"value":"0x0","type":"CALL"}`,
		},
		{
			// prestateTracer non-diff: a CALL to a code-bearing contract
			// surfaces the contract's code in the prestate together with the
			// sender's balance and the (implicit) coinbase account.
			name:   "Prestate-tracer - code capture on STOP",
			code:   []byte{byte(vm.STOP)},
			tracer: mkTracer("prestateTracer", nil),
			want:   `{"Q00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000":{"balance":"0x0"},"Q00000000000000000000000000000000deadbeef00000000000000000000000000000000deadbeef000000000000000000000000000000000000000000000000":{"balance":"0x0","code":"0x00"},"Q00000000000000000000000000000000feed000000000000000000000000000000000000feed0000000000000000000000000000000000000000000000000000":{"balance":"0x1c6bf52647880"}}`,
		},
		{
			// callTracer: a DELEGATECALL forwards the outer context to the
			// inner call. The tracer emits a nested DELEGATECALL entry with
			// the inner `from` equal to the outer callee (not the original
			// caller), confirming the delegate-call semantics.
			name: "CallTracer - DELEGATECALL",
			code: []byte{
				byte(vm.PUSH1), 0x00, // retSize
				byte(vm.PUSH1), 0x00, // retOffset
				byte(vm.PUSH1), 0x00, // argSize
				byte(vm.PUSH1), 0x00, // argOffset
				byte(vm.PUSH1), 0xcc, // target 0xcc
				byte(vm.GAS),
				byte(vm.DELEGATECALL),
			},
			tracer: mkTracer("callTracer", nil),
			want:   `{"from":"Q00000000000000000000000000000000feed000000000000000000000000000000000000feed0000000000000000000000000000000000000000000000000000","gas":"0x13880","gasUsed":"0x5c41","to":"Q00000000000000000000000000000000deadbeef00000000000000000000000000000000deadbeef000000000000000000000000000000000000000000000000","input":"0x","calls":[{"from":"Q00000000000000000000000000000000deadbeef00000000000000000000000000000000deadbeef000000000000000000000000000000000000000000000000","gas":"0xd8cf","gasUsed":"0x0","to":"Q000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000cc","input":"0x","value":"0x0","type":"DELEGATECALL"}],"value":"0x0","type":"CALL"}`,
		},
		{
			// flatCallTracer + INVALID: hitting the INVALID opcode surfaces
			// an "invalid opcode: INVALID" error on the flat-trace entry and
			// drops the `result` object in favour of `error`.
			name:   "FlatCallTracer - invalid opcode error",
			code:   []byte{byte(vm.INVALID)},
			tracer: mkTracer("flatCallTracer", nil),
			want:   `[{"action":{"callType":"call","from":"Q00000000000000000000000000000000feed000000000000000000000000000000000000feed0000000000000000000000000000000000000000000000000000","gas":"0x13880","input":"0x","to":"Q00000000000000000000000000000000deadbeef00000000000000000000000000000000deadbeef000000000000000000000000000000000000000000000000","value":"0x0"},"blockHash":null,"blockNumber":0,"error":"invalid opcode: INVALID","subtraces":0,"traceAddress":[],"transactionHash":null,"transactionPosition":0,"type":"call"}]`,
		},
		{
			// callTracer withLog: a LOG1 with a single topic should appear in
			// the tracer's `logs` array with matching address, topic and
			// zero-length data.
			name: "CallTracer withLog - LOG1",
			code: []byte{
				byte(vm.PUSH1), 0x42, // topic = 0x42
				byte(vm.PUSH1), 0x00, // length = 0
				byte(vm.PUSH1), 0x00, // offset = 0
				byte(vm.LOG1),
			},
			tracer: mkTracer("callTracer", json.RawMessage(`{"withLog":true}`)),
			want:   `{"from":"Q00000000000000000000000000000000feed000000000000000000000000000000000000feed0000000000000000000000000000000000000000000000000000","gas":"0x13880","gasUsed":"0x54ff","to":"Q00000000000000000000000000000000deadbeef00000000000000000000000000000000deadbeef000000000000000000000000000000000000000000000000","input":"0x","logs":[{"address":"Q00000000000000000000000000000000deadbeef00000000000000000000000000000000deadbeef000000000000000000000000000000000000000000000000","topics":["0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000042"],"data":"0x"}],"value":"0x0","type":"CALL"}`,
		},
		{
			name: "CallTracer onlyTopCall withLog - LOG1",
			code: []byte{
				byte(vm.PUSH1), 0x42,
				byte(vm.PUSH1), 0x00,
				byte(vm.PUSH1), 0x00,
				byte(vm.LOG1),
			},
			tracer: mkTracer("callTracer", json.RawMessage(`{"onlyTopCall":true,"withLog":true}`)),
			want:   `{"from":"Q00000000000000000000000000000000feed000000000000000000000000000000000000feed0000000000000000000000000000000000000000000000000000","gas":"0x13880","gasUsed":"0x54ff","to":"Q00000000000000000000000000000000deadbeef00000000000000000000000000000000deadbeef000000000000000000000000000000000000000000000000","input":"0x","logs":[{"address":"Q00000000000000000000000000000000deadbeef00000000000000000000000000000000deadbeef000000000000000000000000000000000000000000000000","topics":["0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000042"],"data":"0x"}],"value":"0x0","type":"CALL"}`,
		},
		{
			// 4byteTracer with inner CALL: the outer call has empty calldata
			// so it contributes nothing; the inner CALL uses 68 bytes of
			// (mostly zero) memory as args, so the tracer keys off the
			// first four memory bytes and the 64-byte ABI tail length.
			name: "FourByteTracer - inner CALL",
			code: []byte{
				// MSTORE8 writes the low byte of val at mem[0]. Writing
				// 0xef here gives the inner call a selector of 0xef000000.
				byte(vm.PUSH1), 0xef,
				byte(vm.PUSH1), 0x00,
				byte(vm.MSTORE8),
				byte(vm.PUSH1), 0x00, // retSize
				byte(vm.PUSH1), 0x00, // retOffset
				byte(vm.PUSH1), 0x44, // argSize = 68
				byte(vm.PUSH1), 0x00, // argOffset
				byte(vm.PUSH1), 0x00, // value = 0
				byte(vm.PUSH1), 0xcc, // address 0xcc
				byte(vm.GAS),
				byte(vm.CALL),
			},
			tracer: mkTracer("4byteTracer", nil),
			want:   `{"0xef000000-64":1}`,
		},
		{
			// callTracer withLog: LOG2 with two topics and non-empty data.
			// Writes 0x12345678 at memory[0] via MSTORE (low-4 bytes land at
			// memory[60:64]) and emits a 64-byte data window + two topics.
			name: "CallTracer withLog - LOG2 with data",
			code: []byte{
				byte(vm.PUSH4), 0x12, 0x34, 0x56, 0x78,
				byte(vm.PUSH1), 0x00,
				byte(vm.MSTORE),      // memory[0:64] with value in low 4 bytes
				byte(vm.PUSH1), 0xcc, // topic2
				byte(vm.PUSH1), 0xbb, // topic1
				byte(vm.PUSH1), 0x40, // length = 64
				byte(vm.PUSH1), 0x00, // offset = 0
				byte(vm.LOG2),
			},
			tracer: mkTracer("callTracer", json.RawMessage(`{"withLog":true}`)),
			want:   `{"from":"Q00000000000000000000000000000000feed000000000000000000000000000000000000feed0000000000000000000000000000000000000000000000000000","gas":"0x13880","gasUsed":"0x5885","to":"Q00000000000000000000000000000000deadbeef00000000000000000000000000000000deadbeef000000000000000000000000000000000000000000000000","input":"0x","logs":[{"address":"Q00000000000000000000000000000000deadbeef00000000000000000000000000000000deadbeef000000000000000000000000000000000000000000000000","topics":["0x000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000bb","0x000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000cc"],"data":"0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000012345678"}],"value":"0x0","type":"CALL"}`,
		},
		{
			// muxTracer: fan-out wrapper running multiple tracers at once.
			// With callTracer + 4byteTracer against a plain STOP, the mux
			// result is a JSON object keyed by tracer name.
			name:   "MuxTracer - callTracer + 4byteTracer",
			code:   []byte{byte(vm.STOP)},
			tracer: mkTracer("muxTracer", json.RawMessage(`{"callTracer":null,"4byteTracer":null}`)),
			want:   `{"4byteTracer":{},"callTracer":{"from":"Q00000000000000000000000000000000feed000000000000000000000000000000000000feed0000000000000000000000000000000000000000000000000000","gas":"0x13880","gasUsed":"0x5208","to":"Q00000000000000000000000000000000deadbeef00000000000000000000000000000000deadbeef000000000000000000000000000000000000000000000000","input":"0x","value":"0x0","type":"CALL"}}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			triedb, _, statedb := tests.MakePreState(rawdb.NewMemoryDatabase(),
				core.GenesisAlloc{
					to: core.GenesisAccount{
						Code: tc.code,
					},
					origin: core.GenesisAccount{
						Balance: big.NewInt(500000000000000),
					},
				}, false, rawdb.HashScheme)
			defer triedb.Close()

			qrvm := vm.NewQRVM(context, txContext, statedb, params.MainnetChainConfig, vm.Config{Tracer: tc.tracer})
			msg := &core.Message{
				To:                &to,
				From:              origin,
				Value:             big.NewInt(0),
				GasLimit:          80000,
				GasPrice:          big.NewInt(0),
				GasFeeCap:         big.NewInt(0),
				GasTipCap:         big.NewInt(0),
				SkipAccountChecks: false,
			}
			st := core.NewStateTransition(qrvm, msg, new(core.GasPool).AddGas(msg.GasLimit))
			if _, err := st.TransitionDb(); err != nil {
				t.Fatalf("test %v: failed to execute transaction: %v", tc.name, err)
			}
			// Retrieve the trace result and compare against the expected
			res, err := tc.tracer.GetResult()
			if err != nil {
				t.Fatalf("test %v: failed to retrieve trace result: %v", tc.name, err)
			}
			if !sameTraceJSON([]byte(tc.want), res) {
				t.Errorf("test %v: trace mismatch\n have: %v\n want: %v\n", tc.name, string(res), tc.want)
			}
		})
	}
}

func sameTraceJSON(want, have []byte) bool {
	var wantJSON, haveJSON any
	if err := json.Unmarshal([]byte(strings.ToLower(string(want))), &wantJSON); err != nil {
		return false
	}
	if err := json.Unmarshal([]byte(strings.ToLower(string(have))), &haveJSON); err != nil {
		return false
	}
	return reflect.DeepEqual(wantJSON, haveJSON)
}
