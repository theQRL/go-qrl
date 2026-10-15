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

package v5test

import (
	"bytes"
	"net"
	"slices"
	"sync"
	"time"

	"github.com/theQRL/go-qrl/internal/utesting"
	"github.com/theQRL/go-qrl/p2p/discover/v5wire"
	"github.com/theQRL/go-qrl/p2p/netutil"
	"github.com/theQRL/go-qrl/p2p/qnode"
	"github.com/theQRL/go-qrl/p2p/qnr"
)

// Suite is the discv5 test suite.
type Suite struct {
	Dest             *qnode.Node
	Listen1, Listen2 string // listening addresses
}

func (s *Suite) listen1(log logger) (*conn, net.PacketConn) {
	c := newConn(s.Dest, log)
	l := c.listen(s.Listen1)
	return c, l
}

func (s *Suite) listen2(log logger) (*conn, net.PacketConn, net.PacketConn) {
	c := newConn(s.Dest, log)
	l1, l2 := c.listen(s.Listen1), c.listen(s.Listen2)
	return c, l1, l2
}

func (s *Suite) AllTests() []utesting.Test {
	return []utesting.Test{
		{Name: "Ping", Fn: s.TestPing},
		{Name: "PingLargeRequestID", Fn: s.TestPingLargeRequestID},
		{Name: "PingMultiIP", Fn: s.TestPingMultiIP},
		{Name: "HandshakeResend", Fn: s.TestHandshakeResend},
		{Name: "TalkRequest", Fn: s.TestTalkRequest},
		{Name: "FindnodeWrongIP", Fn: s.TestFindnodeWrongIP},
		{Name: "FindnodeHandshake", Fn: s.TestFindnodeHandshake},
		{Name: "FindnodeZeroDistance", Fn: s.TestFindnodeZeroDistance},
		{Name: "FindnodeResults", Fn: s.TestFindnodeResults},
		{Name: "UnsolicitedNodes", Fn: s.TestUnsolicitedNodes},
	}
}

func (s *Suite) TestPing(t *utesting.T) {
	t.Log(`This test is just a sanity check. It sends PING and expects a PONG response.`)

	conn, l1 := s.listen1(t)
	defer conn.close()

	ping := &v5wire.Ping{ReqID: conn.nextReqID()}
	switch resp := conn.reqresp(l1, ping).(type) {
	case *v5wire.Pong:
		checkPong(t, resp, ping, l1)
	default:
		t.Fatal("expected PONG, got", resp.Name())
	}
}

func checkPong(t *utesting.T, pong *v5wire.Pong, ping *v5wire.Ping, c net.PacketConn) {
	if !bytes.Equal(pong.ReqID, ping.ReqID) {
		t.Fatalf("wrong request ID %x in PONG, want %x", pong.ReqID, ping.ReqID)
	}
	if !pong.ToIP.Equal(laddr(c).IP) {
		t.Fatalf("wrong destination IP %v in PONG, want %v", pong.ToIP, laddr(c).IP)
	}
	if int(pong.ToPort) != laddr(c).Port {
		t.Fatalf("wrong destination port %v in PONG, want %v", pong.ToPort, laddr(c).Port)
	}
}

func (s *Suite) TestPingLargeRequestID(t *utesting.T) {
	t.Log(`This test sends PING with a 9-byte request ID, which isn't allowed by the spec.
The remote node should not respond.`)

	conn, l1 := s.listen1(t)
	defer conn.close()

	ping := &v5wire.Ping{ReqID: make([]byte, 9)}
	switch resp := conn.reqresp(l1, ping).(type) {
	case *v5wire.Pong:
		t.Errorf("PONG response with unknown request ID %x", resp.ReqID)
	case *readError:
		if resp.err == v5wire.ErrInvalidReqID {
			t.Error("response with oversized request ID")
		} else if !netutil.IsTimeout(resp.err) {
			t.Error(resp)
		}
	}
}

func (s *Suite) TestPingMultiIP(t *utesting.T) {
	t.Log(`This test establishes a session from one IP as usual. The session is then reused
on another IP, which shouldn't work. The remote node should respond with WHOAREYOU for
the attempt from a different IP.`)

	conn, l1, l2 := s.listen2(t)
	defer conn.close()

	// Create the session on l1.
	ping := &v5wire.Ping{ReqID: conn.nextReqID()}
	resp := conn.reqresp(l1, ping)
	if resp.Kind() != v5wire.PongMsg {
		t.Fatal("expected PONG, got", resp)
	}
	checkPong(t, resp.(*v5wire.Pong), ping, l1)

	// Send on l2. This reuses the session because there is only one codec.
	t.Log("sending ping from alternate IP", l2.LocalAddr())
	ping2 := &v5wire.Ping{ReqID: conn.nextReqID()}
	conn.write(l2, ping2, nil)
	switch resp := conn.read(l2).(type) {
	case *v5wire.Pong:
		t.Fatalf("remote responded to PING from %v for session on IP %v", laddr(l2).IP, laddr(l1).IP)
	case *v5wire.Whoareyou:
		t.Logf("got WHOAREYOU for new session as expected")
		resp.Node = s.Dest
		conn.write(l2, ping2, resp)
	default:
		t.Fatal("expected WHOAREYOU, got", resp)
	}

	// Catch the PONG on l2.
	switch resp := conn.read(l2).(type) {
	case *v5wire.Pong:
		checkPong(t, resp, ping2, l2)
	default:
		t.Fatal("expected PONG, got", resp)
	}

	// Try on l1 again.
	ping3 := &v5wire.Ping{ReqID: conn.nextReqID()}
	conn.write(l1, ping3, nil)
	switch resp := conn.read(l1).(type) {
	case *v5wire.Pong:
		t.Fatalf("remote responded to PING from %v for session on IP %v", laddr(l1).IP, laddr(l2).IP)
	case *v5wire.Whoareyou:
		t.Logf("got WHOAREYOU for new session as expected")
	default:
		t.Fatal("expected WHOAREYOU, got", resp)
	}
}

// TestHandshakeResend starts a handshake, but doesn't finish it and sends a second ordinary message
// packet instead of a handshake message packet. The remote node should repeat the previous WHOAREYOU
// challenge for the first PING.
func (s *Suite) TestHandshakeResend(t *utesting.T) {
	conn, l1 := s.listen1(t)
	defer conn.close()

	// First PING triggers challenge.
	ping := &v5wire.Ping{ReqID: conn.nextReqID()}
	conn.write(l1, ping, nil)
	var challenge1 *v5wire.Whoareyou
	switch resp := conn.read(l1).(type) {
	case *v5wire.Whoareyou:
		challenge1 = resp
		t.Logf("got WHOAREYOU for PING")
	default:
		t.Fatal("expected WHOAREYOU, got", resp)
	}

	// Send second PING.
	ping2 := &v5wire.Ping{ReqID: conn.nextReqID()}
	conn.write(l1, ping2, nil)
	switch resp := conn.read(l1).(type) {
	case *v5wire.Whoareyou:
		if resp.Nonce != challenge1.Nonce {
			t.Fatalf("wrong nonce %x in WHOAREYOU (want %x)", resp.Nonce[:], challenge1.Nonce[:])
		}
		if !bytes.Equal(resp.ChallengeData, challenge1.ChallengeData) {
			t.Fatalf("wrong ChallengeData in resent WHOAREYOU: got %x, want %x", resp.ChallengeData, challenge1.ChallengeData)
		}
		resp.Node = conn.remote
	default:
		t.Fatal("expected WHOAREYOU, got", resp)
	}
}

func (s *Suite) TestTalkRequest(t *utesting.T) {
	t.Log(`This test sends some examples of TALKREQ with a protocol-id of "test-protocol"
and expects an empty TALKRESP response.`)

	conn, l1 := s.listen1(t)
	defer conn.close()

	// Non-empty request ID.
	id := conn.nextReqID()
	resp := conn.reqresp(l1, &v5wire.TalkRequest{ReqID: id, Protocol: "test-protocol"})
	switch resp := resp.(type) {
	case *v5wire.TalkResponse:
		if !bytes.Equal(resp.ReqID, id) {
			t.Fatalf("wrong request ID %x in TALKRESP, want %x", resp.ReqID, id)
		}
		if len(resp.Message) > 0 {
			t.Fatalf("non-empty message %x in TALKRESP", resp.Message)
		}
	default:
		t.Fatal("expected TALKRESP, got", resp.Name())
	}

	// Empty request ID.
	t.Log("sending TALKREQ with empty request-id")
	resp = conn.reqresp(l1, &v5wire.TalkRequest{Protocol: "test-protocol"})
	switch resp := resp.(type) {
	case *v5wire.TalkResponse:
		if len(resp.ReqID) > 0 {
			t.Fatalf("wrong request ID %x in TALKRESP, want empty byte array", resp.ReqID)
		}
		if len(resp.Message) > 0 {
			t.Fatalf("non-empty message %x in TALKRESP", resp.Message)
		}
	default:
		t.Fatal("expected TALKRESP, got", resp.Name())
	}
}

func (s *Suite) TestFindnodeWrongIP(t *utesting.T) {
	t.Log(`This test establishes a session on one IP, then sends FINDNODE from another IP.
The remote node should challenge the second endpoint with WHOAREYOU instead of returning NODES.`)

	conn, l1, l2 := s.listen2(t)
	defer conn.close()

	ping := &v5wire.Ping{ReqID: conn.nextReqID()}
	if resp := conn.reqresp(l1, ping); resp.Kind() != v5wire.PongMsg {
		t.Fatal("expected PONG, got", resp)
	}

	req := &v5wire.Findnode{ReqID: conn.nextReqID(), Distances: []uint{0}}
	conn.write(l2, req, nil)
	switch resp := conn.read(l2).(type) {
	case *v5wire.Whoareyou:
		t.Log("got WHOAREYOU for FINDNODE on wrong IP as expected")
	case *v5wire.Nodes:
		t.Fatalf("unexpected NODES response on wrong IP: %+v", resp)
	default:
		t.Fatal("expected WHOAREYOU, got", resp)
	}
}

func (s *Suite) TestFindnodeHandshake(t *utesting.T) {
	t.Log(`This test checks that the remote answers a FINDNODE request only after completing the WHOAREYOU handshake.`)

	conn, l1 := s.listen1(t)
	defer conn.close()

	req := &v5wire.Findnode{ReqID: conn.nextReqID(), Distances: []uint{0}}
	nonce := conn.write(l1, req, nil)

	resp, from := conn.readFrom(l1)
	challenge, ok := resp.(*v5wire.Whoareyou)
	if !ok {
		t.Fatalf("expected WHOAREYOU before NODES, got %T (%v) from %v", resp, resp, from)
	}
	if challenge.Nonce != nonce {
		t.Fatalf("wrong nonce %x in WHOAREYOU (want %x)", challenge.Nonce[:], nonce[:])
	}

	challenge.Node = conn.remote
	conn.writeTo(l1, req, challenge, from)

	for {
		resp, from := conn.readFrom(l1)
		switch resp := resp.(type) {
		case *v5wire.Ping:
			conn.writeTo(l1, &v5wire.Pong{
				ReqID:  resp.ReqID,
				QNRSeq: conn.localNode.Seq(),
				ToIP:   from.IP,
				ToPort: uint16(from.Port),
			}, nil, from)
		case *v5wire.Nodes:
			if !bytes.Equal(resp.ReqID, req.ReqID) {
				t.Fatalf("wrong request ID %x in NODES, want %x", resp.ReqID, req.ReqID)
			}
			return
		default:
			t.Fatalf("expected NODES after completing handshake, got %T (%v) from %v", resp, resp, from)
		}
	}
}

func (s *Suite) TestFindnodeZeroDistance(t *utesting.T) {
	t.Log(`This test checks that the remote node returns itself for FINDNODE with distance zero.`)

	conn, l1 := s.listen1(t)
	defer conn.close()

	nodes, err := conn.findnode(l1, []uint{0})
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 {
		t.Fatalf("remote returned more than one node for FINDNODE [0]")
	}
	if nodes[0].ID() != conn.remote.ID() {
		t.Errorf("ID of response node is %v, want %v", nodes[0].ID(), conn.remote.ID())
	}
}

func (s *Suite) TestUnsolicitedNodes(t *utesting.T) {
	t.Log(`This test sends an unsolicited NODES response advertising a fake node.
The remote node should neither contact the injected node nor return it from later FINDNODE queries.`)

	injectConn, injectL := s.listen1(t)
	defer injectConn.close()

	// Establish a session for the injection path so the unsolicited packet is
	// well-formed and authenticated.
	ping := &v5wire.Ping{ReqID: injectConn.nextReqID()}
	if resp := injectConn.reqresp(injectL, ping); resp.Kind() != v5wire.PongMsg {
		t.Fatal("expected PONG, got", resp)
	}

	fakeConn, fakeL := s.listen1(t)
	defer fakeConn.close()
	fakeConn.setEndpoint(fakeL)

	unsolicited := &v5wire.Nodes{
		ReqID:     injectConn.nextReqID(),
		RespCount: 1,
		Nodes:     []*qnr.Record{fakeConn.localNode.Node().Record()},
	}
	t.Log("sending unsolicited NODES response with injected node")
	injectConn.write(injectL, unsolicited, nil)

	checkNoContact := func(phase string) {
		buf := make([]byte, 1280)
		if err := fakeL.SetReadDeadline(time.Now()); err != nil {
			t.Fatal(err)
		}
		for {
			n, from, err := fakeL.ReadFrom(buf)
			if err == nil {
				t.Fatalf("%s: remote contacted injected node: %d bytes from %v", phase, n, from)
			}
			if netutil.IsTimeout(err) {
				return
			}
			t.Fatalf("%s: checking for unexpected contact failed: %v", phase, err)
		}
	}
	checkNoContact("after unsolicited NODES")

	probeConn, probeL := s.listen1(t)
	defer probeConn.close()

	dist := uint(qnode.LogDist(fakeConn.localNode.ID(), s.Dest.ID()))
	const maxAttempts = 3
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		results, err := probeConn.findnode(probeL, []uint{dist})
		if err != nil {
			t.Fatal(err)
		}
		checkNoContact("during FINDNODE probes")
		for _, n := range results {
			if n.ID() == fakeConn.localNode.ID() {
				t.Fatalf("attempt %d: FINDNODE result contains node from unsolicited NODES response", attempt)
			}
		}
		t.Logf("attempt %d: injected node not returned in FINDNODE results", attempt)
	}
}

func (s *Suite) TestFindnodeResults(t *utesting.T) {
	t.Log(`This test pings the node under test from multiple other endpoints and node identities
(the 'bystanders'). After waiting for them to be accepted into the remote table, the test checks
that they are returned by FINDNODE.`)

	// Create bystanders.
	nodes := make([]*bystander, 5)
	added := make(chan qnode.ID, len(nodes))
	for i := range nodes {
		nodes[i] = newBystander(t, s, added)
		defer nodes[i].close()
	}

	// Get them added to the remote table.
	timeout := 60 * time.Second
	timeoutCh := time.After(timeout)
	for count := 0; count < len(nodes); {
		select {
		case id := <-added:
			t.Logf("bystander node %v added to remote table", id)
			count++
		case <-timeoutCh:
			t.Errorf("remote added %d bystander nodes in %v, need %d to continue", count, timeout, len(nodes))
			t.Logf("this can happen if the node has a non-empty table from previous runs")
			return
		}
	}
	t.Logf("all %d bystander nodes were added", len(nodes))

	// Collect our nodes by distance.
	var dists []uint
	expect := make(map[qnode.ID]*qnode.Node)
	for _, bn := range nodes {
		n := bn.conn.localNode.Node()
		expect[n.ID()] = n
		d := uint(qnode.LogDist(n.ID(), s.Dest.ID()))
		if !slices.Contains(dists, d) {
			dists = append(dists, d)
		}
	}

	// Send FINDNODE for all distances.
	t.Log("requesting nodes")
	conn, l1 := s.listen1(t)
	defer conn.close()
	foundNodes, err := conn.findnode(l1, dists)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("remote returned %d nodes for distance list %v", len(foundNodes), dists)
	for _, n := range foundNodes {
		delete(expect, n.ID())
	}
	if len(expect) > 0 {
		t.Errorf("missing %d nodes in FINDNODE result", len(expect))
		t.Logf("this can happen if the test is run multiple times in quick succession")
		t.Logf("and the remote node hasn't removed dead nodes from previous runs yet")
	} else {
		t.Logf("all %d expected nodes were returned", len(nodes))
	}
}

// A bystander is a node whose only purpose is filling a spot in the remote table.
type bystander struct {
	dest *qnode.Node
	conn *conn
	l    net.PacketConn

	addedCh chan qnode.ID
	done    sync.WaitGroup
}

func newBystander(t *utesting.T, s *Suite, added chan qnode.ID) *bystander {
	conn, l := s.listen1(t)
	conn.setEndpoint(l) // bystander nodes need IP/port to get pinged
	bn := &bystander{
		conn:    conn,
		l:       l,
		dest:    s.Dest,
		addedCh: added,
	}
	bn.done.Add(1)
	go bn.loop()
	return bn
}

// id returns the node ID of the bystander.
func (bn *bystander) id() qnode.ID {
	return bn.conn.localNode.ID()
}

// close shuts down loop.
func (bn *bystander) close() {
	bn.conn.close()
	bn.done.Wait()
}

// loop answers packets from the remote node until quit.
func (bn *bystander) loop() {
	defer bn.done.Done()

	var (
		lastPing time.Time
		wasAdded bool
	)
	for {
		// Ping the remote node.
		if !wasAdded && time.Since(lastPing) > 10*time.Second {
			bn.conn.reqresp(bn.l, &v5wire.Ping{
				ReqID:  bn.conn.nextReqID(),
				QNRSeq: bn.dest.Seq(),
			})
			lastPing = time.Now()
		}
		// Answer packets.
		switch p := bn.conn.read(bn.l).(type) {
		case *v5wire.Ping:
			bn.conn.write(bn.l, &v5wire.Pong{
				ReqID:  p.ReqID,
				QNRSeq: bn.conn.localNode.Seq(),
				ToIP:   bn.dest.IP(),
				ToPort: uint16(bn.dest.UDP()),
			}, nil)
			wasAdded = true
			bn.notifyAdded()
		case *v5wire.Findnode:
			bn.conn.write(bn.l, &v5wire.Nodes{ReqID: p.ReqID, RespCount: 1}, nil)
			wasAdded = true
			bn.notifyAdded()
		case *v5wire.TalkRequest:
			bn.conn.write(bn.l, &v5wire.TalkResponse{ReqID: p.ReqID}, nil)
		case *readError:
			if !netutil.IsTemporaryError(p.err) {
				bn.conn.logf("shutting down: %v", p.err)
				return
			}
		}
	}
}

func (bn *bystander) notifyAdded() {
	if bn.addedCh != nil {
		bn.addedCh <- bn.id()
		bn.addedCh = nil
	}
}
