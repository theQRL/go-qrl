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

package backends

import (
	"bytes"
	"errors"
	"math/big"
	"math/rand"
	"reflect"
	"strings"
	"testing"
	"time"

	qrl "github.com/theQRL/go-qrl"
	"github.com/theQRL/go-qrl/accounts/abi"
	"github.com/theQRL/go-qrl/accounts/abi/bind"
	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/core"
	"github.com/theQRL/go-qrl/core/types"
	"github.com/theQRL/go-qrl/crypto/pqcrypto/wallet"
	"github.com/theQRL/go-qrl/internal/testutil"
	"github.com/theQRL/go-qrl/params"
)

func TestSimulatedBackend(t *testing.T) {
	var gasLimit uint64 = 8000029
	wallet, _ := wallet.Generate(wallet.ML_DSA_87) // nolint: gosec
	auth, _ := bind.NewKeyedTransactorWithChainID(wallet, big.NewInt(1337))
	genAlloc := make(core.GenesisAlloc)
	genAlloc[auth.From] = core.GenesisAccount{Balance: big.NewInt(9223372036854775807)}

	sim := NewSimulatedBackend(genAlloc, gasLimit)
	defer sim.Close()

	// should return an error if the tx is not found
	txHash := common.HexToHash("2")
	_, isPending, err := sim.TransactionByHash(t.Context(), txHash)

	if isPending {
		t.Fatal("transaction should not be pending")
	}
	if err != qrl.NotFound {
		t.Fatalf("err should be `qrl.NotFound` but received %v", err)
	}

	// generate a transaction and confirm you can retrieve it
	head, _ := sim.HeaderByNumber(t.Context(), nil) // Should be child's, good enough
	gasFeeCap := new(big.Int).Add(head.BaseFee, big.NewInt(1))

	code := `6060604052600a8060106000396000f360606040526008565b00`
	var gas uint64 = 3000000
	tx := types.NewTx(&types.DynamicFeeTx{
		Nonce:     0,
		Value:     big.NewInt(0),
		Gas:       gas,
		GasFeeCap: gasFeeCap,
		Data:      common.FromHex(code),
	})
	tx, _ = types.SignTx(tx, types.ZondSigner{ChainId: big.NewInt(1337)}, wallet)

	err = sim.SendTransaction(t.Context(), tx)
	if err != nil {
		t.Fatal("error sending transaction")
	}

	txHash = tx.Hash()
	_, isPending, err = sim.TransactionByHash(t.Context(), txHash)
	if err != nil {
		t.Fatalf("error getting transaction with hash: %v", txHash.String())
	}
	if !isPending {
		t.Fatal("transaction should have pending status")
	}

	sim.Commit()
	_, isPending, err = sim.TransactionByHash(t.Context(), txHash)
	if err != nil {
		t.Fatalf("error getting transaction with hash: %v", txHash.String())
	}
	if isPending {
		t.Fatal("transaction should not have pending status")
	}
}

var testWallet = testutil.MustLoadAccount("alice").MustWallet()

// the following is based on this contract:
//
//	 contract T {
//	 	event received(address sender, uint amount, bytes memo);
//	 	event receivedAddr(address sender);
//
//	 	function receive(bytes calldata memo) external payable returns (string memory res) {
//	 		emit received(msg.sender, msg.value, memo);
//	 		emit receivedAddr(msg.sender);
//			return "hello world";
//	 	}
//	 }
const abiJSON = `[ { "constant": false, "inputs": [ { "name": "memo", "type": "bytes" } ], "name": "receive", "outputs": [ { "name": "res", "type": "string" } ], "payable": true, "stateMutability": "payable", "type": "function" }, { "anonymous": false, "inputs": [ { "indexed": false, "name": "sender", "type": "address" }, { "indexed": false, "name": "amount", "type": "uint256" }, { "indexed": false, "name": "memo", "type": "bytes" } ], "name": "received", "type": "event" }, { "anonymous": false, "inputs": [ { "indexed": false, "name": "sender", "type": "address" } ], "name": "receivedAddr", "type": "event" } ]`

// abiBin is a hand-rolled replacement for the Solidity receive() fixture.
// The 12-byte init CODECOPYs a 34-byte runtime that, regardless of the
// incoming selector, lays out an ABI-encoded string "hello world" under
// the 64-byte slot layout:
//
//	[0:64]    offset = 0x40
//	[64:128]  length = 11
//	[128:192] "hello world" left-aligned, 53 zero-byte tail
//
// and RETURNs those 192 bytes. All opcodes used (PUSH1/2/11, MSTORE, SHL,
// CODECOPY, RETURN) are stable across the 512-bit opcode shift.
const abiBin = `0x6022600c60003960226000f3` +
	`6040600052600b6040526a68656c6c6f20776f726c646101a81b60805260c06000f3`

// deployedCode is the 34-byte runtime returned by the abiBin init code.
const deployedCode = `6040600052600b6040526a68656c6c6f20776f726c646101a81b60805260c06000f3`

// expectedReturn is the ABI-encoded "hello world" string under the 64-byte
// slot layout; matches exactly what the abiBin runtime above RETURNs.
var expectedReturn = func() []byte {
	b := make([]byte, 192)
	b[63] = 0x40 // offset slot: points 64 bytes past itself to the length slot
	b[127] = 11  // length slot: 11 bytes of string data
	copy(b[128:], []byte("hello world"))
	return b
}()

func simTestBackend(testAddr common.Address) *SimulatedBackend {
	return NewSimulatedBackend(
		core.GenesisAlloc{
			testAddr: {Balance: big.NewInt(9223372036854775807)},
		}, 10000000,
	)
}

func TestNewSimulatedBackend(t *testing.T) {
	testAddr := testWallet.GetAddress()
	expectedBal := big.NewInt(9223372036854775807)
	sim := simTestBackend(testAddr)
	defer sim.Close()

	if sim.config != params.AllBeaconProtocolChanges {
		t.Errorf("expected sim config to equal params.AllBeaconProtocolChanges, got %v", sim.config)
	}

	if sim.blockchain.Config() != params.AllBeaconProtocolChanges {
		t.Errorf("expected sim blockchain config to equal params.AllBeaconProtocolChanges, got %v", sim.config)
	}

	stateDB, _ := sim.blockchain.State()
	bal := stateDB.GetBalance(testAddr)
	if bal.Cmp(expectedBal) != 0 {
		t.Errorf("expected balance for test address not received. expected: %v actual: %v", expectedBal, bal)
	}
}

func TestAdjustTime(t *testing.T) {
	sim := NewSimulatedBackend(
		core.GenesisAlloc{}, 10000000,
	)
	defer sim.Close()

	prevTime := sim.pendingBlock.Time()
	if err := sim.AdjustTime(time.Second); err != nil {
		t.Error(err)
	}
	newTime := sim.pendingBlock.Time()

	if newTime-prevTime != uint64(time.Second.Seconds()) {
		t.Errorf("adjusted time not equal to a second. prev: %v, new: %v", prevTime, newTime)
	}
}

func TestNewAdjustTimeFail(t *testing.T) {
	testAddr := common.Address(testWallet.GetAddress())
	sim := simTestBackend(testAddr)
	defer sim.blockchain.Stop()

	// Create tx and send
	head, _ := sim.HeaderByNumber(t.Context(), nil) // Should be child's, good enough
	gasFeeCap := new(big.Int).Add(head.BaseFee, big.NewInt(1))

	tx := types.NewTx(&types.DynamicFeeTx{
		Nonce:     0,
		To:        &testAddr,
		Value:     big.NewInt(1000),
		Gas:       params.TxGas,
		GasFeeCap: gasFeeCap,
		Data:      nil,
	})
	signedTx, err := types.SignTx(tx, types.ZondSigner{ChainId: big.NewInt(1337)}, testWallet)
	if err != nil {
		t.Errorf("could not sign tx: %v", err)
	}
	if err := sim.SendTransaction(t.Context(), signedTx); err != nil {
		t.Error(err)
	}
	// AdjustTime should fail on non-empty block
	if err := sim.AdjustTime(time.Second); err == nil {
		t.Error("Expected adjust time to error on non-empty block")
	}
	sim.Commit()

	prevTime := sim.pendingBlock.Time()
	if err := sim.AdjustTime(time.Minute); err != nil {
		t.Error(err)
	}
	newTime := sim.pendingBlock.Time()
	if newTime-prevTime != uint64(time.Minute.Seconds()) {
		t.Errorf("adjusted time not equal to a minute. prev: %v, new: %v", prevTime, newTime)
	}
	// Put a transaction after adjusting time
	tx2 := types.NewTx(&types.DynamicFeeTx{
		Nonce:     1,
		To:        &testAddr,
		Value:     big.NewInt(1000),
		Gas:       params.TxGas,
		GasFeeCap: gasFeeCap,
		Data:      nil,
	})
	signedTx2, err := types.SignTx(tx2, types.ZondSigner{ChainId: big.NewInt(1337)}, testWallet)
	if err != nil {
		t.Errorf("could not sign tx: %v", err)
	}
	if err := sim.SendTransaction(t.Context(), signedTx2); err != nil {
		t.Error(err)
	}
	sim.Commit()
	newTime = sim.pendingBlock.Time()
	if newTime-prevTime >= uint64(time.Minute.Seconds()) {
		t.Errorf("time adjusted, but shouldn't be: prev: %v, new: %v", prevTime, newTime)
	}
}

func TestBalanceAt(t *testing.T) {
	testAddr := testWallet.GetAddress()
	expectedBal := big.NewInt(9223372036854775807)
	sim := simTestBackend(testAddr)
	defer sim.Close()
	bgCtx := t.Context()

	bal, err := sim.BalanceAt(bgCtx, testAddr, nil)
	if err != nil {
		t.Error(err)
	}

	if bal.Cmp(expectedBal) != 0 {
		t.Errorf("expected balance for test address not received. expected: %v actual: %v", expectedBal, bal)
	}
}

func TestBlockByHash(t *testing.T) {
	sim := NewSimulatedBackend(
		core.GenesisAlloc{}, 10000000,
	)
	defer sim.Close()
	bgCtx := t.Context()

	block, err := sim.BlockByNumber(bgCtx, nil)
	if err != nil {
		t.Errorf("could not get recent block: %v", err)
	}
	blockByHash, err := sim.BlockByHash(bgCtx, block.Hash())
	if err != nil {
		t.Errorf("could not get recent block: %v", err)
	}

	if block.Hash() != blockByHash.Hash() {
		t.Errorf("did not get expected block")
	}
}

func TestBlockByNumber(t *testing.T) {
	sim := NewSimulatedBackend(
		core.GenesisAlloc{}, 10000000,
	)
	defer sim.Close()
	bgCtx := t.Context()

	block, err := sim.BlockByNumber(bgCtx, nil)
	if err != nil {
		t.Errorf("could not get recent block: %v", err)
	}
	if block.NumberU64() != 0 {
		t.Errorf("did not get most recent block, instead got block number %v", block.NumberU64())
	}

	// create one block
	sim.Commit()

	block, err = sim.BlockByNumber(bgCtx, nil)
	if err != nil {
		t.Errorf("could not get recent block: %v", err)
	}
	if block.NumberU64() != 1 {
		t.Errorf("did not get most recent block, instead got block number %v", block.NumberU64())
	}

	blockByNumber, err := sim.BlockByNumber(bgCtx, big.NewInt(1))
	if err != nil {
		t.Errorf("could not get block by number: %v", err)
	}
	if blockByNumber.Hash() != block.Hash() {
		t.Errorf("did not get the same block with height of 1 as before")
	}
}

func TestNonceAt(t *testing.T) {
	testAddr := common.Address(testWallet.GetAddress())

	sim := simTestBackend(testAddr)
	defer sim.Close()
	bgCtx := t.Context()

	nonce, err := sim.NonceAt(bgCtx, testAddr, big.NewInt(0))
	if err != nil {
		t.Errorf("could not get nonce for test addr: %v", err)
	}

	if nonce != uint64(0) {
		t.Errorf("received incorrect nonce. expected 0, got %v", nonce)
	}

	// create a signed transaction to send
	head, _ := sim.HeaderByNumber(t.Context(), nil) // Should be child's, good enough
	gasFeeCap := new(big.Int).Add(head.BaseFee, big.NewInt(1))

	tx := types.NewTx(&types.DynamicFeeTx{
		Nonce:     nonce,
		To:        &testAddr,
		Value:     big.NewInt(1000),
		Gas:       params.TxGas,
		GasFeeCap: gasFeeCap,
		Data:      nil,
	})
	signedTx, err := types.SignTx(tx, types.ZondSigner{ChainId: big.NewInt(1337)}, testWallet)
	if err != nil {
		t.Errorf("could not sign tx: %v", err)
	}

	// send tx to simulated backend
	err = sim.SendTransaction(bgCtx, signedTx)
	if err != nil {
		t.Errorf("could not add tx to pending block: %v", err)
	}
	sim.Commit()

	newNonce, err := sim.NonceAt(bgCtx, testAddr, big.NewInt(1))
	if err != nil {
		t.Errorf("could not get nonce for test addr: %v", err)
	}

	if newNonce != nonce+uint64(1) {
		t.Errorf("received incorrect nonce. expected 1, got %v", nonce)
	}
	// create some more blocks
	sim.Commit()
	// Check that we can get data for an older block/state
	newNonce, err = sim.NonceAt(bgCtx, testAddr, big.NewInt(1))
	if err != nil {
		t.Fatalf("could not get nonce for test addr: %v", err)
	}
	if newNonce != nonce+uint64(1) {
		t.Fatalf("received incorrect nonce. expected 1, got %v", nonce)
	}
}

func TestSendTransaction(t *testing.T) {
	testAddr := common.Address(testWallet.GetAddress())

	sim := simTestBackend(testAddr)
	defer sim.Close()
	bgCtx := t.Context()

	// create a signed transaction to send
	head, _ := sim.HeaderByNumber(t.Context(), nil) // Should be child's, good enough
	gasFeeCap := new(big.Int).Add(head.BaseFee, big.NewInt(1))

	tx := types.NewTx(&types.DynamicFeeTx{
		Nonce:     uint64(0),
		To:        &testAddr,
		Value:     big.NewInt(1000),
		Gas:       params.TxGas,
		GasFeeCap: gasFeeCap,
		Data:      nil,
	})
	signedTx, err := types.SignTx(tx, types.ZondSigner{ChainId: big.NewInt(1337)}, testWallet)
	if err != nil {
		t.Errorf("could not sign tx: %v", err)
	}

	// send tx to simulated backend
	err = sim.SendTransaction(bgCtx, signedTx)
	if err != nil {
		t.Errorf("could not add tx to pending block: %v", err)
	}
	sim.Commit()

	block, err := sim.BlockByNumber(bgCtx, big.NewInt(1))
	if err != nil {
		t.Errorf("could not get block at height 1: %v", err)
	}

	if signedTx.Hash() != block.Transactions()[0].Hash() {
		t.Errorf("did not commit sent transaction. expected hash %v got hash %v", block.Transactions()[0].Hash(), signedTx.Hash())
	}
}

func TestTransactionByHash(t *testing.T) {
	testAddr := common.Address(testWallet.GetAddress())

	sim := NewSimulatedBackend(
		core.GenesisAlloc{
			testAddr: {Balance: big.NewInt(9223372036854775807)},
		}, 10000000,
	)
	defer sim.Close()
	bgCtx := t.Context()

	// create a signed transaction to send
	head, _ := sim.HeaderByNumber(t.Context(), nil) // Should be child's, good enough
	gasFeeCap := new(big.Int).Add(head.BaseFee, big.NewInt(1))

	tx := types.NewTx(&types.DynamicFeeTx{
		Nonce:     uint64(0),
		To:        &testAddr,
		Value:     big.NewInt(1000),
		Gas:       params.TxGas,
		GasFeeCap: gasFeeCap,
		Data:      nil,
	})
	signedTx, err := types.SignTx(tx, types.ZondSigner{ChainId: big.NewInt(1337)}, testWallet)
	if err != nil {
		t.Errorf("could not sign tx: %v", err)
	}

	// send tx to simulated backend
	err = sim.SendTransaction(bgCtx, signedTx)
	if err != nil {
		t.Errorf("could not add tx to pending block: %v", err)
	}

	// ensure tx is committed pending
	receivedTx, pending, err := sim.TransactionByHash(bgCtx, signedTx.Hash())
	if err != nil {
		t.Errorf("could not get transaction by hash %v: %v", signedTx.Hash(), err)
	}
	if !pending {
		t.Errorf("expected transaction to be in pending state")
	}
	if receivedTx.Hash() != signedTx.Hash() {
		t.Errorf("did not received committed transaction. expected hash %v got hash %v", signedTx.Hash(), receivedTx.Hash())
	}

	sim.Commit()

	// ensure tx is not and committed pending
	receivedTx, pending, err = sim.TransactionByHash(bgCtx, signedTx.Hash())
	if err != nil {
		t.Errorf("could not get transaction by hash %v: %v", signedTx.Hash(), err)
	}
	if pending {
		t.Errorf("expected transaction to not be in pending state")
	}
	if receivedTx.Hash() != signedTx.Hash() {
		t.Errorf("did not received committed transaction. expected hash %v got hash %v", signedTx.Hash(), receivedTx.Hash())
	}
}

func TestEstimateGas(t *testing.T) {
	const contractAbi = "[{\"inputs\":[],\"name\":\"Assert\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"OOG\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"PureRevert\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"Revert\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"Valid\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"}]"
	// Hand-rolled GasEstimation replacement. 12-byte init copies a 131-byte
	// runtime that dispatches on the top 4 bytes of CALLDATALOAD(0) (SHR
	// 480) and branches to:
	//   Valid()      (0xe09fface) → STOP
	//   Revert()     (0xd8b98391) → REVERT Error("revert reason") in 64-byte
	//                                slot layout (196 bytes payload)
	//   PureRevert() (0xaa8b1d30) → REVERT(0,0)
	//   OOG()        (0x50f6fe34) → tight JUMP loop → OOG
	//   Assert()     (0xb9b046f9) → INVALID (consumes all gas)
	// Unknown selectors (including a plain-value transfer with empty
	// calldata) fall through to a bare REVERT(0,0), matching the Solidity
	// original's behaviour when no receive/fallback matches.
	const contractBin = "0x6083600c60003960836000f3" +
		"6000356101e01c" + // selector = CALLDATALOAD(0) >> 480
		"a063e09fface1461004357" + // DUP1 == Valid?     → 0x43
		"a063d8b983911461004557" + // DUP1 == Revert?    → 0x45
		"a063aa8b1d301461007657" + // DUP1 == PureRevert?→ 0x76
		"a06350f6fe341461007c57" + // DUP1 == OOG?       → 0x7c
		"a063b9b046f91461008157" + // DUP1 == Assert?    → 0x81
		"60006000fd" + // fallback REVERT(0,0)
		"5b00" + // 0x43 Valid: STOP
		"5b6308c379a06101e01b600052" + // 0x45 Revert: MSTORE selector<<480 at 0
		"6040600452" + // offset=0x40 at memory[4:68]
		"600d604452" + // length=13 at memory[68:132]
		"6c72657665727420726561736f6e" + // PUSH13 "revert reason"
		"6101981b" + // SHL by 408 (=(64-13)*8) → high 13 bytes
		"608452" + // MSTORE at 132
		"60c46000fd" + // REVERT(0, 196)
		"5b60006000fd" + // 0x76 PureRevert: REVERT(0,0)
		"5b61007c56" + // 0x7c OOG: JUMPDEST; PUSH2 0x7c; JUMP
		"5bfe" //       0x81 Assert: INVALID

	wallet, _ := wallet.Generate(wallet.ML_DSA_87)
	var addr common.Address = wallet.GetAddress()
	opts, _ := bind.NewKeyedTransactorWithChainID(wallet, big.NewInt(1337))

	sim := NewSimulatedBackend(core.GenesisAlloc{addr: {Balance: big.NewInt(params.Quanta)}}, 10000000)
	defer sim.Close()

	parsed, _ := abi.JSON(strings.NewReader(contractAbi))
	contractAddr, _, _, _ := bind.DeployContract(opts, parsed, common.FromHex(contractBin), sim)
	sim.Commit()

	var cases = []struct {
		name        string
		message     qrl.CallMsg
		expect      uint64
		expectError error
		expectData  any
	}{
		{"plain transfer(valid)", qrl.CallMsg{
			From:      addr,
			To:        &addr,
			Gas:       0,
			GasFeeCap: big.NewInt(0),
			Value:     big.NewInt(1),
			Data:      nil,
		}, params.TxGas, nil, nil},

		{"plain transfer(invalid)", qrl.CallMsg{
			From:      addr,
			To:        &contractAddr,
			Gas:       0,
			GasFeeCap: big.NewInt(0),
			Value:     big.NewInt(1),
			Data:      nil,
		}, 0, errors.New("execution reverted"), nil},

		{"Revert", qrl.CallMsg{
			From:      addr,
			To:        &contractAddr,
			Gas:       0,
			GasFeeCap: big.NewInt(0),
			Value:     nil,
			Data:      common.Hex2Bytes("d8b98391"),
		}, 0, errors.New("execution reverted: revert reason"),
			// 64-byte slot ABI-encoded Error("revert reason"):
			//   selector(4) | offset=0x40(64) | length=13(64) | "revert reason" + 51 zero pad(64)
			"0x08c379a0" +
				"00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000040" +
				"0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000d" +
				"72657665727420726561736f6e000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"},

		{"PureRevert", qrl.CallMsg{
			From:      addr,
			To:        &contractAddr,
			Gas:       0,
			GasFeeCap: big.NewInt(0),
			Value:     nil,
			Data:      common.Hex2Bytes("aa8b1d30"),
		}, 0, errors.New("execution reverted"), nil},

		{"OOG", qrl.CallMsg{
			From:      addr,
			To:        &contractAddr,
			Gas:       100000,
			GasFeeCap: big.NewInt(0),
			Value:     nil,
			Data:      common.Hex2Bytes("50f6fe34"),
		}, 0, errors.New("gas required exceeds allowance (100000)"), nil},

		{"Assert", qrl.CallMsg{
			From:      addr,
			To:        &contractAddr,
			Gas:       100000,
			GasFeeCap: big.NewInt(0),
			Value:     nil,
			Data:      common.Hex2Bytes("b9b046f9"),
		}, 0, errors.New("invalid opcode: INVALID"), nil},

		{"Valid", qrl.CallMsg{
			From:      addr,
			To:        &contractAddr,
			Gas:       100000,
			GasFeeCap: big.NewInt(0),
			Value:     nil,
			Data:      common.Hex2Bytes("e09fface"),
			// Intrinsic 21064 + 35 dispatcher gas (PUSH/CALLDATALOAD/SHR +
			// one matched DUP1/PUSH4/EQ/JUMPI + JUMPDEST/STOP) = 21099.
		}, 21099, nil, nil},
	}
	for _, c := range cases {
		got, err := sim.EstimateGas(t.Context(), c.message)
		if c.expectError != nil {
			if err == nil {
				t.Fatalf("Expect error, got nil")
			}
			if c.expectError.Error() != err.Error() {
				t.Fatalf("Expect error, want %v, got %v", c.expectError, err)
			}
			if c.expectData != nil {
				if err, ok := err.(*revertError); !ok {
					t.Fatalf("Expect revert error, got %T", err)
				} else if !reflect.DeepEqual(err.ErrorData(), c.expectData) {
					t.Fatalf("Error data mismatch, want %v, got %v", c.expectData, err.ErrorData())
				}
			}
			continue
		}
		if got != c.expect {
			t.Fatalf("Gas estimation mismatch, want %d, got %d", c.expect, got)
		}
	}
}

func TestEstimateGasWithPrice(t *testing.T) {
	wallet, _ := wallet.Generate(wallet.ML_DSA_87)
	addr := common.Address(wallet.GetAddress())

	sim := NewSimulatedBackend(core.GenesisAlloc{addr: {Balance: big.NewInt(params.Quanta*2 + 2e17)}}, 10000000)
	defer sim.Close()

	recipient := common.MustParseAddress("Q000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000deadbeef")
	var cases = []struct {
		name        string
		message     qrl.CallMsg
		expect      uint64
		expectError error
	}{
		{"EstimateWithoutPrice", qrl.CallMsg{
			From:      addr,
			To:        &recipient,
			Gas:       0,
			GasFeeCap: big.NewInt(0),
			Value:     big.NewInt(100000000000),
			Data:      nil,
		}, 21000, nil},

		{"EstimateWithPrice", qrl.CallMsg{
			From:      addr,
			To:        &recipient,
			Gas:       0,
			GasFeeCap: big.NewInt(100000000000),
			Value:     big.NewInt(100000000000),
			Data:      nil,
		}, 21000, nil},

		{"EstimateWithVeryHighPrice", qrl.CallMsg{
			From:      addr,
			To:        &recipient,
			Gas:       0,
			GasFeeCap: big.NewInt(1e14), // gascost = 2.1quanta
			Value:     big.NewInt(1e17), // the remaining balance for fee is 2.1quanta
			Data:      nil,
		}, 21000, nil},

		{"EstimateWithSuperhighPrice", qrl.CallMsg{
			From:      addr,
			To:        &recipient,
			Gas:       0,
			GasFeeCap: big.NewInt(2e14), // gascost = 4.2quanta,
			Value:     big.NewInt(100000000000),
			Data:      nil,
		}, 21000, errors.New("gas required exceeds allowance (10999)")}, // 10999=(2.2quanta-1000planck)/(2e14)

		{"EstimateEIP1559WithHighFees", qrl.CallMsg{
			From:      addr,
			To:        &addr,
			Gas:       0,
			GasFeeCap: big.NewInt(1e14), // maxgascost = 2.1quanta
			GasTipCap: big.NewInt(1),
			Value:     big.NewInt(1e17), // the remaining balance for fee is 2.1quanta
			Data:      nil,
		}, params.TxGas, nil},

		{"EstimateEIP1559WithSuperHighFees", qrl.CallMsg{
			From:      addr,
			To:        &addr,
			Gas:       0,
			GasFeeCap: big.NewInt(1e14), // maxgascost = 2.1quanta
			GasTipCap: big.NewInt(1),
			Value:     big.NewInt(1e17 + 1), // the remaining balance for fee is 2.1quanta
			Data:      nil,
		}, params.TxGas, errors.New("gas required exceeds allowance (20999)")}, // 20999=(2.2quanta-0.1quanta-1planck)/(1e14)
	}
	for i, c := range cases {
		got, err := sim.EstimateGas(t.Context(), c.message)
		if c.expectError != nil {
			if err == nil {
				t.Fatalf("test %d: expect error, got nil", i)
			}
			if c.expectError.Error() != err.Error() {
				t.Fatalf("test %d: expect error, want %v, got %v", i, c.expectError, err)
			}
			continue
		}
		if c.expectError == nil && err != nil {
			t.Fatalf("test %d: didn't expect error, got %v", i, err)
		}
		if got != c.expect {
			t.Fatalf("test %d: gas estimation mismatch, want %d, got %d", i, c.expect, got)
		}
	}
}

func TestHeaderByHash(t *testing.T) {
	testAddr := testWallet.GetAddress()

	sim := simTestBackend(testAddr)
	defer sim.Close()

	header, err := sim.HeaderByNumber(t.Context(), nil)
	if err != nil {
		t.Errorf("could not get recent block: %v", err)
	}
	headerByHash, err := sim.HeaderByHash(t.Context(), header.Hash())
	if err != nil {
		t.Errorf("could not get recent block: %v", err)
	}

	if header.Hash() != headerByHash.Hash() {
		t.Errorf("did not get expected block")
	}
}

func TestHeaderByNumber(t *testing.T) {
	testAddr := testWallet.GetAddress()

	sim := simTestBackend(testAddr)
	defer sim.Close()
	bgCtx := t.Context()

	latestBlockHeader, err := sim.HeaderByNumber(bgCtx, nil)
	if err != nil {
		t.Errorf("could not get header for tip of chain: %v", err)
	}
	if latestBlockHeader == nil {
		t.Errorf("received a nil block header")
	} else if latestBlockHeader.Number.Uint64() != uint64(0) {
		t.Errorf("expected block header number 0, instead got %v", latestBlockHeader.Number.Uint64())
	}

	sim.Commit()

	latestBlockHeader, err = sim.HeaderByNumber(bgCtx, nil)
	if err != nil {
		t.Errorf("could not get header for blockheight of 1: %v", err)
	}

	blockHeader, err := sim.HeaderByNumber(bgCtx, big.NewInt(1))
	if err != nil {
		t.Errorf("could not get header for blockheight of 1: %v", err)
	}

	if blockHeader.Hash() != latestBlockHeader.Hash() {
		t.Errorf("block header and latest block header are not the same")
	}
	if blockHeader.Number.Int64() != int64(1) {
		t.Errorf("did not get blockheader for block 1. instead got block %v", blockHeader.Number.Int64())
	}

	block, err := sim.BlockByNumber(bgCtx, big.NewInt(1))
	if err != nil {
		t.Errorf("could not get block for blockheight of 1: %v", err)
	}

	if block.Hash() != blockHeader.Hash() {
		t.Errorf("block hash and block header hash do not match. expected %v, got %v", block.Hash(), blockHeader.Hash())
	}
}

func TestTransactionCount(t *testing.T) {
	testAddr := common.Address(testWallet.GetAddress())

	sim := simTestBackend(testAddr)
	defer sim.Close()
	bgCtx := t.Context()
	currentBlock, err := sim.BlockByNumber(bgCtx, nil)
	if err != nil || currentBlock == nil {
		t.Error("could not get current block")
	}

	count, err := sim.TransactionCount(bgCtx, currentBlock.Hash())
	if err != nil {
		t.Error("could not get current block's transaction count")
	}

	if count != 0 {
		t.Errorf("expected transaction count of %v does not match actual count of %v", 0, count)
	}
	// create a signed transaction to send
	head, _ := sim.HeaderByNumber(t.Context(), nil) // Should be child's, good enough
	gasFeeCap := new(big.Int).Add(head.BaseFee, big.NewInt(1))

	tx := types.NewTx(&types.DynamicFeeTx{
		Nonce:     uint64(0),
		To:        &testAddr,
		Value:     big.NewInt(1000),
		Gas:       params.TxGas,
		GasFeeCap: gasFeeCap,
		Data:      nil,
	})
	signedTx, err := types.SignTx(tx, types.ZondSigner{ChainId: big.NewInt(1337)}, testWallet)
	if err != nil {
		t.Errorf("could not sign tx: %v", err)
	}

	// send tx to simulated backend
	err = sim.SendTransaction(bgCtx, signedTx)
	if err != nil {
		t.Errorf("could not add tx to pending block: %v", err)
	}

	sim.Commit()

	lastBlock, err := sim.BlockByNumber(bgCtx, nil)
	if err != nil {
		t.Errorf("could not get header for tip of chain: %v", err)
	}

	count, err = sim.TransactionCount(bgCtx, lastBlock.Hash())
	if err != nil {
		t.Error("could not get current block's transaction count")
	}

	if count != 1 {
		t.Errorf("expected transaction count of %v does not match actual count of %v", 1, count)
	}
}

func TestTransactionInBlock(t *testing.T) {
	testAddr := common.Address(testWallet.GetAddress())

	sim := simTestBackend(testAddr)
	defer sim.Close()
	bgCtx := t.Context()

	transaction, err := sim.TransactionInBlock(bgCtx, sim.pendingBlock.Hash(), uint(0))
	if err == nil && err != errTransactionDoesNotExist {
		t.Errorf("expected a transaction does not exist error to be received but received %v", err)
	}
	if transaction != nil {
		t.Errorf("expected transaction to be nil but received %v", transaction)
	}

	// expect pending nonce to be 0 since account has not been used
	pendingNonce, err := sim.PendingNonceAt(bgCtx, testAddr)
	if err != nil {
		t.Errorf("did not get the pending nonce: %v", err)
	}

	if pendingNonce != uint64(0) {
		t.Errorf("expected pending nonce of 0 got %v", pendingNonce)
	}
	// create a signed transaction to send
	head, _ := sim.HeaderByNumber(t.Context(), nil) // Should be child's, good enough
	gasFeeCap := new(big.Int).Add(head.BaseFee, big.NewInt(1))

	tx := types.NewTx(&types.DynamicFeeTx{
		Nonce:     uint64(0),
		To:        &testAddr,
		Value:     big.NewInt(1000),
		Gas:       params.TxGas,
		GasFeeCap: gasFeeCap,
		Data:      nil,
	})
	signedTx, err := types.SignTx(tx, types.ZondSigner{ChainId: big.NewInt(1337)}, testWallet)
	if err != nil {
		t.Errorf("could not sign tx: %v", err)
	}

	// send tx to simulated backend
	err = sim.SendTransaction(bgCtx, signedTx)
	if err != nil {
		t.Errorf("could not add tx to pending block: %v", err)
	}

	sim.Commit()

	lastBlock, err := sim.BlockByNumber(bgCtx, nil)
	if err != nil {
		t.Errorf("could not get header for tip of chain: %v", err)
	}

	transaction, err = sim.TransactionInBlock(bgCtx, lastBlock.Hash(), uint(1))
	if err == nil && err != errTransactionDoesNotExist {
		t.Errorf("expected a transaction does not exist error to be received but received %v", err)
	}
	if transaction != nil {
		t.Errorf("expected transaction to be nil but received %v", transaction)
	}

	transaction, err = sim.TransactionInBlock(bgCtx, lastBlock.Hash(), uint(0))
	if err != nil {
		t.Errorf("could not get transaction in the lastest block with hash %v: %v", lastBlock.Hash().String(), err)
	}

	if signedTx.Hash().String() != transaction.Hash().String() {
		t.Errorf("received transaction that did not match the sent transaction. expected hash %v, got hash %v", signedTx.Hash().String(), transaction.Hash().String())
	}
}

func TestPendingNonceAt(t *testing.T) {
	testAddr := common.Address(testWallet.GetAddress())

	sim := simTestBackend(testAddr)
	defer sim.Close()
	bgCtx := t.Context()

	// expect pending nonce to be 0 since account has not been used
	pendingNonce, err := sim.PendingNonceAt(bgCtx, testAddr)
	if err != nil {
		t.Errorf("did not get the pending nonce: %v", err)
	}

	if pendingNonce != uint64(0) {
		t.Errorf("expected pending nonce of 0 got %v", pendingNonce)
	}

	// create a signed transaction to send
	head, _ := sim.HeaderByNumber(t.Context(), nil) // Should be child's, good enough
	gasFeeCap := new(big.Int).Add(head.BaseFee, big.NewInt(1))

	tx := types.NewTx(&types.DynamicFeeTx{
		Nonce:     uint64(0),
		To:        &testAddr,
		Value:     big.NewInt(1000),
		Gas:       params.TxGas,
		GasFeeCap: gasFeeCap,
		Data:      nil,
	})
	signedTx, err := types.SignTx(tx, types.ZondSigner{ChainId: big.NewInt(1337)}, testWallet)
	if err != nil {
		t.Errorf("could not sign tx: %v", err)
	}

	// send tx to simulated backend
	err = sim.SendTransaction(bgCtx, signedTx)
	if err != nil {
		t.Errorf("could not add tx to pending block: %v", err)
	}

	// expect pending nonce to be 1 since account has submitted one transaction
	pendingNonce, err = sim.PendingNonceAt(bgCtx, testAddr)
	if err != nil {
		t.Errorf("did not get the pending nonce: %v", err)
	}

	if pendingNonce != uint64(1) {
		t.Errorf("expected pending nonce of 1 got %v", pendingNonce)
	}

	// make a new transaction with a nonce of 1
	tx = types.NewTx(&types.DynamicFeeTx{
		Nonce:     uint64(1),
		To:        &testAddr,
		Value:     big.NewInt(1000),
		Gas:       params.TxGas,
		GasFeeCap: gasFeeCap,
		Data:      nil,
	})
	signedTx, err = types.SignTx(tx, types.ZondSigner{ChainId: big.NewInt(1337)}, testWallet)
	if err != nil {
		t.Errorf("could not sign tx: %v", err)
	}
	err = sim.SendTransaction(bgCtx, signedTx)
	if err != nil {
		t.Errorf("could not send tx: %v", err)
	}

	// expect pending nonce to be 2 since account now has two transactions
	pendingNonce, err = sim.PendingNonceAt(bgCtx, testAddr)
	if err != nil {
		t.Errorf("did not get the pending nonce: %v", err)
	}

	if pendingNonce != uint64(2) {
		t.Errorf("expected pending nonce of 2 got %v", pendingNonce)
	}
}

func TestTransactionReceipt(t *testing.T) {
	testAddr := common.Address(testWallet.GetAddress())

	sim := simTestBackend(testAddr)
	defer sim.Close()
	bgCtx := t.Context()

	// create a signed transaction to send
	head, _ := sim.HeaderByNumber(t.Context(), nil) // Should be child's, good enough
	gasFeeCap := new(big.Int).Add(head.BaseFee, big.NewInt(1))

	tx := types.NewTx(&types.DynamicFeeTx{
		Nonce:     uint64(0),
		To:        &testAddr,
		Value:     big.NewInt(1000),
		Gas:       params.TxGas,
		GasFeeCap: gasFeeCap,
		Data:      nil,
	})
	signedTx, err := types.SignTx(tx, types.ZondSigner{ChainId: big.NewInt(1337)}, testWallet)
	if err != nil {
		t.Errorf("could not sign tx: %v", err)
	}

	// send tx to simulated backend
	err = sim.SendTransaction(bgCtx, signedTx)
	if err != nil {
		t.Errorf("could not add tx to pending block: %v", err)
	}
	sim.Commit()

	receipt, err := sim.TransactionReceipt(bgCtx, signedTx.Hash())
	if err != nil {
		t.Errorf("could not get transaction receipt: %v", err)
	}

	if receipt.ContractAddress != testAddr && receipt.TxHash != signedTx.Hash() {
		t.Errorf("received receipt is not correct: %v", receipt)
	}
}

func TestSuggestGasPrice(t *testing.T) {
	sim := NewSimulatedBackend(
		core.GenesisAlloc{},
		10000000,
	)
	defer sim.Close()
	bgCtx := t.Context()
	gasPrice, err := sim.SuggestGasPrice(bgCtx)
	if err != nil {
		t.Errorf("could not get gas price: %v", err)
	}
	if gasPrice.Uint64() != sim.pendingBlock.Header().BaseFee.Uint64() {
		t.Errorf("gas price was not expected value of %v. actual: %v", sim.pendingBlock.Header().BaseFee.Uint64(), gasPrice.Uint64())
	}
}

func TestPendingCodeAt(t *testing.T) {
	testAddr := testWallet.GetAddress()
	sim := simTestBackend(testAddr)
	defer sim.Close()
	bgCtx := t.Context()
	code, err := sim.CodeAt(bgCtx, testAddr, nil)
	if err != nil {
		t.Errorf("could not get code at test addr: %v", err)
	}
	if len(code) != 0 {
		t.Errorf("got code for account that does not have contract code")
	}

	parsed, err := abi.JSON(strings.NewReader(abiJSON))
	if err != nil {
		t.Errorf("could not get code at test addr: %v", err)
	}
	auth, _ := bind.NewKeyedTransactorWithChainID(testWallet, big.NewInt(1337))
	contractAddr, tx, contract, err := bind.DeployContract(auth, parsed, common.FromHex(abiBin), sim)
	if err != nil {
		t.Errorf("could not deploy contract: %v tx: %v contract: %v", err, tx, contract)
	}

	code, err = sim.PendingCodeAt(bgCtx, contractAddr)
	if err != nil {
		t.Errorf("could not get code at test addr: %v", err)
	}
	if len(code) == 0 {
		t.Errorf("did not get code for account that has contract code")
	}
	// ensure code received equals code deployed
	if !bytes.Equal(code, common.FromHex(deployedCode)) {
		t.Errorf("code received did not match expected deployed code:\n expected %v\n actual %v", common.FromHex(deployedCode), code)
	}
}

func TestCodeAt(t *testing.T) {
	testAddr := testWallet.GetAddress()
	sim := simTestBackend(testAddr)
	defer sim.Close()
	bgCtx := t.Context()
	code, err := sim.CodeAt(bgCtx, testAddr, nil)
	if err != nil {
		t.Errorf("could not get code at test addr: %v", err)
	}
	if len(code) != 0 {
		t.Errorf("got code for account that does not have contract code")
	}

	parsed, err := abi.JSON(strings.NewReader(abiJSON))
	if err != nil {
		t.Errorf("could not get code at test addr: %v", err)
	}
	auth, _ := bind.NewKeyedTransactorWithChainID(testWallet, big.NewInt(1337))
	contractAddr, tx, contract, err := bind.DeployContract(auth, parsed, common.FromHex(abiBin), sim)
	if err != nil {
		t.Errorf("could not deploy contract: %v tx: %v contract: %v", err, tx, contract)
	}

	sim.Commit()
	code, err = sim.CodeAt(bgCtx, contractAddr, nil)
	if err != nil {
		t.Errorf("could not get code at test addr: %v", err)
	}
	if len(code) == 0 {
		t.Errorf("did not get code for account that has contract code")
	}
	// ensure code received equals code deployed
	if !bytes.Equal(code, common.FromHex(deployedCode)) {
		t.Errorf("code received did not match expected deployed code:\n expected %v\n actual %v", common.FromHex(deployedCode), code)
	}
}

// When receive("X") is called with sender Q00... and value 1, it produces this tx receipt:
//
//	receipt{status=1 cgas=23949 bloom=00000000004000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000800000000000000000000000000000000000040200000000000000000000000000000000001000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000080000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000 logs=[log: b6818c8064f645cd82d99b59a1a267d6d61117ef [75fd880d39c1daf53b6547ab6cb59451fc6452d27caa90e5b6649dd8293b9eed] 000000000000000000000000376c47978271565f56deb45495afa69e59c16ab200000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000060000000000000000000000000000000000000000000000000000000000000000158 9ae378b6d4409eada347a5dc0c180f186cb62dc68fcc0f043425eb917335aa28 0 95d429d309bb9d753954195fe2d69bd140b4ae731b9b5b605c34323de162cf00 0]}
func TestPendingAndCallContract(t *testing.T) {
	testAddr := testWallet.GetAddress()
	sim := simTestBackend(testAddr)
	defer sim.Close()
	bgCtx := t.Context()

	parsed, err := abi.JSON(strings.NewReader(abiJSON))
	if err != nil {
		t.Errorf("could not get code at test addr: %v", err)
	}
	contractAuth, _ := bind.NewKeyedTransactorWithChainID(testWallet, big.NewInt(1337))
	addr, _, _, err := bind.DeployContract(contractAuth, parsed, common.FromHex(abiBin), sim)
	if err != nil {
		t.Errorf("could not deploy contract: %v", err)
	}

	input, err := parsed.Pack("receive", []byte("X"))
	if err != nil {
		t.Errorf("could not pack receive function on contract: %v", err)
	}

	// make sure you can call the contract in pending state
	res, err := sim.PendingCallContract(bgCtx, qrl.CallMsg{
		From: testAddr,
		To:   &addr,
		Data: input,
	})
	if err != nil {
		t.Errorf("could not call receive method on contract: %v", err)
	}
	if len(res) == 0 {
		t.Errorf("result of contract call was empty: %v", res)
	}

	// while comparing against the byte array is more exact, also compare against the human readable string for readability
	if !bytes.Equal(res, expectedReturn) || !strings.Contains(string(res), "hello world") {
		t.Errorf("response from calling contract was expected to be 'hello world' instead received %v", string(res))
	}

	sim.Commit()

	// make sure you can call the contract
	res, err = sim.CallContract(bgCtx, qrl.CallMsg{
		From: testAddr,
		To:   &addr,
		Data: input,
	}, nil)
	if err != nil {
		t.Errorf("could not call receive method on contract: %v", err)
	}
	if len(res) == 0 {
		t.Errorf("result of contract call was empty: %v", res)
	}

	if !bytes.Equal(res, expectedReturn) || !strings.Contains(string(res), "hello world") {
		t.Errorf("response from calling contract was expected to be 'hello world' instead received %v", string(res))
	}
}

// This test is based on the following contract:
/*
contract Reverter {
	function revertString() public pure{
		require(false, "some error");
	}
	function revertNoString() public pure {
		require(false, "");
	}
	function revertASM() public pure {
		assembly {
			revert(0x0, 0x0)
		}
	}
	function noRevert() public pure {
		assembly {
			// Assembles something that looks like require(false, "some error") but is not reverted
			mstore(0x0, 0x08c379a000000000000000000000000000000000000000000000000000000000)
			mstore(0x4, 0x0000000000000000000000000000000000000000000000000000000000000020)
			mstore(0x24, 0x000000000000000000000000000000000000000000000000000000000000000a)
			mstore(0x44, 0x736f6d65206572726f7200000000000000000000000000000000000000000000)
			return(0x0, 0x64)
		}
	}
}*/
func TestCallContractRevert(t *testing.T) {
	testAddr := testWallet.GetAddress()
	sim := simTestBackend(testAddr)
	defer sim.Close()
	bgCtx := t.Context()

	reverterABI := `[{"inputs": [],"name": "noRevert","outputs": [],"stateMutability": "pure","type": "function"},{"inputs": [],"name": "revertASM","outputs": [],"stateMutability": "pure","type": "function"},{"inputs": [],"name": "revertNoString","outputs": [],"stateMutability": "pure","type": "function"},{"inputs": [],"name": "revertString","outputs": [],"stateMutability": "pure","type": "function"}]`
	// Hand-rolled replacement for the Solidity Reverter fixture. Under the
	// 64-byte ABI slot layout the Error(string) payload becomes
	//   selector(4) | offset=0x40(64) | length(64) | data(64 padded)
	// for a total of 196 bytes (132 for an empty string). Selectors match
	// Solidity's keccak256(sig)[:4]:
	//   revertString()   → 0x9bd61037  (Error("some error"))
	//   revertNoString() → 0x4b409e01  (Error(""))
	//   revertASM()      → 0x9b340e36  (REVERT(0,0))
	//   noRevert()       → 0xb7246fc1  (RETURN 32 zero bytes)
	// 12-byte init copies a 137-byte runtime that dispatches on the top 4
	// bytes of the 64-byte CALLDATALOAD word (SHR 480) and falls through
	// to a bare REVERT for unknown selectors.
	reverterBin := "6089600c60003960896000f3" + // init: CODECOPY 137 / RETURN
		"6000356101e01c" + // selector = CALLDATALOAD(0) >> 480
		"a0639bd6103714610038" + "57" + // DUP1 → revertString
		"a0634b409e0114610066" + "57" + // DUP1 → revertNoString
		"a0639b340e361461007d" + "57" + // DUP1 → revertASM
		"a063b7246fc114610083" + "57" + // DUP1 → noRevert
		"60006000fd" + // fallback REVERT(0,0)
		"5b" + "6308c379a06101e01b600052" + // 0x38 revertString: MSTORE selector<<480 at 0
		"6040600452" + // offset=0x40 at memory[4:68]
		"600a604452" + // length=0x0a at memory[68:132]
		"69736f6d65206572726f72" + "6101b01b" + "608452" + // "some error"<<432 at memory[132:196]
		"60c46000fd" + // REVERT(0, 196)
		"5b" + "6308c379a06101e01b600052" + // 0x66 revertNoString: selector
		"6040600452" + // offset=0x40
		"60846000fd" + // REVERT(0, 132)  (length slot stays zero from fresh memory)
		"5b60006000fd" + // 0x7d revertASM: REVERT(0,0)
		"5b60206000f3" //    0x83 noRevert:  RETURN(0, 32)

	parsed, err := abi.JSON(strings.NewReader(reverterABI))
	if err != nil {
		t.Errorf("could not get code at test addr: %v", err)
	}
	contractAuth, _ := bind.NewKeyedTransactorWithChainID(testWallet, big.NewInt(1337))
	addr, _, _, err := bind.DeployContract(contractAuth, parsed, common.FromHex(reverterBin), sim)
	if err != nil {
		t.Errorf("could not deploy contract: %v", err)
	}
	sim.Commit()

	inputs := make(map[string]any, 3)
	inputs["revertASM"] = nil
	inputs["revertNoString"] = ""
	inputs["revertString"] = "some error"

	call := make([]func([]byte) ([]byte, error), 2)
	call[0] = func(input []byte) ([]byte, error) {
		return sim.PendingCallContract(bgCtx, qrl.CallMsg{
			From: testAddr,
			To:   &addr,
			Data: input,
		})
	}
	call[1] = func(input []byte) ([]byte, error) {
		return sim.CallContract(bgCtx, qrl.CallMsg{
			From: testAddr,
			To:   &addr,
			Data: input,
		}, nil)
	}

	// Run pending calls then commit
	for _, cl := range call {
		for key, val := range inputs {
			input, err := parsed.Pack(key)
			if err != nil {
				t.Errorf("could not pack %v function on contract: %v", key, err)
			}

			res, err := cl(input)
			if err == nil {
				t.Errorf("call to %v was not reverted", key)
			}
			if res != nil {
				t.Errorf("result from %v was not nil: %v", key, res)
			}
			if val != nil {
				rerr, ok := err.(*revertError)
				if !ok {
					t.Errorf("expect revert error")
				}
				if rerr.Error() != "execution reverted: "+val.(string) {
					t.Errorf("error was malformed: got %v want %v", rerr.Error(), val)
				}
			} else {
				// revert(0x0,0x0)
				if err.Error() != "execution reverted" {
					t.Errorf("error was malformed: got %v want %v", err, "execution reverted")
				}
			}
		}
		input, err := parsed.Pack("noRevert")
		if err != nil {
			t.Errorf("could not pack noRevert function on contract: %v", err)
		}
		res, err := cl(input)
		if err != nil {
			t.Error("call to noRevert was reverted")
		}
		if res == nil {
			t.Errorf("result from noRevert was nil")
		}
		sim.Commit()
	}
}

// TestFork check that the chain length after a reorg is correct.
// Steps:
//  1. Save the current block which will serve as parent for the fork.
//  2. Mine n blocks with n ∈ [0, 20].
//  3. Assert that the chain length is n.
//  4. Fork by using the parent block as ancestor.
//  5. Mine n+1 blocks which should trigger a reorg.
//  6. Assert that the chain length is n+1.
//     Since Commit() was called 2n+1 times in total,
//     having a chain length of just n+1 means that a reorg occurred.
func TestFork(t *testing.T) {
	testAddr := testWallet.GetAddress()
	sim := simTestBackend(testAddr)
	defer sim.Close()
	// 1.
	parent := sim.blockchain.CurrentBlock()
	// 2.
	n := int(rand.Int31n(21))
	for range n {
		sim.Commit()
	}
	// 3.
	if sim.blockchain.CurrentBlock().Number.Uint64() != uint64(n) {
		t.Error("wrong chain length")
	}
	// 4.
	sim.Fork(t.Context(), parent.Hash())
	// 5.
	for range n + 1 {
		sim.Commit()
	}
	// 6.
	if sim.blockchain.CurrentBlock().Number.Uint64() != uint64(n+1) {
		t.Error("wrong chain length")
	}
}

/*
Example contract to test event emission:

	// TODO(now.youtrack.cloud/issue/TGZ-30)
	pragma hyperion >=0.7.0 <0.9.0;
	contract Callable {
		event Called();
		function Call() public { emit Called(); }
	}
*/
const callableAbi = "[{\"anonymous\":false,\"inputs\":[],\"name\":\"Called\",\"type\":\"event\"},{\"inputs\":[],\"name\":\"Call\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"}]"

// Hand-rolled replacement for the Solidity fixture above. The original
// bytecode depended on LOG/DUP/SWAP opcodes that shifted when the VM
// widened to 512-bit words. 12-byte init copies a 62-byte runtime that
//   - reads calldata[0:4] (shifted in from the 64-byte CALLDATALOAD word),
//   - dispatches on selector 0x34e22921 (keccak256("Call()")[:4]),
//   - and, on match, emits LOG1 with topic0 = keccak256("Called()") << 256,
//     left-aligning the hash in the 64-byte topic like Hyperion's bytes32
//     event-signature layout, so contract.WatchLogs(nil, "Called") resolves
//     the event.
const callableBin = "603e600c600039603e6000f36000356101e01c63" +
	"34e22921146100125700" +
	"5b7f81fab7a4a0aa961db47eefc81f143a5220e8c8495260dd65b1356f1d19d3c7b8" +
	"6101001b60006000c100"

// TestForkLogsReborn check that the simulated reorgs
// correctly remove and reborn logs.
// Steps:
//  1. Deploy the Callable contract.
//  2. Set up an event subscription.
//  3. Save the current block which will serve as parent for the fork.
//  4. Send a transaction.
//  5. Check that the event was included.
//  6. Fork by using the parent block as ancestor.
//  7. Mine two blocks to trigger a reorg.
//  8. Check that the event was removed.
//  9. Re-send the transaction and mine a block.
//  10. Check that the event was reborn.
func TestForkLogsReborn(t *testing.T) {
	testAddr := testWallet.GetAddress()
	sim := simTestBackend(testAddr)
	defer sim.Close()
	// 1.
	parsed, _ := abi.JSON(strings.NewReader(callableAbi))
	auth, _ := bind.NewKeyedTransactorWithChainID(testWallet, big.NewInt(1337))
	_, _, contract, err := bind.DeployContract(auth, parsed, common.FromHex(callableBin), sim)
	if err != nil {
		t.Errorf("deploying contract: %v", err)
	}
	sim.Commit()
	// 2.
	logs, sub, err := contract.WatchLogs(nil, "Called")
	if err != nil {
		t.Errorf("watching logs: %v", err)
	}
	defer sub.Unsubscribe()
	// 3.
	parent := sim.blockchain.CurrentBlock()
	// 4.
	tx, err := contract.Transact(auth, "Call")
	if err != nil {
		t.Errorf("transacting: %v", err)
	}
	sim.Commit()
	// 5.
	log := <-logs
	if log.TxHash != tx.Hash() {
		t.Error("wrong event tx hash")
	}
	if log.Removed {
		t.Error("Event should be included")
	}
	// 6.
	if err := sim.Fork(t.Context(), parent.Hash()); err != nil {
		t.Errorf("forking: %v", err)
	}
	// 7.
	sim.Commit()
	sim.Commit()
	// 8.
	log = <-logs
	if log.TxHash != tx.Hash() {
		t.Error("wrong event tx hash")
	}
	if !log.Removed {
		t.Error("Event should be removed")
	}
	// 9.
	if err := sim.SendTransaction(t.Context(), tx); err != nil {
		t.Errorf("sending transaction: %v", err)
	}
	sim.Commit()
	// 10.
	log = <-logs
	if log.TxHash != tx.Hash() {
		t.Error("wrong event tx hash")
	}
	if log.Removed {
		t.Error("Event should be included")
	}
}

// TestForkResendTx checks that re-sending a TX after a fork
// is possible and does not cause a "nonce mismatch" panic.
// Steps:
//  1. Save the current block which will serve as parent for the fork.
//  2. Send a transaction.
//  3. Check that the TX is included in block 1.
//  4. Fork by using the parent block as ancestor.
//  5. Mine a block, Re-send the transaction and mine another one.
//  6. Check that the TX is now included in block 2.
func TestForkResendTx(t *testing.T) {
	testAddr := common.Address(testWallet.GetAddress())
	sim := simTestBackend(testAddr)
	defer sim.Close()
	// 1.
	parent := sim.blockchain.CurrentBlock()
	// 2.
	head, _ := sim.HeaderByNumber(t.Context(), nil) // Should be child's, good enough
	gasFeeCap := new(big.Int).Add(head.BaseFee, big.NewInt(1))

	_tx := types.NewTx(&types.DynamicFeeTx{
		Nonce:     0,
		To:        &testAddr,
		Value:     big.NewInt(1000),
		Gas:       params.TxGas,
		GasFeeCap: gasFeeCap,
		Data:      nil,
	})
	tx, _ := types.SignTx(_tx, types.ZondSigner{ChainId: big.NewInt(1337)}, testWallet)
	sim.SendTransaction(t.Context(), tx)
	sim.Commit()
	// 3.
	receipt, _ := sim.TransactionReceipt(t.Context(), tx.Hash())
	if h := receipt.BlockNumber.Uint64(); h != 1 {
		t.Errorf("TX included in wrong block: %d", h)
	}
	// 4.
	if err := sim.Fork(t.Context(), parent.Hash()); err != nil {
		t.Errorf("forking: %v", err)
	}
	// 5.
	sim.Commit()
	if err := sim.SendTransaction(t.Context(), tx); err != nil {
		t.Errorf("sending transaction: %v", err)
	}
	sim.Commit()
	// 6.
	receipt, _ = sim.TransactionReceipt(t.Context(), tx.Hash())
	if h := receipt.BlockNumber.Uint64(); h != 2 {
		t.Errorf("TX included in wrong block: %d", h)
	}
}

func TestCommitReturnValue(t *testing.T) {
	testAddr := common.Address(testWallet.GetAddress())
	sim := simTestBackend(testAddr)
	defer sim.Close()

	startBlockHeight := sim.blockchain.CurrentBlock().Number.Uint64()

	// Test if Commit returns the correct block hash
	h1 := sim.Commit()
	if h1 != sim.blockchain.CurrentBlock().Hash() {
		t.Error("Commit did not return the hash of the last block.")
	}

	// Create a block in the original chain (containing a transaction to force different block hashes)
	head, _ := sim.HeaderByNumber(t.Context(), nil) // Should be child's, good enough
	gasFeeCap := new(big.Int).Add(head.BaseFee, big.NewInt(1))
	_tx := types.NewTx(&types.DynamicFeeTx{
		Nonce:     uint64(0),
		To:        &testAddr,
		Value:     big.NewInt(1000),
		Gas:       params.TxGas,
		GasFeeCap: gasFeeCap,
		Data:      nil,
	})
	tx, _ := types.SignTx(_tx, types.ZondSigner{ChainId: big.NewInt(1337)}, testWallet)
	sim.SendTransaction(t.Context(), tx)
	h2 := sim.Commit()

	// Create another block in the original chain
	sim.Commit()

	// Fork at the first bock
	if err := sim.Fork(t.Context(), h1); err != nil {
		t.Errorf("forking: %v", err)
	}

	// Test if Commit returns the correct block hash after the reorg
	h2fork := sim.Commit()
	if h2 == h2fork {
		t.Error("The block in the fork and the original block are the same block!")
	}
	if sim.blockchain.GetHeader(h2fork, startBlockHeight+2) == nil {
		t.Error("Could not retrieve the just created block (side-chain)")
	}
}

// TestAdjustTimeAfterFork ensures that after a fork, AdjustTime uses the pending fork
// block's parent rather than the canonical head's parent.
func TestAdjustTimeAfterFork(t *testing.T) {
	testAddr := testWallet.GetAddress()
	sim := simTestBackend(testAddr)
	defer sim.Close()

	sim.Commit() // h1
	h1 := sim.blockchain.CurrentHeader().Hash()
	sim.Commit() // h2
	sim.Fork(t.Context(), h1)
	sim.AdjustTime(1 * time.Second)
	sim.Commit()

	head := sim.blockchain.CurrentHeader()
	if head.Number == common.Big2 && head.ParentHash != h1 {
		t.Errorf("failed to build block on fork")
	}
}
