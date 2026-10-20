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
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/common/uint512"
)

func TestEmbeddedWeb3ABICoderUsesVM64Words(t *testing.T) {
	t.Parallel()

	const contractABI = `[
  {
    "inputs": [
      {"name": "to", "type": "address"},
      {"name": "amount", "type": "uint512"},
      {"name": "label", "type": "string"},
      {"name": "active", "type": "bool"},
      {"name": "tag", "type": "bytes33"}
    ],
    "name": "store",
    "outputs": [],
    "stateMutability": "nonpayable",
    "type": "function"
  },
  {
    "inputs": [],
    "name": "load",
    "outputs": [
      {"name": "to", "type": "address"},
      {"name": "amount", "type": "uint512"},
      {"name": "label", "type": "string"},
      {"name": "active", "type": "bool"},
      {"name": "tag", "type": "bytes33"}
    ],
    "stateMutability": "view",
    "type": "function"
  },
  {
    "inputs": [{"name": "tag", "type": "bytes33"}],
    "name": "storeTag",
    "outputs": [],
    "stateMutability": "nonpayable",
    "type": "function"
  },
  {
    "inputs": [],
    "name": "pay",
    "outputs": [],
    "stateMutability": "payable",
    "type": "function"
  }
]`

	re := newEmbeddedWeb3(t)
	address := "Q" + strings.Repeat("a", common.AddressLength*2)
	checksummedAddress, err := common.NewAddressFromString(address)
	if err != nil {
		t.Fatal(err)
	}
	contractAddress := "Q" + strings.Repeat("0", common.AddressLength*2)
	addressWord := strings.Repeat("a", common.AddressLength*2)

	maxUint512 := strings.Repeat("f", uint512.WordBytes*2)
	maxUint512Decimal := "134078079299425970995740249982058461274793658205923933" +
		"77723561443721764030073546976801874298166903427690031" +
		"858186486050853753882811946569946433649006084095"

	bytes33 := strings.Repeat("ab", 33)
	bytes33Word := bytes33 + strings.Repeat("0", (uint512.WordBytes-33)*2)

	labelOffsetWord := abiWordHex(5 * uint512.WordBytes)
	activeWord := abiWordHex(1)
	labelLengthWord := abiWordHex(5)
	labelDataWord := common.Bytes2Hex([]byte("hello")) + strings.Repeat("0", uint512.WordBytes*2-len("hello")*2)
	encodedValues := strings.Join([]string{
		addressWord,
		maxUint512,
		labelOffsetWord,
		activeWord,
		bytes33Word,
		labelLengthWord,
		labelDataWord,
	}, "")

	output := "0x" + encodedValues
	expectedData := "0x" + methodSelector("store(address,uint512,string,bool,bytes33)") + encodedValues
	expectedEmptyTagData := "0x" +
		methodSelector("storeTag(bytes33)") +
		strings.Repeat("0", uint512.WordBytes*2)

	script := fmt.Sprintf(`
currentOutput = %q;

var data = contract.store.getData(%q, %q, "hello", true, "0x%s");
var emptyTagData = contract.storeTag.getData("0x");

var decoded = contract.load();
var loadMethod = lastPayload.method;

contract.pay({from: %q, value: 1});

var rejectsNonpayableValue = false;
try {
  contract.store(%q, %q, "hello", true, "0x%s", {from: %q, value: 1});
} catch (err) {
  rejectsNonpayableValue = true;
}

JSON.stringify({
  data: data,
  emptyTagData: emptyTagData,
  address: decoded[0],
  amount: decoded[1].toString(16),
  label: decoded[2],
  active: decoded[3],
  tag: decoded[4],
  loadMethod: loadMethod,
  payMethod: lastPayload.method,
  rejectsNonpayableValue: rejectsNonpayableValue
});
`, output, address, maxUint512Decimal, bytes33, address, address, maxUint512Decimal, bytes33, address)

	var got struct {
		Data                   string `json:"data"`
		EmptyTagData           string `json:"emptyTagData"`
		Address                string `json:"address"`
		Amount                 string `json:"amount"`
		Label                  string `json:"label"`
		Active                 bool   `json:"active"`
		Tag                    string `json:"tag"`
		LoadMethod             string `json:"loadMethod"`
		PayMethod              string `json:"payMethod"`
		RejectsNonpayableValue bool   `json:"rejectsNonpayableValue"`
	}

	runWeb3ContractJSON(t, re, web3CallProvider, contractABI, contractAddress, script, &got)

	if got.Data != expectedData {
		t.Fatalf("calldata mismatch:\nhave %s\nwant %s", got.Data, expectedData)
	}
	if got.EmptyTagData != expectedEmptyTagData {
		t.Fatalf("empty fixed bytes calldata mismatch:\nhave %s\nwant %s", got.EmptyTagData, expectedEmptyTagData)
	}
	if got.Address != checksummedAddress.Hex() ||
		got.Amount != maxUint512 ||
		got.Label != "hello" ||
		!got.Active ||
		got.Tag != "0x"+bytes33 {
		t.Fatalf("decoded values mismatch: %+v", got)
	}
	if got.LoadMethod != "qrl_call" || got.PayMethod != "qrl_sendTransaction" {
		t.Fatalf("stateMutability routing mismatch: load=%q pay=%q", got.LoadMethod, got.PayMethod)
	}
	if !got.RejectsNonpayableValue {
		t.Fatal("nonpayable function accepted transaction value")
	}
}

func TestEmbeddedWeb3DynamicBytesAndArrays(t *testing.T) {
	t.Parallel()

	const contractABI = `[
  {
    "inputs": [{"name": "value", "type": "bytes"}],
    "name": "storeBytes",
    "outputs": [],
    "stateMutability": "nonpayable",
    "type": "function"
  },
  {
    "inputs": [],
    "name": "loadBytes",
    "outputs": [{"name": "value", "type": "bytes"}],
    "stateMutability": "pure",
    "type": "function"
  },
  {
    "inputs": [
      {"name": "fixedValues", "type": "uint512[2]"},
      {"name": "dynamicValues", "type": "uint512[]"}
    ],
    "name": "storeArrays",
    "outputs": [],
    "stateMutability": "nonpayable",
    "type": "function"
  },
  {
    "inputs": [],
    "name": "loadArrays",
    "outputs": [
      {"name": "fixedValues", "type": "uint512[2]"},
      {"name": "dynamicValues", "type": "uint512[]"}
    ],
    "stateMutability": "view",
    "type": "function"
  }
]`

	re := newEmbeddedWeb3(t)
	contractAddress := "Q" + strings.Repeat("0", common.AddressLength*2)

	payload := "a1b2c3"
	payloadWord := payload + strings.Repeat("0", uint512.WordBytes*2-len(payload))
	encodedBytes := strings.Join([]string{
		abiWordHex(uint512.WordBytes),
		abiWordHex(3),
		payloadWord,
	}, "")
	bytesOutput := "0x" + encodedBytes
	bytesData := "0x" + methodSelector("storeBytes(bytes)") + encodedBytes

	encodedArrays := strings.Join([]string{
		abiWordHex(1),
		abiWordHex(2),
		abiWordHex(3 * uint512.WordBytes),
		abiWordHex(3),
		abiWordHex(3),
		abiWordHex(4),
		abiWordHex(5),
	}, "")
	arraysOutput := "0x" + encodedArrays
	arraysData := "0x" + methodSelector("storeArrays(uint512[2],uint512[])") + encodedArrays

	script := fmt.Sprintf(`
var bytesData = contract.storeBytes.getData(%q);

currentOutput = %q;
var decodedBytes = contract.loadBytes();
var pureMethod = lastPayload.method;

var arraysData = contract.storeArrays.getData([1, 2], [3, 4, 5]);

currentOutput = %q;
var decodedArrays = contract.loadArrays();

JSON.stringify({
  bytesData: bytesData,
  decodedBytes: decodedBytes,
  pureMethod: pureMethod,
  arraysData: arraysData,
  fixedValues: decodedArrays[0].map(function (value) { return value.toString(10); }),
  dynamicValues: decodedArrays[1].map(function (value) { return value.toString(10); })
});
`, "0x"+payload, bytesOutput, arraysOutput)

	var got struct {
		BytesData     string   `json:"bytesData"`
		DecodedBytes  string   `json:"decodedBytes"`
		PureMethod    string   `json:"pureMethod"`
		ArraysData    string   `json:"arraysData"`
		FixedValues   []string `json:"fixedValues"`
		DynamicValues []string `json:"dynamicValues"`
	}

	runWeb3ContractJSON(t, re, web3CallProvider, contractABI, contractAddress, script, &got)

	if got.BytesData != bytesData {
		t.Fatalf("dynamic bytes calldata mismatch:\nhave %s\nwant %s", got.BytesData, bytesData)
	}
	if got.DecodedBytes != "0x"+payload {
		t.Fatalf("decoded dynamic bytes mismatch: have %s, want 0x%s", got.DecodedBytes, payload)
	}
	if got.PureMethod != "qrl_call" {
		t.Fatalf("pure function used %q, want qrl_call", got.PureMethod)
	}
	if got.ArraysData != arraysData {
		t.Fatalf("array calldata mismatch:\nhave %s\nwant %s", got.ArraysData, arraysData)
	}
	if want := []string{"1", "2"}; !slices.Equal(got.FixedValues, want) {
		t.Fatalf("decoded fixed array mismatch: have %v, want %v", got.FixedValues, want)
	}
	if want := []string{"3", "4", "5"}; !slices.Equal(got.DynamicValues, want) {
		t.Fatalf("decoded dynamic array mismatch: have %v, want %v", got.DynamicValues, want)
	}
}

func TestEmbeddedWeb3SignedInt512AndAddressInput(t *testing.T) {
	t.Parallel()

	const contractABI = `[
  {
    "inputs": [{"name": "value", "type": "int512"}],
    "name": "storeInt",
    "outputs": [],
    "stateMutability": "nonpayable",
    "type": "function"
  },
  {
    "inputs": [],
    "name": "loadInt",
    "outputs": [{"name": "value", "type": "int512"}],
    "stateMutability": "view",
    "type": "function"
  },
  {
    "inputs": [{"name": "value", "type": "address"}],
    "name": "storeAddress",
    "outputs": [],
    "stateMutability": "nonpayable",
    "type": "function"
  }
]`

	re := newEmbeddedWeb3(t)
	contractAddress := "Q" + strings.Repeat("0", common.AddressLength*2)

	minimumWord := "8" + strings.Repeat("0", uint512.WordBytes*2-1)
	minimumValue := "-0x" + minimumWord
	negativeOneWord := strings.Repeat("f", uint512.WordBytes*2)
	negativeOneData := "0x" + methodSelector("storeInt(int512)") + negativeOneWord
	minimumData := "0x" + methodSelector("storeInt(int512)") + minimumWord
	negativeOneOutput := "0x" + negativeOneWord
	minimumOutput := "0x" + minimumWord

	lowerAddress := "Q" + strings.Repeat("a", common.AddressLength*2)
	address, err := common.NewAddressFromString(lowerAddress)
	if err != nil {
		t.Fatal(err)
	}
	checksumAddress := address.Hex()
	invalidChecksum := []byte(checksumAddress)
	if invalidChecksum[1] == 'a' {
		invalidChecksum[1] = 'A'
	} else {
		invalidChecksum[1] = 'a'
	}
	addressData := "0x" + methodSelector("storeAddress(address)") + strings.TrimPrefix(lowerAddress, "Q")

	script := fmt.Sprintf(`
var negativeOneData = contract.storeInt.getData(-1);
var minimumData = contract.storeInt.getData(%q);
var addressData = contract.storeAddress.getData(%q);

currentOutput = %q;
var negativeOne = contract.loadInt().toString(16);

currentOutput = %q;
var minimum = contract.loadInt().toString(16);

var rejectsInvalidAddress = false;
try {
  contract.storeAddress.getData("Q1234");
} catch (err) {
  rejectsInvalidAddress = true;
}
var rejectsInvalidChecksum = false;
try {
  contract.storeAddress.getData(%q);
} catch (err) {
  rejectsInvalidChecksum = true;
}

JSON.stringify({
  negativeOneData: negativeOneData,
  minimumData: minimumData,
  negativeOne: negativeOne,
  minimum: minimum,
  addressData: addressData,
  rejectsInvalidAddress: rejectsInvalidAddress,
  rejectsInvalidChecksum: rejectsInvalidChecksum
});
`, minimumValue, checksumAddress, negativeOneOutput, minimumOutput, string(invalidChecksum))

	var got struct {
		NegativeOneData        string `json:"negativeOneData"`
		MinimumData            string `json:"minimumData"`
		NegativeOne            string `json:"negativeOne"`
		Minimum                string `json:"minimum"`
		AddressData            string `json:"addressData"`
		RejectsInvalidAddress  bool   `json:"rejectsInvalidAddress"`
		RejectsInvalidChecksum bool   `json:"rejectsInvalidChecksum"`
	}

	runWeb3ContractJSON(t, re, web3CallProvider, contractABI, contractAddress, script, &got)

	if got.NegativeOneData != negativeOneData || got.MinimumData != minimumData {
		t.Fatalf("signed calldata mismatch: %+v", got)
	}
	if got.NegativeOne != "-1" || got.Minimum != "-"+minimumWord || !strings.EqualFold(got.AddressData, addressData) || !got.RejectsInvalidAddress || !got.RejectsInvalidChecksum {
		t.Fatalf("signed decode or address validation mismatch: %+v", got)
	}
}

func TestEmbeddedWeb3ConstructorStateMutability(t *testing.T) {
	t.Parallel()

	re := newEmbeddedWeb3(t)
	script := `
web3.qrl.sendTransaction = function() { return "0x01"; };
web3.qrl.filter = function() { return {stopWatching: function() {}}; };

function acceptsValue(constructorAbi) {
  try {
    web3.qrl.contract([constructorAbi]).new({data: "0x", value: 1});
    return true;
  } catch (err) {
    return false;
  }
}

JSON.stringify({
  currentPayable: acceptsValue({inputs: [], stateMutability: "payable", type: "constructor"}),
  currentNonpayable: acceptsValue({inputs: [], stateMutability: "nonpayable", type: "constructor"})
});
`

	var got struct {
		CurrentPayable    bool `json:"currentPayable"`
		CurrentNonpayable bool `json:"currentNonpayable"`
	}

	runWeb3JSON(t, re, web3CallProvider, script, &got)

	if !got.CurrentPayable || got.CurrentNonpayable {
		t.Fatalf("constructor stateMutability mismatch: %+v", got)
	}
}
