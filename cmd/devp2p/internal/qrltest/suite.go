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
	"crypto/rand"
	"fmt"
	"reflect"

	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/core/types"
	"github.com/theQRL/go-qrl/crypto"
	"github.com/theQRL/go-qrl/internal/utesting"
	"github.com/theQRL/go-qrl/p2p"
	"github.com/theQRL/go-qrl/p2p/qnode"
	"github.com/theQRL/go-qrl/qrl/protocols/qrl"
)

// Suite represents a structure used to test a node's conformance
// to the qrl protocol.
type Suite struct {
	Dest   *qnode.Node
	chain  *Chain
	engine *EngineClient
}

// NewSuite creates and returns a new qrl-test suite that can
// be used to test the given node against the given blockchain
// data.
func NewSuite(dest *qnode.Node, chainDir, engineURL, jwt string) (*Suite, error) {
	chain, err := NewChain(chainDir)
	if err != nil {
		return nil, err
	}
	engine := NewEngineClient(engineURL, jwt, chain)

	return &Suite{
		Dest:   dest,
		chain:  chain,
		engine: engine,
	}, nil
}

func (s *Suite) QRLTests() []utesting.Test {
	return []utesting.Test{
		// status
		{Name: "Status", Fn: s.TestStatus},
		{Name: "MaliciousHandshake", Fn: s.TestMaliciousHandshake},
		// get block headers
		{Name: "GetBlockHeaders", Fn: s.TestGetBlockHeaders},
		{Name: "GetNonexistentBlockHeaders", Fn: s.TestGetNonexistentBlockHeaders},
		{Name: "SimultaneousRequests", Fn: s.TestSimultaneousRequests},
		{Name: "SameRequestID", Fn: s.TestSameRequestID},
		{Name: "ZeroRequestID", Fn: s.TestZeroRequestID},
		// get history
		{Name: "GetBlockBodies", Fn: s.TestGetBlockBodies},
		{Name: "GetReceipts", Fn: s.TestGetReceipts},
		// test transactions
		{Name: "LargeTxRequest", Fn: s.TestLargeTxRequest},
		{Name: "Transaction", Fn: s.TestTransaction},
		{Name: "InvalidTxs", Fn: s.TestInvalidTxs},
		{Name: "NewPooledTxs", Fn: s.TestNewPooledTxs},
	}
}

func (s *Suite) SnapTests() []utesting.Test {
	return []utesting.Test{
		{Name: "Status", Fn: s.TestSnapStatus},
		{Name: "AccountRange", Fn: s.TestSnapGetAccountRange},
		{Name: "GetByteCodes", Fn: s.TestSnapGetByteCodes},
		{Name: "GetTrieNodes", Fn: s.TestSnapTrieNodes},
		{Name: "GetStorageRanges", Fn: s.TestSnapGetStorageRanges},
	}
}

func (s *Suite) TestStatus(t *utesting.T) {
	t.Log(`This test is just a sanity check. It performs an qrl protocol handshake.`)
	conn, err := s.dialAndPeer(nil)
	if err != nil {
		t.Fatal("peering failed:", err)
	}
	conn.Close()
}

// headersMatch returns whether the received headers match the given request
func headersMatch(expected []*types.Header, headers []*types.Header) bool {
	return reflect.DeepEqual(expected, headers)
}

func (s *Suite) TestGetBlockHeaders(t *utesting.T) {
	t.Log(`This test requests block headers from the node.`)
	conn, err := s.dialAndPeer(nil)
	if err != nil {
		t.Fatalf("peering failed: %v", err)
	}
	defer conn.Close()

	// Send headers request.
	req := &qrl.GetBlockHeadersPacket{
		RequestId: 33,
		GetBlockHeadersRequest: &qrl.GetBlockHeadersRequest{
			Origin:  qrl.HashOrNumber{Hash: s.chain.blocks[1].Hash()},
			Amount:  2,
			Skip:    1,
			Reverse: false,
		},
	}
	// Read headers response.
	if err := conn.Write(qrlProto, qrl.GetBlockHeadersMsg, req); err != nil {
		t.Fatalf("could not write to connection: %v", err)
	}
	headers := new(qrl.BlockHeadersPacket)
	if err := conn.ReadMsg(qrlProto, qrl.BlockHeadersMsg, &headers); err != nil {
		t.Fatalf("error reading msg: %v", err)
	}
	if got, want := headers.RequestId, req.RequestId; got != want {
		t.Fatalf("unexpected request id")
	}
	// Check for correct headers.
	expected, err := s.chain.GetHeaders(req)
	if err != nil {
		t.Fatalf("failed to get headers for given request: %v", err)
	}
	if !headersMatch(expected, headers.BlockHeadersRequest) {
		t.Fatalf("header mismatch: \nexpected %v \ngot %v", expected, headers)
	}
}

func (s *Suite) TestGetNonexistentBlockHeaders(t *utesting.T) {
	t.Log(`This test sends GetBlockHeaders requests for nonexistent blocks (using max uint64 value)
to check if the node disconnects after receiving multiple invalid requests.`)
	conn, err := s.dialAndPeer(nil)
	if err != nil {
		t.Fatalf("peering failed: %v", err)
	}
	defer conn.Close()

	// Create request with max uint64 value for a nonexistent block
	badReq := &qrl.GetBlockHeadersPacket{
		GetBlockHeadersRequest: &qrl.GetBlockHeadersRequest{
			Origin:  qrl.HashOrNumber{Number: ^uint64(0)},
			Amount:  1,
			Skip:    0,
			Reverse: false,
		},
	}

	// Send request 10 times. Some clients are lient on the first few invalids.
	for i := 0; i < 10; i++ {
		badReq.RequestId = uint64(i)
		if err := conn.Write(qrlProto, qrl.GetBlockHeadersMsg, badReq); err != nil {
			if err == errDisc {
				t.Fatalf("peer disconnected after %d requests", i+1)
			}
			t.Fatalf("write failed: %v", err)
		}
	}

	// Check if peer disconnects at the end.
	code, _, err := conn.Read()
	if err == errDisc || code == discMsg {
		t.Fatal("peer improperly disconnected")
	}
}

func (s *Suite) TestSimultaneousRequests(t *utesting.T) {
	t.Log(`This test requests blocks headers from the node, performing two requests
concurrently, with different request IDs.`)

	conn, err := s.dialAndPeer(nil)
	if err != nil {
		t.Fatalf("peering failed: %v", err)
	}
	defer conn.Close()

	// Create two different requests.
	req1 := &qrl.GetBlockHeadersPacket{
		RequestId: uint64(111),
		GetBlockHeadersRequest: &qrl.GetBlockHeadersRequest{
			Origin: qrl.HashOrNumber{
				Hash: s.chain.blocks[1].Hash(),
			},
			Amount:  2,
			Skip:    1,
			Reverse: false,
		},
	}
	req2 := &qrl.GetBlockHeadersPacket{
		RequestId: uint64(222),
		GetBlockHeadersRequest: &qrl.GetBlockHeadersRequest{
			Origin: qrl.HashOrNumber{
				Hash: s.chain.blocks[1].Hash(),
			},
			Amount:  4,
			Skip:    1,
			Reverse: false,
		},
	}

	// Send both requests.
	if err := conn.Write(qrlProto, qrl.GetBlockHeadersMsg, req1); err != nil {
		t.Fatalf("failed to write to connection: %v", err)
	}
	if err := conn.Write(qrlProto, qrl.GetBlockHeadersMsg, req2); err != nil {
		t.Fatalf("failed to write to connection: %v", err)
	}

	// Wait for responses.
	// Note they can arrive in either order.
	resp, err := collectHeaderResponses(conn, 2, func(msg *qrl.BlockHeadersPacket) uint64 {
		if msg.RequestId != 111 && msg.RequestId != 222 {
			t.Fatalf("response with unknown request ID: %v", msg.RequestId)
		}
		return msg.RequestId
	})
	if err != nil {
		t.Fatal(err)
	}

	// Check if headers match.
	if err := s.checkHeadersAgainstChain(req1, resp[111]); err != nil {
		t.Fatal(err)
	}
	if err := s.checkHeadersAgainstChain(req2, resp[222]); err != nil {
		t.Fatal(err)
	}
}

func (s *Suite) TestSameRequestID(t *utesting.T) {
	t.Log(`This test requests block headers, performing two concurrent requests with the
same request ID. The node should handle the request by responding to both requests.`)

	conn, err := s.dialAndPeer(nil)
	if err != nil {
		t.Fatalf("peering failed: %v", err)
	}
	defer conn.Close()

	// Create two different requests with the same ID.
	reqID := uint64(1234)
	request1 := &qrl.GetBlockHeadersPacket{
		RequestId: reqID,
		GetBlockHeadersRequest: &qrl.GetBlockHeadersRequest{
			Origin: qrl.HashOrNumber{
				Number: 1,
			},
			Amount: 2,
		},
	}
	request2 := &qrl.GetBlockHeadersPacket{
		RequestId: reqID,
		GetBlockHeadersRequest: &qrl.GetBlockHeadersRequest{
			Origin: qrl.HashOrNumber{
				Number: 33,
			},
			Amount: 3,
		},
	}

	// Send the requests.
	if err = conn.Write(qrlProto, qrl.GetBlockHeadersMsg, request1); err != nil {
		t.Fatalf("failed to write to connection: %v", err)
	}
	if err = conn.Write(qrlProto, qrl.GetBlockHeadersMsg, request2); err != nil {
		t.Fatalf("failed to write to connection: %v", err)
	}

	// Wait for the responses. They can arrive in either order, and we can't tell them
	// apart by their request ID, so use the number of headers instead.
	resp, err := collectHeaderResponses(conn, 2, func(msg *qrl.BlockHeadersPacket) uint64 {
		id := uint64(len(msg.BlockHeadersRequest))
		if id != 2 && id != 3 {
			t.Fatalf("invalid number of headers in response: %d", id)
		}
		return id
	})
	if err != nil {
		t.Fatal(err)
	}

	// Check if headers match.
	if err := s.checkHeadersAgainstChain(request1, resp[2]); err != nil {
		t.Fatal(err)
	}
	if err := s.checkHeadersAgainstChain(request2, resp[3]); err != nil {
		t.Fatal(err)
	}
}

func (s *Suite) checkHeadersAgainstChain(req *qrl.GetBlockHeadersPacket, resp *qrl.BlockHeadersPacket) error {
	if expected, err := s.chain.GetHeaders(req); err != nil {
		return fmt.Errorf("test chain failed to get expected headers for request: %v", err)
	} else if !headersMatch(expected, resp.BlockHeadersRequest) {
		return fmt.Errorf("header mismatch for request ID %v (%d items): \nexpected %v \ngot %v", resp.RequestId, len(resp.BlockHeadersRequest), expected, resp)
	}
	return nil
}

// collectResponses waits for n messages of type T on the given connection.
// The messages are collected according to the 'identity' function.
//
// This function is written in a generic way to handle
func collectHeaderResponses(conn *Conn, n int, identity func(*qrl.BlockHeadersPacket) uint64) (map[uint64]*qrl.BlockHeadersPacket, error) {
	resp := make(map[uint64]*qrl.BlockHeadersPacket, n)
	for range n {
		r := new(qrl.BlockHeadersPacket)
		if err := conn.ReadMsg(qrlProto, qrl.BlockHeadersMsg, r); err != nil {
			return resp, fmt.Errorf("read error: %v", err)
		}
		id := identity(r)
		if resp[id] != nil {
			return resp, fmt.Errorf("duplicate response %v", r)
		}
		resp[id] = r
	}
	return resp, nil
}

func (s *Suite) TestZeroRequestID(t *utesting.T) {
	t.Log(`This test sends a GetBlockHeaders message with a request-id of zero,
and expects a response.`)
	conn, err := s.dialAndPeer(nil)
	if err != nil {
		t.Fatalf("peering failed: %v", err)
	}
	defer conn.Close()

	req := &qrl.GetBlockHeadersPacket{
		GetBlockHeadersRequest: &qrl.GetBlockHeadersRequest{
			Origin: qrl.HashOrNumber{Number: 0},
			Amount: 2,
		},
	}
	// Read headers response.
	if err := conn.Write(qrlProto, qrl.GetBlockHeadersMsg, req); err != nil {
		t.Fatalf("could not write to connection: %v", err)
	}
	headers := new(qrl.BlockHeadersPacket)
	if err := conn.ReadMsg(qrlProto, qrl.BlockHeadersMsg, &headers); err != nil {
		t.Fatalf("error reading msg: %v", err)
	}
	if got, want := headers.RequestId, req.RequestId; got != want {
		t.Fatalf("unexpected request id")
	}
	if err := s.checkHeadersAgainstChain(req, headers); err != nil {
		t.Fatal(err)
	}
}

func (s *Suite) TestGetBlockBodies(t *utesting.T) {
	t.Log(`This test sends GetBlockBodies requests to the node for known blocks in the test chain.`)
	conn, err := s.dialAndPeer(nil)
	if err != nil {
		t.Fatalf("peering failed: %v", err)
	}
	defer conn.Close()

	// Create block bodies request.
	req := &qrl.GetBlockBodiesPacket{
		RequestId: 55,
		GetBlockBodiesRequest: qrl.GetBlockBodiesRequest{
			s.chain.blocks[54].Hash(),
			s.chain.blocks[75].Hash(),
		},
	}
	if err := conn.Write(qrlProto, qrl.GetBlockBodiesMsg, req); err != nil {
		t.Fatalf("could not write to connection: %v", err)
	}
	// Wait for response.
	resp := new(qrl.BlockBodiesPacket)
	if err := conn.ReadMsg(qrlProto, qrl.BlockBodiesMsg, &resp); err != nil {
		t.Fatalf("error reading block bodies msg: %v", err)
	}
	if got, want := resp.RequestId, req.RequestId; got != want {
		t.Fatalf("unexpected request id in response: got %d, want %d", got, want)
	}
	if len(resp.BlockBodiesResponse) != len(req.GetBlockBodiesRequest) {
		t.Fatalf("wrong bodies in response: expected %d bodies, got %d", len(req.GetBlockBodiesRequest), len(resp.BlockBodiesResponse))
	}
}

func (s *Suite) TestGetReceipts(t *utesting.T) {
	t.Log(`This test sends GetReceipts requests to the node for known blocks in the test chain.`)
	conn, err := s.dialAndPeer(nil)
	if err != nil {
		t.Fatalf("peering failed: %v", err)
	}
	defer conn.Close()

	// Find some blocks containing receipts.
	var hashes = make([]common.Hash, 0, 3)
	for i := range s.chain.Len() {
		block := s.chain.GetBlock(i)
		if len(block.Transactions()) > 0 {
			hashes = append(hashes, block.Hash())
		}
		if len(hashes) == cap(hashes) {
			break
		}
	}
	// Create block bodies request.
	req := &qrl.GetReceiptsPacket{
		RequestId:          66,
		GetReceiptsRequest: (qrl.GetReceiptsRequest)(hashes),
	}
	if err := conn.Write(qrlProto, qrl.GetReceiptsMsg, req); err != nil {
		t.Fatalf("could not write to connection: %v", err)
	}
	// Wait for response.
	resp := new(qrl.ReceiptsPacket)
	if err := conn.ReadMsg(qrlProto, qrl.ReceiptsMsg, &resp); err != nil {
		t.Fatalf("error reading block receipts msg: %v", err)
	}
	if got, want := resp.RequestId, req.RequestId; got != want {
		t.Fatalf("unexpected request id in response: got %d, want %d", got, want)
	}
	if len(resp.ReceiptsResponse) != len(req.GetReceiptsRequest) {
		t.Fatalf("wrong receipts in response: expected %d receipts, got %d", len(req.GetReceiptsRequest), len(resp.ReceiptsResponse))
	}
}

func randBuf(size int) []byte {
	buf := make([]byte, size*1024)
	rand.Read(buf)
	return buf
}

func (s *Suite) TestMaliciousHandshake(t *utesting.T) {
	t.Log(`This test tries to send malicious data during the devp2p handshake, in various ways.`)

	// Write hello to client.
	var (
		key, _  = crypto.GenerateKey()
		pub0    = crypto.FromECDSAPub(&key.PublicKey)[1:]
		version = qrl.ProtocolVersions[0]
	)
	handshakes := []*protoHandshake{
		{
			Version: 5,
			Caps: []p2p.Cap{
				{Name: string(randBuf(2)), Version: version},
			},
			ID: pub0,
		},
		{
			Version: 5,
			Caps: []p2p.Cap{
				{Name: "qrl", Version: version},
			},
			ID: append(pub0, byte(0)),
		},
		{
			Version: 5,
			Caps: []p2p.Cap{
				{Name: "qrl", Version: version},
			},
			ID: append(pub0, pub0...),
		},
		{
			Version: 5,
			Caps: []p2p.Cap{
				{Name: "qrl", Version: version},
			},
			ID: randBuf(2),
		},
		{
			Version: 5,
			Caps: []p2p.Cap{
				{Name: string(randBuf(2)), Version: version},
			},
			ID: randBuf(2),
		},
	}
	for _, handshake := range handshakes {
		conn, err := s.dialAs(key)
		if err != nil {
			t.Fatalf("dial failed: %v", err)
		}
		defer conn.Close()

		if err := conn.Write(baseProto, handshakeMsg, handshake); err != nil {
			t.Fatalf("could not write to connection: %v", err)
		}
		// Check that the peer disconnected
		for i := 0; i < 2; i++ {
			code, _, err := conn.Read()
			if err != nil {
				// Client may have disconnected without sending disconnect msg.
				continue
			}
			switch code {
			case discMsg:
			case handshakeMsg:
				// Discard one hello as Hello's are sent concurrently
				continue
			default:
				t.Fatalf("unexpected msg: code %d", code)
			}
		}
	}
}

func (s *Suite) TestTransaction(t *utesting.T) {
	t.Log(`This test sends a valid transaction to the node and checks if the
transaction gets propagated.`)

	// Nudge client out of syncing mode to accept pending txs.
	if err := s.engine.sendForkchoiceUpdated(); err != nil {
		t.Fatalf("failed to send next block: %v", err)
	}
	from, nonce := s.chain.GetSender(0)
	inner := &types.DynamicFeeTx{
		ChainID:   s.chain.config.ChainID,
		Nonce:     nonce,
		GasTipCap: common.Big1,
		GasFeeCap: s.chain.Head().BaseFee(),
		Gas:       30000,
		To:        &common.Address{0xaa},
		Value:     common.Big1,
	}
	tx, err := s.chain.SignTx(from, types.NewTx(inner))
	if err != nil {
		t.Fatalf("failed to sign tx: %v", err)
	}
	if err := s.sendTxs(t, []*types.Transaction{tx}); err != nil {
		t.Fatal(err)
	}
	s.chain.IncNonce(from, 1)
}

func (s *Suite) TestInvalidTxs(t *utesting.T) {
	t.Log(`This test sends several kinds of invalid transactions and checks that the node
does not propagate them.`)

	// Nudge client out of syncing mode to accept pending txs.
	if err := s.engine.sendForkchoiceUpdated(); err != nil {
		t.Fatalf("failed to send next block: %v", err)
	}

	from, nonce := s.chain.GetSender(0)
	inner := &types.DynamicFeeTx{
		ChainID:   s.chain.config.ChainID,
		Nonce:     nonce,
		GasTipCap: common.Big1,
		GasFeeCap: s.chain.Head().BaseFee(),
		Gas:       30000,
		To:        &common.Address{0xaa},
	}
	tx, err := s.chain.SignTx(from, types.NewTx(inner))
	if err != nil {
		t.Fatalf("failed to sign tx: %v", err)
	}
	if err := s.sendTxs(t, []*types.Transaction{tx}); err != nil {
		t.Fatalf("failed to send txs: %v", err)
	}
	s.chain.IncNonce(from, 1)

	inners := []*types.DynamicFeeTx{
		// Nonce already used
		{
			ChainID:   s.chain.config.ChainID,
			Nonce:     nonce - 1,
			GasTipCap: common.Big1,
			GasFeeCap: s.chain.Head().BaseFee(),
			Gas:       100000,
		},
		// Value exceeds balance
		{
			Nonce:     nonce,
			GasTipCap: common.Big1,
			GasFeeCap: s.chain.Head().BaseFee(),
			Gas:       100000,
			Value:     s.chain.Balance(from),
		},
		// Gas limit too low
		{
			Nonce:     nonce,
			GasTipCap: common.Big1,
			GasFeeCap: s.chain.Head().BaseFee(),
			Gas:       1337,
		},
		// Code size too large
		{
			Nonce:     nonce,
			GasTipCap: common.Big1,
			GasFeeCap: s.chain.Head().BaseFee(),
			Data:      randBuf(50),
			Gas:       1_000_000,
		},
		// Data too large
		{
			Nonce:     nonce,
			GasTipCap: common.Big1,
			GasFeeCap: s.chain.Head().BaseFee(),
			To:        &common.Address{0xaa},
			Data:      randBuf(128),
			Gas:       5_000_000,
		},
	}

	var txs []*types.Transaction
	for _, inner := range inners {
		tx, err := s.chain.SignTx(from, types.NewTx(inner))
		if err != nil {
			t.Fatalf("failed to sign tx: %v", err)
		}
		txs = append(txs, tx)
	}
	if err := s.sendInvalidTxs(t, txs); err != nil {
		t.Fatalf("failed to send invalid txs: %v", err)
	}
}

func (s *Suite) TestLargeTxRequest(t *utesting.T) {
	t.Log(`This test first send ~200 transactions to the node, then requests them
on another peer connection using GetPooledTransactions.`)

	// Nudge client out of syncing mode to accept pending txs.
	if err := s.engine.sendForkchoiceUpdated(); err != nil {
		t.Fatalf("failed to send next block: %v", err)
	}

	// Generate many transactions to seed target with.
	var (
		from, nonce = s.chain.GetSender(1)
		// A transaction is about 7.5KB because of its signature.
		count  = 200
		txs    []*types.Transaction
		hashes []common.Hash
		set    = make(map[common.Hash]struct{})
	)
	for i := 0; i < count; i++ {
		inner := &types.DynamicFeeTx{
			ChainID:   s.chain.config.ChainID,
			Nonce:     nonce + uint64(i),
			GasTipCap: common.Big1,
			GasFeeCap: s.chain.Head().BaseFee(),
			Gas:       75000,
		}
		tx, err := s.chain.SignTx(from, types.NewTx(inner))
		if err != nil {
			t.Fatalf("failed to sign tx: err")
		}
		txs = append(txs, tx)
		set[tx.Hash()] = struct{}{}
		hashes = append(hashes, tx.Hash())
	}
	s.chain.IncNonce(from, uint64(count))

	// Send txs.
	if err := s.sendTxs(t, txs); err != nil {
		t.Fatalf("failed to send txs: %v", err)
	}

	// Set up receive connection to ensure node is peered with the receiving
	// connection before tx request is sent.
	conn, err := s.dial()
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close()
	if err = conn.peer(s.chain, nil); err != nil {
		t.Fatalf("peering failed: %v", err)
	}
	// Create and send pooled tx request.
	req := &qrl.GetPooledTransactionsPacket{
		RequestId:                    1234,
		GetPooledTransactionsRequest: hashes,
	}
	if err = conn.Write(qrlProto, qrl.GetPooledTransactionsMsg, req); err != nil {
		t.Fatalf("could not write to conn: %v", err)
	}
	// Check that all received transactions match those that were sent to node.
	msg := new(qrl.PooledTransactionsPacket)
	if err := conn.ReadMsg(qrlProto, qrl.PooledTransactionsMsg, &msg); err != nil {
		t.Fatalf("error reading from connection: %v", err)
	}
	if got, want := msg.RequestId, req.RequestId; got != want {
		t.Fatalf("unexpected request id in response: got %d, want %d", got, want)
	}
	for _, got := range msg.PooledTransactionsResponse {
		if _, exists := set[got.Hash()]; !exists {
			t.Fatalf("unexpected tx received: %v", got.Hash())
		}
	}
}

func (s *Suite) TestNewPooledTxs(t *utesting.T) {
	t.Log(`This test announces transaction hashes to the node and expects it to fetch
the transactions using a GetPooledTransactions request.`)

	// Nudge client out of syncing mode to accept pending txs.
	if err := s.engine.sendForkchoiceUpdated(); err != nil {
		t.Fatalf("failed to send next block: %v", err)
	}

	var (
		// The node requests at most 32 transactions at a time (maxTxRetrievals).
		count       = 32
		from, nonce = s.chain.GetSender(1)
		hashes      = make([]common.Hash, count)
		txTypes     = make([]byte, count)
		sizes       = make([]uint32, count)
	)
	for i := 0; i < count; i++ {
		inner := &types.DynamicFeeTx{
			ChainID:   s.chain.config.ChainID,
			Nonce:     nonce + uint64(i),
			GasTipCap: common.Big1,
			GasFeeCap: s.chain.Head().BaseFee(),
			Gas:       75000,
		}
		tx, err := s.chain.SignTx(from, types.NewTx(inner))
		if err != nil {
			t.Fatalf("failed to sign tx: err")
		}
		hashes[i] = tx.Hash()
		txTypes[i] = tx.Type()
		sizes[i] = uint32(tx.Size())
	}
	s.chain.IncNonce(from, uint64(count))

	// Connect to peer.
	conn, err := s.dial()
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close()
	if err = conn.peer(s.chain, nil); err != nil {
		t.Fatalf("peering failed: %v", err)
	}

	// Send announcement.
	ann := qrl.NewPooledTransactionHashesPacket{Types: txTypes, Sizes: sizes, Hashes: hashes}
	err = conn.Write(qrlProto, qrl.NewPooledTransactionHashesMsg, ann)
	if err != nil {
		t.Fatalf("failed to write to connection: %v", err)
	}

	// Wait for GetPooledTxs request.
	for {
		msg, err := conn.ReadQRL()
		if err != nil {
			t.Fatalf("failed to read qrl msg: %v", err)
		}
		switch msg := msg.(type) {
		case *qrl.GetPooledTransactionsPacket:
			if len(msg.GetPooledTransactionsRequest) != len(hashes) {
				t.Fatalf("unexpected number of txs requested: wanted %d, got %d", len(hashes), len(msg.GetPooledTransactionsRequest))
			}
			return
		case *qrl.NewPooledTransactionHashesPacket:
			continue
		case *qrl.TransactionsPacket:
			continue
		default:
			t.Fatalf("unexpected %s", pretty.Sdump(msg))
		}
	}
}
