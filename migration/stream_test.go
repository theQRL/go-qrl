package migration

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/core"
	"github.com/theQRL/go-qrl/core/vm/runtime"
	"github.com/theQRL/go-qrl/params"
)

func TestStreamingSnapshotGenesisAndClaim(t *testing.T) {
	path, owner, _, _ := legacyFixture(t)
	work := t.TempDir()
	store, err := Export(path, work)
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := store.WriteJSON(&output); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	var small Snapshot
	if err := json.Unmarshal(output.Bytes(), &small); err != nil {
		t.Fatal(err)
	}
	// Independently verify compatibility with the original json.Marshal digest.
	commitment := small.Commitment
	small.Commitment = ""
	canonical, err := json.Marshal(small)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(canonical)
	if hex.EncodeToString(hash[:]) != commitment {
		t.Fatal("streaming changed the snapshot commitment")
	}
	small.Commitment = commitment
	file := filepath.Join(t.TempDir(), "snapshot.json")
	if err := os.WriteFile(file, output.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	store, err = OpenSnapshot(file, work)
	if err != nil {
		t.Fatal(err)
	}
	if store.Metadata().Accounts != nil {
		t.Fatal("metadata retained accounts in memory")
	}
	defer store.Close()
	// Later use must rely on the private, validated copy, not reopen a mutable
	// input file after authenticating its commitment.
	if err := os.WriteFile(file, []byte("changed after validation"), 0600); err != nil {
		t.Fatal(err)
	}
	contract := common.BytesToAddress([]byte{0x10, 0})
	base := &core.Genesis{Config: &params.ChainConfig{ChainID: big.NewInt(9876)}, GasLimit: 30_000_000,
		Alloc: core.GenesisAlloc{common.BytesToAddress([]byte{0x22}): {Balance: big.NewInt(99)}}}
	var genesisJSON bytes.Buffer
	if err := store.WriteGenesis(&genesisJSON, base, contract); err != nil {
		t.Fatal(err)
	}
	var generated core.Genesis
	if err := json.Unmarshal(genesisJSON.Bytes(), &generated); err != nil {
		t.Fatal(err)
	}
	if _, exists := base.Alloc[contract]; exists {
		t.Fatal("streaming writer modified base genesis")
	}
	if err := AddToGenesis(base, &small, contract); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(base.Alloc, generated.Alloc) {
		t.Fatal("streamed genesis allocation differs from in-memory allocation")
	}
	v := setupVM(t, &small)
	recipient := common.BytesToAddress([]byte{0x99})
	source := "Q" + hex.EncodeToString(owner.address)
	message, err := store.ClaimMessage(big.NewInt(9876), contract, source, recipient)
	if err != nil {
		t.Fatal(err)
	}
	expected, err := ClaimMessage(&small, big.NewInt(9876), contract, source, recipient)
	if err != nil || !bytes.Equal(message, expected) {
		t.Fatal("streaming changed signing bytes", err)
	}
	sig := signedClaim(t, v, owner, 1, owner.address, recipient)
	tx, err := store.PrepareClaim(big.NewInt(9876), contract, source, recipient, []Signature{suppliedSignature(owner.pk, sig)})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := runtime.Call(tx.To, tx.Data, v.cfg); err != nil {
		t.Fatal("streamed claim failed", err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(work)
	if err != nil || len(entries) != 0 {
		t.Fatal("scratch data remained after Close", err)
	}
}

func TestStreamingRejectsCorruptSnapshotAndCleansScratch(t *testing.T) {
	path, _, _, _ := legacyFixture(t)
	s, err := exportSnapshot(path)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	for name, corrupt := range map[string][]byte{
		"truncated":        data[:len(data)-1],
		"trailing":         append(bytes.Clone(data), []byte("{}")...),
		"changed block":    bytes.Replace(data, []byte(s.BlockHash), bytes.Repeat([]byte("a"), 64), 1),
		"duplicate header": append([]byte(`{"height":123,`), data[1:]...),
	} {
		t.Run(name, func(t *testing.T) {
			work := t.TempDir()
			file := filepath.Join(t.TempDir(), "bad.json")
			if err := os.WriteFile(file, corrupt, 0600); err != nil {
				t.Fatal(err)
			}
			if store, err := OpenSnapshot(file, work); err == nil {
				store.Close()
				t.Fatal("accepted corrupt snapshot")
			}
			entries, err := os.ReadDir(work)
			if err != nil || len(entries) != 0 {
				t.Fatal("failed load leaked scratch data", err)
			}
		})
	}
}

type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestStreamingPropagatesWriteErrors(t *testing.T) {
	path, _, _, _ := legacyFixture(t)
	s, err := Export(path, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.WriteJSON(failWriter{}); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal("lost snapshot output error", err)
	}
	g := &core.Genesis{Config: &params.ChainConfig{ChainID: big.NewInt(9876)}}
	if err := s.WriteGenesis(failWriter{}, g, common.BytesToAddress([]byte{0x10, 0})); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal("lost genesis output error", err)
	}
}

// Opt-in disk/CPU stress test. Run the compiled test binary under /usr/bin/time
// with GOMEMLIMIT=128MiB and QRL_MIGRATION_STRESS_ACCOUNTS=50000 to measure RSS
// while exporting and rereading more than 1 GiB of snapshot JSON.
func TestStreamingLargeSnapshot(t *testing.T) {
	setting := os.Getenv("QRL_MIGRATION_STRESS_ACCOUNTS")
	if setting == "" {
		t.Skip("set QRL_MIGRATION_STRESS_ACCOUNTS to run the disk-backed stress test")
	}
	count, err := strconv.Atoi(setting)
	if err != nil || count <= 0 {
		t.Fatal("invalid stress account count")
	}
	path, _, _, _ := legacyFixture(t)
	db, err := leveldb.OpenFile(path, boundedDBOptions())
	if err != nil {
		t.Fatal(err)
	}
	var page []byte
	for i := 0; i < 1024; i++ {
		value := byte(0)
		if i < 256 {
			value = 0x55 // 1024 disjoint used indices per account.
		}
		page = pbBytes(page, 1, []byte{value})
	}
	for i := 0; i < count; i++ {
		pk := make([]byte, 67)
		pk[1] = 6
		binary.BigEndian.PutUint64(pk[3:11], uint64(i)+1000)
		address, err := AddressFromPublicKey(pk)
		if err != nil {
			t.Fatal(err)
		}
		data := pbUint(pbUint(pbBytes(nil, 1, address), 2, 1), 5, 1024)
		if err := db.Put(address, data, nil); err != nil {
			t.Fatal(err)
		}
		key := append(append([]byte("bitfield_"), address...), '_')
		if err := db.Put(binary.BigEndian.AppendUint64(key, 1), page, nil); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Put([]byte("total_coin_supply"), binary.BigEndian.AppendUint64(nil, 3_000_000_003+uint64(count)), nil); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	work := t.TempDir()
	store, err := Export(path, work)
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "large.json")
	f, err := os.Create(file)
	if err != nil {
		t.Fatal(err)
	}
	w := bufio.NewWriterSize(f, 64<<10)
	if err := store.WriteJSON(w); err != nil {
		t.Fatal(err)
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Exported %d accounts: %d bytes (%.2f GiB)", count+3, info.Size(), float64(info.Size())/(1<<30))
	store, err = OpenSnapshot(file, work)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	g := &core.Genesis{Config: &params.ChainConfig{ChainID: big.NewInt(9876)}, GasLimit: 30_000_000}
	if err := store.WriteGenesis(io.Discard, g, common.BytesToAddress([]byte{0x10, 0})); err != nil {
		t.Fatal(err)
	}
	if store.Metadata().TotalBalanceShor != strconv.FormatUint(3_000_000_003+uint64(count), 10) {
		t.Fatal("large export lost balances")
	}
}
