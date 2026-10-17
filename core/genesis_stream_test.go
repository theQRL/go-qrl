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
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/core/rawdb"
	"github.com/theQRL/go-qrl/core/state"
	"github.com/theQRL/go-qrl/crypto"
	"github.com/theQRL/go-qrl/params"
	"github.com/theQRL/go-qrl/qrldb"
	"github.com/theQRL/go-qrl/qrldb/leveldb"
	"github.com/theQRL/go-qrl/trie"
)

func streamGenesisFixture() *Genesis {
	storage := make(map[common.Hash]common.StorageValue64)
	for i := 0; i < 2048; i++ {
		key := crypto.Keccak256Hash([]byte(fmt.Sprint(i)))
		value := common.BytesToStorageValue64(key[:])
		value[0] = 0xab // Verify all 512 bits are retained.
		storage[key] = value
	}
	storage[common.Hash{}] = common.StorageValue64{}
	return &Genesis{
		Config: params.TestChainConfig, Timestamp: 12345, GasLimit: 30_000_000,
		ExtraData: []byte{1, 2, 3},
		Alloc: GenesisAlloc{
			{1}: {Balance: big.NewInt(321), Nonce: 7, Code: []byte{0x60, 0x00}, Storage: storage},
			{2}: {Balance: new(big.Int)},
			{3}: {Balance: big.NewInt(123), Nonce: 8},
		},
	}
}

func assertStreamedState(t *testing.T, db qrldb.Database, tdb *trie.Database, g *Genesis) {
	t.Helper()
	s, err := state.New(g.ToBlock().Root(), state.NewDatabaseWithNodeDB(db, tdb), nil)
	if err != nil {
		t.Fatal(err)
	}
	for address, account := range g.Alloc {
		if !s.Exist(address) || s.GetBalance(address).Cmp(account.Balance) != 0 || s.GetNonce(address) != account.Nonce || !bytes.Equal(s.GetCode(address), account.Code) {
			t.Fatalf("account differs at %s", address)
		}
		for key, value := range account.Storage {
			if got := s.GetState(address, key); got != value {
				t.Fatalf("storage differs at %s/%s: have %s want %s", address, key, got, value)
			}
		}
	}
	if err := s.Error(); err != nil {
		t.Fatal(err)
	}
}

func TestStreamGenesisMatchesLegacy(t *testing.T) {
	for _, scheme := range []string{rawdb.HashScheme, rawdb.PathScheme} {
		t.Run(scheme, func(t *testing.T) {
			g := streamGenesisFixture()
			data, err := json.Marshal(g)
			if err != nil {
				t.Fatal(err)
			}
			db := rawdb.NewMemoryDatabase()
			defer db.Close()
			config := *newDbConfig(scheme)
			config.Preimages = true
			tdb := trie.NewDatabase(db, &config)
			defer tdb.Close()
			work := t.TempDir()
			_, hash, err := SetupGenesisBlockFromJSON(db, tdb, bytes.NewReader(data), work)
			if err != nil {
				t.Fatal(err)
			}
			if hash != g.ToBlock().Hash() {
				t.Fatalf("genesis mismatch: have %s want %s", hash, g.ToBlock().Hash())
			}
			assertStreamedState(t, db, tdb, g)
			address := common.Address{1}
			if got := rawdb.ReadPreimage(db, crypto.Keccak256Hash(address[:])); !bytes.Equal(got, address[:]) {
				t.Fatal("missing account preimage")
			}
			for key, value := range g.Alloc[address].Storage {
				if value != (common.StorageValue64{}) && !bytes.Equal(rawdb.ReadPreimage(db, crypto.Keccak256Hash(key[:])), key[:]) {
					t.Fatal("missing storage preimage")
				}
			}
			read, err := ReadGenesis(db)
			if err != nil || read.ToBlock().Hash() != hash {
				t.Fatalf("ReadGenesis failed: %v", err)
			}
			source, err := rawdb.OpenGenesisJSON(db, hash)
			if err != nil {
				t.Fatal(err)
			}
			archived, err := io.ReadAll(source)
			if err != nil || !bytes.Equal(archived, data) {
				t.Fatalf("recovery source differs: %v", err)
			}
			// Reinitialization is idempotent, and mismatches do not modify state.
			if _, _, err := SetupGenesisBlockFromJSON(db, tdb, bytes.NewReader(data), work); err != nil {
				t.Fatal(err)
			}
			bad := bytes.Replace(data, []byte(`"timestamp":"0x3039"`), []byte(`"timestamp":"0x303a"`), 1)
			if bytes.Equal(bad, data) {
				t.Fatal("mismatch fixture unchanged")
			}
			_, _, err = SetupGenesisBlockFromJSON(db, tdb, bytes.NewReader(bad), work)
			var mismatch *GenesisMismatchError
			if !errors.As(err, &mismatch) {
				t.Fatalf("expected mismatch, got %v", err)
			}
			assertStreamedState(t, db, tdb, g)
			if rawdb.ReadCanonicalHash(db, 0) != hash {
				t.Fatal("mismatch changed genesis")
			}
			entries, err := os.ReadDir(work)
			if err != nil || len(entries) != 0 {
				t.Fatalf("temporary import files remain: %v", err)
			}
			// Rebuild state using only the archived chunks, with no input file.
			recovery := rawdb.NewMemoryDatabase()
			defer recovery.Close()
			if err := rawdb.WriteGenesisJSON(recovery, hash, bytes.NewReader(archived)); err != nil {
				t.Fatal(err)
			}
			recoveryTrie := trie.NewDatabase(recovery, &config)
			defer recoveryTrie.Close()
			if err := CommitGenesisState(recovery, recoveryTrie, hash); err != nil {
				t.Fatal(err)
			}
			assertStreamedState(t, recovery, recoveryTrie, g)
		})
	}
}

func TestStreamGenesisValidation(t *testing.T) {
	address := common.Address{1}.Hex()
	base := `{"gasLimit":"0x100000","config":{},"alloc":{}}`
	for name, input := range map[string]string{
		"truncated":         base[:len(base)-1],
		"trailing":          base + `{}`,
		"no alloc":          `{"gasLimit":1,"config":{}}`,
		"null alloc":        `{"gasLimit":1,"config":{},"alloc":null}`,
		"no gas":            `{"config":{},"alloc":{}}`,
		"no config":         `{"gasLimit":1,"alloc":{}}`,
		"duplicate header":  strings.Replace(base, `"gasLimit":`, `"gaslimit":1,"gasLimit":`, 1),
		"duplicate alloc":   strings.Replace(base, `"alloc":`, `"alloc":{},"alloc":`, 1),
		"no balance":        fmt.Sprintf(`{"gasLimit":1,"config":{},"alloc":{%q:{}}}`, address),
		"negative balance":  fmt.Sprintf(`{"gasLimit":1,"config":{},"alloc":{%q:{"balance":"-1"}}}`, address),
		"duplicate account": fmt.Sprintf(`{"gasLimit":1,"config":{},"alloc":{%q:{"balance":1},%q:{"balance":2}}}`, address, address),
		"duplicate slot":    fmt.Sprintf(`{"gasLimit":1,"config":{},"alloc":{%q:{"balance":1,"storage":{"0x01":"0x01","0x0001":"0x02"}}}}`, address),
		"bad storage":       fmt.Sprintf(`{"gasLimit":1,"config":{},"alloc":{%q:{"balance":1,"storage":{"0x01":"0x1"}}}}`, address),
	} {
		t.Run(name, func(t *testing.T) {
			db := rawdb.NewMemoryDatabase()
			defer db.Close()
			tdb := trie.NewDatabase(db, nil)
			defer tdb.Close()
			work := t.TempDir()
			if _, _, err := SetupGenesisBlockFromJSON(db, tdb, strings.NewReader(input), work); err == nil {
				t.Fatal("accepted invalid genesis")
			}
			it := db.NewIterator(nil, nil)
			defer it.Release()
			if it.Next() {
				t.Fatal("invalid input modified chain database")
			}
			if entries, err := os.ReadDir(work); err != nil || len(entries) != 0 {
				t.Fatalf("temporary files remain: %v", err)
			}
		})
	}
	// Member order, field capitalization, unknown fields, null storage, and
	// short hex storage values retain the normal genesis JSON behavior.
	valid := fmt.Sprintf(`{"unknown":{"nested":[{},2]},"ALLOC":{%q:{"storage":{"0x01":"0xab","0x02":null},"balance":1},%q:{"storage":null,"balance":0}},"GASLIMIT":1,"CONFIG":{}}`, address, common.Address{2}.Hex())
	var g Genesis
	if err := json.Unmarshal([]byte(valid), &g); err != nil {
		t.Fatal(err)
	}
	db := rawdb.NewMemoryDatabase()
	defer db.Close()
	tdb := trie.NewDatabase(db, nil)
	defer tdb.Close()
	_, hash, err := SetupGenesisBlockFromJSON(db, tdb, strings.NewReader(valid), t.TempDir())
	if err != nil || hash != g.ToBlock().Hash() {
		t.Fatalf("valid input differs: %v", err)
	}
}

func TestStreamGenesisPreservesAdvancedState(t *testing.T) {
	for _, scheme := range []string{rawdb.HashScheme, rawdb.PathScheme} {
		t.Run(scheme, func(t *testing.T) {
			g := streamGenesisFixture()
			data, _ := json.Marshal(g)
			db := rawdb.NewMemoryDatabase()
			defer db.Close()
			tdb := trie.NewDatabase(db, newDbConfig(scheme))
			defer tdb.Close()
			if _, _, err := SetupGenesisBlockFromJSON(db, tdb, bytes.NewReader(data), t.TempDir()); err != nil {
				t.Fatal(err)
			}
			s, err := state.New(g.ToBlock().Root(), state.NewDatabaseWithNodeDB(db, tdb), nil)
			if err != nil {
				t.Fatal(err)
			}
			s.SetBalance(common.Address{1}, big.NewInt(999))
			root, err := s.Commit(1, false)
			if err != nil {
				t.Fatal(err)
			}
			if err := tdb.Commit(root, false); err != nil {
				t.Fatal(err)
			}
			head := g.ToBlock().Header()
			head.Number, head.Time, head.Root = big.NewInt(1), g.Timestamp+10, root
			rawdb.WriteHeader(db, head)
			rawdb.WriteHeadHeaderHash(db, head.Hash())
			if _, _, err := SetupGenesisBlockFromJSON(db, tdb, bytes.NewReader(data), t.TempDir()); err != nil {
				t.Fatal(err)
			}
			if rawdb.ReadHeadHeaderHash(db) != head.Hash() {
				t.Fatal("reinitialization changed head")
			}
			s, err = state.New(root, state.NewDatabaseWithNodeDB(db, tdb), nil)
			if err != nil || s.GetBalance(common.Address{1}).Cmp(big.NewInt(999)) != 0 {
				t.Fatalf("reinitialization changed current state: %v", err)
			}
		})
	}
}

func TestStreamGenesisWriteFailure(t *testing.T) {
	for _, scheme := range []string{rawdb.HashScheme, rawdb.PathScheme} {
		t.Run(scheme, func(t *testing.T) {
			g := streamGenesisFixture()
			data, _ := json.Marshal(g)
			db := &failedGenesisDB{Database: rawdb.NewMemoryDatabase()}
			defer db.Close()
			tdb := trie.NewDatabase(db, newDbConfig(scheme))
			defer tdb.Close()
			work := t.TempDir()
			if _, _, err := SetupGenesisBlockFromJSON(db, tdb, bytes.NewReader(data), work); err == nil {
				t.Fatal("expected database error")
			}
			if rawdb.ReadCanonicalHash(db, 0) != (common.Hash{}) || tdb.Initialized(g.ToBlock().Root()) {
				t.Fatal("failed import published incomplete genesis")
			}
			if entries, err := os.ReadDir(work); err != nil || len(entries) != 0 {
				t.Fatalf("temporary files remain: %v", err)
			}
			// Successfully retry after the physical write failure is gone.
			db.allowWrites = true
			if _, _, err := SetupGenesisBlockFromJSON(db, tdb, bytes.NewReader(data), work); err != nil {
				t.Fatal(err)
			}
			assertStreamedState(t, db, tdb, g)
		})
	}
}

type failedGenesisDB struct {
	qrldb.Database
	allowWrites bool
}

func (db *failedGenesisDB) NewBatch() qrldb.Batch {
	return &failedGenesisBatch{Batch: db.Database.NewBatch(), db: db}
}

type failedGenesisBatch struct {
	qrldb.Batch
	db *failedGenesisDB
}

func (b *failedGenesisBatch) Write() error {
	if !b.db.allowWrites {
		return errors.New("injected batch failure")
	}
	return b.Batch.Write()
}

// Opt in explicitly: this fixture uses several GB of scratch space. The JSON
// contains one very large contract, as well as many separately allocated EOAs.
func TestStreamGenesisLarge(t *testing.T) {
	setting := os.Getenv("QRL_GENESIS_STRESS_SLOTS")
	if setting == "" {
		t.Skip("set QRL_GENESIS_STRESS_SLOTS to run the disk-backed genesis stress test")
	}
	count, err := strconv.Atoi(setting)
	if err != nil || count < 2 {
		t.Fatal("invalid stress slot count")
	}
	directory := t.TempDir()
	source, err := os.Create(filepath.Join(directory, "large.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	w := bufio.NewWriterSize(source, 64<<10)
	contract := common.Address{1}
	fmt.Fprintf(w, `{"config":{"chainId":9876},"gasLimit":30000000,"alloc":{%q:{"balance":"0x1234","code":"0x6000","nonce":1,"storage":{`, contract.Hex())
	valueFor := func(i int) common.StorageValue64 {
		value := common.BytesToStorageValue64(new(big.Int).SetUint64(uint64(i) + 1).Bytes())
		value[0] = 0x12
		return value
	}
	for i := count - 1; i >= 0; i-- {
		if i != count-1 {
			w.WriteByte(',')
		}
		key := common.BigToHash(big.NewInt(int64(i)))
		fmt.Fprintf(w, `%q:%q`, key.Hex(), valueFor(i).Hex())
	}
	fmt.Fprint(w, `}}`)
	for i := 0; i < 50_000; i++ {
		address := common.BigToAddress(big.NewInt(int64(i) + 1))
		fmt.Fprintf(w, `,%q:{"balance":1}`, address.Hex())
	}
	fmt.Fprint(w, `}}`)
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}
	info, _ := source.Stat()
	t.Logf("Importing %d slots in one contract and 50,000 EOAs: %d bytes (%.2f GiB)", count, info.Size(), float64(info.Size())/(1<<30))
	if _, err := source.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(directory, "chaindata")
	disk, err := leveldb.New(dbPath, 16, 16, "", false)
	if err != nil {
		t.Fatal(err)
	}
	db := rawdb.NewDatabase(disk)
	tdb := trie.NewDatabase(db, nil)
	_, hash, err := SetupGenesisBlockFromJSON(db, tdb, source, directory)
	if err != nil {
		tdb.Close()
		db.Close()
		t.Fatal(err)
	}
	tdb.Close()
	db.Close()
	// Reopen from the persisted database, without consulting the input file.
	disk, err = leveldb.New(dbPath, 16, 16, "", false)
	if err != nil {
		t.Fatal(err)
	}
	db = rawdb.NewDatabase(disk)
	defer db.Close()
	tdb = trie.NewDatabase(db, nil)
	defer tdb.Close()
	block := rawdb.ReadBlock(db, hash, 0)
	if block == nil {
		t.Fatal("missing genesis block")
	}
	s, err := state.New(block.Root(), state.NewDatabaseWithNodeDB(db, tdb), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, i := range []int{0, count / 2, count - 1} {
		key := common.BigToHash(big.NewInt(int64(i)))
		if s.GetState(contract, key) != valueFor(i) {
			t.Fatalf("lost storage slot %d", i)
		}
	}
	if s.GetBalance(contract).Uint64() != 0x1234 || s.GetNonce(contract) != 1 || !bytes.Equal(s.GetCode(contract), []byte{0x60, 0}) {
		t.Fatal("lost contract account fields")
	}
	for _, i := range []int64{1, 25_000, 50_000} {
		if s.GetBalance(common.BigToAddress(big.NewInt(i))).Uint64() != 1 {
			t.Fatalf("lost allocation %d", i)
		}
	}
	if err := s.Error(); err != nil {
		t.Fatal(err)
	}
	archived, err := rawdb.OpenGenesisJSON(db, hash)
	if err != nil {
		t.Fatal(err)
	}
	if n, err := io.Copy(io.Discard, archived); err != nil || n != info.Size() {
		t.Fatalf("invalid recovery source: %d bytes, %v", n, err)
	}
	t.Logf("Persisted and reopened genesis %s with state root %s", hash, block.Root())
}
