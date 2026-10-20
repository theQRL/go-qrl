// Package conformance provides a minimal runner and test-vector format for
// cross-validating QRVM bytecode execution between this Go implementation and
// the C++ qrvmone VM.
//
// A test vector is a self-contained bytecode program with an explicit input,
// gas limit and expected outcome (return bytes / gas used / error class).
// Both runtimes executing the same vector must produce identical outcomes;
// any divergence is a consensus bug.
//
// This package contains the Go-side runner. The matching C++ runner lives in
// qrvmone/test/conformance (target qrvmone-conformance); TestGoVMMatchesQrvmone
// compares both, and hack/vm-conformance.sh builds the runner and runs it.
package conformance

import (
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/core/rawdb"
	"github.com/theQRL/go-qrl/core/state"
	"github.com/theQRL/go-qrl/core/types"
	"github.com/theQRL/go-qrl/core/vm"
	"github.com/theQRL/go-qrl/params"
)

// ErrorClass is a small enumeration of VM error kinds that conformance tests
// compare across runtimes. A concrete Go error value is not compared directly
// because C++ and Go name errors differently; the class is the portable
// contract.
type ErrorClass string

const (
	ErrNone              ErrorClass = ""
	ErrOutOfGas          ErrorClass = "out-of-gas"
	ErrStackUnderflow    ErrorClass = "stack-underflow"
	ErrStackOverflow     ErrorClass = "stack-overflow"
	ErrInvalidJump       ErrorClass = "invalid-jump"
	ErrInvalidOpcode     ErrorClass = "invalid-opcode"
	ErrExecutionReverted ErrorClass = "reverted"
	ErrWriteProtection   ErrorClass = "write-protection"
	ErrOther             ErrorClass = "other"
)

// Vector describes a single conformance test case.
//
// All byte fields are lowercase hex without a prefix. An empty string denotes
// an empty byte slice.
type Vector struct {
	Name string // human-readable identifier
	// Hex-encoded bytecode to run as contract code.
	BytecodeHex string
	// Hex-encoded calldata (input to the call).
	InputHex string
	// Gas limit for the call. 0 means "use the package default".
	GasLimit uint64
	// Expected return bytes (hex).
	ExpectedReturnHex string
	// Expected error class.
	ExpectedError ErrorClass
	// Expected gas used. 0 means "do not compare gas" (use sparingly; prefer
	// exact comparisons when the value is stable across runtimes).
	ExpectedGasUsed uint64
}

// DefaultGasLimit is used when Vector.GasLimit is 0.
const DefaultGasLimit uint64 = 10_000_000

// Result is the outcome of running a Vector through the Go VM.
type Result struct {
	ReturnHex string
	Error     ErrorClass
	GasUsed   uint64
}

// Run executes v against this package's Go VM and returns a Result. The
// returned Result is suitable for direct comparison with a C++ runner
// producing the same Vector structure.
func Run(v Vector) (Result, error) {
	code, err := hex.DecodeString(v.BytecodeHex)
	if err != nil {
		return Result{}, fmt.Errorf("%s: decode bytecode: %w", v.Name, err)
	}
	var input []byte
	if v.InputHex != "" {
		input, err = hex.DecodeString(v.InputHex)
		if err != nil {
			return Result{}, fmt.Errorf("%s: decode input: %w", v.Name, err)
		}
	}
	gasLimit := v.GasLimit
	if gasLimit == 0 {
		gasLimit = DefaultGasLimit
	}

	return execute(code, input, gasLimit)
}

// execute builds a minimal VM environment and runs the bytecode at a fixed
// contract address. Return, gas-used and classified error are captured.
func execute(code, input []byte, gasLimit uint64) (Result, error) {
	statedb, err := state.New(types.EmptyRootHash,
		state.NewDatabase(rawdb.NewMemoryDatabase()), nil)
	if err != nil {
		return Result{}, fmt.Errorf("state init: %w", err)
	}

	var (
		contract = common.BytesToAddress([]byte("conformance"))
		origin   = common.BytesToAddress([]byte("origin"))
	)

	cfg := &params.ChainConfig{ChainID: big.NewInt(1)}
	vmctx := vm.BlockContext{
		CanTransfer: func(vm.StateDB, common.Address, *big.Int) bool { return true },
		Transfer:    func(vm.StateDB, common.Address, common.Address, *big.Int) {},
		GetHash:     func(uint64) common.Hash { return common.Hash{} },
		BlockNumber: big.NewInt(0),
		Time:        0,
		GasLimit:    gasLimit,
		BaseFee:     big.NewInt(params.InitialBaseFee),
		Random:      &common.Hash{},
	}
	txctx := vm.TxContext{
		Origin:   origin,
		GasPrice: big.NewInt(0),
	}

	vmenv := vm.NewQRVM(vmctx, txctx, statedb, cfg, vm.Config{})

	statedb.Prepare(origin, vmctx.Coinbase, &contract, vm.ActivePrecompiles(), nil)
	statedb.CreateAccount(contract)
	statedb.SetCode(contract, code)

	ret, leftOverGas, err := vmenv.Call(
		vm.AccountRef(origin),
		contract,
		input,
		gasLimit,
		new(big.Int),
	)

	return Result{
		ReturnHex: hex.EncodeToString(ret),
		Error:     classifyError(err),
		GasUsed:   gasLimit - leftOverGas,
	}, nil
}

// classifyError maps Go VM errors into the portable ErrorClass enum.
func classifyError(err error) ErrorClass {
	if err == nil {
		return ErrNone
	}
	switch {
	case errors.Is(err, vm.ErrOutOfGas),
		errors.Is(err, vm.ErrGasUintOverflow),
		errors.Is(err, vm.ErrInsufficientBalance),
		errors.Is(err, vm.ErrCodeStoreOutOfGas):
		return ErrOutOfGas
	case errors.Is(err, vm.ErrInvalidJump):
		return ErrInvalidJump
	case errors.Is(err, vm.ErrExecutionReverted):
		return ErrExecutionReverted
	case errors.Is(err, vm.ErrWriteProtection):
		return ErrWriteProtection
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "stack underflow"):
		return ErrStackUnderflow
	case strings.Contains(msg, "stack limit reached"):
		return ErrStackOverflow
	case strings.Contains(msg, "invalid opcode"):
		return ErrInvalidOpcode
	}
	return ErrOther
}

// Diff compares an actual Result against the Vector's expectations and returns
// a short description of the mismatch, or "" if they agree.
func Diff(v Vector, got Result) string {
	var b strings.Builder
	if got.Error != v.ExpectedError {
		fmt.Fprintf(&b, "error: got %q want %q; ", got.Error, v.ExpectedError)
	}
	if got.ReturnHex != v.ExpectedReturnHex {
		fmt.Fprintf(&b, "return: got %s want %s; ", got.ReturnHex, v.ExpectedReturnHex)
	}
	if v.ExpectedGasUsed != 0 && got.GasUsed != v.ExpectedGasUsed {
		fmt.Fprintf(&b, "gas: got %d want %d; ", got.GasUsed, v.ExpectedGasUsed)
	}
	return strings.TrimSuffix(b.String(), "; ")
}
