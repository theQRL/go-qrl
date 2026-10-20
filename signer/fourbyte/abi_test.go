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

package fourbyte

import (
	"encoding/hex"
	"math/big"
	"reflect"
	"strings"
	"testing"

	"github.com/theQRL/go-qrl/accounts/abi"
	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/signer/core/apitypes"
)

// mustPack packs the given ABI method using the provided JSON spec and returns
// the resulting calldata. Used by fixture generation for tests that round-trip
// calldata through abi.Pack instead of hand-rolled hex.
func mustPack(t *testing.T, jsondata, name string, args ...any) string {
	t.Helper()
	a, err := abi.JSON(strings.NewReader(jsondata))
	if err != nil {
		t.Fatal(err)
	}
	packed, err := a.Pack(name, args...)
	if err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(packed)
}

func verify(t *testing.T, jsondata, calldata string, exp []any) {
	abispec, err := abi.JSON(strings.NewReader(jsondata))
	if err != nil {
		t.Fatal(err)
	}
	cd := common.Hex2Bytes(calldata)
	sigdata, argdata := cd[:4], cd[4:]
	method, err := abispec.MethodById(sigdata)
	if err != nil {
		t.Fatal(err)
	}
	data, err := method.Inputs.UnpackValues(argdata)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != len(exp) {
		t.Fatalf("Mismatched length, expected %d, got %d", len(exp), len(data))
	}
	for i, elem := range data {
		if !reflect.DeepEqual(elem, exp[i]) {
			t.Fatalf("Unpack error, arg %d, got %v, want %v", i, elem, exp[i])
		}
	}
}

func TestNewUnpacker(t *testing.T) {
	t.Parallel()
	type unpackTest struct {
		jsondata string
		calldata string
		exp      []any
	}
	addrRaw, _ := hex.DecodeString("00000133700000deadbeef00000000000000000000000133700000deadbeef0000000000000000000011223344556677")
	address := common.BytesToAddress(addrRaw)

	const (
		specF                 = `[{"type":"function","name":"f", "inputs":[{"type":"uint256"},{"type":"uint32[]"},{"type":"bytes10"},{"type":"bytes"}]}]`
		specSam               = `[{"type":"function","name":"sam","inputs":[{"type":"bytes"},{"type":"bool"},{"type":"uint256[]"}]}]`
		specSend              = `[{"type":"function","name":"send","inputs":[{"type":"uint256"}]}]`
		specCompareAndApprove = `[{"type":"function","name":"compareAndApprove","inputs":[{"type":"address"},{"type":"uint256"},{"type":"uint256"}]}]`
	)
	testcases := []unpackTest{
		{ // https://solidity.readthedocs.io/en/develop/abi-spec.html#use-of-dynamic-types
			specF,
			mustPack(t, specF, "f", big.NewInt(0x123), []uint32{0x456, 0x789}, [10]byte{49, 50, 51, 52, 53, 54, 55, 56, 57, 48}, []byte("Hello, world!")),
			[]any{
				big.NewInt(0x123),
				[]uint32{0x456, 0x789},
				[10]byte{49, 50, 51, 52, 53, 54, 55, 56, 57, 48},
				common.Hex2Bytes("48656c6c6f2c20776f726c6421"),
			},
		}, { // https://docs.soliditylang.org/en/develop/abi-spec.html#examples
			specSam,
			mustPack(t, specSam, "sam", []byte{0x64, 0x61, 0x76, 0x65}, true, []*big.Int{big.NewInt(1), big.NewInt(2), big.NewInt(3)}),
			[]any{
				[]byte{0x64, 0x61, 0x76, 0x65},
				true,
				[]*big.Int{big.NewInt(1), big.NewInt(2), big.NewInt(3)},
			},
		}, {
			specSend,
			mustPack(t, specSend, "send", big.NewInt(0x12)),
			[]any{big.NewInt(0x12)},
		}, {
			specCompareAndApprove,
			mustPack(t, specCompareAndApprove, "compareAndApprove", address, new(big.Int).SetBytes([]byte{0x00}), big.NewInt(0x1)),
			[]any{
				address,
				new(big.Int).SetBytes([]byte{0x00}),
				big.NewInt(0x1),
			},
		},
	}
	for _, c := range testcases {
		verify(t, c.jsondata, c.calldata, c.exp)
	}
}

func TestCalldataDecoding(t *testing.T) {
	t.Parallel()
	// send(uint256)                              : a52c101e
	// compareAndApprove(address,uint256,uint256) : 751e1079
	// issue(address[],uint256)                   : 42958b54
	jsondata := `
[
	{"type":"function","name":"send","inputs":[{"name":"a","type":"uint256"}]},
	{"type":"function","name":"compareAndApprove","inputs":[{"name":"a","type":"address"},{"name":"a","type":"uint256"},{"name":"a","type":"uint256"}]},
	{"type":"function","name":"issue","inputs":[{"name":"a","type":"address[]"},{"name":"a","type":"uint256"}]},
	{"type":"function","name":"sam","inputs":[{"name":"a","type":"bytes"},{"name":"a","type":"bool"},{"name":"a","type":"uint256[]"}]}
]`
	// Baseline: generate a correct calldata for each decoder success case via
	// abi.Pack, then derive the corresponding failure cases (truncations,
	// illegal bool values, mis-aligned lengths) by mutating those payloads.
	const (
		sendSpec    = `[{"type":"function","name":"send","inputs":[{"name":"a","type":"uint256"}]}]`
		compareSpec = `[{"type":"function","name":"compareAndApprove","inputs":[{"name":"a","type":"address"},{"name":"a","type":"uint256"},{"name":"a","type":"uint256"}]}]`
		issueSpec   = `[{"type":"function","name":"issue","inputs":[{"name":"a","type":"address[]"},{"name":"a","type":"uint256"}]}]`
		samSpec     = `[{"type":"function","name":"sam","inputs":[{"name":"a","type":"bytes"},{"name":"a","type":"bool"},{"name":"a","type":"uint256[]"}]}]`
	)
	addrA := common.Address{0xde, 0xad}
	addrB := common.Address{0xbe, 0xef}

	sendOK := mustPack(t, sendSpec, "send", big.NewInt(0x12))
	compareOK := mustPack(t, compareSpec, "compareAndApprove", common.Address{}, big.NewInt(0), big.NewInt(0))
	issueOK := mustPack(t, issueSpec, "issue", []common.Address{addrA, addrB}, big.NewInt(1))
	samOK := mustPack(t, samSpec, "sam", []byte{0x64, 0x61, 0x76, 0x65}, true, []*big.Int{big.NewInt(1), big.NewInt(2), big.NewInt(3)})
	// Tamper the bool slot in samOK to an illegal value (0x11) to produce a
	// decoder failure. samOK layout: 4-byte selector + [bytes offset][bool][uint256[] offset]...
	// The second head slot (bytes 132..260 of the hex, i.e. 8 + 128..8 + 256) is the bool.
	samBadBool := samOK[:8+128] + "00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000011" + samOK[8+256+128:]

	// Expected failures
	for i, hexdata := range []string{
		sendOK + "0000000000000000000000000000000000000000000000000000000000000042", // extra trailing bytes
		sendOK + "00",          // extra single byte
		sendOK[:len(sendOK)-2], // truncated final byte
		sendOK[:8],             // selector only
		"a52c10",
		"",
		// Too short
		"751e10790000000000000000000000000000000000000000000000000000000000000012",
		"751e1079FFffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
		// Not valid multiple of 64-byte slot width
		"deadbeef" + "00000000000000000000000000000000000000000000000000000000000000",
		// Too short 'issue'
		"42958b54" + "00000000000000000000000000000000000000000000000000000000000000120000000000000000000000000000000000000000000000000000000000000042",
		// Too short compareAndApprove (2 slots of 64 bytes instead of 3)
		"751e1079" +
			"00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000" +
			"00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
		// sam with illegal bool byte (tampered copy of samOK).
		samBadBool,
	} {
		_, err := parseCallData(common.Hex2Bytes(hexdata), jsondata)
		if err == nil {
			t.Errorf("test %d: expected decoding to fail: %s", i, hexdata)
		}
	}
	// Expected success
	for i, hexdata := range []string{
		samOK,
		sendOK,
		// sendOK with high bits set in the uint256 slot (still a valid encoding).
		"a52c101e" + "FFffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff" +
			"FFffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
		compareOK,
		issueOK,
	} {
		_, err := parseCallData(common.Hex2Bytes(hexdata), jsondata)
		if err != nil {
			t.Errorf("test %d: unexpected failure on input %s:\n %v (%d bytes) ", i, hexdata, err, len(common.Hex2Bytes(hexdata)))
		}
	}
}

func TestValidateCallDataRequiresABIWordAlignment(t *testing.T) {
	t.Parallel()

	selector := "send(uint256)"
	sendSpec := `[{"type":"function","name":"send","inputs":[{"name":"a","type":"uint256"}]}]`
	want := "Transaction data is not valid ABI (length should be a multiple of 64 (was 32))"
	tests := []struct {
		name string
		data []byte
		warn bool
	}{
		{
			name: "32 byte tail is not aligned",
			data: common.Hex2Bytes("a52c101e" + strings.Repeat("00", 32)),
			warn: true,
		},
		{
			name: "64 byte tail is aligned",
			data: common.Hex2Bytes(mustPack(t, sendSpec, "send", big.NewInt(0))),
			warn: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			messages := new(apitypes.ValidationMessages)
			newEmpty().ValidateCallData(&selector, tt.data, messages)

			var gotWarn bool
			for _, msg := range messages.Messages {
				if msg.Typ == apitypes.WARN && msg.Message == want {
					gotWarn = true
					break
				}
			}
			if gotWarn != tt.warn {
				t.Fatalf("alignment warning = %t, want %t in %#v", gotWarn, tt.warn, messages.Messages)
			}
		})
	}
}

func TestMaliciousABIStrings(t *testing.T) {
	t.Parallel()
	tests := []string{
		"func(uint256,uint256,[]uint256)",
		"func(uint256,uint256,uint256,)",
		"func(,uint256,uint256,uint256)",
	}
	data := common.Hex2Bytes("4401a6e40000000000000000000000000000000000000000000000000000000000000012")
	for i, tt := range tests {
		_, err := verifySelector(tt, data)
		if err == nil {
			t.Errorf("test %d: expected error for selector '%v'", i, tt)
		}
	}
}
