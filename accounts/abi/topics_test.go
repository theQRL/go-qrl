// Copyright 2020 The go-ethereum Authors
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
	gomath "math"
	"math/big"
	"reflect"
	"testing"

	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/common/math"
	"github.com/theQRL/go-qrl/crypto"
)

func TestMakeTopics(t *testing.T) {
	t.Parallel()

	// intTopic returns the 64-byte two's-complement encoding of v for signed
	// integers. Negative numbers get 0xff sign extension; positive numbers
	// are zero-padded in the upper bytes.
	intTopic := func(v int64) common.LogTopic {
		var t common.LogTopic
		bi := new(big.Int).SetInt64(v)
		copy(t[:], math.U512Bytes(bi))
		return t
	}
	bigTopic := func(v *big.Int) common.LogTopic {
		var t common.LogTopic
		copy(t[:], math.U512Bytes(new(big.Int).Set(v)))
		return t
	}
	uintTopic := func(v uint64) common.LogTopic {
		var t common.LogTopic
		copy(t[:], math.U512Bytes(new(big.Int).SetUint64(v)))
		return t
	}
	hashTopic := func(hash []byte) common.LogTopic {
		var t common.LogTopic
		copy(t[:common.HashLength], hash)
		return t
	}
	function := functionValue(1, [4]byte{0xde, 0xad, 0xbe, 0xef})

	type args struct {
		query [][]any
	}
	tests := []struct {
		name    string
		args    args
		want    [][]common.LogTopic
		wantErr bool
	}{
		{
			"support fixed byte types, left-packed into the 64-byte topic",
			args{[][]any{{[5]byte{1, 2, 3, 4, 5}}}},
			[][]common.LogTopic{{common.LogTopic{1, 2, 3, 4, 5}}},
			false,
		},
		{
			"support common hash types in topics",
			args{[][]any{{common.Hash{1, 2, 3, 4, 5}}}},
			// Hash topics are ABI bytes32 values: high 32 bytes hold the
			// hash, low 32 bytes are zero padding.
			[][]common.LogTopic{{func() common.LogTopic {
				h := common.Hash{1, 2, 3, 4, 5}
				return hashTopic(h[:])
			}()}},
			false,
		},
		{
			"support address types in topics",
			args{[][]any{{common.Address{1, 2, 3, 4, 5}}}},
			// Address right-aligned in the 64-byte slot.
			[][]common.LogTopic{{func() common.LogTopic {
				var t common.LogTopic
				addr := common.Address{1, 2, 3, 4, 5}
				copy(t[common.LogTopicLength-common.AddressLength:], addr[:])
				return t
			}()}},
			false,
		},
		{
			"support positive *big.Int types in topics",
			args{[][]any{
				{big.NewInt(1)},
				{new(big.Int).Lsh(big.NewInt(2), 254)},
				{new(big.Int).Lsh(big.NewInt(1), 511)},
			}},
			[][]common.LogTopic{
				{uintTopic(1)},
				// 2 << 254 is 2^255, which sits in the high bit of the low
				// 32 bytes of the slot.
				{func() common.LogTopic {
					var t common.LogTopic
					t[common.LogTopicLength-common.HashLength] = 0x80
					return t
				}()},
				// 2^511 uses the high bit of the full 64-byte VM64 topic.
				{func() common.LogTopic {
					var t common.LogTopic
					t[0] = 0x80
					return t
				}()},
			},
			false,
		},
		{
			"support negative *big.Int types in topics",
			args{[][]any{
				{big.NewInt(-1)},
				{big.NewInt(gomath.MinInt64)},
			}},
			[][]common.LogTopic{
				{bigTopic(big.NewInt(-1))},
				{bigTopic(big.NewInt(gomath.MinInt64))},
			},
			false,
		},
		{
			"support boolean types in topics",
			args{[][]any{
				{true},
				{false},
			}},
			[][]common.LogTopic{
				{uintTopic(1)},
				{common.LogTopic{}},
			},
			false,
		},
		{
			"support int/uint(8/16/32/64) types in topics",
			args{[][]any{
				{int8(-2)},
				{int16(-3)},
				{int32(-4)},
				{int64(-5)},
				{int8(1)},
				{int16(256)},
				{int32(65536)},
				{int64(4294967296)},
				{uint8(1)},
				{uint16(256)},
				{uint32(65536)},
				{uint64(4294967296)},
			}},
			[][]common.LogTopic{
				{intTopic(-2)},
				{intTopic(-3)},
				{intTopic(-4)},
				{intTopic(-5)},
				{intTopic(1)},
				{intTopic(256)},
				{intTopic(65536)},
				{intTopic(4294967296)},
				{uintTopic(1)},
				{uintTopic(256)},
				{uintTopic(65536)},
				{uintTopic(4294967296)},
			},
			false,
		},
		{
			"support string types in topics",
			args{[][]any{{"hello world"}}},
			// Indexed dynamic values store the Keccak256 hash as ABI bytes32.
			[][]common.LogTopic{{hashTopic(crypto.Keccak256([]byte("hello world")))}},
			false,
		},
		{
			"support byte slice types in topics",
			args{[][]any{{[]byte{1, 2, 3}}}},
			[][]common.LogTopic{{hashTopic(crypto.Keccak256([]byte{1, 2, 3}))}},
			false,
		},
		{
			"hash function values wider than a topic",
			args{[][]any{{function}}},
			[][]common.LogTopic{{common.HashToLogTopic(crypto.Keccak256Hash(function[:]))}},
			false,
		},
		{
			"support static byte arrays up to the full topic width",
			args{[][]any{{[64]byte{1, 2, 3}}}},
			[][]common.LogTopic{{common.LogTopic{1, 2, 3}}},
			false,
		},
		{
			"error on static byte arrays wider than the topic",
			args{[][]any{{[65]byte{1, 2, 3}}}},
			nil,
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := MakeTopics(tt.args.query...)
			if (err != nil) != tt.wantErr {
				t.Errorf("makeTopics() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("makeTopics() = %v, want %v", got, tt.want)
			}
		})
	}

	t.Run("does not mutate big.Int", func(t *testing.T) {
		t.Parallel()
		want := [][]common.LogTopic{{func() common.LogTopic {
			var t common.LogTopic
			copy(t[:], math.U512Bytes(big.NewInt(-1)))
			return t
		}()}}

		in := big.NewInt(-1)
		got, err := MakeTopics([]any{in})
		if err != nil {
			t.Fatalf("makeTopics() error = %v", err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("makeTopics() = %v, want %v", got, want)
		}
		if orig := big.NewInt(-1); in.Cmp(orig) != 0 {
			t.Fatalf("makeTopics() mutated an input parameter from %v to %v", orig, in)
		}
	})
}

type args struct {
	createObj func() any
	resultObj func() any
	resultMap func() map[string]any
	fields    Arguments
	topics    []common.LogTopic
}

type bytesStruct struct {
	StaticBytes [5]byte
}
type int8Struct struct {
	Int8Value int8
}
type int256Struct struct {
	Int256Value *big.Int
}

// hashStruct receives the indexed Keccak-256 hash of a dynamic type.
type hashStruct struct {
	HashValue common.Hash
}

// funcStruct receives the hash of an indexed function value.
type funcStruct struct {
	FuncValue common.Hash
}

type topicTest struct {
	name    string
	args    args
	wantErr bool
}

// allOnesTopic is the 64-byte topic whose every byte is 0xff — the canonical
// two's-complement encoding of -1 sign-extended across the whole slot.
var allOnesTopic = func() common.LogTopic {
	var t common.LogTopic
	for i := range t {
		t[i] = 0xff
	}
	return t
}()

func setupTopicsTests() []topicTest {
	bytesType, _ := NewType("bytes5", "", nil)
	int8Type, _ := NewType("int8", "", nil)
	int256Type, _ := NewType("int256", "", nil)
	tupleType, _ := NewType("tuple(int256,int8)", "", nil)
	stringType, _ := NewType("string", "", nil)
	funcType, _ := NewType("function", "", nil)
	hashValue := crypto.Keccak256Hash([]byte("stringtopic"))

	tests := []topicTest{
		{
			name: "support fixed byte types, right padded to 32 bytes",
			args: args{
				createObj: func() any { return &bytesStruct{} },
				resultObj: func() any { return &bytesStruct{StaticBytes: [5]byte{1, 2, 3, 4, 5}} },
				resultMap: func() map[string]any {
					return map[string]any{"staticBytes": [5]byte{1, 2, 3, 4, 5}}
				},
				fields: Arguments{Argument{
					Name:    "staticBytes",
					Type:    bytesType,
					Indexed: true,
				}},
				topics: []common.LogTopic{
					{1, 2, 3, 4, 5},
				},
			},
			wantErr: false,
		},
		{
			name: "int8 with negative value",
			args: args{
				createObj: func() any { return &int8Struct{} },
				resultObj: func() any { return &int8Struct{Int8Value: -1} },
				resultMap: func() map[string]any {
					return map[string]any{"int8Value": int8(-1)}
				},
				fields: Arguments{Argument{
					Name:    "int8Value",
					Type:    int8Type,
					Indexed: true,
				}},
				// Two's complement -1 sign-extended to the 64-byte ABI slot.
				topics: []common.LogTopic{allOnesTopic},
			},
			wantErr: false,
		},
		{
			name: "int256 with negative value",
			args: args{
				createObj: func() any { return &int256Struct{} },
				resultObj: func() any { return &int256Struct{Int256Value: big.NewInt(-1)} },
				resultMap: func() map[string]any {
					return map[string]any{"int256Value": big.NewInt(-1)}
				},
				fields: Arguments{Argument{
					Name:    "int256Value",
					Type:    int256Type,
					Indexed: true,
				}},
				topics: []common.LogTopic{allOnesTopic},
			},
			wantErr: false,
		},
		{
			name: "hash type",
			args: args{
				createObj: func() any { return &hashStruct{} },
				resultObj: func() any {
					return &hashStruct{hashValue}
				},
				resultMap: func() map[string]any {
					return map[string]any{"hashValue": hashValue}
				},
				fields: Arguments{Argument{
					Name:    "hashValue",
					Type:    stringType,
					Indexed: true,
				}},
				topics: []common.LogTopic{
					common.HashToLogTopic(hashValue),
				},
			},
			wantErr: false,
		},
		{
			name: "function type",
			args: args{
				createObj: func() any { return &funcStruct{} },
				resultObj: func() any { return &funcStruct{FuncValue: hashValue} },
				resultMap: func() map[string]any {
					return map[string]any{"funcValue": hashValue}
				},
				fields: Arguments{Argument{
					Name:    "funcValue",
					Type:    funcType,
					Indexed: true,
				}},
				topics: []common.LogTopic{common.HashToLogTopic(hashValue)},
			},
			wantErr: false,
		},
		{
			name: "error on topic/field count mismatch",
			args: args{
				createObj: func() any { return nil },
				resultObj: func() any { return nil },
				resultMap: func() map[string]any { return make(map[string]any) },
				fields: Arguments{Argument{
					Name:    "tupletype",
					Type:    tupleType,
					Indexed: true,
				}},
				topics: []common.LogTopic{},
			},
			wantErr: true,
		},
		{
			name: "error on unindexed arguments",
			args: args{
				createObj: func() any { return &int256Struct{} },
				resultObj: func() any { return &int256Struct{} },
				resultMap: func() map[string]any { return make(map[string]any) },
				fields: Arguments{Argument{
					Name:    "int256Value",
					Type:    int256Type,
					Indexed: false,
				}},
				topics: []common.LogTopic{allOnesTopic},
			},
			wantErr: true,
		},
		{
			name: "error on tuple in topic reconstruction",
			args: args{
				createObj: func() any { return &tupleType },
				resultObj: func() any { return &tupleType },
				resultMap: func() map[string]any { return make(map[string]any) },
				fields: Arguments{Argument{
					Name:    "tupletype",
					Type:    tupleType,
					Indexed: true,
				}},
				topics: []common.LogTopic{{0}},
			},
			wantErr: true,
		},
	}

	return tests
}

func TestParseTopics(t *testing.T) {
	t.Parallel()
	tests := setupTopicsTests()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			createObj := tt.args.createObj()
			if err := ParseTopics(createObj, tt.args.fields, tt.args.topics); (err != nil) != tt.wantErr {
				t.Errorf("parseTopics() error = %v, wantErr %v", err, tt.wantErr)
			}
			resultObj := tt.args.resultObj()
			if !reflect.DeepEqual(createObj, resultObj) {
				t.Errorf("parseTopics() = %v, want %v", createObj, resultObj)
			}
		})
	}
}

func TestParseTopicsIntoMap(t *testing.T) {
	t.Parallel()
	tests := setupTopicsTests()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			outMap := make(map[string]any)
			if err := ParseTopicsIntoMap(outMap, tt.args.fields, tt.args.topics); (err != nil) != tt.wantErr {
				t.Errorf("parseTopicsIntoMap() error = %v, wantErr %v", err, tt.wantErr)
			}
			resultMap := tt.args.resultMap()
			if !reflect.DeepEqual(outMap, resultMap) {
				t.Errorf("parseTopicsIntoMap() = %v, want %v", outMap, resultMap)
			}
		})
	}
}
