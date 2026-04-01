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
	"qnode://2c60682e42ce87a4a6635365f68aa66bf41888360e629d5c56a457a8f37706abe6fd575f08606695b10191b67fd80c5795ce51557496974a89f241b491507a30@35.178.202.23:30303",
	"qnode://b183d5c337088375039249d91f63128b3485689620bd6002c3ceb4a9fb856456b9873cebe2dd63309b091a77e949276a31e41520b33abd06c56bae19f4aa673e@13.41.66.82:30303",
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
