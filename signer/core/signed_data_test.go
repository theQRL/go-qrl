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

package core_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/theQRL/go-qrl/accounts/keystore"
	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/common/hexutil"
	"github.com/theQRL/go-qrl/common/math"
	"github.com/theQRL/go-qrl/crypto"
	"github.com/theQRL/go-qrl/signer/core"
	"github.com/theQRL/go-qrl/signer/core/apitypes"
)

var typesStandard = apitypes.Types{
	"QRLTypedDataDomain": {
		{
			Name: "name",
			Type: "string",
		},
		{
			Name: "version",
			Type: "string",
		},
		{
			Name: "chainId",
			Type: "uint256",
		},
		{
			Name: "verifyingContract",
			Type: "address",
		},
	},
	"Person": {
		{
			Name: "name",
			Type: "string",
		},
		{
			Name: "wallet",
			Type: "address",
		},
	},
	"Mail": {
		{
			Name: "from",
			Type: "Person",
		},
		{
			Name: "to",
			Type: "Person",
		},
		{
			Name: "contents",
			Type: "string",
		},
	},
}

var jsonTypedData = `
    {
      "types": {
        "QRLTypedDataDomain": [
          {
            "name": "name",
            "type": "string"
          },
          {
            "name": "version",
            "type": "string"
          },
          {
            "name": "chainId",
            "type": "uint256"
          },
          {
            "name": "verifyingContract",
            "type": "address"
          }
        ],
        "Person": [
          {
            "name": "name",
            "type": "string"
          },
          {
            "name": "test",
            "type": "uint8"
          },
          {
            "name": "wallet",
            "type": "address"
          }
        ],
        "Mail": [
          {
            "name": "from",
            "type": "Person"
          },
          {
            "name": "to",
            "type": "Person"
          },
          {
            "name": "contents",
            "type": "string"
          }
        ]
      },
      "primaryType": "Mail",
      "domain": {
        "name": "Ether Mail",
        "version": "1",
        "chainId": "1",
        "verifyingContract": "QCCCcCCCcCCCcCCCcCCCcCCCcCCCcCCCcCCCcCCCcCCCcCCCcCCCcCCCcCCCcCCCcCCCcCCCcCCCcCCCcCCCcCCCcCCCcCCCc99aabbccddeeff001122334455667788"
      },
      "message": {
        "from": {
          "name": "Cow",
		  "test": 3,
          "wallet": "QcD2a3d9F938E13CD947Ec05AbC7FE734Df8DD826cD2a3d9F938E13CD947Ec05AbC7FE734Df8DD826aabbccddeeff010299aabbccddeeff001122334455667788"
        },
        "to": {
          "name": "Bob",
          "wallet": "QbBbBBBBbbBBBbbbBbbBbbbbBBbBbbbbBbBbbBBbBbBbBBBBbbBBBbbbBbbBbbbbBBbBbbbbBbBbbBBbBaabbccddee01020399aabbccddeeff001122334455667788"
        },
        "contents": "Hello, Bob!"
      }
    }
`

const primaryType = "Mail"

// 64-byte QRL addresses — any 128-char hex works; these extend the original
// 20-byte fixtures to the full address width.
var domainStandard = apitypes.TypedDataDomain{
	Name:              "Ether Mail",
	Version:           "1",
	ChainId:           math.NewHexOrDecimal256(1),
	VerifyingContract: "QCCCcCCCcCCCcCCCcCCCcCCCcCCCcCCCcCCCcCCCcCCCcCCCcCCCcCCCcCCCcCCCcCCCcCCCcCCCcCCCcCCCcCCCcCCCcCCCc99aabbccddeeff001122334455667788",
	Salt:              "",
}

var messageStandard = map[string]any{
	"from": map[string]any{
		"name":   "Cow",
		"wallet": "QCD2a3d9F938E13CD947Ec05AbC7FE734Df8DD826CD2a3d9F938E13CD947Ec05AbC7FE734Df8DD826aabbccddeeff010299aabbccddeeff001122334455667788",
	},
	"to": map[string]any{
		"name":   "Bob",
		"wallet": "QbBbBBBBbbBBBbbbBbbBbbbbBBbBbbbbBbBbbBBbBbBbBBBBbbBBBbbbBbbBbbbbBBbBbbbbBbBbbBBbBaabbccddee01020399aabbccddeeff001122334455667788",
	},
	"contents": "Hello, Bob!",
}

var typedData = apitypes.TypedData{
	Types:       typesStandard,
	PrimaryType: primaryType,
	Domain:      domainStandard,
	Message:     messageStandard,
}

func TestSignData(t *testing.T) {
	t.Parallel()
	api, control := setup(t)
	//Create two accounts
	createAccount(control, api, t)
	createAccount(control, api, t)
	control.approveCh <- "1"
	list, err := api.List(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	a := common.NewMixedcaseAddress(list[0])

	control.approveCh <- "Y"
	control.inputCh <- "wrongpassword"
	signature, err := api.SignData(t.Context(), apitypes.TextPlain.Mime, a, hexutil.Encode([]byte("EHLO world")))
	if signature != nil {
		t.Errorf("Expected nil-data, got %x", signature)
	}
	if err != keystore.ErrDecrypt {
		t.Errorf("Expected ErrDecrypt! '%v'", err)
	}
	control.approveCh <- "No way"
	signature, err = api.SignData(t.Context(), apitypes.TextPlain.Mime, a, hexutil.Encode([]byte("EHLO world")))
	if signature != nil {
		t.Errorf("Expected nil-data, got %x", signature)
	}
	if err != core.ErrRequestDenied {
		t.Errorf("Expected ErrRequestDenied! '%v'", err)
	}
	// text/plain
	control.approveCh <- "Y"
	control.inputCh <- "a_long_password"
	signature, err = api.SignData(t.Context(), apitypes.TextPlain.Mime, a, hexutil.Encode([]byte("EHLO world")))
	if err != nil {
		t.Fatal(err)
	}
	if signature == nil || len(signature) != 4627 {
		t.Errorf("Expected 4627 byte ML-DSA-87 signature (got %d bytes)", len(signature))
	}
	// data/typed via SignTypeData
	control.approveCh <- "Y"
	control.inputCh <- "a_long_password"
	if signature, err = api.SignTypedData(t.Context(), a, typedData); err != nil {
		t.Fatal(err)
	} else if signature == nil || len(signature) != 4627 {
		t.Errorf("Expected 4627 byte ML-DSA-87 signature (got %d bytes)", len(signature))
	}
	wantHash := append([]byte(nil), control.lastSignDataRequest.Hash...)

	// data/typed via SignData / mimetype typed data
	control.approveCh <- "Y"
	control.inputCh <- "a_long_password"
	if typedDataJson, err := json.Marshal(typedData); err != nil {
		t.Fatal(err)
	} else if signature, err = api.SignData(t.Context(), apitypes.DataTyped.Mime, a, hexutil.Encode(typedDataJson)); err != nil {
		t.Fatal(err)
	} else if signature == nil || len(signature) != 4627 {
		t.Errorf("Expected 4627 byte ML-DSA-87 signature (got %d bytes)", len(signature))
	} else if haveHash := control.lastSignDataRequest.Hash; !bytes.Equal(haveHash, wantHash) {
		t.Fatalf("want hash %x, have hash %x", wantHash, haveHash)
	}
}

func TestDomainChainId(t *testing.T) {
	t.Parallel()
	withoutChainID := apitypes.TypedData{
		Types: apitypes.Types{
			"QRLTypedDataDomain": []apitypes.Type{
				{Name: "name", Type: "string"},
			},
		},
		Domain: apitypes.TypedDataDomain{
			Name: "test",
		},
	}

	if _, ok := withoutChainID.Domain.Map()["chainId"]; ok {
		t.Errorf("Expected the chainId key to not be present in the domain map")
	}
	// should encode successfully
	if _, err := withoutChainID.HashStruct("QRLTypedDataDomain", withoutChainID.Domain.Map()); err != nil {
		t.Errorf("Expected the typedData to encode the domain successfully, got %v", err)
	}
	withChainID := apitypes.TypedData{
		Types: apitypes.Types{
			"QRLTypedDataDomain": []apitypes.Type{
				{Name: "name", Type: "string"},
				{Name: "chainId", Type: "uint256"},
			},
		},
		Domain: apitypes.TypedDataDomain{
			Name:    "test",
			ChainId: math.NewHexOrDecimal256(1),
		},
	}

	if _, ok := withChainID.Domain.Map()["chainId"]; !ok {
		t.Errorf("Expected the chainId key be present in the domain map")
	}
	// should encode successfully
	if _, err := withChainID.HashStruct("QRLTypedDataDomain", withChainID.Domain.Map()); err != nil {
		t.Errorf("Expected the typedData to encode the domain successfully, got %v", err)
	}
}

func TestHashStruct(t *testing.T) {
	t.Parallel()
	hash, err := typedData.HashStruct(typedData.PrimaryType, typedData.Message)
	if err != nil {
		t.Fatal(err)
	}
	mainHash := fmt.Sprintf("0x%s", common.Bytes2Hex(hash))
	if mainHash != "0x77abdbfc62ca407545b8fe30a4592c7f18b9624308e9311931582df2cee8e1a9" {
		t.Errorf("Expected different hashStruct result (got %s)", mainHash)
	}

	hash, err = typedData.HashStruct("QRLTypedDataDomain", typedData.Domain.Map())
	if err != nil {
		t.Error(err)
	}
	domainHash := fmt.Sprintf("0x%s", common.Bytes2Hex(hash))
	if domainHash != "0x79e5614c24184cf7bc90065f1f27e9a5f64ecccee56435164a64ed3c98dfd3a1" {
		t.Errorf("Expected different domain hashStruct result (got %s)", domainHash)
	}
}

func TestTypedDataAndHashGolden(t *testing.T) {
	t.Parallel()
	typedData := apitypes.TypedData{
		Types: apitypes.Types{
			"QRLTypedDataDomain": {
				{Name: "name", Type: "string"},
			},
			"WideValues": {
				{Name: "amount", Type: "uint512"},
				{Name: "payload", Type: "bytes64"},
			},
		},
		PrimaryType: "WideValues",
		Domain: apitypes.TypedDataDomain{
			Name: "QRL VM64 Golden",
		},
		Message: apitypes.TypedDataMessage{
			"amount":  "0x" + strings.Repeat("fedcba9876543210", 8),
			"payload": "0x" + strings.Repeat("0123456789abcdef", 8),
		},
	}

	digest, _, err := apitypes.TypedDataAndHash(typedData)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := hexutil.Encode(digest), "0x054b04b5b0976d8b58cb06fb10c1100af45cf46488d16842897e6b2a81ed6ed3"; got != want {
		t.Fatalf("unexpected VM64 typed-data digest: got %s, want %s", got, want)
	}
}

func TestEncodeType(t *testing.T) {
	t.Parallel()
	domainTypeEncoding := string(typedData.EncodeType("QRLTypedDataDomain"))
	if domainTypeEncoding != "QRLTypedDataDomain(string name,string version,uint256 chainId,address verifyingContract)" {
		t.Errorf("Expected different encodeType result (got %s)", domainTypeEncoding)
	}

	mailTypeEncoding := string(typedData.EncodeType(typedData.PrimaryType))
	if mailTypeEncoding != "Mail(Person from,Person to,string contents)Person(string name,address wallet)" {
		t.Errorf("Expected different encodeType result (got %s)", mailTypeEncoding)
	}

	emptyTypedData := apitypes.TypedData{Types: apitypes.Types{"Empty": {}}}
	if emptyTypeEncoding := string(emptyTypedData.EncodeType("Empty")); emptyTypeEncoding != "Empty()" {
		t.Errorf("Expected empty type encoding Empty() (got %s)", emptyTypeEncoding)
	}
}

func TestTypeHash(t *testing.T) {
	t.Parallel()
	mailTypeHash := fmt.Sprintf("0x%s", common.Bytes2Hex(typedData.TypeHash(typedData.PrimaryType)))
	if mailTypeHash != "0xa0cedeb2dc280ba39b857546d74f5549c3a1d7bdc2dd96bf881f76108e23dac2" {
		t.Errorf("Expected different typeHash result (got %s)", mailTypeHash)
	}
}

func TestEncodeData(t *testing.T) {
	t.Parallel()
	hash, err := typedData.EncodeData(typedData.PrimaryType, typedData.Message, 0)
	if err != nil {
		t.Fatal(err)
	}
	dataEncoding := fmt.Sprintf("0x%s", common.Bytes2Hex(hash))
	if dataEncoding != "0xa0cedeb2dc280ba39b857546d74f5549c3a1d7bdc2dd96bf881f76108e23dac200000000000000000000000000000000000000000000000000000000000000005ff9606cfd3cf02dff608d92b16a99bf08b087ba3f7d50de6c94668c4d911863000000000000000000000000000000000000000000000000000000000000000094329683ebe350d70c5560224b7ce95ef5b12d253a28e2fc0f573da4570a44fc0000000000000000000000000000000000000000000000000000000000000000b5aadf3154a261abdd9086fc627b61efca26ae5702701d05cd2305f7c52a2fc80000000000000000000000000000000000000000000000000000000000000000" {
		t.Errorf("Expected different encodeData result (got %s)", dataEncoding)
	}
}

func TestFormatter(t *testing.T) {
	t.Parallel()
	var d apitypes.TypedData
	err := json.Unmarshal([]byte(jsonTypedData), &d)
	if err != nil {
		t.Fatalf("unmarshalling failed '%v'", err)
	}
	formatted, _ := d.Format()
	for _, item := range formatted {
		t.Logf("'%v'\n", item.Pprint(0))
	}

	j, _ := json.Marshal(formatted)
	t.Logf("'%v'\n", string(j))
}

func sign(typedData apitypes.TypedData) ([]byte, []byte, error) {
	domainSeparator, err := typedData.HashStruct("QRLTypedDataDomain", typedData.Domain.Map())
	if err != nil {
		return nil, nil, err
	}
	typedDataHash, err := typedData.HashStruct(typedData.PrimaryType, typedData.Message)
	if err != nil {
		return nil, nil, err
	}
	rawData := fmt.Appendf(nil, "\x19\x01%s%s", string(domainSeparator), string(typedDataHash))
	sighash := crypto.Keccak256(rawData)
	return typedDataHash, sighash, nil
}

func TestJsonFiles(t *testing.T) {
	t.Parallel()
	testfiles, err := os.ReadDir("testdata/")
	if err != nil {
		t.Fatalf("failed reading files: %v", err)
	}
	for i, fInfo := range testfiles {
		if !strings.HasSuffix(fInfo.Name(), "json") {
			continue
		}
		expectedFailure := strings.HasPrefix(fInfo.Name(), "expfail")
		data, err := os.ReadFile(filepath.Join("testdata", fInfo.Name()))
		if err != nil {
			t.Errorf("Failed to read file %v: %v", fInfo.Name(), err)
			continue
		}
		var typedData apitypes.TypedData
		err = json.Unmarshal(data, &typedData)
		if err != nil {
			t.Errorf("Test %d, file %v, json unmarshalling failed: %v", i, fInfo.Name(), err)
			continue
		}
		_, _, err = sign(typedData)
		t.Logf("Error %v\n", err)
		if err != nil && !expectedFailure {
			t.Errorf("Test %d failed, file %v: %v", i, fInfo.Name(), err)
		}
		if expectedFailure && err == nil {
			t.Errorf("Test %d succeeded (expected failure), file %v: %v", i, fInfo.Name(), err)
		}
	}
}

// TestFuzzerFiles tests some files that have been found by fuzzing to cause
// crashes or hangs.
func TestFuzzerFiles(t *testing.T) {
	t.Parallel()
	corpusdir := filepath.Join("testdata", "fuzzing")
	testfiles, err := os.ReadDir(corpusdir)
	if err != nil {
		t.Fatalf("failed reading files: %v", err)
	}
	verbose := false
	for i, fInfo := range testfiles {
		data, err := os.ReadFile(filepath.Join(corpusdir, fInfo.Name()))
		if err != nil {
			t.Errorf("Failed to read file %v: %v", fInfo.Name(), err)
			continue
		}
		var typedData apitypes.TypedData
		err = json.Unmarshal(data, &typedData)
		if err != nil {
			t.Errorf("Test %d, file %v, json unmarshalling failed: %v", i, fInfo.Name(), err)
			continue
		}
		_, err = typedData.EncodeData("QRLTypedDataDomain", typedData.Domain.Map(), 1)
		if verbose && err != nil {
			t.Logf("%d, EncodeData[1] err: %v\n", i, err)
		}
		_, err = typedData.EncodeData(typedData.PrimaryType, typedData.Message, 1)
		if verbose && err != nil {
			t.Logf("%d, EncodeData[2] err: %v\n", i, err)
		}
		typedData.Format()
	}
}

var complexTypedData = `
{
    "types": {
        "QRLTypedDataDomain": [
            {
                "name": "chainId",
                "type": "uint256"
            },
            {
                "name": "name",
                "type": "string"
            },
            {
                "name": "verifyingContract",
                "type": "address"
            },
            {
                "name": "version",
                "type": "string"
            }
        ],
        "Action": [
            {
                "name": "action",
                "type": "string"
            },
            {
                "name": "params",
                "type": "string"
            }
        ],
        "Cell": [
            {
                "name": "capacity",
                "type": "string"
            },
            {
                "name": "lock",
                "type": "string"
            },
            {
                "name": "type",
                "type": "string"
            },
            {
                "name": "data",
                "type": "string"
            },
            {
                "name": "extraData",
                "type": "string"
            }
        ],
        "Transaction": [
            {
                "name": "DAS_MESSAGE",
                "type": "string"
            },
            {
                "name": "inputsCapacity",
                "type": "string"
            },
            {
                "name": "outputsCapacity",
                "type": "string"
            },
            {
                "name": "fee",
                "type": "string"
            },
            {
                "name": "action",
                "type": "Action"
            },
            {
                "name": "inputs",
                "type": "Cell[]"
            },
            {
                "name": "outputs",
                "type": "Cell[]"
            },
            {
                "name": "digest",
                "type": "bytes32"
            }
        ]
    },
    "primaryType": "Transaction",
    "domain": {
        "chainId": "56",
        "name": "da.systems",
        "verifyingContract": "Q00000000000000000000000000000000202107220000000000000000000000000000000020210722112233445566778899aabbccddeeff001122334455667788",
        "version": "1"
    },
    "message": {
        "DAS_MESSAGE": "SELL mobcion.bit FOR 100000 CKB",
        "inputsCapacity": "1216.9999 CKB",
        "outputsCapacity": "1216.9998 CKB",
        "fee": "0.0001 CKB",
        "digest": "0x53a6c0f19ec281604607f5d6817e442082ad1882bef0df64d84d3810dae561eb",
        "action": {
            "action": "start_account_sale",
            "params": "0x00"
        },
        "inputs": [
            {
                "capacity": "218 CKB",
                "lock": "das-lock,0x01,0x051c152f77f8efa9c7c6d181cc97ee67c165c506...",
                "type": "account-cell-type,0x01,0x",
                "data": "{ account: mobcion.bit, expired_at: 1670913958 }",
                "extraData": "{ status: 0, records_hash: 0x55478d76900611eb079b22088081124ed6c8bae21a05dd1a0d197efcc7c114ce }"
            }
        ],
        "outputs": [
            {
                "capacity": "218 CKB",
                "lock": "das-lock,0x01,0x051c152f77f8efa9c7c6d181cc97ee67c165c506...",
                "type": "account-cell-type,0x01,0x",
                "data": "{ account: mobcion.bit, expired_at: 1670913958 }",
                "extraData": "{ status: 1, records_hash: 0x55478d76900611eb079b22088081124ed6c8bae21a05dd1a0d197efcc7c114ce }"
            },
            {
                "capacity": "201 CKB",
                "lock": "das-lock,0x01,0x051c152f77f8efa9c7c6d181cc97ee67c165c506...",
                "type": "account-sale-cell-type,0x01,0x",
                "data": "0x1209460ef3cb5f1c68ed2c43a3e020eec2d9de6e...",
                "extraData": ""
            }
        ]
    }
}
`

func TestComplexTypedData(t *testing.T) {
	t.Parallel()
	var td apitypes.TypedData
	err := json.Unmarshal([]byte(complexTypedData), &td)
	if err != nil {
		t.Fatalf("unmarshalling failed '%v'", err)
	}
	_, sighash, err := sign(td)
	if err != nil {
		t.Fatal(err)
	}
	expSigHash := common.FromHex("0x428ff10ebad404fcbdc5f2127cf6d09a563032d73d21908d15ac773f278fe78f")
	if !bytes.Equal(expSigHash, sighash) {
		t.Fatalf("Error, got %x, wanted %x", sighash, expSigHash)
	}
}

var complexTypedDataLCRefType = `
{
    "types": {
        "QRLTypedDataDomain": [
            {
                "name": "chainId",
                "type": "uint256"
            },
            {
                "name": "name",
                "type": "string"
            },
            {
                "name": "verifyingContract",
                "type": "address"
            },
            {
                "name": "version",
                "type": "string"
            }
        ],
        "Action": [
            {
                "name": "action",
                "type": "string"
            },
            {
                "name": "params",
                "type": "string"
            }
        ],
        "cCell": [
            {
                "name": "capacity",
                "type": "string"
            },
            {
                "name": "lock",
                "type": "string"
            },
            {
                "name": "type",
                "type": "string"
            },
            {
                "name": "data",
                "type": "string"
            },
            {
                "name": "extraData",
                "type": "string"
            }
        ],
        "Transaction": [
            {
                "name": "DAS_MESSAGE",
                "type": "string"
            },
            {
                "name": "inputsCapacity",
                "type": "string"
            },
            {
                "name": "outputsCapacity",
                "type": "string"
            },
            {
                "name": "fee",
                "type": "string"
            },
            {
                "name": "action",
                "type": "Action"
            },
            {
                "name": "inputs",
                "type": "cCell[]"
            },
            {
                "name": "outputs",
                "type": "cCell[]"
            },
            {
                "name": "digest",
                "type": "bytes32"
            }
        ]
    },
    "primaryType": "Transaction",
    "domain": {
        "chainId": "56",
        "name": "da.systems",
        "verifyingContract": "Q00000000000000000000000000000000202107220000000000000000000000000000000020210722112233445566778899aabbccddeeff001122334455667788",
        "version": "1"
    },
    "message": {
        "DAS_MESSAGE": "SELL mobcion.bit FOR 100000 CKB",
        "inputsCapacity": "1216.9999 CKB",
        "outputsCapacity": "1216.9998 CKB",
        "fee": "0.0001 CKB",
        "digest": "0x53a6c0f19ec281604607f5d6817e442082ad1882bef0df64d84d3810dae561eb",
        "action": {
            "action": "start_account_sale",
            "params": "0x00"
        },
        "inputs": [
            {
                "capacity": "218 CKB",
                "lock": "das-lock,0x01,0x051c152f77f8efa9c7c6d181cc97ee67c165c506...",
                "type": "account-cell-type,0x01,0x",
                "data": "{ account: mobcion.bit, expired_at: 1670913958 }",
                "extraData": "{ status: 0, records_hash: 0x55478d76900611eb079b22088081124ed6c8bae21a05dd1a0d197efcc7c114ce }"
            }
        ],
        "outputs": [
            {
                "capacity": "218 CKB",
                "lock": "das-lock,0x01,0x051c152f77f8efa9c7c6d181cc97ee67c165c506...",
                "type": "account-cell-type,0x01,0x",
                "data": "{ account: mobcion.bit, expired_at: 1670913958 }",
                "extraData": "{ status: 1, records_hash: 0x55478d76900611eb079b22088081124ed6c8bae21a05dd1a0d197efcc7c114ce }"
            },
            {
                "capacity": "201 CKB",
                "lock": "das-lock,0x01,0x051c152f77f8efa9c7c6d181cc97ee67c165c506...",
                "type": "account-sale-cell-type,0x01,0x",
                "data": "0x1209460ef3cb5f1c68ed2c43a3e020eec2d9de6e...",
                "extraData": ""
            }
        ]
    }
}
`

func TestComplexTypedDataWithLowercaseReftype(t *testing.T) {
	t.Parallel()
	var td apitypes.TypedData
	err := json.Unmarshal([]byte(complexTypedDataLCRefType), &td)
	if err != nil {
		t.Fatalf("unmarshalling failed '%v'", err)
	}
	_, sighash, err := sign(td)
	if err != nil {
		t.Fatal(err)
	}
	expSigHash := common.FromHex("0x588de30f07eec7adda5c0772d92b4e8dbf6e04697be2ffba9f7866c10374dd14")
	if !bytes.Equal(expSigHash, sighash) {
		t.Fatalf("Error, got %x, wanted %x", sighash, expSigHash)
	}
}
