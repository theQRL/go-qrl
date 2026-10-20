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
	"github.com/theQRL/go-qrl/crypto"
)

func TestEmbeddedWeb3EventsUseVM64Topics(t *testing.T) {
	t.Parallel()

	const contractABI = `[
  {
    "anonymous": false,
    "inputs": [
      {"indexed": true, "name": "from", "type": "address"},
      {"indexed": true, "name": "label", "type": "string"},
      {"indexed": true, "name": "payload", "type": "bytes"},
      {"indexed": false, "name": "amount", "type": "uint512"}
    ],
    "name": "Transfer",
    "type": "event"
  }
]`

	re := newEmbeddedWeb3(t)
	contractAddress := "Q" + strings.Repeat("0", common.AddressLength*2)
	indexedAddress := "Q" + strings.Repeat("a", common.AddressLength*2)
	checksummedAddress, err := common.NewAddressFromString(indexedAddress)
	if err != nil {
		t.Fatal(err)
	}
	eventID := crypto.Keccak256Hash([]byte("Transfer(address,string,bytes,uint512)"))
	signatureTopic := common.HashToLogTopic(eventID).Hex()
	addressTopic := "0x" + strings.Repeat("a", common.LogTopicLength*2)
	labelTopic := common.HashToLogTopic(crypto.Keccak256Hash([]byte("hello"))).Hex()
	worldTopic := common.HashToLogTopic(crypto.Keccak256Hash([]byte("world"))).Hex()
	payloadTopic := common.HashToLogTopic(crypto.Keccak256Hash([]byte{0xab, 0xcd})).Hex()
	emptyPayloadTopic := common.HashToLogTopic(crypto.Keccak256Hash(nil)).Hex()
	amountWord := abiWordHex(2)

	script := fmt.Sprintf(`
var filter = contract.Transfer({from: %q, label: "hello", payload: "0xabcd"});
contract.Transfer({payload: "0x"});
contract.Transfer({label: ["hello", "world"], payload: ["0xabcd", "0x"]});

var log = {
  address: %q,
  topics: [%q, %q, %q, %q],
  data: "0x%s",
  blockNumber: "0x1",
  transactionIndex: "0x0",
  logIndex: "0x0"
};

var decoded = filter.formatter(JSON.parse(JSON.stringify(log)));
var allEventsDecoded = contract.allEvents().formatter(JSON.parse(JSON.stringify(log)));

JSON.stringify({
  topics: captured[0].topics,
  emptyPayloadTopic: captured[1].topics[3],
  labelAlternatives: captured[2].topics[2],
  payloadAlternatives: captured[2].topics[3],
  event: decoded.event,
  from: decoded.args.from,
  label: decoded.args.label,
  payload: decoded.args.payload,
  amount: decoded.args.amount.toString(10),
  allEventsEvent: allEventsDecoded.event,
  acceptsSignatureTopic: web3._extend.utils.isTopic(%q),
  acceptsRawEventID: web3._extend.utils.isTopic(%q)
});
`, indexedAddress, contractAddress, signatureTopic, addressTopic, labelTopic, payloadTopic, amountWord, signatureTopic, "0x"+common.Bytes2Hex(eventID[:]))

	var got struct {
		Topics                []string `json:"topics"`
		EmptyPayloadTopic     string   `json:"emptyPayloadTopic"`
		LabelAlternatives     []string `json:"labelAlternatives"`
		PayloadAlternatives   []string `json:"payloadAlternatives"`
		Event                 string   `json:"event"`
		From                  string   `json:"from"`
		Label                 string   `json:"label"`
		Payload               string   `json:"payload"`
		Amount                string   `json:"amount"`
		AllEventsEvent        string   `json:"allEventsEvent"`
		AcceptsSignatureTopic bool     `json:"acceptsSignatureTopic"`
		AcceptsRawEventID     bool     `json:"acceptsRawEventID"`
	}

	runWeb3ContractJSON(t, re, web3FilterProvider, contractABI, contractAddress, script, &got)

	wantTopics := []string{signatureTopic, addressTopic, labelTopic, payloadTopic}
	if !slices.Equal(got.Topics, wantTopics) {
		t.Fatalf("event topics mismatch:\nhave %v\nwant %v", got.Topics, wantTopics)
	}
	if got.EmptyPayloadTopic != emptyPayloadTopic {
		t.Fatalf("empty bytes topic mismatch: have %s, want %s", got.EmptyPayloadTopic, emptyPayloadTopic)
	}
	if want := []string{labelTopic, worldTopic}; !slices.Equal(got.LabelAlternatives, want) {
		t.Fatalf("indexed string alternatives mismatch: have %v, want %v", got.LabelAlternatives, want)
	}
	if want := []string{payloadTopic, emptyPayloadTopic}; !slices.Equal(got.PayloadAlternatives, want) {
		t.Fatalf("indexed bytes alternatives mismatch: have %v, want %v", got.PayloadAlternatives, want)
	}
	if got.Event != "Transfer" ||
		got.AllEventsEvent != "Transfer" ||
		got.From != checksummedAddress.Hex() ||
		got.Label != labelTopic ||
		got.Payload != payloadTopic ||
		got.Amount != "2" {
		t.Fatalf("decoded event mismatch: %+v", got)
	}
	if !got.AcceptsSignatureTopic || got.AcceptsRawEventID {
		t.Fatalf("topic width validation mismatch: signature=%t rawEventID=%t", got.AcceptsSignatureTopic, got.AcceptsRawEventID)
	}
}

func TestEmbeddedWeb3IndexedTopicsUseVM64Encoding(t *testing.T) {
	t.Parallel()

	const contractABI = `[
  {
    "anonymous": false,
    "inputs": [{"indexed": true, "name": "value", "type": "uint512"}],
    "name": "Unsigned",
    "type": "event"
  },
  {
    "anonymous": false,
    "inputs": [{"indexed": true, "name": "value", "type": "int512"}],
    "name": "Signed",
    "type": "event"
  },
  {
    "anonymous": false,
    "inputs": [{"indexed": true, "name": "value", "type": "bool"}],
    "name": "Flag",
    "type": "event"
  },
  {
    "anonymous": false,
    "inputs": [{"indexed": true, "name": "value", "type": "bytes32"}],
    "name": "FixedBytes32",
    "type": "event"
  },
  {
    "anonymous": false,
    "inputs": [{"indexed": true, "name": "value", "type": "bytes64"}],
    "name": "FixedBytes64",
    "type": "event"
  }
]`

	re := newEmbeddedWeb3(t)
	contractAddress := "Q" + strings.Repeat("0", common.AddressLength*2)

	fixedBytes32 := strings.Repeat("ab", 32)
	fixedBytes64 := strings.Repeat("cd", common.LogTopicLength)

	script := fmt.Sprintf(`
contract.Unsigned({value: 2});
contract.Signed({value: -2});

contract.Flag({value: true});
contract.Flag({value: false});

contract.FixedBytes32({value: "0x%s"});
contract.FixedBytes64({value: "0x%s"});

JSON.stringify(captured.map(function (filter) {
  return filter.topics[1];
}));
`, fixedBytes32, fixedBytes64)

	var got []string
	runWeb3ContractJSON(t, re, web3FilterProvider, contractABI, contractAddress, script, &got)

	want := []string{
		"0x" + abiWordHex(2),
		"0x" + strings.Repeat("f", common.LogTopicLength*2-1) + "e",
		"0x" + abiWordHex(1),
		"0x" + abiWordHex(0),
		"0x" + fixedBytes32 + strings.Repeat("0", (common.LogTopicLength-32)*2),
		"0x" + fixedBytes64,
	}
	if !slices.Equal(got, want) {
		t.Fatalf("event topic alignment mismatch:\nhave %v\nwant %v", got, want)
	}
}
