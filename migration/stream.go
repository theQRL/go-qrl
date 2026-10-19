package migration

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

// SnapshotStore keeps accounts on temporary disk. Memory is proportional to the
// largest individual account (including its OTS ranges), plus bounded DB caches,
// rather than the complete snapshot. Close removes only its own temporary dir.
type SnapshotStore struct {
	metadata     Snapshot
	db           *leveldb.DB
	directory    string
	nullAccounts bool
}

func boundedDBOptions() *opt.Options {
	return &opt.Options{BlockCacheCapacity: 8 << 20, OpenFilesCacheCapacity: 32,
		WriteBuffer: 4 << 20, Strict: opt.StrictAll}
}

var scanOptions = &opt.ReadOptions{DontFillCache: true}

func newSnapshotStore(workDir string) (*SnapshotStore, error) {
	directory, err := os.MkdirTemp(workDir, "qrl-migrate-*")
	if err != nil {
		return nil, err
	}
	db, err := leveldb.OpenFile(filepath.Join(directory, "accounts"), boundedDBOptions())
	if err != nil {
		os.RemoveAll(directory)
		return nil, err
	}
	return &SnapshotStore{db: db, directory: directory}, nil
}

func (s *SnapshotStore) Close() error {
	if s.db == nil {
		return nil
	}
	err := s.db.Close()
	s.db = nil
	return errors.Join(err, os.RemoveAll(s.directory))
}

// Metadata returns the small manifest header. Accounts are accessed through
// Account or Walk, so callers never need to materialize the complete collection.
func (s *SnapshotStore) Metadata() Snapshot { return s.metadata }

type storedAccount struct {
	Account
	FullyUsedPages uint64 `json:"_fully_used_pages,omitempty"`
}

func (s *SnapshotStore) put(a Account) error {
	data, err := json.Marshal(storedAccount{a, a.fullyUsedPages})
	if err != nil {
		return err
	}
	return s.db.Put([]byte(a.Address), data, nil)
}

func decodeStoredAccount(data []byte) (Account, error) {
	var entry storedAccount
	if err := json.Unmarshal(data, &entry); err != nil {
		return Account{}, err
	}
	entry.Account.fullyUsedPages = entry.FullyUsedPages
	return entry.Account, nil
}

func (s *SnapshotStore) Account(address string) (Account, error) {
	data, err := s.db.Get([]byte(address), scanOptions)
	if err != nil {
		return Account{}, err
	}
	return decodeStoredAccount(data)
}

func (s *SnapshotStore) Walk(visit func(Account) error) error {
	it := s.db.NewIterator(nil, scanOptions)
	defer it.Release()
	for it.Next() {
		a, err := decodeStoredAccount(it.Value())
		if err != nil {
			return err
		}
		if err := visit(a); err != nil {
			return err
		}
	}
	return it.Error()
}

// Preserve the original compact Go JSON commitment exactly, including whether
// an empty accounts array was null or []. Only one account is encoded at a time.
func writeCanonicalSnapshot(w io.Writer, metadata *Snapshot, nullAccounts bool, walk func(func(Account) error) error, commitment string) error {
	header := *metadata
	header.Accounts, header.Commitment = nil, ""
	data, err := json.Marshal(header)
	if err != nil {
		return err
	}
	const suffix = `"accounts":null,"commitment":""}`
	if !bytes.HasSuffix(data, []byte(suffix)) {
		return errors.New("snapshot JSON layout changed")
	}
	if _, err := w.Write(data[:len(data)-len(suffix)]); err != nil {
		return err
	}
	if nullAccounts {
		if _, err := io.WriteString(w, `"accounts":null`); err != nil {
			return err
		}
	} else {
		if _, err := io.WriteString(w, `"accounts":[`); err != nil {
			return err
		}
		first := true
		if err := walk(func(a Account) error {
			if !first {
				if _, err := io.WriteString(w, ","); err != nil {
					return err
				}
			}
			first = false
			data, err := json.Marshal(a)
			if err != nil {
				return err
			}
			_, err = w.Write(data)
			return err
		}); err != nil {
			return err
		}
		if _, err := io.WriteString(w, "]"); err != nil {
			return err
		}
	}
	data, err = json.Marshal(commitment)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, `,"commitment":%s}`, data)
	return err
}

func (s *SnapshotStore) digest() (string, error) {
	h := sha256.New()
	if err := writeCanonicalSnapshot(h, &s.metadata, s.nullAccounts, s.Walk, ""); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func (s *SnapshotStore) WriteJSON(w io.Writer) error {
	if err := writeCanonicalSnapshot(w, &s.metadata, s.nullAccounts, s.Walk, s.metadata.Commitment); err != nil {
		return err
	}
	_, err := io.WriteString(w, "\n")
	return err
}

func (s *SnapshotStore) validate() error {
	total := new(big.Int)
	if err := s.Walk(func(a Account) error {
		if err := validateAccount(a); err != nil {
			return err
		}
		balance, _ := parseAmount(a.BalanceShor)
		total.Add(total, balance)
		return validateSignatories(a, s.Account)
	}); err != nil {
		return err
	}
	if err := s.metadata.validateTotals(total); err != nil {
		return err
	}
	digest, err := s.digest()
	if err != nil {
		return err
	}
	if digest != s.metadata.Commitment {
		return errors.New("snapshot commitment mismatch")
	}
	return nil
}

// OpenSnapshot reads the JSON array one account at a time into a temporary disk
// index, validates the complete snapshot, and verifies its original commitment.
// All consumers subsequently use that same private copy, avoiding input-file
// changes between validation and use. workDir is the scratch-disk parent.
func OpenSnapshot(path, workDir string) (_ *SnapshotStore, resultErr error) {
	input, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer input.Close()
	s, err := newSnapshotStore(workDir)
	if err != nil {
		return nil, err
	}
	defer func() {
		if resultErr != nil {
			s.Close()
		}
	}()
	d := json.NewDecoder(input)
	d.DisallowUnknownFields()
	if token, err := d.Token(); err != nil || token != json.Delim('{') {
		return nil, errors.New("snapshot must be a JSON object")
	}
	m := &s.metadata
	fields := map[string]any{
		"format": &m.Format, "network": &m.Network, "state_version": &m.StateVersion,
		"height": &m.Height, "block_hash": &m.BlockHash, "genesis_hash": &m.GenesisHash,
		"total_balance_shor": &m.TotalBalanceShor, "total_balance_base_units": &m.TotalBalanceBaseUnits,
		"recorded_coin_supply_shor": &m.RecordedCoinSupplyShor, "supply_gap_shor": &m.SupplyGapShor,
		"unmined_reserve_shor": &m.UnminedReserveShor, "commitment": &m.Commitment,
	}
	seen := make(map[string]bool)
	for d.More() {
		token, err := d.Token()
		if err != nil {
			return nil, err
		}
		name, ok := token.(string)
		if !ok || seen[name] {
			return nil, errors.New("invalid or duplicate snapshot field")
		}
		seen[name] = true
		if name != "accounts" {
			target, ok := fields[name]
			if !ok {
				return nil, fmt.Errorf("unknown snapshot field %q", name)
			}
			if err := d.Decode(target); err != nil {
				return nil, err
			}
			continue
		}
		token, err = d.Token()
		if err != nil {
			return nil, err
		}
		if token == nil {
			s.nullAccounts = true
			continue
		}
		if token != json.Delim('[') {
			return nil, errors.New("accounts must be an array or null")
		}
		previous := ""
		for d.More() {
			var a Account
			if err := d.Decode(&a); err != nil {
				return nil, err
			}
			if a.Address <= previous {
				return nil, errors.New("accounts must be sorted without duplicates")
			}
			previous = a.Address
			if err := validateAccount(a); err != nil {
				return nil, err
			}
			if err := s.put(a); err != nil {
				return nil, err
			}
		}
		if _, err := d.Token(); err != nil { // closing array
			return nil, err
		}
	}
	if _, err := d.Token(); err != nil { // closing object
		return nil, err
	}
	if len(seen) != len(fields)+1 {
		return nil, errors.New("snapshot is missing required fields")
	}
	if _, err := d.Token(); err != io.EOF {
		return nil, errors.New("extra data after snapshot")
	}
	if err := s.validate(); err != nil {
		return nil, err
	}
	return s, nil
}
