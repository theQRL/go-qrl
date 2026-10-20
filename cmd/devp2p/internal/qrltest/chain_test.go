// Copyright 2020 The go-ethereum Authors
// This file is part of go-ethereum.
//
// go-ethereum is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// go-ethereum is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with go-ethereum. If not, see <http://www.gnu.org/licenses/>.

package qrltest

import (
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/theQRL/go-qrl/core/types"
	"github.com/theQRL/go-qrl/p2p"
	"github.com/theQRL/go-qrl/qrl/protocols/qrl"
)

// TestQRLProtocolNegotiation tests whether the test suite
// can negotiate the highest qrl protocol in a status message exchange
func TestQRLProtocolNegotiation(t *testing.T) {
	t.Parallel()
	var tests = []struct {
		conn     *Conn
		caps     []p2p.Cap
		expected uint32
	}{
		{
			conn: &Conn{
				ourHighestProtoVersion: 65,
			},
			caps: []p2p.Cap{
				{Name: "qrl", Version: 63},
				{Name: "qrl", Version: 64},
				{Name: "qrl", Version: 65},
			},
			expected: uint32(65),
		},
		{
			conn: &Conn{
				ourHighestProtoVersion: 65,
			},
			caps: []p2p.Cap{
				{Name: "qrl", Version: 63},
				{Name: "qrl", Version: 64},
				{Name: "qrl", Version: 65},
			},
			expected: uint32(65),
		},
		{
			conn: &Conn{
				ourHighestProtoVersion: 65,
			},
			caps: []p2p.Cap{
				{Name: "qrl", Version: 63},
				{Name: "qrl", Version: 64},
				{Name: "qrl", Version: 65},
			},
			expected: uint32(65),
		},
		{
			conn: &Conn{
				ourHighestProtoVersion: 64,
			},
			caps: []p2p.Cap{
				{Name: "qrl", Version: 63},
				{Name: "qrl", Version: 64},
				{Name: "qrl", Version: 65},
			},
			expected: 64,
		},
		{
			conn: &Conn{
				ourHighestProtoVersion: 65,
			},
			caps: []p2p.Cap{
				{Name: "qrl", Version: 0},
				{Name: "qrl", Version: 89},
				{Name: "qrl", Version: 65},
			},
			expected: uint32(65),
		},
		{
			conn: &Conn{
				ourHighestProtoVersion: 64,
			},
			caps: []p2p.Cap{
				{Name: "qrl", Version: 63},
				{Name: "qrl", Version: 64},
				{Name: "wrongProto", Version: 65},
			},
			expected: uint32(64),
		},
		{
			conn: &Conn{
				ourHighestProtoVersion: 65,
			},
			caps: []p2p.Cap{
				{Name: "qrl", Version: 63},
				{Name: "qrl", Version: 64},
				{Name: "wrongProto", Version: 65},
			},
			expected: uint32(64),
		},
	}

	for i, tt := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			tt.conn.negotiateQRLProtocol(tt.caps)
			assert.Equal(t, tt.expected, uint32(tt.conn.negotiatedProtoVersion))
		})
	}
}

// TestChainGetHeaders tests whether the test suite can correctly
// respond to a GetBlockHeaders request from a node.
func TestChainGetHeaders(t *testing.T) {
	t.Parallel()

	dir, err := filepath.Abs("./testdata")
	if err != nil {
		t.Fatal(err)
	}
	chain, err := NewChain(dir)
	if err != nil {
		t.Fatal(err)
	}

	var tests = []struct {
		req      qrl.GetBlockHeadersPacket
		expected []*types.Header
	}{
		{
			req: qrl.GetBlockHeadersPacket{
				GetBlockHeadersRequest: &qrl.GetBlockHeadersRequest{
					Origin:  qrl.HashOrNumber{Number: uint64(2)},
					Amount:  uint64(5),
					Skip:    1,
					Reverse: false,
				},
			},
			expected: []*types.Header{
				chain.blocks[2].Header(),
				chain.blocks[4].Header(),
				chain.blocks[6].Header(),
				chain.blocks[8].Header(),
				chain.blocks[10].Header(),
			},
		},
		{
			req: qrl.GetBlockHeadersPacket{
				GetBlockHeadersRequest: &qrl.GetBlockHeadersRequest{
					Origin:  qrl.HashOrNumber{Number: uint64(chain.Len() - 1)},
					Amount:  uint64(3),
					Skip:    0,
					Reverse: true,
				},
			},
			expected: []*types.Header{
				chain.blocks[chain.Len()-1].Header(),
				chain.blocks[chain.Len()-2].Header(),
				chain.blocks[chain.Len()-3].Header(),
			},
		},
		{
			req: qrl.GetBlockHeadersPacket{
				GetBlockHeadersRequest: &qrl.GetBlockHeadersRequest{
					Origin:  qrl.HashOrNumber{Hash: chain.Head().Hash()},
					Amount:  uint64(1),
					Skip:    0,
					Reverse: false,
				},
			},
			expected: []*types.Header{
				chain.Head().Header(),
			},
		},
		{
			req: qrl.GetBlockHeadersPacket{
				GetBlockHeadersRequest: &qrl.GetBlockHeadersRequest{
					Origin:  qrl.HashOrNumber{Number: uint64(10)},
					Amount:  uint64(3),
					Skip:    1,
					Reverse: true,
				},
			},
			expected: []*types.Header{
				chain.blocks[10].Header(),
				chain.blocks[8].Header(),
				chain.blocks[6].Header(),
			},
		},
	}

	for i, tt := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			headers, err := chain.GetHeaders(&tt.req)
			if err != nil {
				t.Fatal(err)
			}
			assert.Equal(t, headers, tt.expected)
		})
	}
}
