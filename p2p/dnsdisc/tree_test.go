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
	"reflect"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/theQRL/go-qrl/common/hexutil"
	"github.com/theQRL/go-qrl/p2p/qnode"
)

func TestParseRoot(t *testing.T) {
	tests := []struct {
		input string
		e     rootEntry
		err   error
	}{
		{
			input: "qnrtree-root:v1 e=TO4Q75OQ2N7DX4EOOR7X66A6OM seq=3 sig=N-YY6UB9xD0hFx1Gmnt7v0RfSxch5tKyry2SRDoLx7B4GfPXagwLxQqyf7gAMvApFn_ORwZQekMWa_pXrcGCtw",
			err:   entryError{"root", errSyntax},
		},
		{
			input: "qnrtree-root:v1 e=TO4Q75OQ2N7DX4EOOR7X66A6OM l=TO4Q75OQ2N7DX4EOOR7X66A6OM seq=3 sig=N-YY6UB9xD0hFx1Gmnt7v0RfSxch5tKyry2SRDoLx7B4GfPXagwLxQqyf7gAMvApFn_ORwZQekMWa_pXrcGCtw",
			err:   entryError{"root", errInvalidSig},
		},
		{
			input: "qnrtree-root:v1 e=QFT4PBCRX4XQCV3VUYJ6BTCEPU l=JGUFMSAGI7KZYB3P7IZW4S5Y3A seq=3 sig=3FmXuVwpa8Y7OstZTx9PIb1mt8FrW7VpDOFv4AaGCsZ2EIHmhraWhe4NxYhQDlw5MjeFXYMbJjsPeKlHzmJREQE",
			e: rootEntry{
				eroot: "QFT4PBCRX4XQCV3VUYJ6BTCEPU",
				lroot: "JGUFMSAGI7KZYB3P7IZW4S5Y3A",
				seq:   3,
				sig:   hexutil.MustDecode("0xdc5997b95c296bc63b3acb594f1f4f21bd66b7c16b5bb5690ce16fe006860ac6761081e686b69685ee0dc588500e5c393237855d831b263b0f78a947ce62511101"),
			},
		},
	}
	for i, test := range tests {
		e, err := parseRoot(test.input)
		if !reflect.DeepEqual(e, test.e) {
			t.Errorf("test %d: wrong entry %s, want %s", i, spew.Sdump(e), spew.Sdump(test.e))
		}
		if err != test.err {
			t.Errorf("test %d: wrong error %q, want %q", i, err, test.err)
		}
	}
}

func TestParseEntry(t *testing.T) {
	testQNRs := []string{"qnr:-HW4QES8QIeXTYlDzbfr1WEzE-XKY4f8gJFJzjJL-9D7TC9lJb4Z3JPRRz1lP4pL_N_QpT6rGQjAU9Apnc-C1iMP36OAgmlkgnY0iXNlY3AyNTZrMaED5IdwfMxdmR8W37HqSFdQLjDkIwBd4Q_MjxgZifgKSdM"}
	testNodes := parseNodes(testQNRs)

	tests := []struct {
		input string
		e     entry
		err   error
	}{
		// Subtrees:
		{
			input: "qnrtree-branch:1,2",
			err:   entryError{"branch", errInvalidChild},
		},
		{
			input: "qnrtree-branch:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
			err:   entryError{"branch", errInvalidChild},
		},
		{
			input: "qnrtree-branch:",
			e:     &branchEntry{},
		},
		{
			input: "qnrtree-branch:AAAAAAAAAAAAAAAAAAAA",
			e:     &branchEntry{[]string{"AAAAAAAAAAAAAAAAAAAA"}},
		},
		{
			input: "qnrtree-branch:AAAAAAAAAAAAAAAAAAAA,BBBBBBBBBBBBBBBBBBBB",
			e:     &branchEntry{[]string{"AAAAAAAAAAAAAAAAAAAA", "BBBBBBBBBBBBBBBBBBBB"}},
		},
		// Links
		{
			input: "qnrtree://AKPYQIUQIL7PSIACI32J7FGZW56E5FKHEFCCOFHILBIMW3M6LWXS2@nodes.example.org",
			e: &linkEntry{
				str:    "AKPYQIUQIL7PSIACI32J7FGZW56E5FKHEFCCOFHILBIMW3M6LWXS2@nodes.example.org",
				domain: "nodes.example.org",
				pubkey: &signingKeyForTesting.PublicKey,
			},
		},
		{
			input: "qnrtree://nodes.example.org",
			err:   entryError{"link", errNoPubkey},
		},
		{
			input: "qnrtree://AP62DT7WOTEQZGQZOU474PP3KMEGVTTE7A7NPRXKX3DUD57@nodes.example.org",
			err:   entryError{"link", errBadPubkey},
		},
		{
			input: "qnrtree://AP62DT7WONEQZGQZOU474PP3KMEGVTTE7A7NPRXKX3DUD57TQHGIA@nodes.example.org",
			err:   entryError{"link", errBadPubkey},
		},
		// QNRs
		{
			input: testQNRs[0],
			e:     &qnrEntry{node: testNodes[0]},
		},
		{
			input: "qnr:-HW4QLZHjM4vZXkbp-5xJoHsKSbE7W39FPC8283X-y8oHcHPTnDDlIlzL5ArvDUlHZVDPgmFASrh7cWgLOLxj4wprRkHgmlkgnY0iXNlY3AyNTZrMaEC3t2jLMhDpCDX5mbSEwDn4L3iUfyXzoO8G28XvjGRkrAg=",
			err:   entryError{"qnr", errInvalidQNR},
		},
		// Invalid:
		{input: "", err: errUnknownEntry},
		{input: "foo", err: errUnknownEntry},
		{input: "qnrtree", err: errUnknownEntry},
		{input: "qnrtree-x=", err: errUnknownEntry},
	}
	for i, test := range tests {
		e, err := parseEntry(test.input, qnode.ValidSchemes)
		if !reflect.DeepEqual(e, test.e) {
			t.Errorf("test %d: wrong entry %s, want %s", i, spew.Sdump(e), spew.Sdump(test.e))
		}
		if err != test.err {
			t.Errorf("test %d: wrong error %q, want %q", i, err, test.err)
		}
	}
}

func TestMakeTree(t *testing.T) {
	keys := testKeys(50)
	nodes := testNodes(keys)
	tree, err := MakeTree(2, nodes, nil)
	if err != nil {
		t.Fatal(err)
	}
	txt := tree.ToTXT("")
	if len(txt) < len(nodes)+1 {
		t.Fatal("too few TXT records in output")
	}
}
