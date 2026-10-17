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
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-qrl library. If not, see <http://www.gnu.org/licenses/>.

package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/crypto"
)

// readGenesisObject streams members instead of decoding an object into a map.
// Unknown fields are skipped with the same bounded-memory traversal.
func readGenesisObject(d *json.Decoder, member func(string) error) error {
	token, err := d.Token()
	if err != nil {
		return err
	}
	if token != json.Delim('{') {
		return errors.New("expected JSON object")
	}
	for d.More() {
		token, err := d.Token()
		if err != nil {
			return err
		}
		name, ok := token.(string)
		if !ok {
			return errors.New("expected object member name")
		}
		if err := member(name); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
	}
	_, err = d.Token() // Closing delimiter; the decoder checks the matching type.
	return err
}

func skipGenesisValue(d *json.Decoder) error {
	token, err := d.Token()
	if err != nil {
		return err
	}
	if delim, ok := token.(json.Delim); ok && (delim == '{' || delim == '[') {
		for d.More() {
			if err := skipGenesisValue(d); err != nil {
				return err
			}
		}
		_, err = d.Token()
	}
	return err
}

// Only these small fields use Decode. Neither alloc nor storage ever passes
// through json.Unmarshal/Decoder.Decode, which would buffer the whole object.
func genesisHeaderField(name string) bool {
	switch name {
	case "config", "timestamp", "extradata", "gaslimit", "mixhash", "coinbase",
		"number", "gasused", "parenthash", "basefeepergas":
		return true
	}
	return false
}

func (s *streamedGenesis) parse(input io.Reader) (*Genesis, error) {
	d := json.NewDecoder(input)
	d.UseNumber()
	header := make(map[string]json.RawMessage)
	var haveAlloc bool
	err := readGenesisObject(d, func(name string) error {
		name = strings.ToLower(name)
		if name == "alloc" {
			if haveAlloc {
				return errors.New("duplicate genesis allocation")
			}
			haveAlloc = true
			return readGenesisObject(d, func(addressText string) error {
				var address common.Address
				if err := address.UnmarshalText([]byte(addressText)); err != nil {
					return err
				}
				return s.parseAccount(d, address)
			})
		}
		if !genesisHeaderField(name) {
			return skipGenesisValue(d)
		}
		if _, exists := header[name]; exists {
			return errors.New("duplicate genesis field")
		}
		var value json.RawMessage
		if err := d.Decode(&value); err != nil {
			return err
		}
		header[name] = value
		return nil
	})
	if err != nil {
		return nil, err
	}
	if _, err := d.Token(); err != io.EOF {
		if err != nil {
			return nil, err
		}
		return nil, errors.New("extra data after genesis document")
	}
	if !haveAlloc {
		return nil, errors.New("missing required field 'alloc' for Genesis")
	}
	header["alloc"] = json.RawMessage("{}")
	data, err := json.Marshal(header)
	if err != nil {
		return nil, err
	}
	var g Genesis
	if err := json.Unmarshal(data, &g); err != nil {
		return nil, err
	}
	if g.Config == nil {
		return nil, errGenesisNoConfig
	}
	if g.Number != 0 {
		return nil, errors.New("can't commit genesis block with number > 0")
	}
	if err := g.Config.CheckConfigForkOrder(); err != nil {
		return nil, err
	}
	g.Alloc = nil
	return &g, nil
}

func (s *streamedGenesis) parseAccount(d *json.Decoder, address common.Address) error {
	owner := crypto.Keccak256Hash(address[:])
	key := append([]byte{'a'}, owner[:]...)
	if exists, err := s.index.Has(key); err != nil {
		return err
	} else if exists {
		return errors.New("duplicate allocation address")
	}
	fields := make(map[string]json.RawMessage)
	var haveStorage bool
	err := readGenesisObject(d, func(name string) error {
		name = strings.ToLower(name)
		if name == "storage" {
			if haveStorage {
				return errors.New("duplicate storage field")
			}
			haveStorage = true
			// An absent or null storage object is equivalent to an empty one.
			token, err := d.Token()
			if err != nil || token == nil {
				return err
			}
			if token != json.Delim('{') {
				return errors.New("expected storage object")
			}
			for d.More() {
				token, err := d.Token()
				if err != nil {
					return err
				}
				var slot storageJSON
				if err := slot.UnmarshalText([]byte(token.(string))); err != nil {
					return err
				}
				var text *string
				if err := d.Decode(&text); err != nil {
					return err
				}
				var value storageValue64JSON
				if text != nil {
					if err := value.UnmarshalText([]byte(*text)); err != nil {
						return err
					}
				}
				slotHash := crypto.Keccak256Hash(slot[:])
				key := append(append([]byte{'s'}, owner[:]...), slotHash[:]...)
				if exists, err := s.index.Has(key); err != nil {
					return err
				} else if exists {
					return errors.New("duplicate storage slot")
				}
				if err := s.index.Put(key, append(slot[:], value[:]...)); err != nil {
					return err
				}
			}
			_, err = d.Token()
			return err
		}
		switch name {
		case "code", "balance", "nonce", "seed":
			if _, exists := fields[name]; exists {
				return errors.New("duplicate account field")
			}
			var value json.RawMessage
			if err := d.Decode(&value); err != nil {
				return err
			}
			fields[name] = value
			return nil
		default:
			return skipGenesisValue(d)
		}
	})
	if err != nil {
		return err
	}
	data, err := json.Marshal(fields)
	if err != nil {
		return err
	}
	var account GenesisAccount
	if err := json.Unmarshal(data, &account); err != nil {
		return err
	}
	if account.Balance.Sign() < 0 {
		return errors.New("negative genesis balance")
	}
	return s.index.Put(key, append(address[:], data...))
}
