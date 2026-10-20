// Copyright 2017 The go-ethereum Authors
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

package abi

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math"
	"math/big"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/theQRL/go-qrl/common"
)

func BenchmarkUnpack(b *testing.B) {
	testCases := []struct {
		def    string
		packed string
	}{
		{
			def:    `[{"type": "uint32"}]`,
			packed: "0000000000000000000000000000000000000000000000000000000000000001",
		},
		{
			def: `[{"type": "uint32[]"}]`,
			packed: "0000000000000000000000000000000000000000000000000000000000000020" +
				"0000000000000000000000000000000000000000000000000000000000000002" +
				"0000000000000000000000000000000000000000000000000000000000000001" +
				"0000000000000000000000000000000000000000000000000000000000000002",
		},
	}
	for i, test := range testCases {
		b.Run(strconv.Itoa(i), func(b *testing.B) {
			def := fmt.Sprintf(`[{ "name" : "method", "type": "function", "outputs": %s}]`, test.def)
			abi, err := JSON(strings.NewReader(def))
			if err != nil {
				b.Fatalf("invalid ABI definition %s: %v", def, err)
			}
			encb, err := hex.DecodeString(test.packed)
			if err != nil {
				b.Fatalf("invalid hex %s: %v", test.packed, err)
			}

			var result any
			for b.Loop() {
				result, _ = abi.Unpack("method", encb)
			}
			_ = result
		})
	}
}

// TestUnpack tests the general pack/unpack tests in packing_test.go
func TestUnpack(t *testing.T) {
	t.Parallel()
	for i, test := range packUnpackTests {
		t.Run(strconv.Itoa(i)+" "+test.def, func(t *testing.T) {
			//Unpack
			def := fmt.Sprintf(`[{ "name" : "method", "type": "function", "outputs": %s}]`, test.def)
			abi, err := JSON(strings.NewReader(def))
			if err != nil {
				t.Fatalf("invalid ABI definition %s: %v", def, err)
			}
			encb, err := hex.DecodeString(test.packed)
			if err != nil {
				t.Fatalf("invalid hex %s: %v", test.packed, err)
			}
			out, err := abi.Unpack("method", encb)
			if err != nil {
				t.Errorf("test %d (%v) failed: %v", i, test.def, err)
				return
			}
			want := test.values()
			if len(out) != len(want) {
				t.Errorf("test %d (%v) failed: expected %d values, got %d", i, test.def, len(want), len(out))
				return
			}
			for j := range want {
				if !reflect.DeepEqual(want[j], ConvertType(out[j], want[j])) {
					t.Errorf("test %d (%v) value %d failed: expected %v, got %v", i, test.def, j, want[j], out[j])
				}
			}
		})
	}
}

type unpackTest struct {
	def  string // ABI definition JSON
	enc  string // qrvm return data
	want any    // the expected output
	err  string // empty or error if expected
}

func (test unpackTest) checkError(err error) error {
	if err != nil {
		if len(test.err) == 0 {
			return fmt.Errorf("expected no err but got: %v", err)
		} else if err.Error() != test.err {
			return fmt.Errorf("expected err: '%v' got err: %q", test.err, err)
		}
	} else if len(test.err) > 0 {
		return fmt.Errorf("expected err: %v but got none", test.err)
	}
	return nil
}

// z32 is a 32-byte (64 hex chars) zero prefix used to left-pad the legacy
// 32-byte slot hex fixtures below to the 64-byte slot width.
const z32 = "0000000000000000000000000000000000000000000000000000000000000000"

var unpackTests = []unpackTest{
	// Bools
	{
		def:  `[{ "type": "bool" }]`,
		enc:  z32 + "0000000000000000000000000000000000000000000000000001000000000001",
		want: false,
		err:  "abi: improperly encoded boolean value",
	},
	{
		def:  `[{ "type": "bool" }]`,
		enc:  z32 + "0000000000000000000000000000000000000000000000000000000000000003",
		want: false,
		err:  "abi: improperly encoded boolean value",
	},
	// Integers
	{
		def:  `[{"type": "uint32"}]`,
		enc:  z32 + "0000000000000000000000000000000000000000000000000000000000000001",
		want: uint16(0),
		err:  "abi: cannot unmarshal uint32 in to uint16",
	},
	{
		def:  `[{"type": "uint17"}]`,
		enc:  z32 + "0000000000000000000000000000000000000000000000000000000000000001",
		want: uint16(0),
		err:  "abi: cannot unmarshal *big.Int in to uint16",
	},
	{
		def:  `[{"type": "int32"}]`,
		enc:  z32 + "0000000000000000000000000000000000000000000000000000000000000001",
		want: int16(0),
		err:  "abi: cannot unmarshal int32 in to int16",
	},
	{
		def:  `[{"type": "int17"}]`,
		enc:  z32 + "0000000000000000000000000000000000000000000000000000000000000001",
		want: int16(0),
		err:  "abi: cannot unmarshal *big.Int in to int16",
	},
	{
		// Dynamic bytes: offset (0x40, past the single-head slot),
		// length (0x20 = 32), then 32 bytes of data right-padded to the
		// 64-byte boundary.
		def: `[{"type": "bytes"}]`,
		enc: z32 + "0000000000000000000000000000000000000000000000000000000000000040" +
			z32 + "0000000000000000000000000000000000000000000000000000000000000020" +
			"0100000000000000000000000000000000000000000000000000000000000000" + z32,
		want: [32]byte{1},
	},
	{
		// bytes32 is static, but destination is []byte — expects a type
		// mismatch error. Only the leading 64-byte slot is consumed for
		// static decoding; keep the rest as benign padding.
		def: `[{"type": "bytes32"}]`,
		enc: "0100000000000000000000000000000000000000000000000000000000000000" + z32 +
			z32 + z32,
		want: []byte(nil),
		err:  "abi: cannot unmarshal [32]uint8 in to []uint8",
	},
	{
		def: `[{"name":"___","type":"int256"}]`,
		enc: z32 + "0000000000000000000000000000000000000000000000000000000000000001" +
			z32 + "0000000000000000000000000000000000000000000000000000000000000002",
		want: struct {
			IntOne *big.Int
			Intone *big.Int
		}{IntOne: big.NewInt(1)},
	},
	{
		def: `[{"name":"int_one","type":"int256"},{"name":"IntOne","type":"int256"}]`,
		enc: z32 + "0000000000000000000000000000000000000000000000000000000000000001" +
			z32 + "0000000000000000000000000000000000000000000000000000000000000002",
		want: struct {
			Int1 *big.Int
			Int2 *big.Int
		}{},
		err: "abi: multiple outputs mapping to the same struct field 'IntOne'",
	},
	{
		def: `[{"name":"int","type":"int256"},{"name":"Int","type":"int256"}]`,
		enc: z32 + "0000000000000000000000000000000000000000000000000000000000000001" +
			z32 + "0000000000000000000000000000000000000000000000000000000000000002",
		want: struct {
			Int1 *big.Int
			Int2 *big.Int
		}{},
		err: "abi: multiple outputs mapping to the same struct field 'Int'",
	},
	{
		def: `[{"name":"int","type":"int256"},{"name":"_int","type":"int256"}]`,
		enc: z32 + "0000000000000000000000000000000000000000000000000000000000000001" +
			z32 + "0000000000000000000000000000000000000000000000000000000000000002",
		want: struct {
			Int1 *big.Int
			Int2 *big.Int
		}{},
		err: "abi: multiple outputs mapping to the same struct field 'Int'",
	},
	{
		def: `[{"name":"Int","type":"int256"},{"name":"_int","type":"int256"}]`,
		enc: z32 + "0000000000000000000000000000000000000000000000000000000000000001" +
			z32 + "0000000000000000000000000000000000000000000000000000000000000002",
		want: struct {
			Int1 *big.Int
			Int2 *big.Int
		}{},
		err: "abi: multiple outputs mapping to the same struct field 'Int'",
	},
	{
		def: `[{"name":"Int","type":"int256"},{"name":"_","type":"int256"}]`,
		enc: z32 + "0000000000000000000000000000000000000000000000000000000000000001" +
			z32 + "0000000000000000000000000000000000000000000000000000000000000002",
		want: struct {
			Int1 *big.Int
			Int2 *big.Int
		}{},
		err: "abi: purely underscored output cannot unpack to struct",
	},
	// Make sure only the first argument is consumed
	{
		def: `[{"name":"int_one","type":"int256"}]`,
		enc: z32 + "0000000000000000000000000000000000000000000000000000000000000001" +
			z32 + "0000000000000000000000000000000000000000000000000000000000000002",
		want: struct {
			IntOne *big.Int
		}{big.NewInt(1)},
	},
	{
		def: `[{"name":"int__one","type":"int256"}]`,
		enc: z32 + "0000000000000000000000000000000000000000000000000000000000000001" +
			z32 + "0000000000000000000000000000000000000000000000000000000000000002",
		want: struct {
			IntOne *big.Int
		}{big.NewInt(1)},
	},
	{
		def: `[{"name":"int_one_","type":"int256"}]`,
		enc: z32 + "0000000000000000000000000000000000000000000000000000000000000001" +
			z32 + "0000000000000000000000000000000000000000000000000000000000000002",
		want: struct {
			IntOne *big.Int
		}{big.NewInt(1)},
	},
	{
		def:  `[{"type":"bool"}]`,
		enc:  "",
		want: false,
		err:  "abi: attempting to unmarshal an empty string while arguments are expected",
	},
	{
		def:  `[{"type":"bytes32","indexed":true},{"type":"uint256","indexed":false}]`,
		enc:  "",
		want: false,
		err:  "abi: attempting to unmarshal an empty string while arguments are expected",
	},
	{
		def:  `[{"type":"bool","indexed":true},{"type":"uint64","indexed":true}]`,
		enc:  "",
		want: false,
	},
}

// TestLocalUnpackTests runs test specially designed only for unpacking.
// All test cases that can be used to test packing and unpacking should move to packing_test.go
func TestLocalUnpackTests(t *testing.T) {
	t.Parallel()
	for i, test := range unpackTests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			//Unpack
			def := fmt.Sprintf(`[{ "name" : "method", "type": "function", "outputs": %s}]`, test.def)
			abi, err := JSON(strings.NewReader(def))
			if err != nil {
				t.Fatalf("invalid ABI definition %s: %v", def, err)
			}
			encb, err := hex.DecodeString(test.enc)
			if err != nil {
				t.Fatalf("invalid hex %s: %v", test.enc, err)
			}
			outptr := reflect.New(reflect.TypeOf(test.want))
			err = abi.UnpackIntoInterface(outptr.Interface(), "method", encb)
			if err := test.checkError(err); err != nil {
				t.Errorf("test %d (%v) failed: %v", i, test.def, err)
				return
			}
			out := outptr.Elem().Interface()
			if !reflect.DeepEqual(test.want, out) {
				t.Errorf("test %d (%v) failed: expected %v, got %v", i, test.def, test.want, out)
			}
		})
	}
}

func TestUnpackIntoInterfaceSetDynamicArrayOutput(t *testing.T) {
	t.Parallel()
	const outSpec = `[{"constant":true,"inputs":[],"name":"testDynamicFixedBytes15","outputs":[{"name":"","type":"bytes15[]"}],"payable":false,"stateMutability":"view","type":"function"},{"constant":true,"inputs":[],"name":"testDynamicFixedBytes32","outputs":[{"name":"","type":"bytes32[]"}],"payable":false,"stateMutability":"view","type":"function"}]`
	const inSpec = `[{"inputs":[{"name":"","type":"bytes15[]"}],"name":"testDynamicFixedBytes15","type":"function"},{"inputs":[{"name":"","type":"bytes32[]"}],"name":"testDynamicFixedBytes32","type":"function"}]`
	abi, err := JSON(strings.NewReader(outSpec))
	if err != nil {
		t.Fatal(err)
	}
	inAbi, err := JSON(strings.NewReader(inSpec))
	if err != nil {
		t.Fatal(err)
	}
	pad32 := func(s string) [32]byte {
		var b [32]byte
		copy(b[:], s)
		return b
	}
	pad15 := func(s string) [15]byte {
		var b [15]byte
		copy(b[:], s)
		return b
	}
	in32 := [][32]byte{pad32("0x1234567890"), pad32("0x0987654321")}
	in15 := [][15]byte{pad15("0x012345"), pad15("0x987654")}
	packed32, err := inAbi.Pack("testDynamicFixedBytes32", in32)
	if err != nil {
		t.Fatal(err)
	}
	packed15, err := inAbi.Pack("testDynamicFixedBytes15", in15)
	if err != nil {
		t.Fatal(err)
	}
	var (
		marshalledReturn32 = packed32[4:]
		marshalledReturn15 = packed15[4:]

		out32 [][32]byte
		out15 [][15]byte
	)

	// test 32
	err = abi.UnpackIntoInterface(&out32, "testDynamicFixedBytes32", marshalledReturn32)
	if err != nil {
		t.Fatal(err)
	}
	if len(out32) != len(in32) {
		t.Fatalf("expected array with %d values, got %d", len(in32), len(out32))
	}
	for i, want := range in32 {
		if !bytes.Equal(out32[i][:], want[:]) {
			t.Errorf("out32[%d]: expected %x, got %x", i, want, out32[i])
		}
	}

	// test 15
	err = abi.UnpackIntoInterface(&out15, "testDynamicFixedBytes15", marshalledReturn15)
	if err != nil {
		t.Fatal(err)
	}
	if len(out15) != len(in15) {
		t.Fatalf("expected array with %d values, got %d", len(in15), len(out15))
	}
	for i, want := range in15 {
		if !bytes.Equal(out15[i][:], want[:]) {
			t.Errorf("out15[%d]: expected %x, got %x", i, want, out15[i])
		}
	}
}

type methodMultiOutput struct {
	Int    *big.Int
	String string
}

func methodMultiReturn(require *require.Assertions) (ABI, []byte, methodMultiOutput) {
	const outDef = `[
	{ "name" : "multi", "type": "function", "outputs": [ { "name": "Int", "type": "uint256" }, { "name": "String", "type": "string" } ] }]`
	const inDef = `[
	{ "name" : "multi", "type": "function", "inputs":  [ { "name": "Int", "type": "uint256" }, { "name": "String", "type": "string" } ] }]`
	expected := methodMultiOutput{big.NewInt(1), "hello"}

	outAbi, err := JSON(strings.NewReader(outDef))
	require.NoError(err)
	inAbi, err := JSON(strings.NewReader(inDef))
	require.NoError(err)
	packed, err := inAbi.Pack("multi", expected.Int, expected.String)
	require.NoError(err)
	// Trim the 4-byte selector prepended by Pack; Unpack expects return data
	// which starts directly at the first slot.
	return outAbi, packed[4:], expected
}

func TestMethodMultiReturn(t *testing.T) {
	t.Parallel()
	type reversed struct {
		String string
		Int    *big.Int
	}

	newInterfaceSlice := func(len int) any {
		slice := make([]any, len)
		return &slice
	}

	abi, data, expected := methodMultiReturn(require.New(t))
	bigint := new(big.Int)
	var testCases = []struct {
		dest     any
		expected any
		error    string
		name     string
	}{{
		&methodMultiOutput{},
		&expected,
		"",
		"Can unpack into structure",
	}, {
		&reversed{},
		&reversed{expected.String, expected.Int},
		"",
		"Can unpack into reversed structure",
	}, {
		&[]any{&bigint, new(string)},
		&[]any{&expected.Int, &expected.String},
		"",
		"Can unpack into a slice",
	}, {
		&[]any{&bigint, ""},
		&[]any{&expected.Int, expected.String},
		"",
		"Can unpack into a slice without indirection",
	}, {
		&[2]any{&bigint, new(string)},
		&[2]any{&expected.Int, &expected.String},
		"",
		"Can unpack into an array",
	}, {
		&[2]any{},
		&[2]any{expected.Int, expected.String},
		"",
		"Can unpack into interface array",
	}, {
		newInterfaceSlice(2),
		&[]any{expected.Int, expected.String},
		"",
		"Can unpack into interface slice",
	}, {
		&[]any{new(int), new(int)},
		&[]any{&expected.Int, &expected.String},
		"abi: cannot unmarshal *big.Int in to int",
		"Can not unpack into a slice with wrong types",
	}, {
		&[]any{new(int)},
		&[]any{},
		"abi: insufficient number of arguments for unpack, want 2, got 1",
		"Can not unpack into a slice with wrong types",
	}}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require := require.New(t)
			err := abi.UnpackIntoInterface(tc.dest, "multi", data)
			if tc.error == "" {
				require.Nil(err, "Should be able to unpack method outputs.")
				require.Equal(tc.expected, tc.dest)
			} else {
				require.EqualError(err, tc.error)
			}
		})
	}
}

func TestMultiReturnWithArray(t *testing.T) {
	t.Parallel()
	// Pack the expected values against an equivalent input method, then verify
	// the output method unpacks them back. Avoids embedding a hand-rolled hex
	// encoding that would need updating whenever the slot width changes.
	const outDef = `[{"name" : "multi", "type": "function", "outputs": [{"type": "uint64[3]"}, {"type": "uint64"}]}]`
	const inDef = `[{"name" : "multi", "type": "function", "inputs": [{"type": "uint64[3]"}, {"type": "uint64"}]}]`
	outAbi, err := JSON(strings.NewReader(outDef))
	if err != nil {
		t.Fatal(err)
	}
	inAbi, err := JSON(strings.NewReader(inDef))
	if err != nil {
		t.Fatal(err)
	}
	ret1Exp := [3]uint64{9, 9, 9}
	ret2Exp := uint64(8)
	packed, err := inAbi.Pack("multi", ret1Exp, ret2Exp)
	if err != nil {
		t.Fatal(err)
	}
	ret1, ret2 := new([3]uint64), new(uint64)
	if err := outAbi.UnpackIntoInterface(&[]any{ret1, ret2}, "multi", packed[4:]); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(*ret1, ret1Exp) {
		t.Error("array result", *ret1, "!= Expected", ret1Exp)
	}
	if *ret2 != ret2Exp {
		t.Error("int result", *ret2, "!= Expected", ret2Exp)
	}
}

func TestMultiReturnWithStringArray(t *testing.T) {
	t.Parallel()
	const outDef = `[{"name" : "multi", "type": "function", "outputs": [{"name": "","type": "uint256[3]"},{"name": "","type": "address"},{"name": "","type": "string[2]"},{"name": "","type": "bool"}]}]`
	const inDef = `[{"name" : "multi", "type": "function", "inputs": [{"name": "","type": "uint256[3]"},{"name": "","type": "address"},{"name": "","type": "string[2]"},{"name": "","type": "bool"}]}]`
	outAbi, err := JSON(strings.NewReader(outDef))
	if err != nil {
		t.Fatal(err)
	}
	inAbi, err := JSON(strings.NewReader(inDef))
	if err != nil {
		t.Fatal(err)
	}
	temp, _ := new(big.Int).SetString("30000000000000000000", 10)
	ret1Exp := [3]*big.Int{big.NewInt(1545304298), big.NewInt(6), temp}
	ret2Exp := common.MustParseAddress("Q0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000ab1257528b3782fb40d7ed5f72e624b744dffb2f")
	ret3Exp := [2]string{"Ethereum", "Hello, Ethereum!"}
	ret4Exp := false
	packed, err := inAbi.Pack("multi", ret1Exp, ret2Exp, ret3Exp, ret4Exp)
	if err != nil {
		t.Fatal(err)
	}
	ret1 := new([3]*big.Int)
	ret2 := new(common.Address)
	ret3 := new([2]string)
	ret4 := new(bool)
	if err := outAbi.UnpackIntoInterface(&[]any{ret1, ret2, ret3, ret4}, "multi", packed[4:]); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(*ret1, ret1Exp) {
		t.Error("big.Int array result", *ret1, "!= Expected", ret1Exp)
	}
	if !reflect.DeepEqual(*ret2, ret2Exp) {
		t.Error("address result", *ret2, "!= Expected", ret2Exp)
	}
	if !reflect.DeepEqual(*ret3, ret3Exp) {
		t.Error("string array result", *ret3, "!= Expected", ret3Exp)
	}
	if !reflect.DeepEqual(*ret4, ret4Exp) {
		t.Error("bool result", *ret4, "!= Expected", ret4Exp)
	}
}

func TestMultiReturnWithStringSlice(t *testing.T) {
	t.Parallel()
	const outDef = `[{"name" : "multi", "type": "function", "outputs": [{"name": "","type": "string[]"},{"name": "","type": "uint256[]"}]}]`
	const inDef = `[{"name" : "multi", "type": "function", "inputs": [{"name": "","type": "string[]"},{"name": "","type": "uint256[]"}]}]`
	outAbi, err := JSON(strings.NewReader(outDef))
	if err != nil {
		t.Fatal(err)
	}
	inAbi, err := JSON(strings.NewReader(inDef))
	if err != nil {
		t.Fatal(err)
	}
	ret1Exp := []string{"ethereum", "go-ethereum"}
	ret2Exp := []*big.Int{big.NewInt(100), big.NewInt(101)}
	packed, err := inAbi.Pack("multi", ret1Exp, ret2Exp)
	if err != nil {
		t.Fatal(err)
	}
	ret1, ret2 := new([]string), new([]*big.Int)
	if err := outAbi.UnpackIntoInterface(&[]any{ret1, ret2}, "multi", packed[4:]); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(*ret1, ret1Exp) {
		t.Error("string slice result", *ret1, "!= Expected", ret1Exp)
	}
	if !reflect.DeepEqual(*ret2, ret2Exp) {
		t.Error("uint256 slice result", *ret2, "!= Expected", ret2Exp)
	}
}

func TestMultiReturnWithDeeplyNestedArray(t *testing.T) {
	t.Parallel()
	// Similar to TestMultiReturnWithArray, but with a special case in mind:
	// values of nested static arrays count towards the size as well, and any
	// element following such a nested array argument must be read with the
	// correct offset so it doesn't pick up bytes from the previous array.
	const outDef = `[{"name" : "multi", "type": "function", "outputs": [{"type": "uint64[3][2][4]"}, {"type": "uint64"}]}]`
	const inDef = `[{"name" : "multi", "type": "function", "inputs": [{"type": "uint64[3][2][4]"}, {"type": "uint64"}]}]`
	outAbi, err := JSON(strings.NewReader(outDef))
	if err != nil {
		t.Fatal(err)
	}
	inAbi, err := JSON(strings.NewReader(inDef))
	if err != nil {
		t.Fatal(err)
	}
	ret1Exp := [4][2][3]uint64{
		{{0x111, 0x112, 0x113}, {0x121, 0x122, 0x123}},
		{{0x211, 0x212, 0x213}, {0x221, 0x222, 0x223}},
		{{0x311, 0x312, 0x313}, {0x321, 0x322, 0x323}},
		{{0x411, 0x412, 0x413}, {0x421, 0x422, 0x423}},
	}
	ret2Exp := uint64(0x9876)
	packed, err := inAbi.Pack("multi", ret1Exp, ret2Exp)
	if err != nil {
		t.Fatal(err)
	}
	ret1, ret2 := new([4][2][3]uint64), new(uint64)
	if err := outAbi.UnpackIntoInterface(&[]any{ret1, ret2}, "multi", packed[4:]); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(*ret1, ret1Exp) {
		t.Error("array result", *ret1, "!= Expected", ret1Exp)
	}
	if *ret2 != ret2Exp {
		t.Error("int result", *ret2, "!= Expected", ret2Exp)
	}
}

func TestUnmarshal(t *testing.T) {
	t.Parallel()
	const outDef = `[
	{ "name" : "int", "type": "function", "outputs": [ { "type": "uint256" } ] },
	{ "name" : "bool", "type": "function", "outputs": [ { "type": "bool" } ] },
	{ "name" : "bytes", "type": "function", "outputs": [ { "type": "bytes" } ] },
	{ "name" : "fixed", "type": "function", "outputs": [ { "type": "bytes32" } ] },
	{ "name" : "multi", "type": "function", "outputs": [ { "type": "bytes" }, { "type": "bytes" } ] },
	{ "name" : "intArraySingle", "type": "function", "outputs": [ { "type": "uint256[3]" } ] },
	{ "name" : "addressSliceSingle", "type": "function", "outputs": [ { "type": "address[]" } ] },
	{ "name" : "addressSliceDouble", "type": "function", "outputs": [ { "name": "a", "type": "address[]" }, { "name": "b", "type": "address[]" } ] },
	{ "name" : "mixedBytes", "type": "function", "stateMutability" : "view", "outputs": [ { "name": "a", "type": "bytes" }, { "name": "b", "type": "bytes32" } ] }]`
	const inDef = `[
	{ "name" : "int", "type": "function", "inputs": [ { "type": "uint256" } ] },
	{ "name" : "bool", "type": "function", "inputs": [ { "type": "bool" } ] },
	{ "name" : "bytes", "type": "function", "inputs": [ { "type": "bytes" } ] },
	{ "name" : "fixed", "type": "function", "inputs": [ { "type": "bytes32" } ] },
	{ "name" : "multi", "type": "function", "inputs": [ { "type": "bytes" }, { "type": "bytes" } ] },
	{ "name" : "intArraySingle", "type": "function", "inputs": [ { "type": "uint256[3]" } ] },
	{ "name" : "addressSliceSingle", "type": "function", "inputs": [ { "type": "address[]" } ] },
	{ "name" : "addressSliceDouble", "type": "function", "inputs": [ { "name": "a", "type": "address[]" }, { "name": "b", "type": "address[]" } ] },
	{ "name" : "mixedBytes", "type": "function", "inputs": [ { "name": "a", "type": "bytes" }, { "name": "b", "type": "bytes32" } ] }]`

	abi, err := JSON(strings.NewReader(outDef))
	if err != nil {
		t.Fatal(err)
	}
	inAbi, err := JSON(strings.NewReader(inDef))
	if err != nil {
		t.Fatal(err)
	}

	// marshall mixed bytes (mixedBytes)
	p0Exp := common.Hex2Bytes("01020000000000000000")
	var p1Exp [32]byte
	copy(p1Exp[29:], []byte{0xdd, 0xee, 0xff})
	p0, p1 := []byte{}, [32]byte{}
	mixedBytes := []any{&p0, &p1}

	packed, err := inAbi.Pack("mixedBytes", p0Exp, p1Exp)
	if err != nil {
		t.Fatal(err)
	}
	err = abi.UnpackIntoInterface(&mixedBytes, "mixedBytes", packed[4:])
	if err != nil {
		t.Error(err)
	} else {
		if !bytes.Equal(p0, p0Exp) {
			t.Errorf("unexpected value unpacked: want %x, got %x", p0Exp, p0)
		}

		if !bytes.Equal(p1[:], p1Exp[:]) {
			t.Errorf("unexpected value unpacked: want %x, got %x", p1Exp, p1)
		}
	}

	// marshal int
	var Int *big.Int
	packed, err = inAbi.Pack("int", big.NewInt(1))
	if err != nil {
		t.Fatal(err)
	}
	err = abi.UnpackIntoInterface(&Int, "int", packed[4:])
	if err != nil {
		t.Error(err)
	}

	if Int == nil || Int.Cmp(big.NewInt(1)) != 0 {
		t.Error("expected Int to be 1 got", Int)
	}

	// marshal bool
	var Bool bool
	packed, err = inAbi.Pack("bool", true)
	if err != nil {
		t.Fatal(err)
	}
	err = abi.UnpackIntoInterface(&Bool, "bool", packed[4:])
	if err != nil {
		t.Error(err)
	}

	if !Bool {
		t.Error("expected Bool to be true")
	}

	// marshal dynamic bytes, length equal to one slot (32 bytes)
	bytesOut := common.RightPadBytes([]byte("hello"), 32)
	packed, err = inAbi.Pack("bytes", bytesOut)
	if err != nil {
		t.Fatal(err)
	}
	var Bytes []byte
	err = abi.UnpackIntoInterface(&Bytes, "bytes", packed[4:])
	if err != nil {
		t.Error(err)
	}

	if !bytes.Equal(Bytes, bytesOut) {
		t.Errorf("expected %x got %x", bytesOut, Bytes)
	}

	// marshall dynamic bytes, length equal to one 64-byte slot
	bytesOut = common.RightPadBytes([]byte("hello"), 64)
	packed, err = inAbi.Pack("bytes", bytesOut)
	if err != nil {
		t.Fatal(err)
	}
	err = abi.UnpackIntoInterface(&Bytes, "bytes", packed[4:])
	if err != nil {
		t.Error(err)
	}

	if !bytes.Equal(Bytes, bytesOut) {
		t.Errorf("expected %x got %x", bytesOut, Bytes)
	}

	// marshall dynamic bytes, length one byte short of a full slot (63)
	bytesOut = common.RightPadBytes([]byte("hello"), 63)
	packed, err = inAbi.Pack("bytes", bytesOut)
	if err != nil {
		t.Fatal(err)
	}
	err = abi.UnpackIntoInterface(&Bytes, "bytes", packed[4:])
	if err != nil {
		t.Error(err)
	}

	if !bytes.Equal(Bytes, bytesOut) {
		t.Errorf("expected %x got %x", bytesOut, Bytes)
	}

	// marshal dynamic bytes output empty
	err = abi.UnpackIntoInterface(&Bytes, "bytes", nil)
	if err == nil {
		t.Error("expected error")
	}

	// marshal dynamic bytes length 5
	packed, err = inAbi.Pack("bytes", []byte("hello"))
	if err != nil {
		t.Fatal(err)
	}
	err = abi.UnpackIntoInterface(&Bytes, "bytes", packed[4:])
	if err != nil {
		t.Error(err)
	}

	if !bytes.Equal(Bytes, []byte("hello")) {
		t.Errorf("expected %x got %x", []byte("hello"), Bytes)
	}

	// marshal fixed bytes32 "hello"
	var fixedIn [32]byte
	copy(fixedIn[:], common.RightPadBytes([]byte("hello"), 32))
	packed, err = inAbi.Pack("fixed", fixedIn)
	if err != nil {
		t.Fatal(err)
	}
	var hash common.Hash
	err = abi.UnpackIntoInterface(&hash, "fixed", packed[4:])
	if err != nil {
		t.Error(err)
	}

	helloHash := common.BytesToHash(common.RightPadBytes([]byte("hello"), 32))
	if hash != helloHash {
		t.Errorf("Expected %x to equal %x", hash, helloHash)
	}

	// marshal error: dynamic bytes with valid offset but no length/data.
	buff := new(bytes.Buffer)
	buff.Write(common.Hex2Bytes(z32 + "0000000000000000000000000000000000000000000000000000000000000040"))
	err = abi.UnpackIntoInterface(&Bytes, "bytes", buff.Bytes())
	if err == nil {
		t.Error("expected error")
	}

	// multi expects two offsets, so two empty 64-byte slots fail the length sanity check.
	err = abi.UnpackIntoInterface(&Bytes, "multi", make([]byte, 128))
	if err == nil {
		t.Error("expected error")
	}

	// marshal int array
	intArrayIn := [3]*big.Int{big.NewInt(1), big.NewInt(2), big.NewInt(3)}
	packed, err = inAbi.Pack("intArraySingle", intArrayIn)
	if err != nil {
		t.Fatal(err)
	}
	var intArray [3]*big.Int
	err = abi.UnpackIntoInterface(&intArray, "intArraySingle", packed[4:])
	if err != nil {
		t.Error(err)
	}
	for i, want := range intArrayIn {
		if intArray[i].Cmp(want) != 0 {
			t.Errorf("expected %v, got %v", want, intArray[i])
		}
	}

	// marshal address slice
	addrA := common.Address{1}
	packed, err = inAbi.Pack("addressSliceSingle", []common.Address{addrA})
	if err != nil {
		t.Fatal(err)
	}
	var outAddr []common.Address
	err = abi.UnpackIntoInterface(&outAddr, "addressSliceSingle", packed[4:])
	if err != nil {
		t.Fatal("didn't expect error:", err)
	}

	if len(outAddr) != 1 {
		t.Fatal("expected 1 item, got", len(outAddr))
	}

	if outAddr[0] != addrA {
		t.Errorf("expected %x, got %x", addrA, outAddr[0])
	}

	// marshal multiple address slices
	addrB1 := common.Address{2}
	addrB2 := common.Address{3}
	packed, err = inAbi.Pack("addressSliceDouble", []common.Address{addrA}, []common.Address{addrB1, addrB2})
	if err != nil {
		t.Fatal(err)
	}

	var outAddrStruct struct {
		A []common.Address
		B []common.Address
	}
	err = abi.UnpackIntoInterface(&outAddrStruct, "addressSliceDouble", packed[4:])
	if err != nil {
		t.Fatal("didn't expect error:", err)
	}

	if len(outAddrStruct.A) != 1 {
		t.Fatal("expected 1 item, got", len(outAddrStruct.A))
	}

	if outAddrStruct.A[0] != addrA {
		t.Errorf("expected %x, got %x", addrA, outAddrStruct.A[0])
	}

	if len(outAddrStruct.B) != 2 {
		t.Fatal("expected 1 item, got", len(outAddrStruct.B))
	}

	if outAddrStruct.B[0] != addrB1 {
		t.Errorf("expected %x, got %x", addrB1, outAddrStruct.B[0])
	}
	if outAddrStruct.B[1] != addrB2 {
		t.Errorf("expected %x, got %x", addrB2, outAddrStruct.B[1])
	}

	// marshal invalid address slice: offset points past available data.
	buff.Reset()
	buff.Write(common.Hex2Bytes(z32 + "0000000000000000000000000000000000000000000000000000000000000200"))
	err = abi.UnpackIntoInterface(&outAddr, "addressSliceSingle", buff.Bytes())
	if err == nil {
		t.Fatal("expected error:", err)
	}
}

func TestUnpackTuple(t *testing.T) {
	t.Parallel()

	// --- Simple single-tuple output.
	const simpleOut = `[{"name":"tuple","type":"function","outputs":[{"type":"tuple","name":"ret","components":[{"type":"int256","name":"a"},{"type":"int256","name":"b"}]}]}]`
	const simpleIn = `[{"name":"tuple","type":"function","inputs":[{"type":"tuple","name":"ret","components":[{"type":"int256","name":"a"},{"type":"int256","name":"b"}]}]}]`
	abi, err := JSON(strings.NewReader(simpleOut))
	if err != nil {
		t.Fatal(err)
	}
	simpleInAbi, err := JSON(strings.NewReader(simpleIn))
	if err != nil {
		t.Fatal(err)
	}
	type v struct {
		A *big.Int
		B *big.Int
	}
	type r struct {
		Result v
	}
	simpleArg := struct {
		A *big.Int
		B *big.Int
	}{big.NewInt(1), big.NewInt(-1)}
	simplePacked, err := simpleInAbi.Pack("tuple", simpleArg)
	if err != nil {
		t.Fatal(err)
	}
	ret0 := new(r)
	if err := abi.UnpackIntoInterface(ret0, "tuple", simplePacked[4:]); err != nil {
		t.Error(err)
	} else {
		if ret0.Result.A.Cmp(big.NewInt(1)) != 0 {
			t.Errorf("unexpected value unpacked: want %d, got %s", 1, ret0.Result.A)
		}
		if ret0.Result.B.Cmp(big.NewInt(-1)) != 0 {
			t.Errorf("unexpected value unpacked: want %d, got %s", -1, ret0.Result.B)
		}
	}

	// --- Nested tuple output.
	const nestedOut = `[{"name":"tuple","type":"function","outputs":[
		{"type":"tuple","name":"s","components":[{"type":"uint256","name":"a"},{"type":"uint256[]","name":"b"},{"type":"tuple[]","name":"c","components":[{"name":"x", "type":"uint256"},{"name":"y","type":"uint256"}]}]},
		{"type":"tuple","name":"t","components":[{"name":"x", "type":"uint256"},{"name":"y","type":"uint256"}]},
		{"type":"uint256","name":"a"}
	]}]`
	const nestedIn = `[{"name":"tuple","type":"function","inputs":[
		{"type":"tuple","name":"s","components":[{"type":"uint256","name":"a"},{"type":"uint256[]","name":"b"},{"type":"tuple[]","name":"c","components":[{"name":"x", "type":"uint256"},{"name":"y","type":"uint256"}]}]},
		{"type":"tuple","name":"t","components":[{"name":"x", "type":"uint256"},{"name":"y","type":"uint256"}]},
		{"type":"uint256","name":"a"}
	]}]`

	type T struct {
		X *big.Int `abi:"x"`
		Z *big.Int `abi:"y"` // Test whether the abi tag works.
	}
	type S struct {
		A *big.Int
		B []*big.Int
		C []T
	}
	type Ret struct {
		FieldS S `abi:"s"`
		FieldT T `abi:"t"`
		A      *big.Int
	}

	// Go struct for the input side has to match field names exactly (no abi
	// tag remapping on pack), so use a parallel shape where X/Y line up with
	// the component names.
	type compXY struct {
		X *big.Int
		Y *big.Int
	}
	type compS struct {
		A *big.Int
		B []*big.Int
		C []compXY
	}
	inS := compS{
		A: big.NewInt(1),
		B: []*big.Int{big.NewInt(1), big.NewInt(2)},
		C: []compXY{{big.NewInt(1), big.NewInt(2)}, {big.NewInt(2), big.NewInt(1)}},
	}
	inT := compXY{X: big.NewInt(0), Y: big.NewInt(1)}
	inA := big.NewInt(1)

	abi, err = JSON(strings.NewReader(nestedOut))
	if err != nil {
		t.Fatal(err)
	}
	nestedInAbi, err := JSON(strings.NewReader(nestedIn))
	if err != nil {
		t.Fatal(err)
	}
	nestedPacked, err := nestedInAbi.Pack("tuple", inS, inT, inA)
	if err != nil {
		t.Fatal(err)
	}

	expected := Ret{
		FieldS: S{
			A: big.NewInt(1),
			B: []*big.Int{big.NewInt(1), big.NewInt(2)},
			C: []T{
				{big.NewInt(1), big.NewInt(2)},
				{big.NewInt(2), big.NewInt(1)},
			},
		},
		FieldT: T{X: big.NewInt(0), Z: big.NewInt(1)},
		A:      big.NewInt(1),
	}
	var ret Ret
	if err := abi.UnpackIntoInterface(&ret, "tuple", nestedPacked[4:]); err != nil {
		t.Error(err)
	}
	// big.Int values can compare unequal under reflect.DeepEqual when their
	// internal `neg`/`nat` slices differ for the same mathematical number;
	// compare the fields individually.
	if ret.A.Cmp(expected.A) != 0 {
		t.Errorf("A mismatch: got %s want %s", ret.A, expected.A)
	}
	if ret.FieldS.A.Cmp(expected.FieldS.A) != 0 {
		t.Errorf("S.A mismatch: got %s want %s", ret.FieldS.A, expected.FieldS.A)
	}
	if len(ret.FieldS.B) != len(expected.FieldS.B) {
		t.Fatalf("S.B length mismatch: got %d want %d", len(ret.FieldS.B), len(expected.FieldS.B))
	}
	for i, want := range expected.FieldS.B {
		if ret.FieldS.B[i].Cmp(want) != 0 {
			t.Errorf("S.B[%d]: got %s want %s", i, ret.FieldS.B[i], want)
		}
	}
	if len(ret.FieldS.C) != len(expected.FieldS.C) {
		t.Fatalf("S.C length mismatch: got %d want %d", len(ret.FieldS.C), len(expected.FieldS.C))
	}
	for i, want := range expected.FieldS.C {
		if ret.FieldS.C[i].X.Cmp(want.X) != 0 || ret.FieldS.C[i].Z.Cmp(want.Z) != 0 {
			t.Errorf("S.C[%d]: got %+v want %+v", i, ret.FieldS.C[i], want)
		}
	}
	if ret.FieldT.X.Cmp(expected.FieldT.X) != 0 || ret.FieldT.Z.Cmp(expected.FieldT.Z) != 0 {
		t.Errorf("T: got %+v want %+v", ret.FieldT, expected.FieldT)
	}
}

func TestOOMMaliciousInput(t *testing.T) {
	t.Parallel()
	oomTests := []unpackTest{
		{
			def: `[{"type": "uint8[]"}]`,
			enc: "0000000000000000000000000000000000000000000000000000000000000020" + // offset
				"0000000000000000000000000000000000000000000000000000000000000003" + // num elems
				"0000000000000000000000000000000000000000000000000000000000000001" + // elem 1
				"0000000000000000000000000000000000000000000000000000000000000002", // elem 2
		},
		{ // Length larger than 64 bits
			def: `[{"type": "uint8[]"}]`,
			enc: "0000000000000000000000000000000000000000000000000000000000000020" + // offset
				"00ffffffffffffffffffffffffffffffffffffffffffffff0000000000000002" + // num elems
				"0000000000000000000000000000000000000000000000000000000000000001" + // elem 1
				"0000000000000000000000000000000000000000000000000000000000000002", // elem 2
		},
		{ // Offset very large (over 64 bits)
			def: `[{"type": "uint8[]"}]`,
			enc: "00ffffffffffffffffffffffffffffffffffffffffffffff0000000000000020" + // offset
				"0000000000000000000000000000000000000000000000000000000000000002" + // num elems
				"0000000000000000000000000000000000000000000000000000000000000001" + // elem 1
				"0000000000000000000000000000000000000000000000000000000000000002", // elem 2
		},
		{ // Offset very large (below 64 bits)
			def: `[{"type": "uint8[]"}]`,
			enc: "0000000000000000000000000000000000000000000000007ffffffffff00020" + // offset
				"0000000000000000000000000000000000000000000000000000000000000002" + // num elems
				"0000000000000000000000000000000000000000000000000000000000000001" + // elem 1
				"0000000000000000000000000000000000000000000000000000000000000002", // elem 2
		},
		{ // Offset negative (as 64 bit)
			def: `[{"type": "uint8[]"}]`,
			enc: "000000000000000000000000000000000000000000000000f000000000000020" + // offset
				"0000000000000000000000000000000000000000000000000000000000000002" + // num elems
				"0000000000000000000000000000000000000000000000000000000000000001" + // elem 1
				"0000000000000000000000000000000000000000000000000000000000000002", // elem 2
		},

		{ // Negative length
			def: `[{"type": "uint8[]"}]`,
			enc: "0000000000000000000000000000000000000000000000000000000000000020" + // offset
				"000000000000000000000000000000000000000000000000f000000000000002" + // num elems
				"0000000000000000000000000000000000000000000000000000000000000001" + // elem 1
				"0000000000000000000000000000000000000000000000000000000000000002", // elem 2
		},
		{ // Very large length
			def: `[{"type": "uint8[]"}]`,
			enc: "0000000000000000000000000000000000000000000000000000000000000020" + // offset
				"0000000000000000000000000000000000000000000000007fffffffff000002" + // num elems
				"0000000000000000000000000000000000000000000000000000000000000001" + // elem 1
				"0000000000000000000000000000000000000000000000000000000000000002", // elem 2
		},
	}
	for i, test := range oomTests {
		def := fmt.Sprintf(`[{ "name" : "method", "type": "function", "outputs": %s}]`, test.def)
		abi, err := JSON(strings.NewReader(def))
		if err != nil {
			t.Fatalf("invalid ABI definition %s: %v", def, err)
		}
		encb, err := hex.DecodeString(test.enc)
		if err != nil {
			t.Fatalf("invalid hex: %s", test.enc)
		}
		_, err = abi.Methods["method"].Outputs.UnpackValues(encb)
		if err == nil {
			t.Fatalf("Expected error on malicious input, test %d", i)
		}
	}
}

func TestPackAndUnpackIncompatibleNumber(t *testing.T) {
	t.Parallel()
	var encodeABI Arguments
	uint256Ty, err := NewType("uint256", "", nil)
	if err != nil {
		panic(err)
	}
	encodeABI = Arguments{
		{Type: uint256Ty},
	}

	maxU64, ok := new(big.Int).SetString(strconv.FormatUint(math.MaxUint64, 10), 10)
	if !ok {
		panic("bug")
	}
	maxU64Plus1 := new(big.Int).Add(maxU64, big.NewInt(1))
	cases := []struct {
		decodeType  string
		inputValue  *big.Int
		unpackErr   error
		packErr     error
		expectValue any
	}{
		{
			decodeType: "uint8",
			inputValue: big.NewInt(math.MaxUint8 + 1),
			unpackErr:  errBadUint8,
		},
		{
			decodeType:  "uint8",
			inputValue:  big.NewInt(math.MaxUint8),
			unpackErr:   nil,
			expectValue: uint8(math.MaxUint8),
		},
		{
			decodeType: "uint16",
			inputValue: big.NewInt(math.MaxUint16 + 1),
			unpackErr:  errBadUint16,
		},
		{
			decodeType:  "uint16",
			inputValue:  big.NewInt(math.MaxUint16),
			unpackErr:   nil,
			expectValue: uint16(math.MaxUint16),
		},
		{
			decodeType: "uint32",
			inputValue: big.NewInt(math.MaxUint32 + 1),
			unpackErr:  errBadUint32,
		},
		{
			decodeType:  "uint32",
			inputValue:  big.NewInt(math.MaxUint32),
			unpackErr:   nil,
			expectValue: uint32(math.MaxUint32),
		},
		{
			decodeType: "uint64",
			inputValue: maxU64Plus1,
			unpackErr:  errBadUint64,
		},
		{
			decodeType:  "uint64",
			inputValue:  maxU64,
			unpackErr:   nil,
			expectValue: uint64(math.MaxUint64),
		},
		{
			decodeType:  "uint256",
			inputValue:  maxU64Plus1,
			unpackErr:   nil,
			expectValue: maxU64Plus1,
		},
		{
			decodeType: "int8",
			inputValue: big.NewInt(math.MaxInt8 + 1),
			unpackErr:  errBadInt8,
		},
		{
			inputValue: big.NewInt(math.MinInt8 - 1),
			packErr:    errInvalidSign,
		},
		{
			decodeType:  "int8",
			inputValue:  big.NewInt(math.MaxInt8),
			unpackErr:   nil,
			expectValue: int8(math.MaxInt8),
		},
		{
			decodeType: "int16",
			inputValue: big.NewInt(math.MaxInt16 + 1),
			unpackErr:  errBadInt16,
		},
		{
			inputValue: big.NewInt(math.MinInt16 - 1),
			packErr:    errInvalidSign,
		},
		{
			decodeType:  "int16",
			inputValue:  big.NewInt(math.MaxInt16),
			unpackErr:   nil,
			expectValue: int16(math.MaxInt16),
		},
		{
			decodeType: "int32",
			inputValue: big.NewInt(math.MaxInt32 + 1),
			unpackErr:  errBadInt32,
		},
		{
			inputValue: big.NewInt(math.MinInt32 - 1),
			packErr:    errInvalidSign,
		},
		{
			decodeType:  "int32",
			inputValue:  big.NewInt(math.MaxInt32),
			unpackErr:   nil,
			expectValue: int32(math.MaxInt32),
		},
		{
			decodeType: "int64",
			inputValue: new(big.Int).Add(big.NewInt(math.MaxInt64), big.NewInt(1)),
			unpackErr:  errBadInt64,
		},
		{
			inputValue: new(big.Int).Sub(big.NewInt(math.MinInt64), big.NewInt(1)),
			packErr:    errInvalidSign,
		},
		{
			decodeType:  "int64",
			inputValue:  big.NewInt(math.MaxInt64),
			unpackErr:   nil,
			expectValue: int64(math.MaxInt64),
		},
	}
	for i, testCase := range cases {
		packed, err := encodeABI.Pack(testCase.inputValue)
		if testCase.packErr != nil {
			if err == nil {
				t.Fatalf("expected packing of testcase input value to fail")
			}
			if err != testCase.packErr {
				t.Fatalf("expected error '%v', got '%v'", testCase.packErr, err)
			}
			continue
		}
		if err != nil && err != testCase.packErr {
			panic(fmt.Errorf("unexpected error packing test-case input: %v", err))
		}
		ty, err := NewType(testCase.decodeType, "", nil)
		if err != nil {
			panic(err)
		}
		decodeABI := Arguments{
			{Type: ty},
		}
		decoded, err := decodeABI.Unpack(packed)
		if err != testCase.unpackErr {
			t.Fatalf("Expected error %v, actual error %v. case %d", testCase.unpackErr, err, i)
		}
		if err != nil {
			continue
		}
		if !reflect.DeepEqual(decoded[0], testCase.expectValue) {
			t.Fatalf("Expected value %v, actual value %v", testCase.expectValue, decoded[0])
		}
	}
}
