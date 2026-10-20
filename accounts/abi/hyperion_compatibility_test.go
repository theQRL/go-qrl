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

package abi_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"math/big"
	"os"
	"reflect"
	"testing"

	"github.com/theQRL/go-qrl/accounts/abi"
	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/core/vm"
	"github.com/theQRL/go-qrl/core/vm/runtime"
)

// These vectors contain results from Hyperion 0.2.0, not from either SDK's
// encoder. The same JSON is used by web3.js/fixtures/abi/hyperion-0.2.0.json.
func TestHyperionABICompatibility(t *testing.T) {
	data, err := os.ReadFile("testdata/hyperion-0.2.0.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Bytecode string
		Cases    []struct {
			Name     string
			Inputs   []abi.ArgumentMarshaling
			Values   []any
			CallData string
			Encoded  string
			Packed   bool
			Panic    bool
			Reason   string
		}
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	bytesType, err := abi.NewType("bytes", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range fixture.Cases {
		t.Run(c.Name, func(t *testing.T) {
			want := common.FromHex(c.Encoded)
			output, _, err := runtime.Execute(
				common.FromHex(fixture.Bytecode),
				common.FromHex(c.CallData),
				&runtime.Config{GasLimit: 10_000_000},
			)
			if c.Panic {
				if !errors.Is(err, vm.ErrExecutionReverted) {
					t.Fatalf("expected compiler panic, got %v", err)
				}
				if !bytes.Equal(output, want) {
					t.Fatalf("compiler panic mismatch: got %x, want %x", output, want)
				}
				if len(output) != 4+64 || !bytes.Equal(output[:4], common.FromHex("4e487b71")) {
					t.Fatalf("unexpected panic selector or payload size: %x", output)
				}
				reason, err := abi.UnpackRevert(output)
				if err != nil || reason != c.Reason {
					t.Fatalf("UnpackRevert: got %q, %v; want %q", reason, err, c.Reason)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			// Each successful fixture method returns its ABI result as bytes.
			returned, err := (abi.Arguments{{Type: bytesType}}).Unpack(output)
			if err != nil {
				t.Fatal(err)
			}
			if got := returned[0].([]byte); !bytes.Equal(got, want) {
				t.Fatalf("compiler output mismatch: got %x, want %x", got, want)
			}
			// Go's ABI package does not expose an encodePacked operation.
			if c.Packed {
				return
			}
			args := make(abi.Arguments, len(c.Inputs))
			values := make([]any, len(c.Values))
			for i, input := range c.Inputs {
				typ, err := abi.NewType(input.Type, input.InternalType, input.Components)
				if err != nil {
					t.Fatal(err)
				}
				args[i] = abi.Argument{Name: input.Name, Type: typ}
				values[i] = hyperionFixtureValue(t, typ, c.Values[i]).Interface()
			}
			got, err := args.Pack(values...)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("Pack mismatch: got %x, want %x", got, want)
			}
			unpacked, err := args.Unpack(want)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(unpacked, values) {
				t.Fatalf("Unpack mismatch: got %#v, want %#v", unpacked, values)
			}
		})
	}
}

func hyperionFixtureValue(t *testing.T, typ abi.Type, value any) reflect.Value {
	t.Helper()
	switch typ.T {
	case abi.IntTy, abi.UintTy:
		n, ok := new(big.Int).SetString(value.(string), 10)
		if !ok {
			t.Fatalf("invalid fixture integer: %v", value)
		}
		return reflect.ValueOf(n)
	case abi.StringTy:
		return reflect.ValueOf(value.(string))
	case abi.BytesTy:
		return reflect.ValueOf(common.FromHex(value.(string)))
	case abi.FunctionTy:
		data := common.FromHex(value.(string))
		result := reflect.New(typ.GetType()).Elem()
		if len(data) != result.Len() {
			t.Fatalf("invalid function fixture length: %d", len(data))
		}
		reflect.Copy(result, reflect.ValueOf(data))
		return result
	case abi.ArrayTy, abi.SliceTy:
		items := value.([]any)
		var result reflect.Value
		if typ.T == abi.SliceTy {
			result = reflect.MakeSlice(typ.GetType(), len(items), len(items))
		} else {
			result = reflect.New(typ.GetType()).Elem()
			if len(items) != result.Len() {
				t.Fatalf("invalid array fixture length: %d", len(items))
			}
		}
		for i, item := range items {
			result.Index(i).Set(hyperionFixtureValue(t, *typ.Elem, item))
		}
		return result
	case abi.TupleTy:
		result := reflect.New(typ.GetType()).Elem()
		for i, item := range value.([]any) {
			result.Field(i).Set(hyperionFixtureValue(t, *typ.TupleElems[i], item))
		}
		return result
	default:
		t.Fatalf("unsupported fixture type: %v", typ)
		return reflect.Value{}
	}
}
