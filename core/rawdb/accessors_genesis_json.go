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
	"encoding/binary"
	"errors"
	"fmt"
	"hash"
	"io"

	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/qrldb"
)

const genesisJSONChunkSize = 1 << 20

// Streamed genesis files are stored separately from the legacy allocation JSON
// blob. Chunks are content-addressed, so an interrupted replacement cannot
// corrupt the previously published file. The small manifest is written last.
func genesisJSONKey(block common.Hash) []byte {
	return append([]byte("qrl-genesis-json-"), block[:]...)
}

func genesisJSONChunkKey(block common.Hash, digest []byte, index uint64) []byte {
	key := append(genesisJSONKey(block), digest...)
	return binary.BigEndian.AppendUint64(key, index)
}

// WriteGenesisJSON saves a complete genesis document in bounded-size chunks.
// The source must be a private, immutable file for the duration of this call.
func WriteGenesisJSON(db qrldb.KeyValueWriter, block common.Hash, source io.ReadSeeker) error {
	if _, err := source.Seek(0, io.SeekStart); err != nil {
		return err
	}
	h := sha256.New()
	size, err := io.Copy(h, source)
	if err != nil {
		return err
	}
	if size == 0 {
		return errors.New("empty genesis JSON")
	}
	digest := h.Sum(nil)
	if _, err := source.Seek(0, io.SeekStart); err != nil {
		return err
	}
	buffer := make([]byte, genesisJSONChunkSize)
	for index, remaining := uint64(0), size; remaining > 0; index++ {
		n := int(min(remaining, int64(len(buffer))))
		if _, err := io.ReadFull(source, buffer[:n]); err != nil {
			return err
		}
		if err := db.Put(genesisJSONChunkKey(block, digest, index), buffer[:n]); err != nil {
			return err
		}
		remaining -= int64(n)
	}
	manifest := binary.BigEndian.AppendUint64([]byte{1}, uint64(size))
	manifest = append(manifest, digest...)
	return db.Put(genesisJSONKey(block), manifest)
}

// OpenGenesisJSON returns the stored genesis document, or nil if this database
// uses the legacy allocation blob. It checks chunk lengths and the digest while
// reading. Callers must read through EOF to verify the entire document.
func OpenGenesisJSON(db qrldb.KeyValueReader, block common.Hash) (io.Reader, error) {
	key := genesisJSONKey(block)
	exists, err := db.Has(key)
	if err != nil || !exists {
		return nil, err
	}
	manifest, err := db.Get(key)
	if err != nil {
		return nil, err
	}
	if len(manifest) != 41 || manifest[0] != 1 {
		return nil, errors.New("invalid streamed genesis manifest")
	}
	size := binary.BigEndian.Uint64(manifest[1:9])
	if size == 0 {
		return nil, errors.New("empty streamed genesis manifest")
	}
	return &genesisJSONReader{db: db, block: block, digest: manifest[9:], remaining: size, hash: sha256.New()}, nil
}

type genesisJSONReader struct {
	db        qrldb.KeyValueReader
	block     common.Hash
	digest    []byte
	remaining uint64
	index     uint64
	buffer    []byte
	hash      hash.Hash
}

func (r *genesisJSONReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if len(r.buffer) == 0 {
		if r.remaining == 0 {
			if !bytes.Equal(r.hash.Sum(nil), r.digest) {
				return 0, errors.New("streamed genesis checksum mismatch")
			}
			return 0, io.EOF
		}
		chunk, err := r.db.Get(genesisJSONChunkKey(r.block, r.digest, r.index))
		if err != nil {
			return 0, fmt.Errorf("read genesis chunk %d: %w", r.index, err)
		}
		if uint64(len(chunk)) != min(r.remaining, uint64(genesisJSONChunkSize)) {
			return 0, fmt.Errorf("invalid genesis chunk %d length", r.index)
		}
		r.hash.Write(chunk)
		r.remaining -= uint64(len(chunk))
		r.index++
		r.buffer = chunk
	}
	n := copy(p, r.buffer)
	r.buffer = r.buffer[n:]
	return n, nil
}
