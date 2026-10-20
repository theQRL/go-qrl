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

package gqrlclient

import (
	"bytes"
	"encoding/json"
	"math/big"
	"testing"

	qrl "github.com/theQRL/go-qrl"
	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/consensus/beacon"
	"github.com/theQRL/go-qrl/core"
	"github.com/theQRL/go-qrl/core/types"
	"github.com/theQRL/go-qrl/crypto"
	"github.com/theQRL/go-qrl/internal/testutil"
	"github.com/theQRL/go-qrl/node"
	"github.com/theQRL/go-qrl/params"
	qrlsvc "github.com/theQRL/go-qrl/qrl"
	"github.com/theQRL/go-qrl/qrl/filters"
	"github.com/theQRL/go-qrl/qrl/qrlconfig"
	"github.com/theQRL/go-qrl/qrlclient"
	"github.com/theQRL/go-qrl/rpc"
)

var (
	testWallet   = testutil.MustLoadAccount("alice").MustWallet()
	testAddr     = testWallet.GetAddress()
	zeroAddr     = common.Address{}
	testContract = common.BytesToAddress(common.FromHex("000000000000000000000000000000000000beef000000000000000000000000000000000000beef11223344556677"))
	testEmpty    = common.BytesToAddress(common.FromHex("000000000000000000000000000000000000eeee000000000000000000000000000000000000eeee11223344556677"))
	testSlot     = common.HexToHash("0xdeadbeef")
	testValue    = common.BytesToStorageValue64(bytes.Repeat([]byte{0x42}, common.StorageValue64Length))
	testBalance  = big.NewInt(2e18)
)

func newTestBackend(t *testing.T) (*node.Node, []*types.Block) {
	// Generate test chain.
	genesis, blocks := generateTestChain()
	// Create node
	n, err := node.New(&node.Config{})
	if err != nil {
		t.Fatalf("can't create new node: %v", err)
	}
	// Create QRL Service
	config := &qrlconfig.Config{Genesis: genesis}
	qrlservice, err := qrlsvc.New(n, config)
	if err != nil {
		t.Fatalf("can't create new qrl service: %v", err)
	}
	filterSystem := filters.NewFilterSystem(qrlservice.APIBackend, filters.Config{})
	n.RegisterAPIs([]rpc.API{{
		Namespace: "qrl",
		Service:   filters.NewFilterAPI(filterSystem),
	}})

	// Import the test chain.
	if err := n.Start(); err != nil {
		t.Fatalf("can't start test node: %v", err)
	}
	if _, err := qrlservice.BlockChain().InsertChain(blocks[1:]); err != nil {
		t.Fatalf("can't import test blocks: %v", err)
	}
	return n, blocks
}

func generateTestChain() (*core.Genesis, []*types.Block) {
	genesis := &core.Genesis{
		Config: params.AllBeaconProtocolChanges,
		Alloc: core.GenesisAlloc{
			testAddr:     {Balance: testBalance, Storage: map[common.Hash]common.StorageValue64{testSlot: testValue}},
			testContract: {Nonce: 1, Code: []byte{0x13, 0x37}},
			testEmpty:    {Balance: big.NewInt(1)},
		},
		ExtraData: []byte("test genesis"),
		Timestamp: 9000,
	}
	generate := func(i int, g *core.BlockGen) {
		g.OffsetTime(5)
		g.SetExtra([]byte("test"))
	}
	_, blocks, _ := core.GenerateChainWithGenesis(genesis, beacon.NewFaker(), 1, generate)
	blocks = append([]*types.Block{genesis.ToBlock()}, blocks...)
	return genesis, blocks
}

func TestGqrlClient(t *testing.T) {
	backend, _ := newTestBackend(t)
	client := backend.Attach()
	defer backend.Close()
	defer client.Close()

	tests := []struct {
		name string
		test func(t *testing.T)
	}{
		{
			"TestGetProof1",
			func(t *testing.T) { testGetProof(t, client, testAddr) },
		},
		{
			"TestGetProof2",
			func(t *testing.T) { testGetProof(t, client, testContract) },
		},
		{
			"TestGetProofEmpty",
			func(t *testing.T) { testGetProof(t, client, testEmpty) },
		},
		{
			"TestGetProofNonExistent",
			func(t *testing.T) { testGetProofNonExistent(t, client) },
		},
		{
			"TestGetProofCanonicalizeKeys",
			func(t *testing.T) { testGetProofCanonicalizeKeys(t, client) },
		},
		{
			"TestGCStats",
			func(t *testing.T) { testGCStats(t, client) },
		},
		{
			"TestMemStats",
			func(t *testing.T) { testMemStats(t, client) },
		},
		{
			"TestGetNodeInfo",
			func(t *testing.T) { testGetNodeInfo(t, client) },
		},
		{
			"TestSubscribePendingTxHashes",
			func(t *testing.T) { testSubscribePendingTransactions(t, client) },
		},
		{
			"TestSubscribePendingTxs",
			func(t *testing.T) { testSubscribeFullPendingTransactions(t, client) },
		},
		{
			"TestCallContract",
			func(t *testing.T) { testCallContract(t, client) },
		},
		{
			"TestCallContractWithBlockOverrides",
			func(t *testing.T) { testCallContractWithBlockOverrides(t, client) },
		},
		// The testaccesslist is a bit time-sensitive: the newTestBackend imports
		// one block. The `testAccessList` fails if the miner has not yet created a
		// new pending-block after the import event.
		// Hence: this test should be last, execute the tests serially.
		{
			"TestAccessList",
			func(t *testing.T) { testAccessList(t, client) },
		},
		{
			"TestSetHead",
			func(t *testing.T) { testSetHead(t, client) },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, tt.test)
	}
}

func testAccessList(t *testing.T, client *rpc.Client) {
	zc := New(client)
	// Test transfer
	msg := qrl.CallMsg{
		From:      testAddr,
		To:        &common.Address{},
		Gas:       21000,
		GasFeeCap: big.NewInt(100000000000),
		Value:     big.NewInt(1),
	}
	al, gas, vmErr, err := zc.CreateAccessList(t.Context(), msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if vmErr != "" {
		t.Fatalf("unexpected vm error: %v", vmErr)
	}
	if gas != 21000 {
		t.Fatalf("unexpected gas used: %v", gas)
	}
	if len(*al) != 0 {
		t.Fatalf("unexpected length of accesslist: %v", len(*al))
	}
	// Test reverting transaction
	msg = qrl.CallMsg{
		From:      testAddr,
		To:        nil,
		Gas:       100000,
		GasFeeCap: big.NewInt(100000000000),
		Value:     big.NewInt(1),
		Data:      common.FromHex("0x608060806080608155fd"),
	}
	al, gas, vmErr, err = zc.CreateAccessList(t.Context(), msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if vmErr == "" {
		t.Fatalf("wanted vmErr, got none")
	}
	if gas == 21000 {
		t.Fatalf("unexpected gas used: %v", gas)
	}
	if len(*al) != 1 || al.StorageKeys() != 1 {
		t.Fatalf("unexpected length of accesslist: %v", len(*al))
	}
	// address changes between calls, so we can't test for it.
	if (*al)[0].Address == zeroAddr {
		t.Fatalf("unexpected address: %v", (*al)[0].Address)
	}
	if (*al)[0].StorageKeys[0] != common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000081") {
		t.Fatalf("unexpected storage key: %v", (*al)[0].StorageKeys[0])
	}
}

func testGetProof(t *testing.T, client *rpc.Client, addr common.Address) {
	zc := New(client)
	qrlcl := qrlclient.NewClient(client)
	result, err := zc.GetProof(t.Context(), addr, []string{testSlot.String()}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Address != addr {
		t.Fatalf("unexpected address, have: %v want: %v", result.Address, addr)
	}
	// test nonce
	if nonce, _ := qrlcl.NonceAt(t.Context(), result.Address, nil); result.Nonce != nonce {
		t.Fatalf("invalid nonce, want: %v got: %v", nonce, result.Nonce)
	}
	// test balance
	if balance, _ := qrlcl.BalanceAt(t.Context(), result.Address, nil); result.Balance.Cmp(balance) != 0 {
		t.Fatalf("invalid balance, want: %v got: %v", balance, result.Balance)
	}

	// test storage
	if len(result.StorageProof) != 1 {
		t.Fatalf("invalid storage proof, want 1 proof, got %v proof(s)", len(result.StorageProof))
	}
	for _, proof := range result.StorageProof {
		if proof.Key != testSlot.String() {
			t.Fatalf("invalid storage proof key, want: %q, got: %q", testSlot.String(), proof.Key)
		}
		slotValue, err := qrlcl.StorageAt(t.Context(), addr, common.HexToHash(proof.Key), nil)
		if err != nil {
			t.Fatalf("failed to fetch storage for %x: %v", addr, err)
		}
		if have, want := common.BytesToStorageValue64(proof.Value.Bytes()), common.BytesToStorageValue64(slotValue); have != want {
			t.Fatalf("addr %x, invalid storage proof value: have: %v, want: %v", addr, have, want)
		}
	}
	// test code
	code, _ := qrlcl.CodeAt(t.Context(), addr, nil)
	if have, want := result.CodeHash, crypto.Keccak256Hash(code); have != want {
		t.Fatalf("codehash wrong, have %v want %v ", have, want)
	}
}

func testGetProofCanonicalizeKeys(t *testing.T, client *rpc.Client) {
	zc := New(client)

	// Tests with non-canon input for storage keys.
	// Here we check that the storage key is canonicalized.
	result, err := zc.GetProof(t.Context(), testAddr, []string{"0x0dEadbeef"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.StorageProof[0].Key != "0xdeadbeef" {
		t.Fatalf("wrong storage key encoding in proof: %q", result.StorageProof[0].Key)
	}
	if result, err = zc.GetProof(t.Context(), testAddr, []string{"0x000deadbeef"}, nil); err != nil {
		t.Fatal(err)
	}
	if result.StorageProof[0].Key != "0xdeadbeef" {
		t.Fatalf("wrong storage key encoding in proof: %q", result.StorageProof[0].Key)
	}

	// If the requested storage key is 32 bytes long, it will be returned as is.
	hashSizedKey := "0x00000000000000000000000000000000000000000000000000000000deadbeef"
	result, err = zc.GetProof(t.Context(), testAddr, []string{hashSizedKey}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.StorageProof[0].Key != hashSizedKey {
		t.Fatalf("wrong storage key encoding in proof: %q", result.StorageProof[0].Key)
	}
}

func testGetProofNonExistent(t *testing.T, client *rpc.Client) {
	addr := common.MustParseAddress("Q00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000001")
	ec := New(client)
	result, err := ec.GetProof(t.Context(), addr, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Address != addr {
		t.Fatalf("unexpected address, have: %v want: %v", result.Address, addr)
	}
	// test nonce
	if result.Nonce != 0 {
		t.Fatalf("invalid nonce, want: %v got: %v", 0, result.Nonce)
	}
	// test balance
	if result.Balance.Sign() != 0 {
		t.Fatalf("invalid balance, want: %v got: %v", 0, result.Balance)
	}
	// test storage
	if have := len(result.StorageProof); have != 0 {
		t.Fatalf("invalid storage proof, want 0 proof, got %v proof(s)", have)
	}
	// test codeHash
	if have, want := result.CodeHash, (common.Hash{}); have != want {
		t.Fatalf("codehash wrong, have %v want %v ", have, want)
	}
	// test codeHash
	if have, want := result.StorageHash, (common.Hash{}); have != want {
		t.Fatalf("storagehash wrong, have %v want %v ", have, want)
	}
}

func testGCStats(t *testing.T, client *rpc.Client) {
	zc := New(client)
	_, err := zc.GCStats(t.Context())
	if err != nil {
		t.Fatal(err)
	}
}

func testMemStats(t *testing.T, client *rpc.Client) {
	zc := New(client)
	stats, err := zc.MemStats(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if stats.Alloc == 0 {
		t.Fatal("Invalid mem stats retrieved")
	}
}

func testGetNodeInfo(t *testing.T, client *rpc.Client) {
	zc := New(client)
	info, err := zc.GetNodeInfo(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	if info.Name == "" {
		t.Fatal("Invalid node info retrieved")
	}
}

func testSetHead(t *testing.T, client *rpc.Client) {
	zc := New(client)
	err := zc.SetHead(t.Context(), big.NewInt(0))
	if err != nil {
		t.Fatal(err)
	}
}

func testSubscribePendingTransactions(t *testing.T, client *rpc.Client) {
	qc := New(client)
	qrlcl := qrlclient.NewClient(client)
	// Subscribe to Transactions
	ch := make(chan common.Hash)
	qc.SubscribePendingTransactions(t.Context(), ch)
	// Send a transaction
	chainID, err := qrlcl.ChainID(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	// Create transaction
	tx := types.NewTx(&types.DynamicFeeTx{
		Nonce:     0,
		To:        &common.Address{1},
		Value:     big.NewInt(1),
		Gas:       22000,
		GasFeeCap: big.NewInt(100000000000),
		Data:      nil,
	})
	signer := types.NewZondSigner(chainID)
	signedTx, err := types.SignTx(tx, signer, testWallet)
	if err != nil {
		t.Fatal(err)
	}
	// Send transaction
	err = qrlcl.SendTransaction(t.Context(), signedTx)
	if err != nil {
		t.Fatal(err)
	}
	// Check that the transaction was sent over the channel
	hash := <-ch
	if hash != signedTx.Hash() {
		t.Fatalf("Invalid tx hash received, got %v, want %v", hash, signedTx.Hash())
	}
}

func testSubscribeFullPendingTransactions(t *testing.T, client *rpc.Client) {
	zc := New(client)
	qrlcl := qrlclient.NewClient(client)
	// Subscribe to Transactions
	ch := make(chan *types.Transaction)
	zc.SubscribeFullPendingTransactions(t.Context(), ch)
	// Send a transaction
	chainID, err := qrlcl.ChainID(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	// Create transaction
	tx := types.NewTx(&types.DynamicFeeTx{
		Nonce:     1,
		To:        &common.Address{1},
		Value:     big.NewInt(1),
		Gas:       22000,
		GasFeeCap: big.NewInt(100000000000),
		Data:      nil,
	})
	signer := types.NewZondSigner(chainID)
	signedTx, err := types.SignTx(tx, signer, testWallet)
	if err != nil {
		t.Fatal(err)
	}
	// Send transaction
	err = qrlcl.SendTransaction(t.Context(), signedTx)
	if err != nil {
		t.Fatal(err)
	}
	// Check that the transaction was sent over the channel
	tx = <-ch
	if tx.Hash() != signedTx.Hash() {
		t.Fatalf("Invalid tx hash received, got %v, want %v", tx.Hash(), signedTx.Hash())
	}
}

func testCallContract(t *testing.T, client *rpc.Client) {
	zc := New(client)
	msg := qrl.CallMsg{
		From:      testAddr,
		To:        &common.Address{},
		Gas:       21000,
		GasFeeCap: big.NewInt(100000000000),
		Value:     big.NewInt(1),
	}
	// CallContract without override
	if _, err := zc.CallContract(t.Context(), msg, big.NewInt(0), nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// CallContract with override
	override := OverrideAccount{
		Nonce: 1,
	}
	mapAcc := make(map[common.Address]OverrideAccount)
	mapAcc[testAddr] = override
	if _, err := zc.CallContract(t.Context(), msg, big.NewInt(0), &mapAcc); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOverrideAccountMarshal(t *testing.T) {
	om := map[common.Address]OverrideAccount{
		{0x11}: {
			// Zero-valued nonce is not overridden, but simply dropped by the encoder.
			Nonce: 0,
		},
		{0xaa}: {
			Nonce: 5,
		},
		{0xbb}: {
			Code: []byte{1},
		},
		{0xcc}: {
			// 'code', 'balance', 'state' should be set when input is
			// a non-nil but empty value.
			Code:    []byte{},
			Balance: big.NewInt(0),
			State:   map[common.Hash]common.StorageValue64{},
			// For 'stateDiff' the behavior is different, empty map
			// is ignored because it makes no difference.
			StateDiff: map[common.Hash]common.StorageValue64{},
		},
		{0xdd}: {
			StateDiff: map[common.Hash]common.StorageValue64{
				{0x01}: common.BytesToStorageValue64(bytes.Repeat([]byte{0x22}, common.StorageValue64Length)),
			},
		},
	}

	marshalled, err := json.MarshalIndent(&om, "", "  ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 64-byte Address values render as a Q prefix plus 128 lowercase hex
	// characters with the seed byte at position 0 and the remaining 63
	// bytes zeroed, rendered with QIP-55 checksum casing.
	expected := `{
  "Q11000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000": {},
  "QBB000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000": {
    "code": "0x01"
  },
  "QCC000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000": {
    "code": "0x",
    "balance": "0x0",
    "state": {}
  },
  "QaA000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000": {
    "nonce": "0x5"
  },
  "Qdd000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000": {
    "stateDiff": {
      "0x0100000000000000000000000000000000000000000000000000000000000000": "0x22222222222222222222222222222222222222222222222222222222222222222222222222222222222222222222222222222222222222222222222222222222"
    }
  }
}`

	if string(marshalled) != expected {
		t.Error("wrong output:", string(marshalled))
		t.Error("want:", expected)
	}
}

func TestBlockOverridesMarshal(t *testing.T) {
	coinbase := common.BytesToAddress(bytes.Repeat([]byte{0x11}, common.AddressLength))

	for i, tt := range []struct {
		bo   BlockOverrides
		want string
	}{
		{
			bo:   BlockOverrides{},
			want: `{}`,
		},
		{
			bo: BlockOverrides{
				Coinbase: coinbase,
			},
			want: `{"coinbase":"Q11111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111"}`,
		},
		{
			bo: BlockOverrides{
				Number:   big.NewInt(1),
				Time:     3,
				GasLimit: 4,
				BaseFee:  big.NewInt(5),
			},
			want: `{"number":"0x1","time":"0x3","gasLimit":"0x4","baseFee":"0x5"}`,
		},
	} {
		marshalled, err := json.Marshal(&tt.bo)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(marshalled) != tt.want {
			t.Errorf("Testcase #%d failed. expected\n%s\ngot\n%s", i, tt.want, string(marshalled))
		}
	}
}

func testCallContractWithBlockOverrides(t *testing.T, client *rpc.Client) {
	zc := New(client)
	msg := qrl.CallMsg{
		From:      testAddr,
		To:        &common.Address{},
		Gas:       50000,
		GasFeeCap: big.NewInt(100000000000),
		Value:     big.NewInt(1),
	}
	// Returns the coinbase address:
	//   41       COINBASE           ; push 64-byte word == 64-byte address
	//   60 00    PUSH1 0x00         ; memory offset
	//   52       MSTORE             ; write 64 bytes to memory[0]
	//   60 40    PUSH1 0x40         ; return length = 64 bytes
	//   60 00    PUSH1 0x00         ; return offset = 0
	//   f3       RETURN
	override := OverrideAccount{
		Code: common.FromHex("0x4160005260406000f3"),
	}
	mapAcc := make(map[common.Address]OverrideAccount)
	mapAcc[common.Address{}] = override
	res, err := zc.CallContract(t.Context(), msg, big.NewInt(0), &mapAcc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(res, make([]byte, common.AddressLength)) {
		t.Fatalf("unexpected result: %x", res)
	}

	// Now test with block overrides
	coinbase := common.BytesToAddress(bytes.Repeat([]byte{0x11}, common.AddressLength))
	bo := BlockOverrides{
		Coinbase: coinbase,
	}
	res, err = zc.CallContractWithBlockOverrides(t.Context(), msg, big.NewInt(0), &mapAcc, bo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(res, coinbase[:]) {
		t.Fatalf("unexpected result: %x", res)
	}
}
