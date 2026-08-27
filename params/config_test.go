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

package params

import (
	"encoding/json"
	"math"
	"math/big"
	"reflect"
	"testing"
	"time"

	"github.com/theQRL/go-qrl/common"
)

func TestCheckCompatible(t *testing.T) {
	type test struct {
		stored, new   *ChainConfig
		headBlock     uint64
		headTimestamp uint64
		wantErr       *ConfigCompatError
	}
	tests := []test{
		{stored: AllBeaconProtocolChanges, new: AllBeaconProtocolChanges, headBlock: 0, headTimestamp: 0, wantErr: nil},
		{stored: AllBeaconProtocolChanges, new: AllBeaconProtocolChanges, headBlock: 0, headTimestamp: uint64(time.Now().Unix()), wantErr: nil},
		{stored: AllBeaconProtocolChanges, new: AllBeaconProtocolChanges, headBlock: 100, wantErr: nil},
		{
			stored: &ChainConfig{},
			new:    &ChainConfig{},
			// headBlock: 9,
			wantErr: nil,
		},
		{
			stored: &ChainConfig{ChainID: common.Big1},
			new:    &ChainConfig{ChainID: common.Big32},
			wantErr: &ConfigCompatError{
				What:        "chain ID",
				StoredBlock: common.Big1,
				NewBlock:    common.Big32,
			},
		},
		{
			stored:        &ChainConfig{ChainID: common.Big1, QRL2PQPrecompilesTime: newUint64(10)},
			new:           &ChainConfig{ChainID: common.Big1, QRL2PQPrecompilesTime: newUint64(20)},
			headTimestamp: 9,
			wantErr:       nil,
		},
		{
			stored:        &ChainConfig{ChainID: common.Big1, QRL2PQPrecompilesTime: newUint64(10)},
			new:           &ChainConfig{ChainID: common.Big1, QRL2PQPrecompilesTime: newUint64(20)},
			headTimestamp: 10,
			wantErr: &ConfigCompatError{
				What:         "QRL2 PQ precompiles fork timestamp",
				StoredTime:   newUint64(10),
				NewTime:      newUint64(20),
				RewindToTime: 9,
			},
		},
		{
			stored:        &ChainConfig{ChainID: common.Big1, QRL2PQPrecompilesTime: newUint64(10)},
			new:           &ChainConfig{ChainID: common.Big1},
			headTimestamp: 10,
			wantErr: &ConfigCompatError{
				What:         "QRL2 PQ precompiles fork timestamp",
				StoredTime:   newUint64(10),
				NewTime:      nil,
				RewindToTime: 9,
			},
		},
		// NOTE(rgeraldes24): not valid at the moment
		/*
			{
				stored:    AllBeaconProtocolChanges,
				new:       &ChainConfig{},
				headBlock: 3,
				wantErr: &ConfigCompatError{
					What:          "Homestead fork block",
					StoredBlock:   big.NewInt(0),
					NewBlock:      nil,
					RewindToBlock: 0,
				},
			},
			{
				stored:    AllBeaconProtocolChanges,
				new:       &ChainConfig{},
				headBlock: 3,
				wantErr: &ConfigCompatError{
					What:          "Homestead fork block",
					StoredBlock:   big.NewInt(0),
					NewBlock:      big.NewInt(1),
					RewindToBlock: 0,
				},
			},
			{
				stored:    &ChainConfig{},
				new:       &ChainConfig{},
				headBlock: 25,
				wantErr: &ConfigCompatError{
					What:          "EIP150 fork block",
					StoredBlock:   big.NewInt(10),
					NewBlock:      big.NewInt(20),
					RewindToBlock: 9,
				},
			},
			{
				stored:    &ChainConfig{},
				new:       &ChainConfig{},
				headBlock: 40,
				wantErr:   nil,
			},
			{
				stored:    &ChainConfig{},
				new:       &ChainConfig{},
				headBlock: 40,
				wantErr: &ConfigCompatError{
					What:          "Petersburg fork block",
					StoredBlock:   nil,
					NewBlock:      big.NewInt(31),
					RewindToBlock: 30,
				},
			},
			{
				stored:        &ChainConfig{},
				new:           &ChainConfig{},
				headTimestamp: 9,
				wantErr:       nil,
			},
			{
				stored:        &ChainConfig{},
				new:           &ChainConfig{},
				headTimestamp: 25,
				wantErr: &ConfigCompatError{
					What:         "Zond fork timestamp",
					StoredTime:   newUint64(10),
					NewTime:      newUint64(20),
					RewindToTime: 9,
				},
			},
		*/
	}

	for _, test := range tests {
		err := test.stored.CheckCompatible(test.new, test.headBlock, test.headTimestamp)
		if !reflect.DeepEqual(err, test.wantErr) {
			t.Errorf("error mismatch:\nstored: %v\nnew: %v\nheadBlock: %v\nheadTimestamp: %v\nerr: %v\nwant: %v", test.stored, test.new, test.headBlock, test.headTimestamp, err, test.wantErr)
		}
	}
}

func TestConfigRules(t *testing.T) {
	c := &ChainConfig{
		ChainID:               big.NewInt(1),
		QRL2PQPrecompilesTime: newUint64(500),
	}
	for _, test := range []struct {
		timestamp uint64
		active    bool
	}{
		{timestamp: 0, active: false},
		{timestamp: 499, active: false},
		{timestamp: 500, active: true},
		{timestamp: math.MaxUint64, active: true},
	} {
		if got := c.Rules(big.NewInt(0), test.timestamp).IsQRL2PQPrecompiles; got != test.active {
			t.Errorf("timestamp %d: activation is %t, want %t", test.timestamp, got, test.active)
		}
	}
}

func TestQRL2PQPrecompilesGenesisJSON(t *testing.T) {
	var config ChainConfig
	if err := json.Unmarshal(
		[]byte(`{"chainId":3151908,"qrl2PQPrecompilesTime":0}`),
		&config,
	); err != nil {
		t.Fatal(err)
	}
	if config.QRL2PQPrecompilesTime == nil || *config.QRL2PQPrecompilesTime != 0 {
		t.Fatalf("activation timestamp is %v, want pointer to zero", config.QRL2PQPrecompilesTime)
	}
	if !config.Rules(big.NewInt(0), 0).IsQRL2PQPrecompiles {
		t.Fatal("genesis timestamp does not activate QRL2 PQ precompiles")
	}
	encoded, err := json.Marshal(&config)
	if err != nil {
		t.Fatal(err)
	}
	var encodedConfig map[string]any
	if err := json.Unmarshal(encoded, &encodedConfig); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(
		map[string]any{"chainId": float64(3151908), "qrl2PQPrecompilesTime": float64(0)},
		encodedConfig,
	) {
		t.Fatalf("unexpected encoded config: %s", encoded)
	}
}

// NOTE(rgeraldes24): not valid at the moment
/*
func TestConfigRules(t *testing.T) {
	c := &ChainConfig{
		ZondTime: newUint64(500),
	}
	var stamp uint64
	if r := c.Rules(big.NewInt(0), true, stamp); r.IsZond {
		t.Errorf("expected %v to not be zond", stamp)
	}
	stamp = 500
	if r := c.Rules(big.NewInt(0), true, stamp); !r.IsZond {
		t.Errorf("expected %v to be zond", stamp)
	}
	stamp = math.MaxInt64
	if r := c.Rules(big.NewInt(0), true, stamp); !r.IsZond {
		t.Errorf("expected %v to be zond", stamp)
	}
}
*/
