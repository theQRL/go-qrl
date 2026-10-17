package migration

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"strconv"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/iterator"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/util"
	"google.golang.org/protobuf/encoding/protowire"
)

// Legacy format references are pinned to theQRL/QRL commit
// a98af67d8d525e4ae102f77dc41c0ec0fb66db80: qrl.proto, OptimizedAddressState,
// MultiSigAddressState, PaginatedBitfield, Block, and State. No Python node or
// private keys are required. Only native currency balances are exported.
type legacyReader interface {
	Get([]byte, *opt.ReadOptions) ([]byte, error)
	NewIterator(*util.Range, *opt.ReadOptions) iterator.Iterator
}

type protoValue struct {
	kind   protowire.Type
	number uint64
	data   []byte
}
type protoMessage map[protowire.Number][]protoValue

func decodeProto(input []byte) (protoMessage, error) {
	m := make(protoMessage)
	for len(input) > 0 {
		number, kind, size := protowire.ConsumeTag(input)
		if size < 0 {
			return nil, protowire.ParseError(size)
		}
		input = input[size:]
		value := protoValue{kind: kind}
		switch kind {
		case protowire.VarintType:
			value.number, size = protowire.ConsumeVarint(input)
		case protowire.BytesType:
			value.data, size = protowire.ConsumeBytes(input)
		default:
			size = protowire.ConsumeFieldValue(number, kind, input)
		}
		if size < 0 {
			return nil, protowire.ParseError(size)
		}
		m[number] = append(m[number], value)
		input = input[size:]
	}
	return m, nil
}

func (m protoMessage) integers(fields ...protowire.Number) ([]uint64, error) {
	values := make([]uint64, len(fields))
	for i, field := range fields {
		entries := m[field]
		if len(entries) > 1 || (len(entries) == 1 && entries[0].kind != protowire.VarintType) {
			return nil, fmt.Errorf("invalid scalar protobuf field %d", field)
		}
		if len(entries) == 1 {
			values[i] = entries[0].number
		}
	}
	return values, nil
}

func (m protoMessage) blob(field protowire.Number) ([]byte, error) {
	entries := m[field]
	if len(entries) > 1 || (len(entries) == 1 && entries[0].kind != protowire.BytesType) {
		return nil, fmt.Errorf("invalid bytes protobuf field %d", field)
	}
	if len(entries) == 0 {
		return nil, nil
	}
	return entries[0].data, nil
}

// Export reads the LAST canonical block and current state of a stopped mainnet
// node into a temporary disk index. Call WriteJSON to stream the result and Close
// to release scratch space. workDir is a user-selected scratch parent (empty uses
// the OS temporary directory). No complete account collection is kept in RAM.
func Export(path, workDir string) (_ *SnapshotStore, resultErr error) {
	options := boundedDBOptions()
	options.ReadOnly, options.ErrorIfMissing = true, true
	db, err := leveldb.OpenFile(path, options)
	if err != nil {
		return nil, fmt.Errorf("open stopped legacy LevelDB: %w", err)
	}
	defer db.Close()
	view, err := db.GetSnapshot()
	if err != nil {
		return nil, err
	}
	defer view.Release()
	store, err := newSnapshotStore(workDir)
	if err != nil {
		return nil, err
	}
	defer func() {
		if resultErr != nil {
			store.Close()
		}
	}()
	if err := exportState(view, store); err != nil {
		return nil, err
	}
	return store, nil
}

func blockAt(db legacyReader, height uint64) ([]byte, error) {
	data, err := db.Get([]byte(strconv.FormatUint(height, 10)), nil)
	if err != nil {
		return nil, fmt.Errorf("canonical block %d: %w", height, err)
	}
	var mapping struct {
		HeaderHash []byte `json:"headerhash"`
	}
	if err := json.Unmarshal(data, &mapping); err != nil || len(mapping.HeaderHash) != 32 {
		return nil, errors.New("invalid canonical block mapping")
	}
	blockData, err := db.Get(mapping.HeaderHash, nil)
	if err != nil {
		return nil, err
	}
	block, err := decodeProto(blockData)
	if err != nil {
		return nil, err
	}
	headerData, err := block.blob(1)
	if err != nil {
		return nil, err
	}
	header, err := decodeProto(headerData)
	if err != nil {
		return nil, err
	}
	hash, err := header.blob(1)
	if err != nil {
		return nil, err
	}
	fields, err := header.integers(2)
	if err != nil || !bytes.Equal(hash, mapping.HeaderHash) || fields[0] != height {
		return nil, errors.New("canonical block header mismatch")
	}
	return hash, nil
}

func readBigEndian(db legacyReader, key string) (uint64, error) {
	b, err := db.Get([]byte(key), nil)
	if err != nil {
		return 0, err
	}
	if len(b) != 8 {
		return 0, fmt.Errorf("%s must contain eight bytes", key)
	}
	return binary.BigEndian.Uint64(b), nil
}

func exportState(db legacyReader, store *SnapshotStore) error {
	metadata, err := readLegacyMetadata(db)
	if err != nil {
		return err
	}
	store.metadata = metadata
	store.nullAccounts = true
	// Persist positive-balance addresses first. LevelDB provides deterministic
	// address ordering and deduplication without an in-memory accounts map.
	it := db.NewIterator(nil, scanOptions)
	for it.Next() {
		key := it.Key()
		if len(key) != 39 {
			continue
		}
		if err := validateAddress(key); err != nil {
			it.Release()
			return fmt.Errorf("state key %x: %w", key, err)
		}
		a, err := decodeAccount(key, it.Value())
		if err != nil {
			it.Release()
			return err
		}
		if a.BalanceShor != "0" {
			if err := store.put(a); err != nil {
				it.Release()
				return err
			}
			store.nullAccounts = false
		}
	}
	err = it.Error()
	it.Release()
	if err != nil {
		return err
	}
	// A multisig signer may have zero balance or no address-state record at all.
	// Deduplicate these lookups on disk, including owners shared by many groups.
	if err := store.Walk(func(a Account) error {
		for _, owner := range a.Signatories {
			exists, err := store.db.Has([]byte(owner.Address), scanOptions)
			if err != nil {
				return err
			}
			if exists {
				continue
			}
			address, err := LegacyAddress(owner.Address)
			if err != nil {
				return err
			}
			data, err := db.Get(address, scanOptions)
			var signer Account
			if errors.Is(err, leveldb.ErrNotFound) {
				signer = newAccount(address, 0)
				signer.Kind = "xmss"
				signer.Height = uint32(address[1]&15) * 2
			} else if err != nil {
				return err
			} else if signer, err = decodeAccount(address, data); err != nil {
				return err
			}
			if err := store.put(signer); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	total := new(big.Int)
	if err := store.Walk(func(a Account) error {
		if a.Kind == "xmss" {
			address, _ := LegacyAddress(a.Address)
			ranges, count, err := readOTS(db, address, a.Height)
			if err != nil {
				return fmt.Errorf("%s: %w", a.Address, err)
			}
			if count != a.UsedOTSCount {
				return fmt.Errorf("%s: OTS pages contain %d used indices, address state records %d", a.Address, count, a.UsedOTSCount)
			}
			capacity := uint64(1) << a.Height
			if a.fullyUsedPages > (capacity+8191)/8192 {
				return fmt.Errorf("%s: fully-used OTS page count exceeds tree", a.Address)
			}
			prefixLength := min(a.fullyUsedPages*8192, capacity)
			if prefixLength > 0 && (len(ranges) == 0 || ranges[0].First != 0 || uint64(ranges[0].Last)+1 < prefixLength) {
				return fmt.Errorf("%s: fully-used OTS page is missing used bits", a.Address)
			}
			a.UsedOTS = ranges
		}
		if err := validateAccount(a); err != nil {
			return err
		}
		if err := validateSignatories(a, store.Account); err != nil {
			return err
		}
		amount, _ := parseAmount(a.BalanceShor)
		total.Add(total, amount)
		return store.put(a)
	}); err != nil {
		return err
	}
	store.metadata.TotalBalanceShor = total.String()
	store.metadata.TotalBalanceBaseUnits = new(big.Int).Mul(total, big.NewInt(BalanceScale)).String()
	supply, _ := parseAmount(store.metadata.RecordedCoinSupplyShor)
	store.metadata.SupplyGapShor = new(big.Int).Sub(supply, total).String()
	if err := store.metadata.validateTotals(total); err != nil {
		return err
	}
	store.metadata.Commitment, err = store.digest()
	return err
}

func readLegacyMetadata(db legacyReader) (Snapshot, error) {
	var s Snapshot
	version, err := db.Get([]byte("state_version"), scanOptions)
	if err != nil || string(version) != "3" {
		return s, errors.New("only migrated legacy state version 3 is supported")
	}
	if _, err := db.Get([]byte("fork_state"), scanOptions); err == nil {
		return s, errors.New("legacy state has an unfinished reorganization")
	} else if !errors.Is(err, leveldb.ErrNotFound) {
		return s, err
	}
	height, err := readBigEndian(db, "blockheight")
	if err != nil {
		return s, err
	}
	genesisHash, err := blockAt(db, 0)
	if err != nil {
		return s, err
	}
	if hex.EncodeToString(genesisHash) != MainnetGenesisHash {
		return s, errors.New("state is not QRL mainnet")
	}
	blockHash, err := blockAt(db, height)
	if err != nil {
		return s, err
	}
	s = Snapshot{Format: Format, Network: "qrl-mainnet", StateVersion: 3,
		Height: height, BlockHash: hex.EncodeToString(blockHash), GenesisHash: MainnetGenesisHash}
	supply, err := readBigEndian(db, "total_coin_supply")
	if err != nil {
		return s, fmt.Errorf("read recorded coin supply: %w", err)
	}
	s.RecordedCoinSupplyShor = strconv.FormatUint(supply, 10)
	reserveData, err := db.Get(make([]byte, 32), scanOptions)
	if err != nil {
		return s, fmt.Errorf("read unmined coinbase reserve: %w", err)
	}
	reserve, err := decodeProto(reserveData)
	if err != nil {
		return s, err
	}
	reserveAddress, err := reserve.blob(1)
	if err != nil || !bytes.Equal(reserveAddress, make([]byte, 32)) {
		return s, errors.New("coinbase reserve address mismatch")
	}
	values, err := reserve.integers(2)
	if err != nil {
		return s, err
	}
	s.UnminedReserveShor = strconv.FormatUint(values[0], 10)
	return s, nil
}

func decodeAccount(address, data []byte) (Account, error) {
	var a Account
	m, err := decodeProto(data)
	if err != nil {
		return a, err
	}
	stored, err := m.blob(1)
	if err != nil || !bytes.Equal(stored, address) {
		return a, errors.New("legacy address-state key mismatch")
	}
	if address[0] != 0x11 {
		fields, err := m.integers(2, 5, 4)
		if err != nil {
			return a, err
		}
		a = newAccount(address, fields[0])
		a.Kind = "xmss"
		a.Height = uint32(address[1]&15) * 2
		a.UsedOTSCount = fields[1]
		a.fullyUsedPages = fields[2]
		return a, nil
	}
	fields, err := m.integers(4, 7)
	if err != nil || fields[1] > uint64(^uint32(0)) {
		return a, errors.New("invalid multisig balance or threshold")
	}
	a = newAccount(address, fields[0])
	a.Kind = "multisig"
	a.Threshold = uint32(fields[1])
	var weights []uint64
	for _, v := range m[6] {
		switch v.kind {
		case protowire.VarintType:
			weights = append(weights, v.number)
		case protowire.BytesType:
			for b := v.data; len(b) > 0; {
				n, size := protowire.ConsumeVarint(b)
				if size < 0 {
					return a, protowire.ParseError(size)
				}
				weights = append(weights, n)
				b = b[size:]
			}
		default:
			return a, errors.New("invalid multisig weight encoding")
		}
	}
	if len(weights) != len(m[5]) {
		return a, errors.New("multisig signatory/weight count mismatch")
	}
	for i, v := range m[5] {
		if v.kind != protowire.BytesType || validateAddress(v.data) != nil || v.data[0]>>4 != 0 || weights[i] == 0 || weights[i] > uint64(^uint32(0)) {
			return a, errors.New("invalid multisig signatory")
		}
		a.Signatories = append(a.Signatories, Signatory{Address: "Q" + hex.EncodeToString(v.data), Weight: uint32(weights[i])})
	}
	sort.Slice(a.Signatories, func(i, j int) bool { return a.Signatories[i].Address < a.Signatories[j].Address })
	return a, nil
}

func readOTS(db legacyReader, address []byte, height uint32) ([]OTSRange, uint64, error) {
	prefix := append(append([]byte("bitfield_"), address...), '_')
	it := db.NewIterator(util.BytesPrefix(prefix), scanOptions)
	defer it.Release()
	var ranges []OTSRange
	var count uint64
	for it.Next() {
		if len(it.Key()) != len(prefix)+8 {
			return nil, 0, errors.New("invalid OTS page key")
		}
		page := binary.BigEndian.Uint64(it.Key()[len(prefix):])
		capacity := uint64(1) << height
		if page == 0 || page > (capacity+8191)/8192 {
			return nil, 0, errors.New("OTS page outside XMSS tree")
		}
		m, err := decodeProto(it.Value())
		if err != nil {
			return nil, 0, err
		}
		if len(m[1]) != 1024 {
			return nil, 0, errors.New("OTS page must contain 1024 one-byte values")
		}
		for offset, v := range m[1] {
			if v.kind != protowire.BytesType || len(v.data) != 1 {
				return nil, 0, errors.New("invalid OTS byte")
			}
			for bit := uint64(0); bit < 8; bit++ {
				if v.data[0]&(1<<bit) == 0 {
					continue
				}
				index := (page-1)*8192 + uint64(offset)*8 + bit
				if index >= capacity {
					return nil, 0, errors.New("used OTS bit outside XMSS tree")
				}
				count++
				if len(ranges) > 0 && uint64(ranges[len(ranges)-1].Last)+1 == index {
					ranges[len(ranges)-1].Last = uint32(index)
				} else {
					ranges = append(ranges, OTSRange{uint32(index), uint32(index)})
				}
			}
		}
	}
	return ranges, count, it.Error()
}
