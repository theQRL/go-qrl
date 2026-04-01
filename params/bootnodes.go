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

import "github.com/theQRL/go-qrl/common"

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
	"qnode://a236febea3d7795166a80912a61b7d892ec5db37ce64f412cfe1a149b371ff98a797f25b2a121fabab50608e366c4c643eea9c26ee80707347a95e2a8409bab8@35.178.202.23:30303",
	"qnode://c787514869780bc361d1931e602344700366f9c5a741688da4cc5264a4058416260bdcc3c127a3259d099dca7ac3e49942608748c5d4f9a525a5809b3f7326dc@13.41.66.82:30303",
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
