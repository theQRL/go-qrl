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
	"github.com/theQRL/go-qrl/core"
	"github.com/theQRL/go-qrl/core/forkid"
	"github.com/theQRL/go-qrl/p2p/qnode"
	"github.com/theQRL/go-qrl/rlp"
)

// qnrEntry is the QNR entry which advertises `qrl` protocol on the discovery.
type qnrEntry struct {
	ForkID forkid.ID // Fork identifier per EIP-2124

	// Ignore additional fields (for forward compatibility).
	Rest []rlp.RawValue `rlp:"tail"`
}

// QNRKey implements qnr.Entry.
func (q qnrEntry) QNRKey() string {
	return "qrl"
}

// StartQNRUpdater starts the `qrl` QNR updater loop, which listens for chain
// head events and updates the requested node record whenever a fork is passed.
func StartQNRUpdater(chain *core.BlockChain, ln *qnode.LocalNode) {
	var newHead = make(chan core.ChainHeadEvent, 10)
	sub := chain.SubscribeChainHeadEvent(newHead)

	go func() {
		defer sub.Unsubscribe()
		for {
			select {
			case <-newHead:
				ln.Set(currentQNREntry(chain))
			case <-sub.Err():
				// Would be nice to sync with Stop, but there is no
				// good way to do that.
				return
			}
		}
	}()
}

// currentQNREntry constructs an `qrl` QNR entry based on the current state of the chain.
func currentQNREntry(chain *core.BlockChain) *qnrEntry {
	head := chain.CurrentHeader()
	return &qnrEntry{
		ForkID: forkid.NewID(chain.Config(), chain.Genesis(), head.Number.Uint64(), head.Time),
	}
}
