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

package core

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/core/rawdb"
	"github.com/theQRL/go-qrl/core/types"
	"github.com/theQRL/go-qrl/crypto"
	"github.com/theQRL/go-qrl/log"
	"github.com/theQRL/go-qrl/params"
	"github.com/theQRL/go-qrl/qrldb"
	"github.com/theQRL/go-qrl/qrldb/leveldb"
	"github.com/theQRL/go-qrl/rlp"
	"github.com/theQRL/go-qrl/trie"
)

// streamedGenesis owns a temporary disk index, derived trie nodes, and a private
// copy of the input JSON. Account and storage maps stay on disk; memory includes
// bounded caches and database file metadata. Individual header fields and
// contract bytecode still need to fit in memory.
type streamedGenesis struct {
	root      common.Hash
	rootNode  []byte
	scheme    string
	index     qrldb.Database
	nodes     qrldb.Database
	source    *os.File
	directory string
}

// SetupGenesisBlockFromJSON is the bounded-memory counterpart of
// SetupGenesisBlock. It accepts ordinary genesis JSON, including unsorted
// allocations and large individual storage objects. workDir selects a scratch
// filesystem (not tmpfs); an empty value uses the OS temporary directory.
// Temporary files are removed on return. No chain state is changed until the
// entire document, state root, and existing genesis compatibility are checked.
func SetupGenesisBlockFromJSON(db qrldb.Database, triedb *trie.Database, input io.Reader, workDir string) (*params.ChainConfig, common.Hash, error) {
	g, err := prepareGenesisJSON(input, workDir, triedb.Scheme(), triedb.PreimagesEnabled())
	if err != nil {
		return nil, common.Hash{}, err
	}
	defer g.stream.close()
	return SetupGenesisBlock(db, triedb, g)
}

func prepareGenesisJSON(input io.Reader, workDir, scheme string, preimages bool) (_ *Genesis, resultErr error) {
	if scheme != rawdb.HashScheme && scheme != rawdb.PathScheme {
		return nil, fmt.Errorf("unsupported state scheme %q", scheme)
	}
	directory, err := os.MkdirTemp(workDir, "gqrl-genesis-*")
	if err != nil {
		return nil, err
	}
	s := &streamedGenesis{directory: directory, scheme: scheme}
	defer func() {
		if resultErr != nil {
			s.close()
		}
	}()
	disk, err := leveldb.New(filepath.Join(directory, "index"), 16, 16, "genesis/import/", false)
	if err != nil {
		return nil, err
	}
	s.index = rawdb.NewDatabase(disk)
	s.nodes = rawdb.NewTable(s.index, "nodes-")
	s.source, err = os.Create(filepath.Join(directory, "genesis.json"))
	if err != nil {
		return nil, err
	}
	log.Info("Reading genesis into temporary disk index", "directory", directory)
	copyWriter := bufio.NewWriterSize(s.source, 64<<10)
	g, err := s.parse(io.TeeReader(input, copyWriter))
	if err != nil {
		return nil, err
	}
	if err := copyWriter.Flush(); err != nil {
		return nil, err
	}
	log.Info("Building genesis state from disk index")
	s.root, err = s.buildState(preimages)
	if err != nil {
		return nil, err
	}
	g.stream = s
	return g, nil
}

func (s *streamedGenesis) close() error {
	var errs []error
	if s.source != nil {
		errs = append(errs, s.source.Close())
	}
	if s.index != nil {
		errs = append(errs, s.index.Close())
	}
	errs = append(errs, os.RemoveAll(s.directory))
	return errors.Join(errs...)
}

// genesisBatch keeps even a many-million-node trie within a small write buffer.
// Rawdb accessors write into the batch; physical I/O errors are returned by flush.
type genesisBatch struct {
	qrldb.Batch
}

func (b genesisBatch) flush(force bool) error {
	if !force && b.ValueSize() < qrldb.IdealBatchSize {
		return nil
	}
	if err := b.Write(); err != nil {
		return err
	}
	b.Reset()
	return nil
}

func (s *streamedGenesis) buildState(preimages bool) (common.Hash, error) {
	batch := genesisBatch{s.nodes.NewBatch()}
	var writeErr error
	writeNode := func(owner common.Hash, path []byte, hash common.Hash, blob []byte) {
		if writeErr == nil {
			if owner == (common.Hash{}) && len(path) == 0 {
				s.rootNode = common.CopyBytes(blob)
				return
			}
			rawdb.WriteTrieNode(batch, owner, path, hash, blob, s.scheme)
			writeErr = batch.flush(false)
		}
	}
	accounts := trie.NewStackTrie(writeNode)
	it := s.index.NewIterator([]byte{'a'}, nil)
	defer it.Release()
	for it.Next() {
		owner := common.BytesToHash(it.Key()[1:])
		address := common.BytesToAddress(it.Value()[:common.AddressLength])
		var account GenesisAccount
		if err := json.Unmarshal(it.Value()[common.AddressLength:], &account); err != nil {
			return common.Hash{}, err
		}
		storageRoot, err := s.buildStorage(owner, writeNode, batch, preimages, &writeErr)
		if err != nil {
			return common.Hash{}, err
		}
		codeHash := crypto.Keccak256Hash(account.Code)
		if len(account.Code) > 0 {
			rawdb.WriteCode(batch, codeHash, account.Code)
		}
		if preimages {
			rawdb.WritePreimages(batch, map[common.Hash][]byte{owner: address[:]})
		}
		encoded, err := rlp.EncodeToBytes(&types.StateAccount{
			Nonce: account.Nonce, Balance: account.Balance, Root: storageRoot, CodeHash: codeHash[:],
		})
		if err != nil {
			return common.Hash{}, err
		}
		if err := accounts.Update(owner[:], encoded); err != nil {
			return common.Hash{}, err
		}
		if writeErr != nil {
			return common.Hash{}, writeErr
		}
		if err := batch.flush(false); err != nil {
			return common.Hash{}, err
		}
	}
	if err := it.Error(); err != nil {
		return common.Hash{}, err
	}
	root, err := accounts.Commit()
	if err != nil {
		return common.Hash{}, err
	}
	if writeErr != nil {
		return common.Hash{}, writeErr
	}
	return root, batch.flush(true)
}

func (s *streamedGenesis) buildStorage(owner common.Hash, writeNode trie.NodeWriteFunc, batch genesisBatch, preimages bool, writeErr *error) (common.Hash, error) {
	storage := trie.NewStackTrieWithOwner(writeNode, owner)
	it := s.index.NewIterator(append([]byte{'s'}, owner[:]...), nil)
	defer it.Release()
	for it.Next() {
		key, value := it.Value()[:common.HashLength], it.Value()[common.HashLength:]
		if common.BytesToStorageValue64(value) == (common.StorageValue64{}) {
			continue // A zero value is absent from the storage trie.
		}
		hash := common.BytesToHash(it.Key()[1+common.HashLength:])
		if preimages {
			rawdb.WritePreimages(batch, map[common.Hash][]byte{hash: key})
		}
		encoded, err := rlp.EncodeToBytes(common.TrimLeftZeroes(value))
		if err != nil {
			return common.Hash{}, err
		}
		if err := storage.Update(hash[:], encoded); err != nil {
			return common.Hash{}, err
		}
		if *writeErr != nil {
			return common.Hash{}, *writeErr
		}
	}
	if err := it.Error(); err != nil {
		return common.Hash{}, err
	}
	root, err := storage.Commit()
	if err != nil {
		return common.Hash{}, err
	}
	return root, *writeErr
}

func (s *streamedGenesis) flush(db qrldb.Database, triedb *trie.Database, block common.Hash) error {
	if s.scheme != triedb.Scheme() {
		return errors.New("genesis state scheme changed")
	}
	// Archive the exact validated input in chunks for dumpgenesis and state
	// recovery. This private file cannot change between validation and use.
	if err := rawdb.WriteGenesisJSON(db, block, s.source); err != nil {
		return err
	}
	batch := genesisBatch{db.NewBatch()}
	it := s.nodes.NewIterator(nil, nil)
	defer it.Release()
	// Publish the account root last in both schemes. Interrupted imports can
	// leave unreachable nodes, but never an apparently complete partial state.
	for it.Next() {
		if err := batch.Put(it.Key(), it.Value()); err != nil {
			return err
		}
		if err := batch.flush(false); err != nil {
			return err
		}
	}
	if err := it.Error(); err != nil {
		return err
	}
	if err := batch.flush(true); err != nil {
		return err
	}
	if s.root != types.EmptyRootHash && len(s.rootNode) == 0 {
		return errors.New("missing staged genesis root")
	}
	if len(s.rootNode) > 0 {
		rawdb.WriteTrieNode(batch, common.Hash{}, nil, s.root, s.rootNode, s.scheme)
		if err := batch.flush(true); err != nil {
			return err
		}
	}
	if s.scheme == rawdb.PathScheme {
		return triedb.Reset(s.root)
	}
	return nil
}
