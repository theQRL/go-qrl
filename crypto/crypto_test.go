// Copyright 2014 The go-ethereum Authors
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

package crypto

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/hex"
	"os"
	"reflect"
	"testing"

	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/common/hexutil"
)

var testAddr = common.MustParseAddress("Q43261cfab65ca3444b9a1001528c60d9a23d19bad2bc21051f24bf4bccd804dc63e35f36b63ee11ca7775ec832d6580929136989106db0231c6f04840cf8ee16")
var testPrivHex = "289c2857d4598e37fb9647507e47a309d6133539bf21a8b9cb6df88fd5232032"

// These tests are sanity checks.
// They should ensure that we don't e.g. use Sha3-224 instead of Sha3-256
// and that the sha3 library uses keccak-f permutation.
func TestKeccak256Hash(t *testing.T) {
	msg := []byte("abc")
	exp, _ := hex.DecodeString("4e03657aea45a94fc7d47ba826c8d667c0d1e6e33a64a036ec44f58fa12d6c45")
	checkhash(t, "Sha3-256-array", func(in []byte) []byte { h := Keccak256Hash(in); return h[:] }, msg, exp)
}

func TestKeccak256Hasher(t *testing.T) {
	msg := []byte("abc")
	exp, _ := hex.DecodeString("4e03657aea45a94fc7d47ba826c8d667c0d1e6e33a64a036ec44f58fa12d6c45")
	hasher := NewKeccakState()
	checkhash(t, "Sha3-256-array", func(in []byte) []byte { h := HashData(hasher, in); return h[:] }, msg, exp)
}

func TestToECDSAErrors(t *testing.T) {
	if _, err := HexToECDSA("0000000000000000000000000000000000000000000000000000000000000000"); err == nil {
		t.Fatal("HexToECDSA should've returned error")
	}
	if _, err := HexToECDSA("ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"); err == nil {
		t.Fatal("HexToECDSA should've returned error")
	}
}

func BenchmarkSha3(b *testing.B) {
	a := []byte("hello world")
	for b.Loop() {
		Keccak256(a)
	}
}

func TestUnmarshalPubkey(t *testing.T) {
	key, err := UnmarshalPubkey(nil)
	if err != errInvalidPubkey || key != nil {
		t.Fatalf("expected error, got %v, %v", err, key)
	}
	key, err = UnmarshalPubkey([]byte{1, 2, 3})
	if err != errInvalidPubkey || key != nil {
		t.Fatalf("expected error, got %v, %v", err, key)
	}

	var (
		enc, _ = hex.DecodeString("04760c4460e5336ac9bbd87952a3c7ec4363fc0a97bd31c86430806e287b437fd1b01abc6e1db640cf3106b520344af1d58b00b57823db3e1407cbc433e1b6d04d")
		dec    = &ecdsa.PublicKey{
			Curve: S256(),
			X:     hexutil.MustDecodeBig("0x760c4460e5336ac9bbd87952a3c7ec4363fc0a97bd31c86430806e287b437fd1"),
			Y:     hexutil.MustDecodeBig("0xb01abc6e1db640cf3106b520344af1d58b00b57823db3e1407cbc433e1b6d04d"),
		}
	)
	key, err = UnmarshalPubkey(enc)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !reflect.DeepEqual(key, dec) {
		t.Fatal("wrong result")
	}
}

func TestInvalidSign(t *testing.T) {
	if _, err := Sign(make([]byte, 1), nil); err == nil {
		t.Errorf("expected sign with hash 1 byte to error")
	}
	if _, err := Sign(make([]byte, 33), nil); err == nil {
		t.Errorf("expected sign with hash 33 byte to error")
	}
}

func TestNewContractAddress(t *testing.T) {
	caddr0 := CreateAddress(testAddr, 0)
	caddr1 := CreateAddress(testAddr, 1)
	caddr2 := CreateAddress(testAddr, 2)
	addr0 := common.MustParseAddress("Qf2a607fd56723c510a1bf4b000fd1e778aedc94e78fcc8cb18d531ec673212284520097bbed26e2df76ce4a5360c73add776771dcd5c0ce56751e30c2413c366")
	addr1 := common.MustParseAddress("Q5c2ef024f69257cab379fd37fe2dc6c9242320f8771b1a6d6c86b6cefadcfc449c473414b5647b4ee3361f28a7a4cd47b10b6c6fa3399d056f851d97998f7748")
	addr2 := common.MustParseAddress("Qb000412862016ae61f4003114cc4eaa6ba614ccbf8b096e5686a7df6305cb82743e72610275437d11f05758befb7ba44b17fb05f588615893d661e787cd0738e")
	checkAddr(t, addr0, caddr0)
	checkAddr(t, addr1, caddr1)
	checkAddr(t, addr2, caddr2)
}

func TestLoadECDSA(t *testing.T) {
	tests := []struct {
		input string
		err   string
	}{
		// good
		{input: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"},
		{input: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef\n"},
		{input: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef\n\r"},
		{input: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef\r\n"},
		{input: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef\n\n"},
		{input: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef\n\r"},
		// bad
		{
			input: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcde",
			err:   "key file too short, want 64 hex characters",
		},
		{
			input: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcde\n",
			err:   "key file too short, want 64 hex characters",
		},
		{
			input: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdeX",
			err:   "invalid hex character 'X' in private key",
		},
		{
			input: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdefX",
			err:   "invalid character 'X' at end of key file",
		},
		{
			input: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef\n\n\n",
			err:   "key file too long, want 64 hex characters",
		},
	}

	for _, test := range tests {
		f, err := os.CreateTemp(t.TempDir(), "loadecdsa_test.*.txt")
		if err != nil {
			t.Fatal(err)
		}
		filename := f.Name()
		f.WriteString(test.input)
		f.Close()

		_, err = LoadECDSA(filename)
		switch {
		case err != nil && test.err == "":
			t.Fatalf("unexpected error for input %q:\n  %v", test.input, err)
		case err != nil && err.Error() != test.err:
			t.Fatalf("wrong error for input %q:\n  %v", test.input, err)
		case err == nil && test.err != "":
			t.Fatalf("LoadECDSA did not return error for input %q", test.input)
		}
	}
}

func TestSaveECDSA(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "saveecdsa_test.*.txt")
	if err != nil {
		t.Fatal(err)
	}
	file := f.Name()
	f.Close()
	defer os.Remove(file)

	key, _ := HexToECDSA(testPrivHex)
	if err := SaveECDSA(file, key); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadECDSA(file)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(key, loaded) {
		t.Fatal("loaded key not equal to saved key")
	}
}

func checkhash(t *testing.T, name string, f func([]byte) []byte, msg, exp []byte) {
	sum := f(msg)
	if !bytes.Equal(exp, sum) {
		t.Fatalf("hash %s mismatch: want: %x have: %x", name, exp, sum)
	}
}

func checkAddr(t *testing.T, addr0, addr1 common.Address) {
	if addr0 != addr1 {
		t.Fatalf("address mismatch: want: %x have: %x", addr0, addr1)
	}
}

// test to help Python team with integration of libsecp256k1
// skip but keep it after they are done
func TestPythonIntegration(t *testing.T) {
	kh := "289c2857d4598e37fb9647507e47a309d6133539bf21a8b9cb6df88fd5232032"
	k0, _ := HexToECDSA(kh)

	msg0 := Keccak256([]byte("foo"))
	sig0, _ := Sign(msg0, k0)

	msg1 := common.FromHex("00000000000000000000000000000000")
	sig1, _ := Sign(msg0, k0)

	t.Logf("msg: %x, privkey: %s sig: %x\n", msg0, kh, sig0)
	t.Logf("msg: %x, privkey: %s sig: %x\n", msg1, kh, sig1)
}

// goos: darwin
// goarch: arm64
// pkg: github.com/theQRL/go-qrl/crypto
// cpu: Apple M1 Pro
// BenchmarkKeccak256Hash
// BenchmarkKeccak256Hash-8   	  931095	      1270 ns/op	      32 B/op	       1 allocs/op
func BenchmarkKeccak256Hash(b *testing.B) {
	var input [512]byte
	rand.Read(input[:])

	b.ReportAllocs()
	for b.Loop() {
		Keccak256Hash(input[:])
	}
}

// goos: darwin
// goarch: arm64
// pkg: github.com/theQRL/go-qrl/crypto
// cpu: Apple M1 Pro
// BenchmarkHashData
// BenchmarkHashData-8   	  793386	      1278 ns/op	      32 B/op	       1 allocs/op
func BenchmarkHashData(b *testing.B) {
	var (
		input  [512]byte
		buffer = NewKeccakState()
	)
	rand.Read(input[:])

	b.ReportAllocs()
	for b.Loop() {
		HashData(buffer, input[:])
	}
}
