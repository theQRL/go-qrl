package migration

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/syndtr/goleveldb/leveldb"
)

// Tests use small fixtures. Production callers keep the disk-backed store and
// stream its JSON instead of constructing a full Snapshot in memory.
func exportSnapshot(path string) (*Snapshot, error) {
	store, err := Export(path, "")
	if err != nil {
		return nil, err
	}
	defer store.Close()
	var output bytes.Buffer
	if err := store.WriteJSON(&output); err != nil {
		return nil, err
	}
	var s Snapshot
	if err := json.Unmarshal(output.Bytes(), &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func TestExportLaterOTSPagesAndAbsentOwner(t *testing.T) {
	path, _, other, _ := legacyFixture(t)
	db, err := leveldb.OpenFile(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	// A multisig owner can exist solely in its group's signatory list.
	if err := db.Delete(other.address, nil); err != nil {
		t.Fatal(err)
	}
	key := append(append([]byte("bitfield_"), other.address...), '_')
	if err := db.Delete(binary.BigEndian.AppendUint64(key, 1), nil); err != nil {
		t.Fatal(err)
	}
	pk := bytes.Repeat([]byte{7}, 67)
	pk[0], pk[1], pk[2] = 2, 7, 0 // Synthetic height-14 SHAKE256 address.
	address, err := AddressFromPublicKey(pk)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Put(address, pbUint(pbUint(pbBytes(nil, 1, address), 2, 1), 5, 4), nil); err != nil {
		t.Fatal(err)
	}
	for page := uint64(1); page <= 2; page++ {
		bits := make([]byte, 1024)
		if page == 1 {
			bits[1023] = 128 // 8191
		} else {
			bits[0] = 3      // 8192, 8193
			bits[1023] = 128 // 16383
		}
		var data []byte
		for _, value := range bits {
			data = pbBytes(data, 1, []byte{value})
		}
		key := append(append([]byte("bitfield_"), address...), '_')
		if err := db.Put(binary.BigEndian.AppendUint64(key, page), data, nil); err != nil {
			t.Fatal(err)
		}
	}
	// Simulate a cleared balance: issuance is greater than extant wallet funds.
	if err := db.Put([]byte("total_coin_supply"), binary.BigEndian.AppendUint64(nil, 3_000_000_104), nil); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	s, err := exportSnapshot(path)
	if err != nil {
		t.Fatal(err)
	}
	if s.TotalBalanceShor != "3000000004" || s.SupplyGapShor != "100" {
		t.Fatal("incorrect supply accounting", s.TotalBalanceShor, s.SupplyGapShor)
	}
	var foundOwner, foundPages bool
	for _, account := range s.Accounts {
		switch account.Address {
		case "Q" + hex.EncodeToString(other.address):
			foundOwner = account.BalanceShor == "0" && account.UsedOTSCount == 0
		case "Q" + hex.EncodeToString(address):
			foundPages = account.UsedOTSCount == 4 && reflect.DeepEqual(account.UsedOTS, []OTSRange{{8191, 8193}, {16383, 16383}})
		}
	}
	if !foundOwner || !foundPages {
		t.Fatal("missing default multisig owner or high-index OTS history")
	}
	second, err := exportSnapshot(path)
	if err != nil || s.Commitment != second.Commitment {
		t.Fatal("export is not deterministic", err)
	}
}

func TestExportNeverCreatesMissingDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing-state")
	if _, err := exportSnapshot(path); err == nil {
		t.Fatal("opened missing database")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("missing database path was created", err)
	}
}
