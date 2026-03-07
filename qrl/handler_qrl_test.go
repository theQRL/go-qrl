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
	"math/big"
	"testing"
	"time"

	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/consensus/beacon"
	"github.com/theQRL/go-qrl/core"
	"github.com/theQRL/go-qrl/core/forkid"
	"github.com/theQRL/go-qrl/core/rawdb"
	"github.com/theQRL/go-qrl/core/types"
	"github.com/theQRL/go-qrl/core/vm"
	"github.com/theQRL/go-qrl/event"
	"github.com/theQRL/go-qrl/p2p"
	"github.com/theQRL/go-qrl/p2p/qnode"
	"github.com/theQRL/go-qrl/params"
	"github.com/theQRL/go-qrl/qrl/downloader"
	"github.com/theQRL/go-qrl/qrl/protocols/qrl"
)

// testQRLHandler is a mock event handler to listen for inbound network requests
// on the `qrl` protocol and convert them into a more easily testable form.
type testQRLHandler struct {
	txAnnounces  event.Feed
	txBroadcasts event.Feed
}

func (h *testQRLHandler) Chain() *core.BlockChain              { panic("no backing chain") }
func (h *testQRLHandler) TxPool() qrl.TxPool                   { panic("no backing tx pool") }
func (h *testQRLHandler) AcceptTxs() bool                      { return true }
func (h *testQRLHandler) RunPeer(*qrl.Peer, qrl.Handler) error { panic("not used in tests") }
func (h *testQRLHandler) PeerInfo(qnode.ID) any                { panic("not used in tests") }

func (h *testQRLHandler) Handle(peer *qrl.Peer, packet qrl.Packet) error {
	switch packet := packet.(type) {
	case *qrl.NewPooledTransactionHashesPacket:
		h.txAnnounces.Send(packet.Hashes)
		return nil

	case *qrl.TransactionsPacket:
		h.txBroadcasts.Send(([]*types.Transaction)(*packet))
		return nil

	case *qrl.PooledTransactionsResponse:
		h.txBroadcasts.Send(([]*types.Transaction)(*packet))
		return nil

	default:
		panic(fmt.Sprintf("unexpected qrl packet type in tests: %T", packet))
	}
}

// Tests that peers are correctly accepted (or rejected) based on the advertised
// fork IDs in the protocol handshake.
func TestForkIDSplit1(t *testing.T) { testForkIDSplit(t, qrl.QRL1) }

func testForkIDSplit(t *testing.T, protocol uint) {
	t.Parallel()

	var (
		engine = beacon.NewFaker()

		configNoFork  = &params.ChainConfig{}
		configProFork = &params.ChainConfig{}
		dbNoFork      = rawdb.NewMemoryDatabase()
		dbProFork     = rawdb.NewMemoryDatabase()

		gspecNoFork  = &core.Genesis{Config: configNoFork}
		gspecProFork = &core.Genesis{Config: configProFork}

		chainNoFork, _  = core.NewBlockChain(dbNoFork, nil, gspecNoFork, engine, vm.Config{}, nil)
		chainProFork, _ = core.NewBlockChain(dbProFork, nil, gspecProFork, engine, vm.Config{}, nil)

		_, blocksNoFork, _  = core.GenerateChainWithGenesis(gspecNoFork, engine, 2, nil)
		_, blocksProFork, _ = core.GenerateChainWithGenesis(gspecProFork, engine, 2, nil)

		qrlNoFork, _ = newHandler(&handlerConfig{
			Database:   dbNoFork,
			Chain:      chainNoFork,
			TxPool:     newTestTxPool(),
			Network:    1,
			Sync:       downloader.FullSync,
			BloomCache: 1,
		})
		qrlProFork, _ = newHandler(&handlerConfig{
			Database:   dbProFork,
			Chain:      chainProFork,
			TxPool:     newTestTxPool(),
			Network:    1,
			Sync:       downloader.FullSync,
			BloomCache: 1,
		})
	)
	qrlNoFork.Start(1000)
	qrlProFork.Start(1000)

	// Clean up everything after ourselves
	defer chainNoFork.Stop()
	defer chainProFork.Stop()

	defer qrlNoFork.Stop()
	defer qrlProFork.Stop()

	// Both nodes should allow the other to connect (same genesis, next fork is the same)
	p2pNoFork, p2pProFork := p2p.MsgPipe()
	defer p2pNoFork.Close()
	defer p2pProFork.Close()

	peerNoFork := qrl.NewPeer(protocol, p2p.NewPeerPipe(qnode.ID{1}, "", nil, p2pNoFork), p2pNoFork, nil)
	peerProFork := qrl.NewPeer(protocol, p2p.NewPeerPipe(qnode.ID{2}, "", nil, p2pProFork), p2pProFork, nil)
	defer peerNoFork.Close()
	defer peerProFork.Close()

	errc := make(chan error, 2)
	go func(errc chan error) {
		errc <- qrlNoFork.runQRLPeer(peerProFork, func(peer *qrl.Peer) error { return nil })
	}(errc)
	go func(errc chan error) {
		errc <- qrlProFork.runQRLPeer(peerNoFork, func(peer *qrl.Peer) error { return nil })
	}(errc)

	for range 2 {
		select {
		case err := <-errc:
			if err != nil {
				t.Fatalf("frontier nofork <-> profork failed: %v", err)
			}
		case <-time.After(250 * time.Millisecond):
			t.Fatalf("frontier nofork <-> profork handler timeout")
		}
	}
	// Progress into Homestead. Fork's match, so we don't care what the future holds
	chainNoFork.InsertChain(blocksNoFork[:1])
	chainProFork.InsertChain(blocksProFork[:1])

	p2pNoFork, p2pProFork = p2p.MsgPipe()
	defer p2pNoFork.Close()
	defer p2pProFork.Close()

	peerNoFork = qrl.NewPeer(protocol, p2p.NewPeer(qnode.ID{1}, "", nil), p2pNoFork, nil)
	peerProFork = qrl.NewPeer(protocol, p2p.NewPeer(qnode.ID{2}, "", nil), p2pProFork, nil)
	defer peerNoFork.Close()
	defer peerProFork.Close()

	errc = make(chan error, 2)
	go func(errc chan error) {
		errc <- qrlNoFork.runQRLPeer(peerProFork, func(peer *qrl.Peer) error { return nil })
	}(errc)
	go func(errc chan error) {
		errc <- qrlProFork.runQRLPeer(peerNoFork, func(peer *qrl.Peer) error { return nil })
	}(errc)

	for range 2 {
		select {
		case err := <-errc:
			if err != nil {
				t.Fatalf("homestead nofork <-> profork failed: %v", err)
			}
		case <-time.After(250 * time.Millisecond):
			t.Fatalf("homestead nofork <-> profork handler timeout")
		}
	}
	// NOTE(rgeraldes24): revisit upon new fork
	/*
		// Progress into Spurious. Forks mismatch, signalling differing chains, reject
		chainNoFork.InsertChain(blocksNoFork[1:2])
		chainProFork.InsertChain(blocksProFork[1:2])

		p2pNoFork, p2pProFork = p2p.MsgPipe()
		defer p2pNoFork.Close()
		defer p2pProFork.Close()

		peerNoFork = qrl.NewPeer(protocol, p2p.NewPeerPipe(qnode.ID{1}, "", nil, p2pNoFork), p2pNoFork, nil)
		peerProFork = qrl.NewPeer(protocol, p2p.NewPeerPipe(qnode.ID{2}, "", nil, p2pProFork), p2pProFork, nil)
		defer peerNoFork.Close()
		defer peerProFork.Close()

		errc = make(chan error, 2)
		go func(errc chan error) {
			errc <- qrlNoFork.runQRLPeer(peerProFork, func(peer *qrl.Peer) error { return nil })
		}(errc)
		go func(errc chan error) {
			errc <- qrlProFork.runQRLPeer(peerNoFork, func(peer *qrl.Peer) error { return nil })
		}(errc)

		var successes int
		for i := range 2 {
			select {
			case err := <-errc:
				if err == nil {
					successes++
					if successes == 2 { // Only one side disconnects
						t.Fatalf("fork ID rejection didn't happen")
					}
				}
			case <-time.After(250 * time.Millisecond):
				t.Fatalf("split peers not rejected")
			}
		}
	*/
}

// Tests that received transactions are added to the local pool.
func TestRecvTransactions1(t *testing.T) { testRecvTransactions(t, qrl.QRL1) }

func testRecvTransactions(t *testing.T, protocol uint) {
	t.Parallel()

	// Create a message handler, configure it to accept transactions and watch them
	handler := newTestHandler()
	defer handler.close()

	handler.handler.synced.Store(true) // mark synced to accept transactions

	txs := make(chan core.NewTxsEvent)
	sub := handler.txpool.SubscribeTransactions(txs)
	defer sub.Unsubscribe()

	// Create a source peer to send messages through and a sink handler to receive them
	p2pSrc, p2pSink := p2p.MsgPipe()
	defer p2pSrc.Close()
	defer p2pSink.Close()

	src := qrl.NewPeer(protocol, p2p.NewPeerPipe(qnode.ID{1}, "", nil, p2pSrc), p2pSrc, handler.txpool)
	sink := qrl.NewPeer(protocol, p2p.NewPeerPipe(qnode.ID{2}, "", nil, p2pSink), p2pSink, handler.txpool)
	defer src.Close()
	defer sink.Close()

	go handler.handler.runQRLPeer(sink, func(peer *qrl.Peer) error {
		return qrl.Handle((*qrlHandler)(handler.handler), peer)
	})
	// Run the handshake locally to avoid spinning up a source handler
	var (
		genesis = handler.chain.Genesis()
		head    = handler.chain.CurrentBlock()
	)
	if err := src.Handshake(1, head.Hash(), genesis.Hash(), forkid.NewIDWithChain(handler.chain), forkid.NewFilter(handler.chain)); err != nil {
		t.Fatalf("failed to run protocol handshake")
	}
	// Send the transaction to the sink and verify that it's added to the tx pool
	tx := types.NewTx(&types.DynamicFeeTx{
		Nonce:     0,
		To:        &common.Address{},
		Value:     big.NewInt(0),
		Gas:       100000,
		GasFeeCap: big.NewInt(0),
		Data:      nil,
	})
	tx, _ = types.SignTx(tx, types.ShanghaiSigner{ChainId: big.NewInt(0)}, testWallet)

	if err := src.SendTransactions([]*types.Transaction{tx}); err != nil {
		t.Fatalf("failed to send transaction: %v", err)
	}
	select {
	case event := <-txs:
		if len(event.Txs) != 1 {
			t.Errorf("wrong number of added transactions: got %d, want 1", len(event.Txs))
		} else if event.Txs[0].Hash() != tx.Hash() {
			t.Errorf("added wrong tx hash: got %v, want %v", event.Txs[0].Hash(), tx.Hash())
		}
	case <-time.After(2 * time.Second):
		t.Errorf("no NewTxsEvent received within 2 seconds")
	}
}

// This test checks that pending transactions are sent.
func TestSendTransactions1(t *testing.T) { testSendTransactions(t, qrl.QRL1) }

func testSendTransactions(t *testing.T, protocol uint) {
	t.Parallel()

	// Create a message handler and fill the pool with big transactions
	handler := newTestHandler()
	defer handler.close()

	insert := make([]*types.Transaction, 100)
	for nonce := range insert {
		tx := types.NewTx(&types.DynamicFeeTx{
			Nonce:     uint64(nonce),
			To:        &common.Address{},
			Value:     big.NewInt(0),
			Gas:       100000,
			GasFeeCap: big.NewInt(0),
			Data:      make([]byte, 10240),
		})
		tx, _ = types.SignTx(tx, types.ShanghaiSigner{ChainId: big.NewInt(0)}, testWallet)
		insert[nonce] = tx
	}
	go handler.txpool.Add(insert, false, false) // Need goroutine to not block on feed
	time.Sleep(250 * time.Millisecond)          // Wait until tx events get out of the system (can't use events, tx broadcaster races with peer join)

	// Create a source handler to send messages through and a sink peer to receive them
	p2pSrc, p2pSink := p2p.MsgPipe()
	defer p2pSrc.Close()
	defer p2pSink.Close()

	src := qrl.NewPeer(protocol, p2p.NewPeerPipe(qnode.ID{1}, "", nil, p2pSrc), p2pSrc, handler.txpool)
	sink := qrl.NewPeer(protocol, p2p.NewPeerPipe(qnode.ID{2}, "", nil, p2pSink), p2pSink, handler.txpool)
	defer src.Close()
	defer sink.Close()

	go handler.handler.runQRLPeer(src, func(peer *qrl.Peer) error {
		return qrl.Handle((*qrlHandler)(handler.handler), peer)
	})
	// Run the handshake locally to avoid spinning up a source handler
	var (
		genesis = handler.chain.Genesis()
		head    = handler.chain.CurrentBlock()
	)
	if err := sink.Handshake(1, head.Hash(), genesis.Hash(), forkid.NewIDWithChain(handler.chain), forkid.NewFilter(handler.chain)); err != nil {
		t.Fatalf("failed to run protocol handshake")
	}
	// After the handshake completes, the source handler should stream the sink
	// the transactions, subscribe to all inbound network events
	backend := new(testQRLHandler)

	anns := make(chan []common.Hash)
	annSub := backend.txAnnounces.Subscribe(anns)
	defer annSub.Unsubscribe()

	bcasts := make(chan []*types.Transaction)
	bcastSub := backend.txBroadcasts.Subscribe(bcasts)
	defer bcastSub.Unsubscribe()

	go qrl.Handle(backend, sink)

	// Make sure we get all the transactions on the correct channels
	seen := make(map[common.Hash]struct{})
	for len(seen) < len(insert) {
		switch protocol {
		case 1:
			select {
			case hashes := <-anns:
				for _, hash := range hashes {
					if _, ok := seen[hash]; ok {
						t.Errorf("duplicate transaction announced: %x", hash)
					}
					seen[hash] = struct{}{}
				}
			case <-bcasts:
				t.Errorf("initial tx broadcast received on post qrl/1")
			}

		default:
			panic("unsupported protocol, please extend test")
		}
	}
	for _, tx := range insert {
		if _, ok := seen[tx.Hash()]; !ok {
			t.Errorf("missing transaction: %x", tx.Hash())
		}
	}
}

// Tests that transactions get propagated to all attached peers, either via direct
// broadcasts or via announcements/retrievals.
func TestTransactionPropagation1(t *testing.T) { testTransactionPropagation(t, qrl.QRL1) }

func testTransactionPropagation(t *testing.T, protocol uint) {
	t.Parallel()

	// Create a source handler to send transactions from and a number of sinks
	// to receive them. We need multiple sinks since a one-to-one peering would
	// broadcast all transactions without announcement.
	source := newTestHandler()
	source.handler.snapSync.Store(false) // Avoid requiring snap, otherwise some will be dropped below
	defer source.close()

	sinks := make([]*testHandler, 10)
	for i := range sinks {
		sinks[i] = newTestHandler()
		defer sinks[i].close()

		sinks[i].handler.synced.Store(true) // mark synced to accept transactions
	}
	// Interconnect all the sink handlers with the source handler
	for i, sink := range sinks {
		sourcePipe, sinkPipe := p2p.MsgPipe()
		defer sourcePipe.Close()
		defer sinkPipe.Close()

		sourcePeer := qrl.NewPeer(protocol, p2p.NewPeerPipe(qnode.ID{byte(i + 1)}, "", nil, sourcePipe), sourcePipe, source.txpool)
		sinkPeer := qrl.NewPeer(protocol, p2p.NewPeerPipe(qnode.ID{0}, "", nil, sinkPipe), sinkPipe, sink.txpool)
		defer sourcePeer.Close()
		defer sinkPeer.Close()

		go source.handler.runQRLPeer(sourcePeer, func(peer *qrl.Peer) error {
			return qrl.Handle((*qrlHandler)(source.handler), peer)
		})
		go sink.handler.runQRLPeer(sinkPeer, func(peer *qrl.Peer) error {
			return qrl.Handle((*qrlHandler)(sink.handler), peer)
		})
	}
	// Subscribe to all the transaction pools
	txChs := make([]chan core.NewTxsEvent, len(sinks))
	for i := range sinks {
		txChs[i] = make(chan core.NewTxsEvent, 1024)

		sub := sinks[i].txpool.SubscribeTransactions(txChs[i])
		defer sub.Unsubscribe()
	}
	// Fill the source pool with transactions and wait for them at the sinks
	txs := make([]*types.Transaction, 1024)
	for nonce := range txs {
		tx := types.NewTx(&types.DynamicFeeTx{
			Nonce:     uint64(nonce),
			To:        &common.Address{},
			Value:     big.NewInt(0),
			Gas:       100000,
			GasFeeCap: big.NewInt(0),
			Data:      nil,
		})
		tx, _ = types.SignTx(tx, types.ShanghaiSigner{ChainId: big.NewInt(0)}, testWallet)

		txs[nonce] = tx
	}
	source.txpool.Add(txs, false, false)

	// Iterate through all the sinks and ensure they all got the transactions
	for i := range sinks {
		for arrived, timeout := 0, false; arrived < len(txs) && !timeout; {
			select {
			case event := <-txChs[i]:
				arrived += len(event.Txs)
			case <-time.After(2 * time.Second):
				t.Errorf("sink %d: transaction propagation timed out: have %d, want %d", i, arrived, len(txs))
				timeout = true
			}
		}
	}
}
