// Copyright 2026 The go-qrl Authors
// This file is part of the go-qrl library.

package runtime

import (
	"bytes"
	"crypto/sha3"
	"testing"

	"github.com/theQRL/go-qrl/core/vm"
	"github.com/theQRL/go-qrl/params"
	cryptomldsa87 "github.com/theQRL/go-qrllib/crypto/ml_dsa_87"
)

func TestMLDSA87VerifyPrecompileStaticCall(t *testing.T) {
	signer, err := cryptomldsa87.New()
	if err != nil {
		t.Fatal(err)
	}
	digest := sha3.SumSHAKE256([]byte("QRL ML-DSA-87 runtime test"), vm.WordBytes)
	context := []byte("QRL runtime precompile test")
	signature, err := signer.Sign(context, digest)
	if err != nil {
		t.Fatal(err)
	}
	publicKey := signer.GetPK()
	var input []byte
	input = append(input, digest...)
	input = append(input, publicKey[:]...)
	input = append(input, signature[:]...)
	input = append(input, byte(len(context)))
	input = append(input, context...)

	code := []byte{
		byte(vm.CALLDATASIZE),
		byte(vm.PUSH1), 0,
		byte(vm.PUSH1), 0,
		byte(vm.CALLDATACOPY),
		byte(vm.PUSH1), byte(vm.WordBytes),
		byte(vm.PUSH1), 0,
		byte(vm.CALLDATASIZE),
		byte(vm.PUSH1), 0,
		byte(vm.PUSH1), 3,
		byte(vm.GAS),
		byte(vm.STATICCALL),
		byte(vm.POP),
		byte(vm.PUSH1), byte(vm.WordBytes),
		byte(vm.PUSH1), 0,
		byte(vm.RETURN),
	}
	output, _, err := Execute(code, input, &Config{ChainConfig: params.AllBeaconProtocolChanges})
	if err != nil {
		t.Fatal(err)
	}
	want := make([]byte, vm.WordBytes)
	want[vm.WordBytes-1] = 1
	if !bytes.Equal(output, want) {
		t.Fatalf("verification output %x, want %x", output, want)
	}
}
