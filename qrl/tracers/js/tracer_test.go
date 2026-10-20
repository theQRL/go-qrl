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

package js

import (
	"encoding/json"
	"errors"
	"math/big"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/dop251/goja"
	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/core/state"
	"github.com/theQRL/go-qrl/core/vm"
	"github.com/theQRL/go-qrl/params"
	"github.com/theQRL/go-qrl/qrl/tracers"
	assettracers "github.com/theQRL/go-qrl/qrl/tracers/js/internal/tracers"
)

type account struct{}

func (account) SubBalance(amount *big.Int)                          {}
func (account) AddBalance(amount *big.Int)                          {}
func (account) SetAddress(common.Address)                           {}
func (account) Value() *big.Int                                     { return nil }
func (account) SetBalance(*big.Int)                                 {}
func (account) SetNonce(uint64)                                     {}
func (account) Balance() *big.Int                                   { return nil }
func (account) Address() common.Address                             { return common.Address{} }
func (account) SetCode(common.Hash, []byte)                         {}
func (account) ForEachStorage(cb func(key, value common.Hash) bool) {}

type dummyStatedb struct {
	state.StateDB
}

func (*dummyStatedb) GetRefund() uint64                       { return 1337 }
func (*dummyStatedb) GetBalance(addr common.Address) *big.Int { return new(big.Int) }

type vmContext struct {
	blockCtx vm.BlockContext
	txCtx    vm.TxContext
}

func testCtx() *vmContext {
	return &vmContext{blockCtx: vm.BlockContext{BlockNumber: big.NewInt(1)}, txCtx: vm.TxContext{GasPrice: big.NewInt(100000)}}
}

func runTrace(tracer tracers.Tracer, vmctx *vmContext, chaincfg *params.ChainConfig, contractCode []byte) (json.RawMessage, error) {
	var (
		env             = vm.NewQRVM(vmctx.blockCtx, vmctx.txCtx, &dummyStatedb{}, chaincfg, vm.Config{Tracer: tracer})
		gasLimit uint64 = 31000
		startGas uint64 = 10000
		value           = big.NewInt(0)
		contract        = vm.NewContract(account{}, account{}, value, startGas)
	)
	contract.Code = []byte{byte(vm.PUSH1), 0x1, byte(vm.PUSH1), 0x1, 0x0}
	if contractCode != nil {
		contract.Code = contractCode
	}

	tracer.CaptureTxStart(gasLimit)
	tracer.CaptureStart(env, contract.Caller(), contract.Address(), false, []byte{}, startGas, value)
	ret, err := env.Interpreter().Run(contract, []byte{}, false)
	tracer.CaptureEnd(ret, startGas-contract.Gas, err)
	// Rest gas assumes no refund
	tracer.CaptureTxEnd(contract.Gas)
	if err != nil {
		return nil, err
	}
	return tracer.GetResult()
}

func TestTracer(t *testing.T) {
	execTracer := func(code string, contract []byte) ([]byte, string) {
		t.Helper()
		tracer, err := newJsTracer(code, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		ret, err := runTrace(tracer, testCtx(), params.TestChainConfig, contract)
		if err != nil {
			return nil, err.Error() // Stringify to allow comparison without nil checks
		}
		return ret, ""
	}
	for i, tt := range []struct {
		code     string
		want     string
		fail     string
		contract []byte
	}{
		{ // tests that we don't panic on bad arguments to memory access
			code: "{depths: [], step: function(log) { this.depths.push(log.memory.slice(-1,-2)); }, fault: function() {}, result: function() { return this.depths; }}",
			want: ``,
			fail: "tracer accessed out of bound memory: offset -1, end -2 at step (<eval>:1:53(13))    in server-side tracer function 'step'",
		}, { // tests that we don't panic on bad arguments to stack peeks
			code: "{depths: [], step: function(log) { this.depths.push(log.stack.peek(-1)); }, fault: function() {}, result: function() { return this.depths; }}",
			want: ``,
			fail: "tracer accessed out of bound stack: size 0, index -1 at step (<eval>:1:53(11))    in server-side tracer function 'step'",
		}, { //  tests that we don't panic on bad arguments to memory getUint
			code: "{ depths: [], step: function(log, db) { this.depths.push(log.memory.getUint(-64));}, fault: function() {}, result: function() { return this.depths; }}",
			want: ``,
			fail: "tracer accessed out of bound memory: available 0, offset -64, size 64 at step (<eval>:1:58(11))    in server-side tracer function 'step'",
		}, { // tests some general counting
			code: "{count: 0, step: function() { this.count += 1; }, fault: function() {}, result: function() { return this.count; }}",
			want: `3`,
		}, { // tests that depth is reported correctly
			code: "{depths: [], step: function(log) { this.depths.push(log.stack.length()); }, fault: function() {}, result: function() { return this.depths; }}",
			want: `[0,1,2]`,
		}, { // tests memory length
			code: "{lengths: [], step: function(log) { this.lengths.push(log.memory.length()); }, fault: function() {}, result: function() { return this.lengths; }}",
			want: `[0,0,0]`,
		}, { // tests to-string of opcodes
			code: "{opcodes: [], step: function(log) { this.opcodes.push(log.op.toString()); }, fault: function() {}, result: function() { return this.opcodes; }}",
			want: `["PUSH1","PUSH1","STOP"]`,
		}, { // tests gasUsed
			code: "{depths: [], step: function() {}, fault: function() {}, result: function(ctx) { return ctx.gasPrice+'.'+ctx.gasUsed; }}",
			want: `"100000.21006"`,
		}, {
			code: "{res: null, step: function(log) {}, fault: function() {}, result: function() { return toWord('0xffaa') }}",
			want: `{"0":0,"1":0,"2":0,"3":0,"4":0,"5":0,"6":0,"7":0,"8":0,"9":0,"10":0,"11":0,"12":0,"13":0,"14":0,"15":0,"16":0,"17":0,"18":0,"19":0,"20":0,"21":0,"22":0,"23":0,"24":0,"25":0,"26":0,"27":0,"28":0,"29":0,"30":0,"31":0,"32":0,"33":0,"34":0,"35":0,"36":0,"37":0,"38":0,"39":0,"40":0,"41":0,"42":0,"43":0,"44":0,"45":0,"46":0,"47":0,"48":0,"49":0,"50":0,"51":0,"52":0,"53":0,"54":0,"55":0,"56":0,"57":0,"58":0,"59":0,"60":0,"61":0,"62":255,"63":170}`,
		}, {
			code: "{res: null, step: function(log) {}, fault: function() {}, result: function() { return toWord('0xff0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f202122232425262728292a2b2c2d2e2f303132333435363738393a3b3c3d3e3f40') }}",
			want: `{"0":1,"1":2,"2":3,"3":4,"4":5,"5":6,"6":7,"7":8,"8":9,"9":10,"10":11,"11":12,"12":13,"13":14,"14":15,"15":16,"16":17,"17":18,"18":19,"19":20,"20":21,"21":22,"22":23,"23":24,"24":25,"25":26,"26":27,"27":28,"28":29,"29":30,"30":31,"31":32,"32":33,"33":34,"34":35,"35":36,"36":37,"37":38,"38":39,"39":40,"40":41,"41":42,"42":43,"43":44,"44":45,"45":46,"46":47,"47":48,"48":49,"49":50,"50":51,"51":52,"52":53,"53":54,"54":55,"55":56,"56":57,"57":58,"58":59,"59":60,"60":61,"61":62,"62":63,"63":64}`,
		}, { // test feeding a buffer back into go
			code: "{res: null, step: function(log) { var address = log.contract.getAddress(); this.res = toAddress(address); }, fault: function() {}, result: function() { return this.res }}",
			want: `{"0":0,"1":0,"2":0,"3":0,"4":0,"5":0,"6":0,"7":0,"8":0,"9":0,"10":0,"11":0,"12":0,"13":0,"14":0,"15":0,"16":0,"17":0,"18":0,"19":0,"20":0,"21":0,"22":0,"23":0,"24":0,"25":0,"26":0,"27":0,"28":0,"29":0,"30":0,"31":0,"32":0,"33":0,"34":0,"35":0,"36":0,"37":0,"38":0,"39":0,"40":0,"41":0,"42":0,"43":0,"44":0,"45":0,"46":0,"47":0,"48":0,"49":0,"50":0,"51":0,"52":0,"53":0,"54":0,"55":0,"56":0,"57":0,"58":0,"59":0,"60":0,"61":0,"62":0,"63":0}`,
		}, {
			code: "{res: null, step: function(log) { var address = 'Q00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000'; this.res = toAddress(address); }, fault: function() {}, result: function() { return this.res }}",
			want: `{"0":0,"1":0,"2":0,"3":0,"4":0,"5":0,"6":0,"7":0,"8":0,"9":0,"10":0,"11":0,"12":0,"13":0,"14":0,"15":0,"16":0,"17":0,"18":0,"19":0,"20":0,"21":0,"22":0,"23":0,"24":0,"25":0,"26":0,"27":0,"28":0,"29":0,"30":0,"31":0,"32":0,"33":0,"34":0,"35":0,"36":0,"37":0,"38":0,"39":0,"40":0,"41":0,"42":0,"43":0,"44":0,"45":0,"46":0,"47":0,"48":0,"49":0,"50":0,"51":0,"52":0,"53":0,"54":0,"55":0,"56":0,"57":0,"58":0,"59":0,"60":0,"61":0,"62":0,"63":0}`,
		}, {
			code: "{res: null, step: function(log) { var address = Array.prototype.slice.call(log.contract.getAddress()); this.res = toAddress(address); }, fault: function() {}, result: function() { return this.res }}",
			want: `{"0":0,"1":0,"2":0,"3":0,"4":0,"5":0,"6":0,"7":0,"8":0,"9":0,"10":0,"11":0,"12":0,"13":0,"14":0,"15":0,"16":0,"17":0,"18":0,"19":0,"20":0,"21":0,"22":0,"23":0,"24":0,"25":0,"26":0,"27":0,"28":0,"29":0,"30":0,"31":0,"32":0,"33":0,"34":0,"35":0,"36":0,"37":0,"38":0,"39":0,"40":0,"41":0,"42":0,"43":0,"44":0,"45":0,"46":0,"47":0,"48":0,"49":0,"50":0,"51":0,"52":0,"53":0,"54":0,"55":0,"56":0,"57":0,"58":0,"59":0,"60":0,"61":0,"62":0,"63":0}`,
		}, {
			code:     "{res: [], step: function(log) { var op = log.op.toString(); if (op === 'MSTORE8' || op === 'STOP') { this.res.push(log.memory.slice(0, 2)) } }, fault: function() {}, result: function() { return this.res }}",
			want:     `[{"0":0,"1":0},{"0":255,"1":0}]`,
			contract: []byte{byte(vm.PUSH1), byte(0xff), byte(vm.PUSH1), byte(0x00), byte(vm.MSTORE8), byte(vm.STOP)},
		}, {
			code:     "{res: [], step: function(log) { if (log.op.toString() === 'STOP') { this.res.push(log.memory.slice(5, 1025 * 1024)) } }, fault: function() {}, result: function() { return this.res }}",
			want:     "",
			fail:     "reached limit for padding memory slice: 1049536 at step (<eval>:1:83(20))    in server-side tracer function 'step'",
			contract: []byte{byte(vm.PUSH1), byte(0xff), byte(vm.PUSH1), byte(0x00), byte(vm.MSTORE8), byte(vm.STOP)},
		},
	} {
		if have, err := execTracer(tt.code, tt.contract); tt.want != string(have) || tt.fail != err {
			t.Errorf("testcase %d: expected return value to be \n'%s'\n\tgot\n'%s'\nerror to be\n'%s'\n\tgot\n'%s'\n\tcode: %v", i, tt.want, string(have), tt.fail, err, tt.code)
		}
	}
}

func TestQRVMDisTracerOpcodeRanges(t *testing.T) {
	push0Ops := runQRVMDisTrace(t, []byte{byte(vm.PUSH0), byte(vm.STOP)})
	if len(push0Ops) != 2 {
		t.Fatalf("unexpected qrvmdis PUSH0 op count: have %d want 2", len(push0Ops))
	}
	if push0Ops[0].Op != int(vm.PUSH0) || push0Ops[0].Len != 1 || !slices.Equal(push0Ops[0].Result, []string{"0"}) {
		t.Fatalf("unexpected PUSH0 trace result: %+v", push0Ops[0])
	}

	currentOps := runQRVMDisTrace(t, []byte{byte(vm.PUSH1), 0x01, byte(vm.PUSH1), 0x01, byte(vm.SHL), byte(vm.RETURNDATASIZE), byte(vm.STOP)})
	if len(currentOps) != 5 {
		t.Fatalf("unexpected qrvmdis current op count: have %d want 5", len(currentOps))
	}
	if currentOps[2].Op != int(vm.SHL) || !slices.Equal(currentOps[2].Result, []string{"2"}) {
		t.Fatalf("unexpected SHL trace result: %+v", currentOps[2])
	}
	if currentOps[3].Op != int(vm.RETURNDATASIZE) || !slices.Equal(currentOps[3].Result, []string{"0"}) {
		t.Fatalf("unexpected RETURNDATASIZE trace result: %+v", currentOps[3])
	}

	contract := []byte{byte(vm.PUSH33)}
	contract = append(contract, make([]byte, 32)...)
	contract = append(contract, 0x01, byte(vm.PUSH1), 0x02, byte(vm.DUP2), byte(vm.SWAP1), byte(vm.STOP))

	ops := runQRVMDisTrace(t, contract)
	if len(ops) != 5 {
		t.Fatalf("unexpected qrvmdis op count: have %d want 5", len(ops))
	}
	if ops[0].Op != int(vm.PUSH33) || ops[0].Len != 34 || !slices.Equal(ops[0].Result, []string{"1"}) {
		t.Fatalf("unexpected PUSH33 trace result: %+v", ops[0])
	}
	if ops[2].Op != int(vm.DUP2) || !slices.Equal(ops[2].Result, []string{"1", "2", "1"}) {
		t.Fatalf("unexpected DUP2 trace result: %+v", ops[2])
	}
	if ops[3].Op != int(vm.SWAP1) || !slices.Equal(ops[3].Result, []string{"2", "1"}) {
		t.Fatalf("unexpected SWAP1 trace result: %+v", ops[3])
	}

	endpointContract := []byte{byte(vm.PUSH64)}
	endpointContract = append(endpointContract, make([]byte, 63)...)
	endpointContract = append(endpointContract, 0x03)
	for i := 1; i <= 16; i++ {
		endpointContract = append(endpointContract, byte(vm.PUSH1), byte(i))
	}
	endpointContract = append(endpointContract, byte(vm.DUP16), byte(vm.PUSH1), 0x11, byte(vm.SWAP16), byte(vm.STOP))

	endpointOps := runQRVMDisTrace(t, endpointContract)
	if len(endpointOps) != 21 {
		t.Fatalf("unexpected qrvmdis endpoint op count: have %d want 21", len(endpointOps))
	}
	if endpointOps[0].Op != int(vm.PUSH64) || endpointOps[0].Len != 65 || !slices.Equal(endpointOps[0].Result, []string{"3"}) {
		t.Fatalf("unexpected PUSH64 trace result: %+v", endpointOps[0])
	}
	if endpointOps[17].Op != int(vm.DUP16) || !slices.Equal(endpointOps[17].Result, []string{"1", "10", "f", "e", "d", "c", "b", "a", "9", "8", "7", "6", "5", "4", "3", "2", "1"}) {
		t.Fatalf("unexpected DUP16 trace result: %+v", endpointOps[17])
	}
	if endpointOps[19].Op != int(vm.SWAP16) || !slices.Equal(endpointOps[19].Result, []string{"2", "1", "10", "f", "e", "d", "c", "b", "a", "9", "8", "7", "6", "5", "4", "3", "11"}) {
		t.Fatalf("unexpected SWAP16 trace result: %+v", endpointOps[19])
	}
}

func TestQRVMDisTracerResultCounts(t *testing.T) {
	resultCount := qrvmdisResultCount(t)
	for i := 0; i <= 0xff; i++ {
		op := vm.OpCode(byte(i))
		name := op.String()
		got := resultCount(op)
		want, ok := expectedQRVMDisResultCount(op)
		if strings.Contains(name, "not defined") {
			if got != 0 {
				t.Fatalf("undefined opcode %#x result count = %d, want 0", i, got)
			}
			continue
		}
		if !ok {
			t.Fatalf("missing qrvmdis result-count expectation for %s (%#x)", name, i)
		}
		if got != want {
			t.Fatalf("%s (%#x) result count = %d, want %d", name, i, got, want)
		}
	}
}

func qrvmdisResultCount(t *testing.T) func(vm.OpCode) int {
	t.Helper()

	tracerFiles, err := assettracers.Load()
	if err != nil {
		t.Fatal(err)
	}
	code, ok := tracerFiles["qrvmdisTracer"]
	if !ok {
		t.Fatal("missing qrvmdisTracer asset")
	}
	runtime := goja.New()
	value, err := runtime.RunString("(" + code + ")")
	if err != nil {
		t.Fatal(err)
	}
	obj := value.ToObject(runtime)
	resultCount, ok := goja.AssertFunction(obj.Get("resultCount"))
	if !ok {
		t.Fatal("qrvmdisTracer resultCount is not callable")
	}
	return func(op vm.OpCode) int {
		result, err := resultCount(value, runtime.ToValue(int(op)))
		if err != nil {
			t.Fatalf("resultCount(%#x): %v", byte(op), err)
		}
		return int(result.ToInteger())
	}
}

func expectedQRVMDisResultCount(op vm.OpCode) (int, bool) {
	switch {
	case op >= vm.ADD && op <= vm.SIGNEXTEND:
		return 1, true
	case op >= vm.LT && op <= vm.SAR:
		return 1, true
	case op >= vm.ADDRESS && op <= vm.CALLDATASIZE:
		return 1, true
	case op >= vm.BLOCKHASH && op <= vm.BASEFEE:
		return 1, true
	case op == vm.PUSH0:
		return 1, true
	case op >= vm.PUSH1 && op <= vm.PUSH64:
		return 1, true
	case op >= vm.DUP1 && op <= vm.DUP16:
		return int(op-vm.DUP1) + 2, true
	case op >= vm.SWAP1 && op <= vm.SWAP16:
		return int(op-vm.SWAP1) + 2, true
	case op >= vm.LOG0 && op <= vm.LOG4:
		return 0, true
	}
	switch op {
	case vm.STOP,
		vm.CALLDATACOPY,
		vm.CODECOPY,
		vm.EXTCODECOPY,
		vm.RETURNDATACOPY,
		vm.POP,
		vm.MSTORE,
		vm.MSTORE8,
		vm.SSTORE,
		vm.JUMP,
		vm.JUMPI,
		vm.JUMPDEST,
		vm.RETURN,
		vm.REVERT,
		vm.INVALID:
		return 0, true
	case vm.KECCAK256,
		vm.CODESIZE,
		vm.GASPRICE,
		vm.EXTCODESIZE,
		vm.RETURNDATASIZE,
		vm.EXTCODEHASH,
		vm.MLOAD,
		vm.SLOAD,
		vm.PC,
		vm.MSIZE,
		vm.GAS,
		vm.CREATE,
		vm.CALL,
		vm.DELEGATECALL,
		vm.CREATE2,
		vm.STATICCALL:
		return 1, true
	default:
		return 0, false
	}
}

type qrvmdisOp struct {
	Op     int      `json:"op"`
	Len    int      `json:"len"`
	Result []string `json:"result"`
}

func runQRVMDisTrace(t *testing.T, contract []byte) []qrvmdisOp {
	t.Helper()

	tracer, err := tracers.DefaultDirectory.New("qrvmdisTracer", new(tracers.Context), nil)
	if err != nil {
		t.Fatal(err)
	}
	ret, err := runTrace(tracer, testCtx(), params.TestChainConfig, contract)
	if err != nil {
		t.Fatal(err)
	}
	var ops []qrvmdisOp
	if err := json.Unmarshal(ret, &ops); err != nil {
		t.Fatal(err)
	}
	return ops
}

func TestHalt(t *testing.T) {
	timeout := errors.New("stahp")
	tracer, err := newJsTracer("{step: function() { while(1); }, result: function() { return null; }, fault: function(){}}", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		time.Sleep(1 * time.Second)
		tracer.Stop(timeout)
	}()
	if _, err = runTrace(tracer, testCtx(), params.TestChainConfig, nil); !strings.Contains(err.Error(), "stahp") {
		t.Errorf("Expected timeout error, got %v", err)
	}
}

func TestHaltBetweenSteps(t *testing.T) {
	tracer, err := newJsTracer("{step: function() {}, fault: function() {}, result: function() { return null; }}", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	env := vm.NewQRVM(vm.BlockContext{BlockNumber: big.NewInt(1)}, vm.TxContext{GasPrice: big.NewInt(1)}, &dummyStatedb{}, params.TestChainConfig, vm.Config{Tracer: tracer})
	scope := &vm.ScopeContext{
		Contract: vm.NewContract(&account{}, &account{}, big.NewInt(0), 0),
	}
	tracer.CaptureStart(env, common.Address{}, common.Address{}, false, []byte{}, 0, big.NewInt(0))
	tracer.CaptureState(0, 0, 0, 0, scope, nil, 0, nil)
	timeout := errors.New("stahp")
	tracer.Stop(timeout)
	tracer.CaptureState(0, 0, 0, 0, scope, nil, 0, nil)

	if _, err := tracer.GetResult(); !strings.Contains(err.Error(), timeout.Error()) {
		t.Errorf("Expected timeout error, got %v", err)
	}
}

// testNoStepExec tests a regular value transfer (no exec), and accessing the statedb
// in 'result'
func TestNoStepExec(t *testing.T) {
	execTracer := func(code string) []byte {
		t.Helper()
		tracer, err := newJsTracer(code, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		env := vm.NewQRVM(vm.BlockContext{BlockNumber: big.NewInt(1)}, vm.TxContext{GasPrice: big.NewInt(100)}, &dummyStatedb{}, params.TestChainConfig, vm.Config{Tracer: tracer})
		tracer.CaptureStart(env, common.Address{}, common.Address{}, false, []byte{}, 1000, big.NewInt(0))
		tracer.CaptureEnd(nil, 0, nil)
		ret, err := tracer.GetResult()
		if err != nil {
			t.Fatal(err)
		}
		return ret
	}
	for i, tt := range []struct {
		code string
		want string
	}{
		{ // tests that we don't panic on accessing the db methods
			code: "{depths: [], step: function() {}, fault: function() {},  result: function(ctx, db){ return db.getBalance(ctx.to)} }",
			want: `"0"`,
		},
	} {
		if have := execTracer(tt.code); tt.want != string(have) {
			t.Errorf("testcase %d: expected return value to be %s got %s\n\tcode: %v", i, tt.want, string(have), tt.code)
		}
	}
}

func TestIsPrecompile(t *testing.T) {
	chaincfg := &params.ChainConfig{ChainID: big.NewInt(1)}
	txCtx := vm.TxContext{GasPrice: big.NewInt(100000)}
	// Addresses are widened to the 64-byte QRL form; the unreserved 0x20
	// slot is used to check a non-precompile address, while the 0x01 slot
	// points at the deposit contract.
	tracer, err := newJsTracer("{addr: toAddress('Q00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000020'), res: null, step: function() { this.res = isPrecompiled(this.addr); }, fault: function() {}, result: function() { return this.res; }}", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	blockCtx := vm.BlockContext{BlockNumber: big.NewInt(150)}
	res, err := runTrace(tracer, &vmContext{blockCtx, txCtx}, chaincfg, nil)
	if err != nil {
		t.Error(err)
	}
	if string(res) != "false" {
		t.Errorf("tracer should not consider unavailable contract as precompile in zond")
	}

	tracer, _ = newJsTracer("{addr: toAddress('Q00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000001'), res: null, step: function() { this.res = isPrecompiled(this.addr); }, fault: function() {}, result: function() { return this.res; }}", nil, nil)
	blockCtx = vm.BlockContext{BlockNumber: big.NewInt(250)}
	res, err = runTrace(tracer, &vmContext{blockCtx, txCtx}, chaincfg, nil)
	if err != nil {
		t.Error(err)
	}
	if string(res) != "true" {
		t.Errorf("tracer should consider deposit contract as precompile in zond")
	}
}

func TestEnterExit(t *testing.T) {
	// test that either both or none of enter() and exit() are defined
	if _, err := newJsTracer("{step: function() {}, fault: function() {}, result: function() { return null; }, enter: function() {}}", new(tracers.Context), nil); err == nil {
		t.Fatal("tracer creation should've failed without exit() definition")
	}
	if _, err := newJsTracer("{step: function() {}, fault: function() {}, result: function() { return null; }, enter: function() {}, exit: function() {}}", new(tracers.Context), nil); err != nil {
		t.Fatal(err)
	}
	// test that the enter and exit method are correctly invoked and the values passed
	tracer, err := newJsTracer("{enters: 0, exits: 0, enterGas: 0, gasUsed: 0, step: function() {}, fault: function() {}, result: function() { return {enters: this.enters, exits: this.exits, enterGas: this.enterGas, gasUsed: this.gasUsed} }, enter: function(frame) { this.enters++; this.enterGas = frame.getGas(); }, exit: function(res) { this.exits++; this.gasUsed = res.getGasUsed(); }}", new(tracers.Context), nil)
	if err != nil {
		t.Fatal(err)
	}
	scope := &vm.ScopeContext{
		Contract: vm.NewContract(&account{}, &account{}, big.NewInt(0), 0),
	}
	tracer.CaptureEnter(vm.CALL, scope.Contract.Caller(), scope.Contract.Address(), []byte{}, 1000, new(big.Int))
	tracer.CaptureExit([]byte{}, 400, nil)

	have, err := tracer.GetResult()
	if err != nil {
		t.Fatal(err)
	}
	want := `{"enters":1,"exits":1,"enterGas":1000,"gasUsed":400}`
	if string(have) != want {
		t.Errorf("Number of invocations of enter() and exit() is wrong. Have %s, want %s\n", have, want)
	}
}

func TestSetup(t *testing.T) {
	// Test empty config
	_, err := newJsTracer(`{setup: function(cfg) { if (cfg !== "{}") { throw("invalid empty config") } }, fault: function() {}, result: function() {}}`, new(tracers.Context), nil)
	if err != nil {
		t.Error(err)
	}

	cfg, err := json.Marshal(map[string]string{"foo": "bar"})
	if err != nil {
		t.Fatal(err)
	}
	// Test no setup func
	_, err = newJsTracer(`{fault: function() {}, result: function() {}}`, new(tracers.Context), cfg)
	if err != nil {
		t.Fatal(err)
	}
	// Test config value
	tracer, err := newJsTracer("{config: null, setup: function(cfg) { this.config = JSON.parse(cfg) }, step: function() {}, fault: function() {}, result: function() { return this.config.foo }}", new(tracers.Context), cfg)
	if err != nil {
		t.Fatal(err)
	}
	have, err := tracer.GetResult()
	if err != nil {
		t.Fatal(err)
	}
	if string(have) != `"bar"` {
		t.Errorf("tracer returned wrong result. have: %s, want: \"bar\"\n", string(have))
	}
}
