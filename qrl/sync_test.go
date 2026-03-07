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

package qrl

import (
	"testing"
	"time"

	"github.com/theQRL/go-qrl/p2p"
	"github.com/theQRL/go-qrl/p2p/qnode"
	"github.com/theQRL/go-qrl/qrl/downloader"
	"github.com/theQRL/go-qrl/qrl/protocols/qrl"
	"github.com/theQRL/go-qrl/qrl/protocols/snap"
)

// Tests that snap sync is disabled after a successful sync cycle.
func TestSnapSyncDisabling1(t *testing.T) { testSnapSyncDisabling(t, qrl.QRL1, snap.SNAP1) }

// Tests that snap sync gets disabled as soon as a real block is successfully
// imported into the blockchain.
func testSnapSyncDisabling(t *testing.T, qrlVer uint, snapVer uint) {
	t.Parallel()

	// Create an empty handler and ensure it's in snap sync mode
	empty := newTestHandler()
	if !empty.handler.snapSync.Load() {
		t.Fatalf("snap sync disabled on pristine blockchain")
	}
	defer empty.close()

	// Create a full handler and ensure snap sync ends up disabled
	full := newTestHandlerWithBlocks(1024)
	if full.handler.snapSync.Load() {
		t.Fatalf("snap sync not disabled on non-empty blockchain")
	}
	defer full.close()

	// Sync up the two handlers via both `qrl` and `snap`
	caps := []p2p.Cap{{Name: "qrl", Version: qrlVer}, {Name: "snap", Version: snapVer}}

	emptyPipeQRL, fullPipeQRL := p2p.MsgPipe()
	defer emptyPipeQRL.Close()
	defer fullPipeQRL.Close()

	emptyPeerQRL := qrl.NewPeer(qrlVer, p2p.NewPeer(qnode.ID{1}, "", caps), emptyPipeQRL, empty.txpool)
	fullPeerQRL := qrl.NewPeer(qrlVer, p2p.NewPeer(qnode.ID{2}, "", caps), fullPipeQRL, full.txpool)
	defer emptyPeerQRL.Close()
	defer fullPeerQRL.Close()

	go empty.handler.runQRLPeer(emptyPeerQRL, func(peer *qrl.Peer) error {
		return qrl.Handle((*qrlHandler)(empty.handler), peer)
	})
	go full.handler.runQRLPeer(fullPeerQRL, func(peer *qrl.Peer) error {
		return qrl.Handle((*qrlHandler)(full.handler), peer)
	})

	emptyPipeSnap, fullPipeSnap := p2p.MsgPipe()
	defer emptyPipeSnap.Close()
	defer fullPipeSnap.Close()

	emptyPeerSnap := snap.NewPeer(snapVer, p2p.NewPeer(qnode.ID{1}, "", caps), emptyPipeSnap)
	fullPeerSnap := snap.NewPeer(snapVer, p2p.NewPeer(qnode.ID{2}, "", caps), fullPipeSnap)

	go empty.handler.runSnapExtension(emptyPeerSnap, func(peer *snap.Peer) error {
		return snap.Handle((*snapHandler)(empty.handler), peer)
	})
	go full.handler.runSnapExtension(fullPeerSnap, func(peer *snap.Peer) error {
		return snap.Handle((*snapHandler)(full.handler), peer)
	})
	// Wait a bit for the above handlers to start
	time.Sleep(250 * time.Millisecond)

	// Check that snap sync was disabled
	if err := empty.handler.downloader.BeaconSync(downloader.SnapSync, full.chain.CurrentBlock(), nil); err != nil {
		t.Fatal("sync failed:", err)
	}
	empty.handler.enableSyncedFeatures()

	if empty.handler.snapSync.Load() {
		t.Fatalf("snap sync not disabled after successful synchronisation")
	}
}
