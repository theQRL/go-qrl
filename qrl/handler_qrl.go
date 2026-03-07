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

package qrl

import (
	"fmt"

	"github.com/theQRL/go-qrl/core"
	"github.com/theQRL/go-qrl/p2p/qnode"
	"github.com/theQRL/go-qrl/qrl/protocols/qrl"
)

// qrlHandler implements the qrl.Backend interface to handle the various network
// packets that are sent as replies or broadcasts.
type qrlHandler handler

func (h *qrlHandler) Chain() *core.BlockChain { return h.chain }
func (h *qrlHandler) TxPool() qrl.TxPool      { return h.txpool }

// RunPeer is invoked when a peer joins on the `qrl` protocol.
func (h *qrlHandler) RunPeer(peer *qrl.Peer, hand qrl.Handler) error {
	return (*handler)(h).runQRLPeer(peer, hand)
}

// PeerInfo retrieves all known `qrl` information about a peer.
func (h *qrlHandler) PeerInfo(id qnode.ID) any {
	if p := h.peers.peer(id.String()); p != nil {
		return p.info()
	}
	return nil
}

// AcceptTxs retrieves whether transaction processing is enabled on the node
// or if inbound transactions should simply be dropped.
func (h *qrlHandler) AcceptTxs() bool {
	return h.synced.Load()
}

// Handle is invoked from a peer's message handler when it receives a new remote
// message that the handler couldn't consume and serve itself.
func (h *qrlHandler) Handle(peer *qrl.Peer, packet qrl.Packet) error {
	// Consume any broadcasts and announces, forwarding the rest to the downloader
	switch packet := packet.(type) {
	case *qrl.NewPooledTransactionHashesPacket:
		return h.txFetcher.Notify(peer.ID(), packet.Types, packet.Sizes, packet.Hashes)

	case *qrl.TransactionsPacket:
		return h.txFetcher.Enqueue(peer.ID(), *packet, false)

	case *qrl.PooledTransactionsResponse:
		return h.txFetcher.Enqueue(peer.ID(), *packet, true)

	default:
		return fmt.Errorf("unexpected qrl packet type: %T", packet)
	}
}
