// Copyright 2026 The go-ethereum Authors
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

package main

import (
	"net"
	"path/filepath"
	"testing"

	"github.com/theQRL/go-qrl/crypto"
	"github.com/theQRL/go-qrl/p2p/qnode"
)

// TestDiscv4ResolveJSONCommandLineNodes checks that nodes given on the
// resolve-json command line count as crawl input. This matters when the nodes
// file doesn't exist yet and no bootnodes are configured, which is the default
// for QRL mainnet.
func TestDiscv4ResolveJSONCommandLineNodes(t *testing.T) {
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	// Nobody listens on UDP port 1, so the crawler can't resolve the node, but
	// the command must still accept it and write the nodes file.
	n := qnode.NewV4(&key.PublicKey, net.IP{127, 0, 0, 1}, 1, 1)
	file := filepath.Join(t.TempDir(), "nodes.json")

	err = app.Run([]string{"devp2p", "discv4", "resolve-json", "--addr", "127.0.0.1:0", file, n.URLv4()})
	if err != nil {
		t.Fatalf("resolve-json failed: %v", err)
	}
	nodes := loadNodesJSON(file)
	if _, ok := nodes[n.ID()]; !ok {
		t.Fatalf("node %v missing from %s (%d nodes)", n.ID(), file, len(nodes))
	}
}
