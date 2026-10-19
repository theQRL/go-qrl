// Copyright 2026 The go-qrl Authors
// This file is part of the go-qrl library.
//
// The go-qrl library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-qrl library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-qrl library. If not, see <http://www.gnu.org/licenses/>.

package rawdb

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"io"
	"testing"

	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/qrldb"
)

func TestGenesisJSONChunks(t *testing.T) {
	db := NewMemoryDatabase()
	defer db.Close()
	block := common.Hash{1}
	if r, err := OpenGenesisJSON(db, block); err != nil || r != nil {
		t.Fatalf("missing manifest: %v", err)
	}
	data := bytes.Repeat([]byte("0123456789"), genesisJSONChunkSize/4)
	if err := WriteGenesisJSON(db, block, bytes.NewReader(data)); err != nil {
		t.Fatal(err)
	}
	read := func() ([]byte, error) {
		r, err := OpenGenesisJSON(db, block)
		if err != nil {
			return nil, err
		}
		return io.ReadAll(r)
	}
	if got, err := read(); err != nil || !bytes.Equal(got, data) {
		t.Fatalf("chunk round trip failed: %v", err)
	}
	digest := sha256.Sum256(data)
	key := genesisJSONChunkKey(block, digest[:], 1)
	chunk, _ := db.Get(key)
	corrupt := bytes.Clone(chunk)
	corrupt[0] ^= 1
	db.Put(key, corrupt)
	if _, err := read(); err == nil {
		t.Fatal("accepted corrupt chunk")
	}
	db.Put(key, chunk[:len(chunk)-1])
	if _, err := read(); err == nil {
		t.Fatal("accepted truncated chunk")
	}
	db.Delete(key)
	if _, err := read(); err == nil {
		t.Fatal("accepted missing chunk")
	}
	db.Put(key, chunk)
	// A failed replacement must leave the previous manifest and file usable.
	failing := &failGenesisWriter{KeyValueWriter: db, remaining: 1}
	if err := WriteGenesisJSON(failing, block, bytes.NewReader(bytes.Repeat([]byte("x"), len(data)))); err == nil {
		t.Fatal("expected failed write")
	}
	if got, err := read(); err != nil || !bytes.Equal(got, data) {
		t.Fatalf("replacement damaged existing source: %v", err)
	}
}

type failGenesisWriter struct {
	qrldb.KeyValueWriter
	remaining int
}

func (w *failGenesisWriter) Put(key, value []byte) error {
	if w.remaining == 0 {
		return errors.New("injected write failure")
	}
	w.remaining--
	return w.KeyValueWriter.Put(key, value)
}
