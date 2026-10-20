// Copyright 2015 The go-ethereum Authors
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

package common

import (
	"bytes"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"math/big"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestBytesConversion(t *testing.T) {
	bytes := []byte{5}
	hash := BytesToHash(bytes)

	var exp Hash
	exp[31] = 5

	if hash != exp {
		t.Errorf("expected %x got %x", exp, hash)
	}
}

func TestLogTopicAlignmentHelpers(t *testing.T) {
	input := []byte{1, 2, 3}

	rightAligned := BytesToRightAlignedLogTopic(input)
	var wantRight LogTopic
	copy(wantRight[LogTopicLength-len(input):], input)
	if rightAligned != wantRight {
		t.Fatalf("right-aligned topic mismatch: have %x want %x", rightAligned, wantRight)
	}

	leftAligned := BytesToLeftAlignedLogTopic(input)
	var wantLeft LogTopic
	copy(wantLeft[:], input)
	if leftAligned != wantLeft {
		t.Fatalf("left-aligned topic mismatch: have %x want %x", leftAligned, wantLeft)
	}

	var hash Hash
	for i := range hash {
		hash[i] = byte(i + 1)
	}
	topic := HashToLogTopic(hash)

	// The hash occupies the high 32 bytes; the low 32 bytes are zero padding.
	for i := 0; i < HashLength; i++ {
		if topic[i] != hash[i] {
			t.Fatalf("byte %d: expected %x got %x", i, hash[i], topic[i])
		}
	}
	for i := HashLength; i < LogTopicLength; i++ {
		if topic[i] != 0 {
			t.Fatalf("byte %d: expected zero padding, got %x", i, topic[i])
		}
	}

	var addr Address
	for i := range addr {
		addr[i] = byte(i + 1)
	}
	if got := AddressToLogTopic(addr); !bytes.Equal(got.Bytes(), addr.Bytes()) {
		t.Fatalf("address topic mismatch: have %x want %x", got, addr)
	}
}

func TestParseLogTopic(t *testing.T) {
	// Complete topics are preserved verbatim, whatever their alignment.
	for _, want := range []LogTopic{
		BytesToRightAlignedLogTopic([]byte{1, 2, 3}),
		BytesToLeftAlignedLogTopic([]byte{1, 2, 3}),
	} {
		got, err := ParseLogTopic(want.Hex())
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("topic changed during parsing: got %x, want %x", got, want)
		}
		if must := MustParseLogTopic(want.Hex()); must != want {
			t.Fatalf("MustParseLogTopic mismatch: got %x, want %x", must, want)
		}
	}

	// Anything but a 0x-prefixed string of exactly LogTopicLength bytes is rejected.
	for _, invalid := range []string{
		"",
		"0x",
		"0x010203",
		"0x" + strings.Repeat("11", LogTopicLength-1),
		"0x" + strings.Repeat("11", LogTopicLength+1),
		strings.Repeat("11", LogTopicLength),
	} {
		if _, err := ParseLogTopic(invalid); err == nil {
			t.Fatalf("expected error parsing %q", invalid)
		}
	}
}

func TestIsAddress(t *testing.T) {
	tests := []struct {
		str string
		exp bool
	}{
		// Valid: Q + 128 hex chars (64-byte address)
		{"Q" + strings.Repeat("5a", AddressLength), true},
		{"Q" + strings.Repeat("0", 2*AddressLength), true},
		{"Q" + strings.Repeat("A", 2*AddressLength), true},
		// Invalid: missing Q prefix
		{strings.Repeat("5a", AddressLength), false},
		// Invalid: old 20-byte length
		{"Q" + strings.Repeat("0", 40), false},
		// Invalid: off-by-one lengths (127 / 129 hex chars)
		{"Q" + strings.Repeat("0", 2*AddressLength-1), false},
		{"Q" + strings.Repeat("0", 2*AddressLength+1), false},
		// Invalid: non-hex character
		{"Q" + strings.Repeat("0", 2*AddressLength-1) + "x", false},
	}

	for _, test := range tests {
		if result := IsAddress(test.str); result != test.exp {
			t.Errorf("IsAddress(%s) == %v; expected %v",
				test.str, result, test.exp)
		}
	}
}

func TestIsHexEncodedHash(t *testing.T) {
	tests := []struct {
		str string
		exp bool
	}{
		{strings.Repeat("5a", HashLength), true},
		{"0x" + strings.Repeat("5a", HashLength), true},
		{"0X" + strings.Repeat("5a", HashLength), true},
		{strings.Repeat("0", 2*HashLength), true},
		{"0x" + strings.Repeat("0", 2*HashLength), true},
		{strings.Repeat("0", 2*HashLength-1), false},
		{strings.Repeat("0", 2*HashLength+1), false},
		{"0x" + strings.Repeat("0", 2*HashLength-1), false},
		{"0x" + strings.Repeat("0", 2*HashLength+1), false},
		{"0x" + strings.Repeat("0", 2*HashLength-1) + "x", false},
		{"Q" + strings.Repeat("0", 2*HashLength), false},
	}

	for _, test := range tests {
		if result := IsHexEncodedHash(test.str); result != test.exp {
			t.Errorf("IsHexEncodedHash(%s) == %v; expected %v",
				test.str, result, test.exp)
		}
	}
}

func TestMustParseAddress(t *testing.T) {
	input := "Q" + strings.Repeat("5a", AddressLength)
	if got, want := MustParseAddress(input), BytesToAddress(Hex2Bytes(strings.TrimPrefix(input, "Q"))); got != want {
		t.Fatalf("unexpected address: got %s want %s", got, want)
	}

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for invalid address")
		}
	}()
	MustParseAddress("Q" + strings.Repeat("0", 40))
}

func TestMustParseMixedcaseAddress(t *testing.T) {
	input := "Q" + strings.Repeat("5a", AddressLength)
	if got := MustParseMixedcaseAddress(input); got.String() != input {
		t.Fatalf("unexpected mixed-case address: got %s want %s", got, input)
	}

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for invalid mixed-case address")
		}
	}()
	MustParseMixedcaseAddress("Q" + strings.Repeat("0", 40))
}

func TestHashJsonValidation(t *testing.T) {
	var tests = []struct {
		Prefix string
		Size   int
		Error  string
	}{
		{"", 62, "json: cannot unmarshal hex string without 0x prefix into Go value of type common.Hash"},
		{"0x", 66, "hex string has length 66, want 64 for common.Hash"},
		{"0x", 63, "json: cannot unmarshal hex string of odd length into Go value of type common.Hash"},
		{"0x", 0, "hex string has length 0, want 64 for common.Hash"},
		{"0x", 64, ""},
		{"0X", 64, ""},
	}
	for _, test := range tests {
		input := `"` + test.Prefix + strings.Repeat("0", test.Size) + `"`
		var v Hash
		err := json.Unmarshal([]byte(input), &v)
		if err == nil {
			if test.Error != "" {
				t.Errorf("%s: error mismatch: have nil, want %q", input, test.Error)
			}
		} else {
			if err.Error() != test.Error {
				t.Errorf("%s: error mismatch: have %q, want %q", input, err, test.Error)
			}
		}
	}
}

func TestAddressUnmarshalJSON(t *testing.T) {
	var tests = []struct {
		Input     string
		ShouldErr bool
		Output    *big.Int
	}{
		{"", true, nil},
		{`""`, true, nil},
		{`"Q"`, true, nil},
		{`"Q00"`, true, nil},
		{`"QG` + strings.Repeat("0", 2*AddressLength-1) + `"`, true, nil},
		{`"Q` + strings.Repeat("0", 2*AddressLength) + `"`, false, big.NewInt(0)},
		{`"Q` + strings.Repeat("0", 2*AddressLength-2) + `10"`, false, big.NewInt(16)},
	}
	for i, test := range tests {
		var v Address
		err := json.Unmarshal([]byte(test.Input), &v)
		if err != nil && !test.ShouldErr {
			t.Errorf("test #%d: unexpected error: %v", i, err)
		}
		if err == nil {
			if test.ShouldErr {
				t.Errorf("test #%d: expected error, got none", i)
			}
			if got := new(big.Int).SetBytes(v.Bytes()); got.Cmp(test.Output) != 0 {
				t.Errorf("test #%d: address mismatch: have %v, want %v", i, got, test.Output)
			}
		}
	}
}

func TestAddressHexQIP55Checksum(t *testing.T) {
	// QRL addresses are canonicalized as Q + 128 QIP-55 mixed-case hex
	// characters. Mixed-case input remains accepted by parsers, but output is
	// always normalized to the SHAKE256 checksum casing.
	var tests = []struct {
		Input  string
		Output string
	}{
		{"Q" + strings.Repeat("0", 88) + "5AAEB6053F3E94C9B9A09F33669435E7EF1BEAED", "Q" + strings.Repeat("0", 88) + "5AAeb6053F3e94c9b9A09f33669435e7EF1BEaEd"},
		{"Q" + strings.Repeat("0", 88) + "fb6916095CA1dF60bB79CE92ce3eA74c37c5d359", "Q" + strings.Repeat("0", 88) + "Fb6916095Ca1df60BB79CE92CE3ea74c37c5d359"},
		{"Q" + strings.Repeat("0", 88) + "DBf03B407C01e7CD3cBEa99509D93F8ddDc8C6FB", "Q" + strings.Repeat("0", 88) + "DBf03B407c01E7cD3Cbea99509D93f8DDdc8c6FB"},
		{"Q" + strings.Repeat("0", 88) + "D1220a0cf47c7B9bE7a2e6bA89F429762E7b9ADb", "Q" + strings.Repeat("0", 88) + "d1220A0cf47c7B9BE7a2e6BA89f429762e7B9AdB"},
	}
	for i, test := range tests {
		addr := MustParseAddress(test.Input)
		output := addr.Hex()
		if output != test.Output {
			t.Errorf("test #%d: failed to match when it should (%s != %s)", i, output, test.Output)
		}
	}
}

func BenchmarkAddressHex(b *testing.B) {
	testAddr := MustParseAddress("Q" + strings.Repeat("0", 88) + "5aaeb6053f3e94c9b9a09f33669435e7ef1beaed")
	for b.Loop() {
		testAddr.Hex()
	}
}

// Test checks if the customized json marshaller of MixedcaseAddress object
// is invoked correctly. In golang the struct pointer will inherit the
// non-pointer receiver methods, the reverse is not true. In the case of
// MixedcaseAddress, it must define the MarshalJSON method in the object
// but not the pointer level, so that this customized marshalled can be used
// for both MixedcaseAddress object and pointer.
func TestMixedcaseAddressMarshal(t *testing.T) {
	var (
		output string
		input  = "Q" + strings.Repeat("0", 88) + "ae967917c465db8578ca9024c205720b1a3651A9"
	)
	addr, err := NewMixedcaseAddressFromString(input)
	if err != nil {
		t.Fatal(err)
	}
	blob, err := json.Marshal(*addr)
	if err != nil {
		t.Fatal(err)
	}
	json.Unmarshal(blob, &output)
	if output != input {
		t.Fatal("Failed to marshal/unmarshal MixedcaseAddress object")
	}
}

func TestMixedcaseAccount_Address(t *testing.T) {
	var res []struct {
		A     MixedcaseAddress
		Valid bool
	}
	lowercase := "Q" + strings.Repeat("0", 88) + "ae967917c465db8578ca9024c205720b1a3651a9"
	addr := MustParseAddress(lowercase)
	canonical := addr.Hex()
	if err := json.Unmarshal([]byte(fmt.Sprintf(`[
			{"A" : "%s", "Valid": true},
			{"A" : "%s", "Valid": false}
		]`, canonical, lowercase)), &res); err != nil {
		t.Fatal(err)
	}

	for _, r := range res {
		if got := r.A.ValidChecksum(); got != r.Valid {
			t.Errorf("Expected valid address %v, got %v, input %v", r.Valid, got, r.A.String())
		}
	}

	// These should throw exceptions:
	var r2 []MixedcaseAddress
	for _, r := range []string{
		`["Q11111111111111111111122222222222233333"]`,                                                           // Too short
		`["Q111111111111111111111222222222222333332"]`,                                                          // Too short
		`["Q11111111111111111111122222222222233333234"]`,                                                        // Too short (40 chars is the old address length)
		`["Q111111111111111111111222222222222333332344"]`,                                                       // Too short
		`["1111111111111111111112222222222223333323"]`,                                                          // Missing Q
		`["q1111111111111111111112222222222223333323"]`,                                                         // Lower case Q
		`["QG00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"]`, // Non-hex
	} {
		if err := json.Unmarshal([]byte(r), &r2); err == nil {
			t.Errorf("Expected failure, input %v", r)
		}
	}
}

func TestHash_Scan(t *testing.T) {
	type args struct {
		src any
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "working scan",
			args: args{src: []byte{
				0xb2, 0x6f, 0x2b, 0x34, 0x2a, 0xab, 0x24, 0xbc, 0xf6, 0x3e,
				0xa2, 0x18, 0xc6, 0xa9, 0x27, 0x4d, 0x30, 0xab, 0x9a, 0x15,
				0xa2, 0x18, 0xc6, 0xa9, 0x27, 0x4d, 0x30, 0xab, 0x9a, 0x15,
				0x10, 0x00,
			}},
			wantErr: false,
		},
		{
			name:    "non working scan",
			args:    args{src: int64(1234567890)},
			wantErr: true,
		},
		{
			name: "invalid length scan",
			args: args{src: []byte{
				0xb2, 0x6f, 0x2b, 0x34, 0x2a, 0xab, 0x24, 0xbc, 0xf6, 0x3e,
				0xa2, 0x18, 0xc6, 0xa9, 0x27, 0x4d, 0x30, 0xab, 0x9a, 0x15,
				0xa2, 0x18, 0xc6, 0xa9, 0x27, 0x4d, 0x30, 0xab, 0x9a, 0x15,
			}},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &Hash{}
			if err := h.Scan(tt.args.src); (err != nil) != tt.wantErr {
				t.Errorf("Hash.Scan() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				for i := range h {
					if h[i] != tt.args.src.([]byte)[i] {
						t.Errorf(
							"Hash.Scan() didn't scan the %d src correctly (have %X, want %X)",
							i, h[i], tt.args.src.([]byte)[i],
						)
					}
				}
			}
		})
	}
}

func TestHash_Value(t *testing.T) {
	b := []byte{
		0xb2, 0x6f, 0x2b, 0x34, 0x2a, 0xab, 0x24, 0xbc, 0xf6, 0x3e,
		0xa2, 0x18, 0xc6, 0xa9, 0x27, 0x4d, 0x30, 0xab, 0x9a, 0x15,
		0xa2, 0x18, 0xc6, 0xa9, 0x27, 0x4d, 0x30, 0xab, 0x9a, 0x15,
		0x10, 0x00,
	}
	var usedH Hash
	usedH.SetBytes(b)
	tests := []struct {
		name    string
		h       Hash
		want    driver.Value
		wantErr bool
	}{
		{
			name:    "Working value",
			h:       usedH,
			want:    b,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.h.Value()
			if (err != nil) != tt.wantErr {
				t.Errorf("Hash.Value() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Hash.Value() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAddress_Scan(t *testing.T) {
	type args struct {
		src any
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name:    "working scan",
			args:    args{src: bytes.Repeat([]byte{0xb2}, AddressLength)},
			wantErr: false,
		},
		{
			name:    "non working scan",
			args:    args{src: int64(1234567890)},
			wantErr: true,
		},
		{
			name: "invalid length scan",
			args: args{src: []byte{
				0xb2, 0x6f, 0x2b, 0x34, 0x2a, 0xab, 0x24, 0xbc, 0xf6,
				0x3e, 0xa2, 0x18, 0xc6, 0xa9, 0x27, 0x4d, 0x30, 0xab, 0x9a,
			}},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Address{}
			if err := a.Scan(tt.args.src); (err != nil) != tt.wantErr {
				t.Errorf("Address.Scan() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				for i := range a {
					if a[i] != tt.args.src.([]byte)[i] {
						t.Errorf(
							"Address.Scan() didn't scan the %d src correctly (have %X, want %X)",
							i, a[i], tt.args.src.([]byte)[i],
						)
					}
				}
			}
		})
	}
}

func TestAddress_Value(t *testing.T) {
	b := bytes.Repeat([]byte{0xb2}, AddressLength)
	var usedA Address
	usedA.SetBytes(b)
	tests := []struct {
		name    string
		a       Address
		want    driver.Value
		wantErr bool
	}{
		{
			name:    "Working value",
			a:       usedA,
			want:    b,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.a.Value()
			if (err != nil) != tt.wantErr {
				t.Errorf("Address.Value() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Address.Value() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAddress_Format(t *testing.T) {
	b := []byte{
		0xb2, 0x6f, 0x2b, 0x34, 0x2a, 0xab, 0x24, 0xbc, 0xf6, 0x3e,
		0xa2, 0x18, 0xc6, 0xa9, 0x27, 0x4d, 0x30, 0xab, 0x9a, 0x15,
	}
	var addr Address
	addr.SetBytes(b)

	// The 20-byte seed is right-aligned in the 64-byte Address, so the
	// canonical forms carry a 44-byte (88 hex char) zero prefix before the
	// familiar suffix.
	const (
		suffixLow  = "b26f2b342aab24bcf63ea218c6a9274d30ab9a15"
		suffixUp   = "B26F2B342AAB24BCF63EA218C6A9274D30AB9A15"
		zeroPrefix = "0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"
	)
	checksum := addr.Hex()

	tests := []struct {
		name string
		out  string
		want string
	}{
		{
			name: "println",
			out:  fmt.Sprintln(addr),
			want: checksum + "\n",
		},
		{
			name: "print",
			out:  fmt.Sprint(addr),
			want: checksum,
		},
		{
			name: "printf-s",
			out: func() string {
				buf := new(bytes.Buffer)
				fmt.Fprintf(buf, "%s", addr)
				return buf.String()
			}(),
			want: checksum,
		},
		{
			name: "printf-q",
			out:  fmt.Sprintf("%q", addr),
			want: `"` + checksum + `"`,
		},
		{
			name: "printf-x",
			out:  fmt.Sprintf("%x", addr),
			want: zeroPrefix + suffixLow,
		},
		{
			name: "printf-X",
			out:  fmt.Sprintf("%X", addr),
			want: zeroPrefix + suffixUp,
		},
		{
			name: "printf-#x",
			out:  fmt.Sprintf("%#x", addr),
			want: "Q" + zeroPrefix + suffixLow,
		},
		{
			name: "printf-v",
			out:  fmt.Sprintf("%v", addr),
			want: checksum,
		},
		// The original default formatter for byte slice
		{
			name: "printf-d",
			out:  fmt.Sprintf("%d", addr),
			want: fmt.Sprint(([AddressLength]byte)(addr)),
		},
		// Invalid format char.
		{
			name: "printf-t",
			out:  fmt.Sprintf("%t", addr),
			want: "%!t(address=" + zeroPrefix + suffixLow + ")",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.out != tt.want {
				t.Errorf("%s does not render as expected:\n got %s\nwant %s", tt.name, tt.out, tt.want)
			}
		})
	}
}

func TestHash_Format(t *testing.T) {
	var hash Hash
	hash.SetBytes([]byte{
		0xb2, 0x6f, 0x2b, 0x34, 0x2a, 0xab, 0x24, 0xbc, 0xf6, 0x3e,
		0xa2, 0x18, 0xc6, 0xa9, 0x27, 0x4d, 0x30, 0xab, 0x9a, 0x15,
		0xa2, 0x18, 0xc6, 0xa9, 0x27, 0x4d, 0x30, 0xab, 0x9a, 0x15,
		0x10, 0x00,
	})

	tests := []struct {
		name string
		out  string
		want string
	}{
		{
			name: "println",
			out:  fmt.Sprintln(hash),
			want: "0xb26f2b342aab24bcf63ea218c6a9274d30ab9a15a218c6a9274d30ab9a151000\n",
		},
		{
			name: "print",
			out:  fmt.Sprint(hash),
			want: "0xb26f2b342aab24bcf63ea218c6a9274d30ab9a15a218c6a9274d30ab9a151000",
		},
		{
			name: "printf-s",
			out: func() string {
				buf := new(bytes.Buffer)
				fmt.Fprintf(buf, "%s", hash)
				return buf.String()
			}(),
			want: "0xb26f2b342aab24bcf63ea218c6a9274d30ab9a15a218c6a9274d30ab9a151000",
		},
		{
			name: "printf-q",
			out:  fmt.Sprintf("%q", hash),
			want: `"0xb26f2b342aab24bcf63ea218c6a9274d30ab9a15a218c6a9274d30ab9a151000"`,
		},
		{
			name: "printf-x",
			out:  fmt.Sprintf("%x", hash),
			want: "b26f2b342aab24bcf63ea218c6a9274d30ab9a15a218c6a9274d30ab9a151000",
		},
		{
			name: "printf-X",
			out:  fmt.Sprintf("%X", hash),
			want: "B26F2B342AAB24BCF63EA218C6A9274D30AB9A15A218C6A9274D30AB9A151000",
		},
		{
			name: "printf-#x",
			out:  fmt.Sprintf("%#x", hash),
			want: "0xb26f2b342aab24bcf63ea218c6a9274d30ab9a15a218c6a9274d30ab9a151000",
		},
		{
			name: "printf-#X",
			out:  fmt.Sprintf("%#X", hash),
			want: "0XB26F2B342AAB24BCF63EA218C6A9274D30AB9A15A218C6A9274D30AB9A151000",
		},
		{
			name: "printf-v",
			out:  fmt.Sprintf("%v", hash),
			want: "0xb26f2b342aab24bcf63ea218c6a9274d30ab9a15a218c6a9274d30ab9a151000",
		},
		// The original default formatter for byte slice
		{
			name: "printf-d",
			out:  fmt.Sprintf("%d", hash),
			want: "[178 111 43 52 42 171 36 188 246 62 162 24 198 169 39 77 48 171 154 21 162 24 198 169 39 77 48 171 154 21 16 0]",
		},
		// Invalid format char.
		{
			name: "printf-t",
			out:  fmt.Sprintf("%t", hash),
			want: "%!t(hash=b26f2b342aab24bcf63ea218c6a9274d30ab9a15a218c6a9274d30ab9a151000)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.out != tt.want {
				t.Errorf("%s does not render as expected:\n got %s\nwant %s", tt.name, tt.out, tt.want)
			}
		})
	}
}

func BenchmarkPrettyDuration(b *testing.B) {
	var x = PrettyDuration(time.Duration(int64(1203123912312)))
	b.Logf("Pre %s", time.Duration(x).String())
	var a string
	for b.Loop() {
		a = x.String()
	}
	b.Logf("Post %s", a)
}
