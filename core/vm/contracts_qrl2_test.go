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

package vm

import (
	"bytes"
	"crypto/sha3"
	"encoding/hex"
	"math"
	"reflect"
	"sort"
	"testing"

	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/params"
	cryptomldsa87 "github.com/theQRL/go-qrllib/crypto/ml_dsa_87"
)

func TestShake256Hash(t *testing.T) {
	want, err := hex.DecodeString("46b9dd2b0ba88d13233b3feb743eeb243fcd52ea62b81b82b50c27646ed5762fd75dc4ddd8c0f200cb05019d67b592f6fc821c49479ab48640292eacb3b7c4be")
	if err != nil {
		t.Fatal(err)
	}

	got, err := new(shake256hash).Run(nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected SHAKE256 output: got %x, want %x", got, want)
	}
	if len(got) != 64 {
		t.Fatalf("unexpected SHAKE256 output length: got %d, want 64", len(got))
	}
}

func TestShake256Gas(t *testing.T) {
	precompile := new(shake256hash)
	tests := []struct {
		length int
		gas    uint64
	}{
		{length: 0, gas: 240},
		{length: 1, gas: 288},
		{length: 63, gas: 288},
		{length: 64, gas: 288},
		{length: 65, gas: 336},
	}
	for _, test := range tests {
		if got := precompile.RequiredGas(make([]byte, test.length)); got != test.gas {
			t.Errorf("length %d: got %d gas, want %d", test.length, got, test.gas)
		}
	}

	if _, _, err := RunPrecompiledContract(precompile, nil, params.Shake256BaseGas-1); err != ErrOutOfGas {
		t.Fatalf("got %v, want %v", err, ErrOutOfGas)
	}
	// With 48 gas per 64-byte word the largest input cannot overflow uint64:
	// toWordSize(MaxUint64) = 2^58 + 1 words, so the cost is
	// (2^58 + 1) * 48 + 240 = 13835058055282163952 and the saturation guard
	// stays a defensive branch. The value must be exact and must not wrap.
	const largest = uint64(13835058055282163952)
	if got := shake256Gas(math.MaxUint64); got != largest {
		t.Fatalf("largest input gas: got %d, want %d", got, largest)
	}
	if shake256Gas(math.MaxUint64) < shake256Gas(math.MaxUint64-64) {
		t.Fatal("SHAKE256 gas wrapped around for the largest input")
	}
}

func TestQRL2PrecompileRegistry(t *testing.T) {
	verifyAddress := common.BytesToAddress([]byte{3})
	if _, ok := PrecompiledContractsZond[verifyAddress].(*mldsa87VerifyLegacy32); !ok {
		t.Fatal("baseline precompile slot 3 is not the legacy 32-byte ML-DSA-87 verifier")
	}
	if _, ok := PrecompiledContractsQRL2PQ[verifyAddress].(*mldsa87Verify); !ok {
		t.Fatal("QRL2 PQ precompile slot 3 is not the 64-byte ML-DSA-87 verifier")
	}

	shakeAddress := common.BytesToAddress([]byte{6})
	if _, ok := PrecompiledContractsZond[shakeAddress]; ok {
		t.Fatal("baseline precompile set unexpectedly contains slot 6")
	}
	if _, ok := PrecompiledContractsQRL2PQ[shakeAddress].(*shake256hash); !ok {
		t.Fatal("QRL2 PQ precompile slot 6 is not SHAKE256")
	}
}

func TestQRL2PrecompileRegistryConsistency(t *testing.T) {
	verifyAddress := common.BytesToAddress([]byte{3})
	for address, precompile := range PrecompiledContractsZond {
		if address == verifyAddress {
			continue
		}
		if got, want := reflect.TypeOf(PrecompiledContractsQRL2PQ[address]), reflect.TypeOf(precompile); got != want {
			t.Errorf("precompile %s implementation: got %v, want %v", address, got, want)
		}
	}

	for _, test := range []struct {
		name        string
		precompiles map[common.Address]PrecompiledContract
		addresses   []common.Address
	}{
		{"zond", PrecompiledContractsZond, PrecompiledAddressesZond},
		{"qrl2pq", PrecompiledContractsQRL2PQ, PrecompiledAddressesQRL2PQ},
	} {
		t.Run(test.name, func(t *testing.T) {
			want := make([]common.Address, 0, len(test.precompiles))
			for address := range test.precompiles {
				want = append(want, address)
			}
			got := append([]common.Address(nil), test.addresses...)
			sort.Slice(want, func(i, j int) bool { return bytes.Compare(want[i][:], want[j][:]) < 0 })
			sort.Slice(got, func(i, j int) bool { return bytes.Compare(got[i][:], got[j][:]) < 0 })
			if !reflect.DeepEqual(got, want) {
				t.Errorf("precompile addresses: got %v, want %v", got, want)
			}
		})
	}
}

func TestActivePrecompilesUsesActivationRule(t *testing.T) {
	shakeAddress := common.BytesToAddress([]byte{6})
	xmssAddress := common.BytesToAddress([]byte{7})
	contains := func(addresses []common.Address, want common.Address) bool {
		for _, address := range addresses {
			if address == want {
				return true
			}
		}
		return false
	}

	if contains(ActivePrecompiles(params.Rules{}), shakeAddress) {
		t.Fatal("baseline active precompile list unexpectedly contains slot 6")
	}
	if !contains(ActivePrecompiles(params.Rules{IsQRL2PQPrecompiles: true}), shakeAddress) {
		t.Fatal("activated precompile list does not contain slot 6")
	}
	if !contains(ActivePrecompiles(params.Rules{}), xmssAddress) {
		t.Error("baseline active precompile list does not contain slot 7")
	}
	if !contains(ActivePrecompiles(params.Rules{IsQRL2PQPrecompiles: true}), xmssAddress) {
		t.Error("activated precompile list does not contain slot 7")
	}
}

func TestQRVMPrecompileSelectionUsesActivationRule(t *testing.T) {
	verifyAddress := common.BytesToAddress([]byte{3})
	shakeAddress := common.BytesToAddress([]byte{6})
	xmssAddress := common.BytesToAddress([]byte{7})

	legacyVM := &QRVM{chainRules: params.Rules{}}
	if precompile, ok := legacyVM.precompile(verifyAddress); !ok {
		t.Fatal("baseline QRVM does not expose slot 3")
	} else if _, ok := precompile.(*mldsa87VerifyLegacy32); !ok {
		t.Fatalf("baseline QRVM selected %T for slot 3", precompile)
	}
	if _, ok := legacyVM.precompile(shakeAddress); ok {
		t.Fatal("baseline QRVM unexpectedly exposes slot 6")
	}

	qrl2VM := &QRVM{chainRules: params.Rules{IsQRL2PQPrecompiles: true}}
	if precompile, ok := qrl2VM.precompile(verifyAddress); !ok {
		t.Fatal("activated QRVM does not expose slot 3")
	} else if _, ok := precompile.(*mldsa87Verify); !ok {
		t.Fatalf("activated QRVM selected %T for slot 3", precompile)
	}
	if precompile, ok := qrl2VM.precompile(shakeAddress); !ok {
		t.Fatal("activated QRVM does not expose slot 6")
	} else if _, ok := precompile.(*shake256hash); !ok {
		t.Fatalf("activated QRVM selected %T for slot 6", precompile)
	}
	for _, qrvm := range []*QRVM{legacyVM, qrl2VM} {
		if precompile, ok := qrvm.precompile(xmssAddress); !ok {
			t.Errorf("QRL2 PQ activation %t: QRVM does not expose slot 7", qrvm.chainRules.IsQRL2PQPrecompiles)
		} else if _, ok := precompile.(*xmssVerify); !ok {
			t.Errorf("QRL2 PQ activation %t: QRVM selected %T for slot 7", qrvm.chainRules.IsQRL2PQPrecompiles, precompile)
		}
	}
}

func TestMLDSA87VerifyActivationBoundary(t *testing.T) {
	context := []byte("QNS-SIGN-v1")
	legacyInput := newRawMLDSA87VerifyInputWithDigestLength(t, context, common.HashLength)
	qrl2Input := newRawMLDSA87VerifyInputWithDigestLength(t, context, mldsa87VerifyDigestLength)

	legacyOutput, err := new(mldsa87VerifyLegacy32).Run(legacyInput)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(legacyOutput, common.LeftPadBytes([]byte{1}, WordBytes)) {
		t.Fatalf("legacy verifier output %x, want true word", legacyOutput)
	}
	if output, err := new(mldsa87Verify).Run(legacyInput); err != nil {
		t.Fatal(err)
	} else if output != nil {
		t.Fatalf("64-byte verifier accepted legacy frame: %x", output)
	}

	qrl2Output, err := new(mldsa87Verify).Run(qrl2Input)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(qrl2Output, common.LeftPadBytes([]byte{1}, WordBytes)) {
		t.Fatalf("64-byte verifier output %x, want true word", qrl2Output)
	}
	if output, err := new(mldsa87VerifyLegacy32).Run(qrl2Input); err != nil {
		t.Fatal(err)
	} else if output != nil {
		t.Fatalf("legacy verifier accepted 64-byte frame: %x", output)
	}
}

func TestMLDSA87VerifyOutputIsolation(t *testing.T) {
	input := newRawMLDSA87VerifyInput(t, []byte("QNS-SIGN-v1"))
	first, err := new(mldsa87Verify).Run(input)
	if err != nil {
		t.Fatal(err)
	}
	first[len(first)-1] = 0

	second, err := new(mldsa87Verify).Run(input)
	if err != nil {
		t.Fatal(err)
	}
	if second[len(second)-1] != 1 {
		t.Fatalf("mutating one success result changed a later result: %x", second)
	}
}

func TestMLDSA87VerifyLayout(t *testing.T) {
	if mldsa87VerifyDigestLength != 64 {
		t.Fatalf("digest length %d, want 64", mldsa87VerifyDigestLength)
	}
	if mldsa87VerifyMinInputLength != 7284 {
		t.Fatalf("fixed input portion %d, want 7284", mldsa87VerifyMinInputLength)
	}
}

// TestMLDSA87VerifyVectorProvenance regenerates the valid entry of
// testdata/precompiles/mldsa87_verify.json from its published inputs so the
// vector stays reproducible: seed = 32 bytes of 0x51, message representative
// = SHAKE256("QNS local integration vector", 64), context = "QNS-SIGN-v1",
// deterministic (non-hedged) FIPS 204 signing.
func TestMLDSA87VerifyVectorProvenance(t *testing.T) {
	var seed [cryptomldsa87.SEED_BYTES]uint8
	for i := range seed {
		seed[i] = 0x51
	}
	signer, err := cryptomldsa87.NewMLDSA87FromSeed(seed)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha3.SumSHAKE256([]byte("QNS local integration vector"), mldsa87VerifyDigestLength)
	context := []byte("QNS-SIGN-v1")
	signature, err := signer.SignDeterministic(context, digest)
	if err != nil {
		t.Fatal(err)
	}
	publicKey := signer.GetPK()
	want := make([]byte, 0, mldsa87VerifyMinInputLength+len(context))
	want = append(want, digest...)
	want = append(want, publicKey[:]...)
	want = append(want, signature[:]...)
	want = append(want, byte(len(context)))
	want = append(want, context...)

	tests, err := loadJson("mldsa87_verify")
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range tests {
		if test.Name != "mldsa87_verify_valid" {
			continue
		}
		got, err := hex.DecodeString(test.Input)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Fatal("mldsa87_verify_valid input drifted from its deterministic provenance")
		}
		if want := common.Bytes2Hex(common.LeftPadBytes([]byte{1}, WordBytes)); test.Expected != want {
			t.Fatalf("expected output %s, want %s", test.Expected, want)
		}
		return
	}
	t.Fatal("mldsa87_verify_valid vector missing")
}

func BenchmarkPrecompiledShake256(b *testing.B) {
	input := make([]byte, 64)
	precompile := new(shake256hash)
	gas := precompile.RequiredGas(input)
	b.ReportAllocs()
	for b.Loop() {
		if _, _, err := RunPrecompiledContract(precompile, input, gas); err != nil {
			b.Fatal(err)
		}
	}
}
