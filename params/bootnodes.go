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

package params

import "github.com/theQRL/go-zond/common"

// TODO(now.youtrack.cloud/issue/TGZ-14)
// MainnetBootnodes are the qnode URLs of the P2P bootstrap nodes running on
// the main QRL network.
var MainnetBootnodes = []string{
	// QRL Go Bootnodes
}

// BetaNetBootnodes are the qnode URLs of the P2P bootstrap nodes running on the
// BetaNet test network.
var BetaNetBootnodes = []string{}

var TestnetBootnodes = []string{
	"qnode://b28adace666b4b8fa5ca37f7e7a2bd9fdde6b9098b63b48d45d0d3a78e6420a2c1c1d0563ca893455c08ef3f89883d424a3998e1d5df7c9d7b3f068494053a6a@45.32.234.157:30303",
	"qnode://7d0e448ee220cd1de57ea43ce158649a4b895585a90a8f69e79f9d8efc2b252512a560ca6b8edd43b8addfa8bee4a5ea23d9ded6549fd54e5612b0926692895e@45.76.39.66:30303",
}

// TODO(now.youtrack.cloud/issue/TGZ-21)
var V5Bootnodes = []string{
	// QRL bootnodes
}

// TODO(now.youtrack.cloud/issue/TGZ-21)
// const dnsPrefix = "qnrtree://AKA3AM6LPBYEUDMVNU3BSVQJ5AD45Y7YPOHJLEF6W26QOE4VTUDPE@"

// TODO(now.youtrack.cloud/issue/TGZ-21)
// KnownDNSNetwork returns the address of a public DNS-based node list for the given
// genesis hash and protocol. See https://github.com/ethereum/discv4-dns-lists for more
// information.
func KnownDNSNetwork(genesis common.Hash, protocol string) string {
	/*
		var net string
		switch genesis {
		case MainnetGenesisHash:
			net = "mainnet"
		default:
			return ""
		}
		return dnsPrefix + protocol + "." + net + ".ethdisco.net"
	*/
	return ""
}
