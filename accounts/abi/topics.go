// Copyright 2018 The go-ethereum Authors
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
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
	"reflect"

	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/common/math"
	"github.com/theQRL/go-qrl/crypto"
)

// MakeTopics converts a filter query argument list into a filter topic set.
// Topics are 64-byte ABI slots: numeric values are right-aligned, while
// fixed bytes and Keccak hash topics are left-aligned like ABI bytes32 values.
func MakeTopics(query ...[]any) ([][]common.LogTopic, error) {
	topics := make([][]common.LogTopic, len(query))
	for i, filter := range query {
		for _, rule := range filter {
			var topic common.LogTopic

			// Try to generate the topic based on simple types
			switch rule := rule.(type) {
			case common.LogTopic:
				topic = rule
			case common.Hash:
				topic = common.HashToLogTopic(rule)
			case common.Address:
				topic = common.AddressToLogTopic(rule)
			case *big.Int:
				topic = common.BytesToRightAlignedLogTopic(math.U512Bytes(new(big.Int).Set(rule)))
			case bool:
				if rule {
					topic[common.LogTopicLength-1] = 1
				}
			case int8:
				topic = genIntType(int64(rule), 1)
			case int16:
				topic = genIntType(int64(rule), 2)
			case int32:
				topic = genIntType(int64(rule), 4)
			case int64:
				topic = genIntType(rule, 8)
			case uint8:
				topic = genUintType(uint64(rule))
			case uint16:
				topic = genUintType(uint64(rule))
			case uint32:
				topic = genUintType(uint64(rule))
			case uint64:
				topic = genUintType(rule)
			case string:
				topic = common.HashToLogTopic(crypto.Keccak256Hash([]byte(rule)))
			case []byte:
				topic = common.HashToLogTopic(crypto.Keccak256Hash(rule))

			default:
				// todo(rjl493456442) according to hyperion documentation, indexed event
				// parameters that are not value types i.e. arrays and structs are not
				// stored directly but instead a keccak256-hash of an encoding is stored.
				//
				// We only convert strings, bytes, and functions to hash, still need to deal
				// with arrays and structs.

				// Attempt to generate the topic from funky types
				val := reflect.ValueOf(rule)
				switch {
				// static byte array
				case val.Kind() == reflect.Array && reflect.TypeOf(rule).Elem().Kind() == reflect.Uint8:
					if val.Len() == common.AddressLength+4 {
						value := make([]byte, val.Len())
						reflect.Copy(reflect.ValueOf(value), val)
						topic = common.HashToLogTopic(crypto.Keccak256Hash(value))
						break
					}
					if val.Len() > common.LogTopicLength {
						return nil, fmt.Errorf("unsupported indexed type: %T exceeds the %d-byte topic width", rule, common.LogTopicLength)
					}
					b := make([]byte, val.Len())
					reflect.Copy(reflect.ValueOf(b), val)
					topic = common.BytesToLeftAlignedLogTopic(b)
				default:
					return nil, fmt.Errorf("unsupported indexed type: %T", rule)
				}
			}
			topics[i] = append(topics[i], topic)
		}
	}
	return topics, nil
}

func genUintType(rule uint64) common.LogTopic {
	var topic common.LogTopic
	binary.BigEndian.PutUint64(topic[common.LogTopicLength-8:], rule)
	return topic
}

func genIntType(rule int64, size uint) common.LogTopic {
	var topic common.LogTopic
	if rule < 0 {
		// if a rule is negative, we need to put it into two's complement,
		// extended to common.LogTopicLength bytes.
		for i := range topic {
			topic[i] = 0xff
		}
	}
	for i := range size {
		topic[common.LogTopicLength-i-1] = byte(rule >> (i * 8))
	}
	return topic
}

// ParseTopics converts the indexed topic fields into actual log field values.
func ParseTopics(out any, fields Arguments, topics []common.LogTopic) error {
	return parseTopicWithSetter(fields, topics,
		func(arg Argument, reconstr any) {
			field := reflect.ValueOf(out).Elem().FieldByName(ToCamelCase(arg.Name))
			field.Set(reflect.ValueOf(reconstr))
		})
}

// ParseTopicsIntoMap converts the indexed topic field-value pairs into map key-value pairs.
func ParseTopicsIntoMap(out map[string]any, fields Arguments, topics []common.LogTopic) error {
	return parseTopicWithSetter(fields, topics,
		func(arg Argument, reconstr any) {
			out[arg.Name] = reconstr
		})
}

// parseTopicWithSetter converts the indexed topic field-value pairs and stores them using the
// provided set function.
//
// Note, dynamic types cannot be reconstructed since they get mapped to Keccak256
// hashes as the topic value!
func parseTopicWithSetter(fields Arguments, topics []common.LogTopic, setter func(Argument, any)) error {
	// Sanity check that the fields and topics match up
	if len(fields) != len(topics) {
		return errors.New("topic/field count mismatch")
	}
	// Iterate over all the fields and reconstruct them from topics
	for i, arg := range fields {
		if !arg.Indexed {
			return errors.New("non-indexed field in topic reconstruction")
		}
		var reconstr any
		switch arg.Type.T {
		case TupleTy:
			return errors.New("tuple type in topic reconstruction")
		case StringTy, BytesTy, SliceTy, ArrayTy, FunctionTy:
			// Indexed strings, bytes, slices, arrays, and function values
			// store their Keccak-256 hash in the high 32 bytes of the topic.
			var hash common.Hash
			copy(hash[:], topics[i][:common.HashLength])
			reconstr = hash
		default:
			// Topic is already the width of an ABI slot (64 bytes); decode directly.
			var err error
			reconstr, err = toGoType(0, arg.Type, topics[i][:])
			if err != nil {
				return err
			}
		}
		// Use the setter function to store the value
		setter(arg, reconstr)
	}

	return nil
}
