// Copyright 2019 The go-ethereum Authors
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

package dnsdisc

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"maps"
	"reflect"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/theQRL/go-qrl/common/hexutil"
	"github.com/theQRL/go-qrl/common/mclock"
	"github.com/theQRL/go-qrl/crypto"
	"github.com/theQRL/go-qrl/internal/testlog"
	"github.com/theQRL/go-qrl/log"
	"github.com/theQRL/go-qrl/p2p/qnode"
	"github.com/theQRL/go-qrl/p2p/qnr"
)

var signingKeyForTesting, _ = crypto.ToECDSA(hexutil.MustDecode("0xdc599867fc513f8f5e2c2c9c489cde5e71362d1d9ec6e693e0de063236ed1240"))

func TestClientSyncTree(t *testing.T) {
	nodes := []string{
		"qnr:-HW4QOFzoVLaFJnNhbgMoDXPnOvcdVuj7pDpqRvh6BRDO68aVi5ZcjB3vzQRZH2IcLBGHzo8uUN3snqmgTiE56CH3AMBgmlkgnY0iXNlY3AyNTZrMaECC2_24YYkYHEgdzxlSNKQEnHhuNAbNlMlWJxrJxbAFvA",
		"qnr:-HW4QAggRauloj2SDLtIHN1XBkvhFZ1vtf1raYQp9TBW2RD5EEawDzbtSmlXUfnaHcvwOizhVYLtr7e6vw7NAf6mTuoCgmlkgnY0iXNlY3AyNTZrMaECjrXI8TLNXU0f8cthpAMxEshUyQlK-AM0PW2wfrnacNI",
		"qnr:-HW4QLAYqmrwllBEnzWWs7I5Ev2IAs7x_dZlbYdRdMUx5EyKHDXp7AV5CkuPGUPdvbv1_Ms1CPfhcGCvSElSosZmyoqAgmlkgnY0iXNlY3AyNTZrMaECriawHKWdDRk2xeZkrOXBQ0dfMFLHY4eENZwdufn1S1o",
	}

	r := mapResolver{
		"n":                            "qnrtree-root:v1 e=XJQBKU4LLJBNWT7BOGCZGSODG4 l=I4EQVGEUFFSFYVEIZTXGWFRKWA seq=1 sig=2eC2RXndIOgMSZSHkUpeJhTkRRcQIiOi271kXaa5KfZ2P4ijpP9GEnxx9QekBzq1db1lGjaf5k26F7wt4VldkAA",
		"I4EQVGEUFFSFYVEIZTXGWFRKWA.n": "qnrtree://AM5FCQLWIZX2QFPNJAP7VUERCCRNGRHWZG3YYHIUV7BVDQ5FDPRT2@morenodes.example.org",
		"XJQBKU4LLJBNWT7BOGCZGSODG4.n": "qnrtree-branch:VN37HFRVMVYCNLEOCKSNASP6GI,TBNBZMPRFCSNXN7M4CDQ4GYVXE,5UOZBDYWZMT44OHU5BOSAJLL7I",
		"VN37HFRVMVYCNLEOCKSNASP6GI.n": nodes[0],
		"TBNBZMPRFCSNXN7M4CDQ4GYVXE.n": nodes[1],
		"5UOZBDYWZMT44OHU5BOSAJLL7I.n": nodes[2],
	}
	var (
		wantNodes = sortByID(parseNodes(nodes))
		wantLinks = []string{"qnrtree://AM5FCQLWIZX2QFPNJAP7VUERCCRNGRHWZG3YYHIUV7BVDQ5FDPRT2@morenodes.example.org"}
		wantSeq   = uint(1)
	)

	c := NewClient(Config{Resolver: r, Logger: testlog.Logger(t, log.LvlTrace)})
	stree, err := c.SyncTree("qnrtree://AKPYQIUQIL7PSIACI32J7FGZW56E5FKHEFCCOFHILBIMW3M6LWXS2@n")
	if err != nil {
		t.Fatal("sync error:", err)
	}
	if !reflect.DeepEqual(sortByID(stree.Nodes()), wantNodes) {
		t.Errorf("wrong nodes in synced tree:\nhave %v\nwant %v", spew.Sdump(stree.Nodes()), spew.Sdump(wantNodes))
	}
	if !reflect.DeepEqual(stree.Links(), wantLinks) {
		t.Errorf("wrong links in synced tree: %v", stree.Links())
	}
	if stree.Seq() != wantSeq {
		t.Errorf("synced tree has wrong seq: %d", stree.Seq())
	}
}

// In this test, syncing the tree fails because it contains an invalid QNR entry.
func TestClientSyncTreeBadNode(t *testing.T) {
	// var b strings.Builder
	// b.WriteString(qnrPrefix)
	// b.WriteString("-----")
	// badHash := subdomain(&b)
	// tree, _ := MakeTree(3, nil, []string{"qnrtree://AM5FCQLWIZX2QFPNJAP7VUERCCRNGRHWZG3YYHIUV7BVDQ5FDPRT2@morenodes.example.org"})
	// tree.entries[badHash] = &b
	// tree.root.eroot = badHash
	// url, _ := tree.Sign(signingKeyForTesting, "n")
	// fmt.Println(url)
	// fmt.Printf("%#v\n", tree.ToTXT("n"))

	r := mapResolver{
		"n":                            "qnrtree-root:v1 e=USMXFT4JNYMLNTSJBJEEBUSTRQ l=I4EQVGEUFFSFYVEIZTXGWFRKWA seq=3 sig=i9dkL1DjNPk_mLZXaTvsAZmcSvdHSggrkwRM4bTvzGdBHN9BOST7PapHQG8djtX18hhtikK2jcXHsfFCeuAwcgE",
		"I4EQVGEUFFSFYVEIZTXGWFRKWA.n": "qnrtree://AM5FCQLWIZX2QFPNJAP7VUERCCRNGRHWZG3YYHIUV7BVDQ5FDPRT2@morenodes.example.org",
		"USMXFT4JNYMLNTSJBJEEBUSTRQ.n": "qnr:-----",
	}
	c := NewClient(Config{Resolver: r, Logger: testlog.Logger(t, log.LvlTrace)})
	_, err := c.SyncTree("qnrtree://AKPYQIUQIL7PSIACI32J7FGZW56E5FKHEFCCOFHILBIMW3M6LWXS2@n")
	wantErr := nameError{name: "USMXFT4JNYMLNTSJBJEEBUSTRQ.n", err: entryError{typ: "qnr", err: errInvalidQNR}}
	if err != wantErr {
		t.Fatalf("expected sync error %q, got %q", wantErr, err)
	}
}

// This test checks that randomIterator finds all entries.
func TestIterator(t *testing.T) {
	var (
		keys      = testKeys(30)
		nodes     = testNodes(keys)
		tree, url = makeTestTree("n", nodes, nil)
		r         = mapResolver(tree.ToTXT("n"))
	)

	c := NewClient(Config{
		Resolver:  r,
		Logger:    testlog.Logger(t, log.LvlTrace),
		RateLimit: 500,
	})
	it, err := c.NewIterator(url)
	if err != nil {
		t.Fatal(err)
	}

	checkIterator(t, it, nodes)
}

func TestIteratorCloseWithoutNext(t *testing.T) {
	tree1, url1 := makeTestTree("t1", nil, nil)
	c := NewClient(Config{Resolver: newMapResolver(tree1.ToTXT("t1"))})
	it, err := c.NewIterator(url1)
	if err != nil {
		t.Fatal(err)
	}

	it.Close()
	ok := it.Next()
	if ok {
		t.Fatal("Next returned true after Close")
	}
}

// This test checks if closing randomIterator races.
func TestIteratorClose(t *testing.T) {
	var (
		keys        = testKeys(500)
		nodes       = testNodes(keys)
		tree1, url1 = makeTestTree("t1", nodes, nil)
	)

	c := NewClient(Config{Resolver: newMapResolver(tree1.ToTXT("t1"))})
	it, err := c.NewIterator(url1)
	if err != nil {
		t.Fatal(err)
	}

	done := make(chan struct{})
	go func() {
		for it.Next() {
			_ = it.Node()
		}
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	it.Close()
	<-done
}

// This test checks that randomIterator traverses linked trees as well as explicitly added trees.
func TestIteratorLinks(t *testing.T) {
	var (
		keys        = testKeys(40)
		nodes       = testNodes(keys)
		tree1, url1 = makeTestTree("t1", nodes[:10], nil)
		tree2, url2 = makeTestTree("t2", nodes[10:], []string{url1})
	)

	c := NewClient(Config{
		Resolver:  newMapResolver(tree1.ToTXT("t1"), tree2.ToTXT("t2")),
		Logger:    testlog.Logger(t, log.LvlTrace),
		RateLimit: 500,
	})
	it, err := c.NewIterator(url2)
	if err != nil {
		t.Fatal(err)
	}

	checkIterator(t, it, nodes)
}

// This test verifies that randomIterator re-checks the root of the tree to catch
// updates to nodes.
func TestIteratorNodeUpdates(t *testing.T) {
	var (
		clock    = new(mclock.Simulated)
		keys     = testKeys(30)
		nodes    = testNodes(keys)
		resolver = newMapResolver()
		c        = NewClient(Config{
			Resolver:        resolver,
			Logger:          testlog.Logger(t, log.LvlTrace),
			RecheckInterval: 20 * time.Minute,
			RateLimit:       500,
		})
	)
	c.clock = clock
	tree1, url := makeTestTree("n", nodes[:25], nil)
	it, err := c.NewIterator(url)
	if err != nil {
		t.Fatal(err)
	}

	// Sync the original tree.
	resolver.add(tree1.ToTXT("n"))
	checkIterator(t, it, nodes[:25])

	// Ensure RandomNode returns the new nodes after the tree is updated.
	updateSomeNodes(keys, nodes)
	tree2, _ := makeTestTree("n", nodes, nil)
	resolver.clear()
	resolver.add(tree2.ToTXT("n"))
	t.Log("tree updated")

	clock.Run(c.cfg.RecheckInterval + 1*time.Second)
	checkIterator(t, it, nodes)
}

// This test checks that the tree root is rechecked when a couple of leaf
// requests have failed. The test is just like TestIteratorNodeUpdates, but
// without advancing the clock by recheckInterval after the tree update.
func TestIteratorRootRecheckOnFail(t *testing.T) {
	var (
		clock    = new(mclock.Simulated)
		keys     = testKeys(30)
		nodes    = testNodes(keys)
		resolver = newMapResolver()
		c        = NewClient(Config{
			Resolver:        resolver,
			Logger:          testlog.Logger(t, log.LvlTrace),
			RecheckInterval: 20 * time.Minute,
			RateLimit:       500,
			// Disabling the cache is required for this test because the client doesn't
			// notice leaf failures if all records are cached.
			CacheLimit: 1,
		})
	)
	c.clock = clock
	tree1, url := makeTestTree("n", nodes[:25], nil)
	it, err := c.NewIterator(url)
	if err != nil {
		t.Fatal(err)
	}

	// Sync the original tree.
	resolver.add(tree1.ToTXT("n"))
	checkIterator(t, it, nodes[:25])

	// Ensure RandomNode returns the new nodes after the tree is updated.
	updateSomeNodes(keys, nodes)
	tree2, _ := makeTestTree("n", nodes, nil)
	resolver.clear()
	resolver.add(tree2.ToTXT("n"))
	t.Log("tree updated")

	checkIterator(t, it, nodes)
}

// This test checks that the iterator works correctly when the tree is initially empty.
func TestIteratorEmptyTree(t *testing.T) {
	var (
		clock    = new(mclock.Simulated)
		keys     = testKeys(1)
		nodes    = testNodes(keys)
		resolver = newMapResolver()
		c        = NewClient(Config{
			Resolver:        resolver,
			Logger:          testlog.Logger(t, log.LvlTrace),
			RecheckInterval: 20 * time.Minute,
			RateLimit:       500,
		})
	)
	c.clock = clock
	tree1, url := makeTestTree("n", nil, nil)
	tree2, _ := makeTestTree("n", nodes, nil)
	resolver.add(tree1.ToTXT("n"))

	// Start the iterator.
	node := make(chan *qnode.Node, 1)
	it, err := c.NewIterator(url)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		it.Next()
		node <- it.Node()
	}()

	// Wait for the client to get stuck in waitForRootUpdates.
	clock.WaitForTimers(1)

	// Now update the root.
	resolver.add(tree2.ToTXT("n"))

	// Wait for it to pick up the root change.
	clock.Run(c.cfg.RecheckInterval)
	select {
	case n := <-node:
		if n.ID() != nodes[0].ID() {
			t.Fatalf("wrong node returned")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("it.Next() did not unblock within 5s of real time")
	}
}

// updateSomeNodes applies QNR updates to some of the given nodes.
func updateSomeNodes(keys []*ecdsa.PrivateKey, nodes []*qnode.Node) {
	for i, n := range nodes[:len(nodes)/2] {
		r := n.Record()
		r.Set(qnr.IP{127, 0, 0, 1})
		r.SetSeq(55)
		qnode.SignV4(r, keys[i])
		n2, _ := qnode.New(qnode.ValidSchemes, r)
		nodes[i] = n2
	}
}

// This test verifies that randomIterator re-checks the root of the tree to catch
// updates to links.
func TestIteratorLinkUpdates(t *testing.T) {
	var (
		clock    = new(mclock.Simulated)
		keys     = testKeys(30)
		nodes    = testNodes(keys)
		resolver = newMapResolver()
		c        = NewClient(Config{
			Resolver:        resolver,
			Logger:          testlog.Logger(t, log.LvlTrace),
			RecheckInterval: 20 * time.Minute,
			RateLimit:       500,
		})
	)
	c.clock = clock
	tree3, url3 := makeTestTree("t3", nodes[20:30], nil)
	tree2, url2 := makeTestTree("t2", nodes[10:20], nil)
	tree1, url1 := makeTestTree("t1", nodes[0:10], []string{url2})
	resolver.add(tree1.ToTXT("t1"))
	resolver.add(tree2.ToTXT("t2"))
	resolver.add(tree3.ToTXT("t3"))

	it, err := c.NewIterator(url1)
	if err != nil {
		t.Fatal(err)
	}

	// Sync tree1 using RandomNode.
	checkIterator(t, it, nodes[:20])

	// Add link to tree3, remove link to tree2.
	tree1, _ = makeTestTree("t1", nodes[:10], []string{url3})
	resolver.add(tree1.ToTXT("t1"))
	t.Log("tree1 updated")

	clock.Run(c.cfg.RecheckInterval + 1*time.Second)

	var wantNodes []*qnode.Node
	wantNodes = append(wantNodes, tree1.Nodes()...)
	wantNodes = append(wantNodes, tree3.Nodes()...)
	checkIterator(t, it, wantNodes)

	// Check that linked trees are GCed when they're no longer referenced.
	knownTrees := it.(*randomIterator).trees
	if len(knownTrees) != 2 {
		t.Errorf("client knows %d trees, want 2", len(knownTrees))
	}
}

func checkIterator(t *testing.T, it qnode.Iterator, wantNodes []*qnode.Node) {
	t.Helper()

	var (
		want     = make(map[qnode.ID]*qnode.Node)
		maxCalls = len(wantNodes) * 3
		calls    = 0
	)
	for _, n := range wantNodes {
		want[n.ID()] = n
	}
	for ; len(want) > 0 && calls < maxCalls; calls++ {
		if !it.Next() {
			t.Fatalf("Next returned false (call %d)", calls)
		}
		n := it.Node()
		delete(want, n.ID())
	}
	t.Logf("checkIterator called Next %d times to find %d nodes", calls, len(wantNodes))
	for _, n := range want {
		t.Errorf("iterator didn't discover node %v", n.ID())
	}
}

func makeTestTree(domain string, nodes []*qnode.Node, links []string) (*Tree, string) {
	tree, err := MakeTree(1, nodes, links)
	if err != nil {
		panic(err)
	}
	url, err := tree.Sign(signingKeyForTesting, domain)
	if err != nil {
		panic(err)
	}
	return tree, url
}

// testKeys creates deterministic private keys for testing.
func testKeys(n int) []*ecdsa.PrivateKey {
	keys := make([]*ecdsa.PrivateKey, n)
	for i := range n {
		key, err := crypto.GenerateKey()
		if err != nil {
			panic("can't generate key: " + err.Error())
		}
		keys[i] = key
	}
	return keys
}

func testNodes(keys []*ecdsa.PrivateKey) []*qnode.Node {
	nodes := make([]*qnode.Node, len(keys))
	for i, key := range keys {
		record := new(qnr.Record)
		record.SetSeq(uint64(i))
		qnode.SignV4(record, key)
		n, err := qnode.New(qnode.ValidSchemes, record)
		if err != nil {
			panic(err)
		}
		nodes[i] = n
	}
	return nodes
}

type mapResolver map[string]string

func newMapResolver(maps ...map[string]string) mapResolver {
	mr := make(mapResolver, len(maps))
	for _, m := range maps {
		mr.add(m)
	}
	return mr
}

func (mr mapResolver) clear() {
	clear(mr)
}

func (mr mapResolver) add(m map[string]string) {
	maps.Copy(mr, m)
}

func (mr mapResolver) LookupTXT(ctx context.Context, name string) ([]string, error) {
	if record, ok := mr[name]; ok {
		return []string{record}, nil
	}
	return nil, errors.New("not found")
}

func parseNodes(rec []string) []*qnode.Node {
	var ns []*qnode.Node
	for _, r := range rec {
		var n qnode.Node
		if err := n.UnmarshalText([]byte(r)); err != nil {
			panic(err)
		}
		ns = append(ns, &n)
	}
	return ns
}
