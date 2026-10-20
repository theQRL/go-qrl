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

package graphql

import (
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/consensus"
	"github.com/theQRL/go-qrl/consensus/beacon"
	"github.com/theQRL/go-qrl/core"
	"github.com/theQRL/go-qrl/core/types"
	"github.com/theQRL/go-qrl/core/vm"
	"github.com/theQRL/go-qrl/crypto/pqcrypto/wallet"
	"github.com/theQRL/go-qrl/internal/testutil"
	"github.com/theQRL/go-qrl/node"
	"github.com/theQRL/go-qrl/params"
	"github.com/theQRL/go-qrl/qrl"
	"github.com/theQRL/go-qrl/qrl/filters"
	"github.com/theQRL/go-qrl/qrl/qrlconfig"

	"github.com/stretchr/testify/assert"
)

func TestBuildSchema(t *testing.T) {
	ddir := t.TempDir()
	// Copy config
	conf := node.DefaultConfig
	conf.DataDir = ddir
	stack, err := node.New(&conf)
	if err != nil {
		t.Fatalf("could not create new node: %v", err)
	}
	defer stack.Close()
	// Make sure the schema can be parsed and matched up to the object model.
	if _, err := newHandler(stack, nil, nil, []string{}, []string{}); err != nil {
		t.Errorf("Could not construct GraphQL handler: %v", err)
	}
}

// Tests that a graphQL request is successfully handled when graphql is enabled on the specified endpoint
func TestGraphQLBlockSerialization(t *testing.T) {
	stack := createNode(t)
	defer stack.Close()
	genesis := &core.Genesis{
		Config:   params.AllBeaconProtocolChanges,
		GasLimit: 11500000,
	}
	newGQLService(t, stack, genesis, 10, func(i int, gen *core.BlockGen) {})
	// start node
	if err := stack.Start(); err != nil {
		t.Fatalf("could not start node: %v", err)
	}

	for i, tt := range []struct {
		body string
		want string
		code int
	}{
		{ // Should return latest block
			body: `{"query": "{block{number}}","variables": null}`,
			want: `{"data":{"block":{"number":"0xa"}}}`,
			code: 200,
		},
		{ // Should return info about latest block
			body: `{"query": "{block{number,gasUsed,gasLimit}}","variables": null}`,
			want: `{"data":{"block":{"number":"0xa","gasUsed":"0x0","gasLimit":"0xaf79e0"}}}`,
			code: 200,
		},
		{
			body: `{"query": "{block(number:0){number,gasUsed,gasLimit}}","variables": null}`,
			want: `{"data":{"block":{"number":"0x0","gasUsed":"0x0","gasLimit":"0xaf79e0"}}}`,
			code: 200,
		},
		{
			body: `{"query": "{block(number:-1){number,gasUsed,gasLimit}}","variables": null}`,
			want: `{"data":{"block":null}}`,
			code: 200,
		},
		{
			body: `{"query": "{block(number:-500){number,gasUsed,gasLimit}}","variables": null}`,
			want: `{"data":{"block":null}}`,
			code: 200,
		},
		{
			body: `{"query": "{block(number:\"0\"){number,gasUsed,gasLimit}}","variables": null}`,
			want: `{"data":{"block":{"number":"0x0","gasUsed":"0x0","gasLimit":"0xaf79e0"}}}`,
			code: 200,
		},
		{
			body: `{"query": "{block(number:\"-33\"){number,gasUsed,gasLimit}}","variables": null}`,
			want: `{"data":{"block":null}}`,
			code: 200,
		},
		{
			body: `{"query": "{block(number:\"1337\"){number,gasUsed,gasLimit}}","variables": null}`,
			want: `{"data":{"block":null}}`,
			code: 200,
		},
		{
			body: `{"query": "{block(number:\"0x0\"){number,gasUsed,gasLimit}}","variables": null}`,
			want: `{"data":{"block":{"number":"0x0","gasUsed":"0x0","gasLimit":"0xaf79e0"}}}`,
			//want: `{"errors":[{"message":"strconv.ParseInt: parsing \"0x0\": invalid syntax"}],"data":{}}`,
			code: 200,
		},
		{
			body: `{"query": "{block(number:\"a\"){number,gasUsed,gasLimit}}","variables": null}`,
			want: `{"errors":[{"message":"strconv.ParseInt: parsing \"a\": invalid syntax"}],"data":{}}`,
			code: 400,
		},
		{
			body: `{"query": "{bleh{number}}","variables": null}"`,
			want: `{"errors":[{"message":"Cannot query field \"bleh\" on type \"Query\".","locations":[{"line":1,"column":2}]}]}`,
			code: 400,
		},
		// should return `estimateGas` as decimal
		{
			body: `{"query": "{block{ estimateGas(data:{}) }}"}`,
			want: `{"data":{"block":{"estimateGas":"0xcf08"}}}`,
			code: 200,
		},
		// should return `status` as decimal
		{
			body: `{"query": "{block {number call (data : {from : \"Qbe6c1fd78f40b86a24dc2d7d633e2912d71e5d166f8be2c850d5727f0adcc170c7741b784295eae0c4f28291d0928dc7a94f5374fce5edbc8e2a8697c1533167\", to: \"Q6295ee1b4f6dd65047762f924ecd367c17eabf8f6295ee1b4f6dd65047762f924ecd367c17eabf8f11223344556677886295ee1b4f6dd65047762f924ecd367c\", data :\"0x12a7b914\"}){data status}}}"}`,
			want: `{"data":{"block":{"number":"0xa","call":{"data":"0x","status":"0x1"}}}}`,
			code: 200,
		},
		{
			body: `{"query": "{blocks {number}}"}`,
			want: `{"errors":[{"message":"from block number must be specified","path":["blocks"]}],"data":null}`,
			code: 400,
		},
	} {
		resp, err := http.Post(fmt.Sprintf("%s/graphql", stack.HTTPEndpoint()), "application/json", strings.NewReader(tt.body))
		if err != nil {
			t.Fatalf("could not post: %v", err)
		}
		bodyBytes, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			t.Fatalf("could not read from response body: %v", err)
		}
		if have := string(bodyBytes); have != tt.want {
			t.Errorf("testcase %d %s,\nhave:\n%v\nwant:\n%v", i, tt.body, have, tt.want)
		}
		if tt.code != resp.StatusCode {
			t.Errorf("testcase %d %s,\nwrong statuscode, have: %v, want: %v", i, tt.body, resp.StatusCode, tt.code)
		}
		if ctype := resp.Header.Get("Content-Type"); ctype != "application/json" {
			t.Errorf("testcase %d \nwrong Content-Type, have: %v, want: %v", i, ctype, "application/json")
		}
	}
}

func TestGraphQLBlockSerializationEIP2718(t *testing.T) {
	// Account for signing txes
	var (
		wallet  = testutil.LoadAccount(t, "dave").Wallet(t)
		address = wallet.GetAddress()
		funds   = big.NewInt(1000000000000000000)
		// 64-byte "dad" address: the 0xdad marker lives in the lowest two
		// bytes, everything else is zero.
		dad = common.BytesToAddress([]byte{0x0d, 0xad})
	)
	stack := createNode(t)
	defer stack.Close()
	genesis := &core.Genesis{
		Config:   params.AllBeaconProtocolChanges,
		GasLimit: 11500000,
		Alloc: core.GenesisAlloc{
			address: {Balance: funds},
			// The address 0xdad sloads 0x00 and 0x01
			dad: {
				Code:    []byte{byte(vm.PC), byte(vm.PC), byte(vm.SLOAD), byte(vm.SLOAD)},
				Nonce:   0,
				Balance: big.NewInt(0),
			},
		},
		BaseFee: big.NewInt(params.InitialBaseFee),
	}
	signer := types.NewZondSigner(genesis.Config.ChainID)
	var txHashes []common.Hash
	newGQLService(t, stack, genesis, 1, func(i int, gen *core.BlockGen) {
		gen.SetCoinbase(common.Address{1})
		tx, _ := types.SignNewTx(wallet, signer, &types.DynamicFeeTx{
			Nonce:     uint64(0),
			To:        &dad,
			Value:     big.NewInt(100),
			Gas:       50000,
			GasFeeCap: big.NewInt(params.InitialBaseFee),
		})
		gen.AddTx(tx)
		txHashes = append(txHashes, tx.Hash())
		tx, _ = types.SignNewTx(wallet, signer, &types.DynamicFeeTx{
			ChainID:   genesis.Config.ChainID,
			Nonce:     uint64(1),
			To:        &dad,
			Gas:       30000,
			GasFeeCap: big.NewInt(params.InitialBaseFee),
			Value:     big.NewInt(50),
			AccessList: types.AccessList{{
				Address:     dad,
				StorageKeys: []common.Hash{{0}},
			}},
		})
		gen.AddTx(tx)
		txHashes = append(txHashes, tx.Hash())
	})
	// start node
	if err := stack.Start(); err != nil {
		t.Fatalf("could not start node: %v", err)
	}

	for i, tt := range []struct {
		body string
		want string
		code int
	}{
		{
			body: `{"query": "{block {number transactions { from { address } to { address } value hash type accessList { address storageKeys } index}}}"}`,
			// Addresses widen to 128 hex chars and tx hashes change because
			// the from/to/accessList fields now carry 64-byte values.
			want: fmt.Sprintf(`{"data":{"block":{"number":"0x1","transactions":[{"from":{"address":"%s"},"to":{"address":"%s"},"value":"0x64","hash":"%s","type":"0x2","accessList":[],"index":"0x0"},{"from":{"address":"%s"},"to":{"address":"%s"},"value":"0x32","hash":"%s","type":"0x2","accessList":[{"address":"%s","storageKeys":["0x0000000000000000000000000000000000000000000000000000000000000000"]}],"index":"0x1"}]}}}`,
				common.Address(address).Hex(), dad.Hex(), txHashes[0].Hex(),
				common.Address(address).Hex(), dad.Hex(), txHashes[1].Hex(), dad.Hex()),
			code: 200,
		},
	} {
		resp, err := http.Post(fmt.Sprintf("%s/graphql", stack.HTTPEndpoint()), "application/json", strings.NewReader(tt.body))
		if err != nil {
			t.Fatalf("could not post: %v", err)
		}
		bodyBytes, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			t.Fatalf("could not read from response body: %v", err)
		}
		if have := string(bodyBytes); have != tt.want {
			t.Errorf("testcase %d %s,\nhave:\n%v\nwant:\n%v", i, tt.body, have, tt.want)
		}
		if tt.code != resp.StatusCode {
			t.Errorf("testcase %d %s,\nwrong statuscode, have: %v, want: %v", i, tt.body, resp.StatusCode, tt.code)
		}
	}
}

// Tests that a graphQL request is not handled successfully when graphql is not enabled on the specified endpoint
func TestGraphQLHTTPOnSamePort_GQLRequest_Unsuccessful(t *testing.T) {
	stack := createNode(t)
	defer stack.Close()
	if err := stack.Start(); err != nil {
		t.Fatalf("could not start node: %v", err)
	}
	body := strings.NewReader(`{"query": "{block{number}}","variables": null}`)
	resp, err := http.Post(fmt.Sprintf("%s/graphql", stack.HTTPEndpoint()), "application/json", body)
	if err != nil {
		t.Fatalf("could not post: %v", err)
	}
	resp.Body.Close()
	// make sure the request is not handled successfully
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestGraphQLConcurrentResolvers(t *testing.T) {
	var (
		wallet, _ = wallet.Generate(wallet.ML_DSA_87)
		// A 64-byte address with the "dad" marker at the lowest three
		// bytes so the test logs have an identifiable address.
		dad     = common.BytesToAddress([]byte{0x0d, 0xad})
		dadStr  = dad.Hex()
		genesis = &core.Genesis{
			Config:   params.AllBeaconProtocolChanges,
			GasLimit: 11500000,
			Alloc: core.GenesisAlloc{
				wallet.GetAddress(): {Balance: big.NewInt(params.Quanta)},
				dad: {
					// LOG0(0, 0), LOG0(0, 0), RETURN(0, 0). LOG0 moved
					// from 0xa0 to 0xc0 in the 512-bit VM opcode shift.
					Code:    common.Hex2Bytes("60006000c060006000c060006000f3"),
					Nonce:   0,
					Balance: big.NewInt(0),
				},
			},
		}
		signer = types.NewZondSigner(genesis.Config.ChainID)
		stack  = createNode(t)
	)
	defer stack.Close()

	var tx *types.Transaction
	handler, chain := newGQLService(t, stack, genesis, 1, func(i int, gen *core.BlockGen) {
		tx, _ = types.SignNewTx(wallet, signer, &types.DynamicFeeTx{To: &dad, Gas: 100000, GasFeeCap: big.NewInt(params.InitialBaseFee)})
		gen.AddTx(tx)
		tx, _ = types.SignNewTx(wallet, signer, &types.DynamicFeeTx{To: &dad, Nonce: 1, Gas: 100000, GasFeeCap: big.NewInt(params.InitialBaseFee)})
		gen.AddTx(tx)
		tx, _ = types.SignNewTx(wallet, signer, &types.DynamicFeeTx{To: &dad, Nonce: 2, Gas: 100000, GasFeeCap: big.NewInt(params.InitialBaseFee)})
		gen.AddTx(tx)
	})
	// start node
	if err := stack.Start(); err != nil {
		t.Fatalf("could not start node: %v", err)
	}

	for i, tt := range []struct {
		body string
		want string
	}{
		// Multiple txes race to get/set the block hash.
		{
			body: "{block { transactions { logs { account { address } } } } }",
			want: fmt.Sprintf(`{"block":{"transactions":[{"logs":[{"account":{"address":"%s"}},{"account":{"address":"%s"}}]},{"logs":[{"account":{"address":"%s"}},{"account":{"address":"%s"}}]},{"logs":[{"account":{"address":"%s"}},{"account":{"address":"%s"}}]}]}}`, dadStr, dadStr, dadStr, dadStr, dadStr, dadStr),
		},
		// Multiple fields of a tx race to resolve it. Happens in this case
		// because resolving the tx body belonging to a log is delayed.
		{
			body: `{block { logs(filter: {}) { transaction { nonce value maxFeePerGas }}}}`,
			want: `{"block":{"logs":[{"transaction":{"nonce":"0x0","value":"0x0","maxFeePerGas":"0x174876e800"}},{"transaction":{"nonce":"0x0","value":"0x0","maxFeePerGas":"0x174876e800"}},{"transaction":{"nonce":"0x1","value":"0x0","maxFeePerGas":"0x174876e800"}},{"transaction":{"nonce":"0x1","value":"0x0","maxFeePerGas":"0x174876e800"}},{"transaction":{"nonce":"0x2","value":"0x0","maxFeePerGas":"0x174876e800"}},{"transaction":{"nonce":"0x2","value":"0x0","maxFeePerGas":"0x174876e800"}}]}}`,
		},
		// Multiple txes of a block race to set/retrieve receipts of a block.
		{
			body: "{block { transactions { status gasUsed } } }",
			want: `{"block":{"transactions":[{"status":"0x1","gasUsed":"0x5508"},{"status":"0x1","gasUsed":"0x5508"},{"status":"0x1","gasUsed":"0x5508"}]}}`,
		},
		// Multiple fields of block race to resolve header and body.
		{
			body: "{ block { number hash gasLimit transactionCount } }",
			want: fmt.Sprintf(`{"block":{"number":"0x1","hash":"%s","gasLimit":"0xaf79e0","transactionCount":"0x3"}}`, chain[len(chain)-1].Hash()),
		},
		// Multiple fields of a block race to resolve the header and body.
		{
			body: fmt.Sprintf(`{ transaction(hash: "%s") { block { number hash gasLimit transactionCount } } }`, tx.Hash()),
			want: fmt.Sprintf(`{"transaction":{"block":{"number":"0x1","hash":"%s","gasLimit":"0xaf79e0","transactionCount":"0x3"}}}`, chain[len(chain)-1].Hash()),
		},
		// Account fields race the resolve the state object.
		{
			body: fmt.Sprintf(`{ block { account(address: "%s") { balance transactionCount code } } }`, dadStr),
			want: `{"block":{"account":{"balance":"0x0","transactionCount":"0x0","code":"0x60006000c060006000c060006000f3"}}}`,
		},
		// Test values for a non-existent account.
		{
			body: fmt.Sprintf(`{ block { account(address: "%s") { balance transactionCount code } } }`, "Q11111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111"),
			want: `{"block":{"account":{"balance":"0x0","transactionCount":"0x0","code":"0x"}}}`,
		},
	} {
		res := handler.Schema.Exec(t.Context(), tt.body, "", map[string]any{})
		if res.Errors != nil {
			t.Fatalf("failed to execute query for testcase #%d: %v", i, res.Errors)
		}
		have, err := json.Marshal(res.Data)
		if err != nil {
			t.Fatalf("failed to encode graphql response for testcase #%d: %s", i, err)
		}
		if string(have) != tt.want {
			t.Errorf("response unmatch for testcase #%d.\nExpected:\n%s\nGot:\n%s\n", i, tt.want, have)
		}
	}
}

func TestWithdrawals(t *testing.T) {
	var (
		wallet, _ = wallet.Generate(wallet.ML_DSA_87)

		genesis = &core.Genesis{
			Config:   params.AllBeaconProtocolChanges,
			GasLimit: 11500000,
			Alloc: core.GenesisAlloc{
				wallet.GetAddress(): {Balance: big.NewInt(params.Quanta)},
			},
		}
		signer = types.NewZondSigner(genesis.Config.ChainID)
		stack  = createNode(t)
	)
	defer stack.Close()

	handler, _ := newGQLService(t, stack, genesis, 1, func(i int, gen *core.BlockGen) {
		tx, _ := types.SignNewTx(wallet, signer, &types.DynamicFeeTx{To: &common.Address{}, Gas: 100000, GasFeeCap: big.NewInt(params.InitialBaseFee)})
		gen.AddTx(tx)
		gen.AddWithdrawal(&types.Withdrawal{
			Validator: 5,
			Address:   common.Address{},
			Amount:    10,
		})
	})
	// start node
	if err := stack.Start(); err != nil {
		t.Fatalf("could not start node: %v", err)
	}

	for i, tt := range []struct {
		body string
		want string
	}{
		{
			body: "{block(number: 0) { withdrawalsRoot withdrawals { index } } }",
			want: `{"block":{"withdrawalsRoot":"0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421","withdrawals":[]}}`,
		},
		{
			body: "{block(number: 1) { withdrawalsRoot withdrawals { validator amount } } }",
			want: `{"block":{"withdrawalsRoot":"0xaa813d09af1a68c5a1504f6b586e5803e976b3270992d4c4efa074289841228b","withdrawals":[{"validator":"0x5","amount":"0xa"}]}}`,
		},
	} {
		res := handler.Schema.Exec(t.Context(), tt.body, "", map[string]any{})
		if res.Errors != nil {
			t.Fatalf("failed to execute query for testcase #%d: %v", i, res.Errors)
		}
		have, err := json.Marshal(res.Data)
		if err != nil {
			t.Fatalf("failed to encode graphql response for testcase #%d: %s", i, err)
		}
		if string(have) != tt.want {
			t.Errorf("response unmatch for testcase #%d.\nhave:\n%s\nwant:\n%s", i, have, tt.want)
		}
	}
}

func createNode(t *testing.T) *node.Node {
	stack, err := node.New(&node.Config{
		HTTPHost:     "127.0.0.1",
		HTTPPort:     0,
		WSHost:       "127.0.0.1",
		WSPort:       0,
		HTTPTimeouts: node.DefaultConfig.HTTPTimeouts,
	})
	if err != nil {
		t.Fatalf("could not create node: %v", err)
	}
	return stack
}

func newGQLService(t *testing.T, stack *node.Node, gspec *core.Genesis, genBlocks int, genfunc func(i int, gen *core.BlockGen)) (*handler, []*types.Block) {
	qrlConf := &qrlconfig.Config{
		Genesis:        gspec,
		NetworkId:      1337,
		TrieCleanCache: 5,
		TrieDirtyCache: 5,
		TrieTimeout:    60 * time.Minute,
		SnapshotCache:  5,
	}
	var engine consensus.Engine = beacon.NewFaker()
	qrlBackend, err := qrl.New(stack, qrlConf)
	if err != nil {
		t.Fatalf("could not create qrl backend: %v", err)
	}
	// Create some blocks and import them
	chain, _ := core.GenerateChain(params.AllBeaconProtocolChanges, qrlBackend.BlockChain().Genesis(),
		engine, qrlBackend.ChainDb(), genBlocks, genfunc)
	_, err = qrlBackend.BlockChain().InsertChain(chain)
	if err != nil {
		t.Fatalf("could not create import blocks: %v", err)
	}
	// Set up handler
	filterSystem := filters.NewFilterSystem(qrlBackend.APIBackend, filters.Config{})
	handler, err := newHandler(stack, qrlBackend.APIBackend, filterSystem, []string{}, []string{})
	if err != nil {
		t.Fatalf("could not create graphql service: %v", err)
	}
	return handler, chain
}
