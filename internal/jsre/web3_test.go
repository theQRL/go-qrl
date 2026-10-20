// Copyright 2026 The go-qrl Authors
// This file is part of the go-qrl library.
//
// The go-qrl library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-qrl library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the go-qrl library. If not, see <http://www.gnu.org/licenses/>.

package jsre

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/common/uint512"
	"github.com/theQRL/go-qrl/crypto"
	"github.com/theQRL/go-qrl/internal/jsre/deps"
)

const (
	web3CallProvider = `
var currentOutput = null;
var lastPayload = null;
var provider = {
  send: function(payload) {
    lastPayload = payload;
    return {jsonrpc: "2.0", id: payload.id, result: currentOutput};
  },
  sendAsync: function(payload, cb) {
    lastPayload = payload;
    cb(null, {jsonrpc: "2.0", id: payload.id, result: currentOutput});
  }
};
`

	web3FilterProvider = `
var captured = [];
var provider = {
  send: function(payload) {
    return {jsonrpc: "2.0", id: payload.id, result: null};
  },
  sendAsync: function(payload, cb) {
    if (payload.method === "qrl_newFilter") {
      captured.push(payload.params[0]);
    }
    cb(null, {jsonrpc: "2.0", id: payload.id, result: "0x1"});
  }
};
`

	web3Setup = `
var Web3 = require("web3");
var web3 = new Web3(provider);
`
)

func newEmbeddedWeb3(t *testing.T) *JSRE {
	t.Helper()

	re := New("", os.Stdout)
	t.Cleanup(func() { re.Stop(false) })

	if err := re.Compile("bignumber.js", deps.BigNumberJS); err != nil {
		t.Fatalf("compile bignumber.js: %v", err)
	}
	if err := re.Compile("web3.js", deps.Web3JS); err != nil {
		t.Fatalf("compile web3.js: %v", err)
	}
	return re
}

func runWeb3ContractJSON(t *testing.T, re *JSRE, provider, contractABI, address, script string, result any) {
	t.Helper()

	contractSetup := fmt.Sprintf(`
var contract = web3.qrl.contract(%s).at(%q);
`, contractABI, address)

	runWeb3JSON(t, re, provider, contractSetup+script, result)
}

func runWeb3JSON(t *testing.T, re *JSRE, provider, script string, result any) {
	t.Helper()

	value, err := re.Run(provider + web3Setup + script)
	if err != nil {
		t.Fatalf("run web3 script: %v", err)
	}

	if err := json.Unmarshal([]byte(value.String()), result); err != nil {
		t.Fatalf("decode web3 result %q: %v", value.String(), err)
	}
}

func abiWordHex(value uint64) string {
	return fmt.Sprintf("%0*x", uint512.WordBytes*2, value)
}

func methodSelector(signature string) string {
	return common.Bytes2Hex(crypto.Keccak256([]byte(signature))[:4])
}
