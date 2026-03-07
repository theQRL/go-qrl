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
	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/core/txpool"
	"github.com/theQRL/go-qrl/qrl/protocols/qrl"
)

// syncTransactions starts sending all currently pending transactions to the given peer.
func (h *handler) syncTransactions(p *qrl.Peer) {
	var hashes []common.Hash
	for _, batch := range h.txpool.Pending(txpool.PendingFilter{}) {
		for _, tx := range batch {
			hashes = append(hashes, tx.Hash)
		}
	}
	if len(hashes) == 0 {
		return
	}
	p.AsyncSendPooledTransactionHashes(hashes)
}
