package migration

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"sort"

	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/core"
)

// walkStorage emits each nonzero contract slot exactly once. In particular,
// separated OTS ranges within one 512-bit word are merged before emitting it.
// No map containing the complete migration storage is allocated.
func walkStorage(s *Snapshot, chainID *big.Int, walk func(func(Account) error) error, visit func(common.Hash, common.StorageValue64) error) error {
	put := func(key common.Hash, value *big.Int) error {
		if value.Sign() == 0 {
			return nil
		}
		return visit(key, common.BytesToStorageValue64(value.Bytes()))
	}
	total, _ := parseAmount(s.TotalBalanceBaseUnits)
	blockHash, _ := hex.DecodeString(s.BlockHash)
	commitment, _ := hex.DecodeString(s.Commitment)
	for i, value := range []*big.Int{new(big.Int).SetUint64(s.Height), new(big.Int).SetBytes(blockHash), new(big.Int).SetBytes(commitment), chainID, total} {
		if err := put(slot(uint64(i)), value); err != nil {
			return err
		}
	}
	return walk(func(entry Account) error {
		address, _ := LegacyAddress(entry.Address)
		id := AccountID(address)
		balance, _ := parseAmount(entry.BalanceBaseUnits)
		kind := int64(1)
		if entry.Kind == "multisig" {
			kind = 2
		}
		for _, field := range []struct {
			slot  uint64
			value *big.Int
		}{{5, balance}, {6, big.NewInt(kind)}, {8, new(big.Int).SetUint64(uint64(entry.Height))}, {9, new(big.Int).SetUint64(uint64(entry.Threshold))}} {
			if err := put(mappingSlot(id, slot(field.slot)), field.value); err != nil {
				return err
			}
		}
		bitmapBase := mappingSlot(id, slot(7))
		var currentWord uint64
		bits := new(big.Int)
		flush := func() error { return put(mappingSlot(new(big.Int).SetUint64(currentWord), bitmapBase), bits) }
		for _, used := range entry.UsedOTS {
			for start := uint64(used.First); start <= uint64(used.Last); {
				word, bit := start/512, start%512
				if word != currentWord {
					if err := flush(); err != nil {
						return err
					}
					currentWord = word
					bits.SetUint64(0)
				}
				end := min(uint64(used.Last), word*512+511)
				mask := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), uint(end-start+1)), big.NewInt(1))
				bits.Or(bits, mask.Lsh(mask, uint(bit)))
				start = end + 1
			}
		}
		if err := flush(); err != nil {
			return err
		}
		weightBase := mappingSlot(id, slot(10))
		for _, signer := range entry.Signatories {
			address, _ := LegacyAddress(signer.Address)
			if err := put(mappingSlot(AccountID(address), weightBase), new(big.Int).SetUint64(uint64(signer.Weight))); err != nil {
				return err
			}
		}
		return nil
	})
}

type jsonWriter struct {
	w   io.Writer
	err error
}

func (w *jsonWriter) text(s string) {
	if w.err == nil {
		_, w.err = io.WriteString(w.w, s)
	}
}

func (w *jsonWriter) value(value any) {
	if w.err == nil {
		var data []byte
		data, w.err = json.Marshal(value)
		if w.err == nil {
			_, w.err = w.w.Write(data)
		}
	}
}

// WriteGenesis streams the migration account's storage to ordinary genesis JSON.
// The caller's base genesis is expected to be small and remains in memory. The
// gqrl init also streams the generated allocation and individual storage slots.
func (s *SnapshotStore) WriteGenesis(output io.Writer, genesis *core.Genesis, contract common.Address) error {
	if err := validateBaseGenesis(genesis, contract); err != nil {
		return err
	}
	total, _ := parseAmount(s.metadata.TotalBalanceBaseUnits)
	if total.BitLen() > 256 {
		return errors.New("target balance exceeds uint256")
	}
	_, _, code, err := Contract()
	if err != nil {
		return err
	}
	// Encode only the small header, then stream existing base allocations and
	// the migration contract. core.Genesis.MarshalJSON would otherwise buffer
	// the entire contract storage object, even when given a json.Encoder.
	header := *genesis
	header.Alloc = nil
	data, err := json.Marshal(header)
	if err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	delete(fields, "alloc")
	data, err = json.Marshal(fields)
	if err != nil {
		return err
	}
	w := &jsonWriter{w: output}
	w.text(string(data[:len(data)-1]))
	w.text(`,"alloc":{`)
	addresses := make([]common.Address, 0, len(genesis.Alloc))
	for address := range genesis.Alloc {
		addresses = append(addresses, address)
	}
	sort.Slice(addresses, func(i, j int) bool { return addresses[i].Cmp(addresses[j]) < 0 })
	for _, address := range addresses {
		w.value(address)
		w.text(":")
		w.value(genesis.Alloc[address])
		w.text(",")
		if w.err != nil {
			return w.err
		}
	}
	w.value(contract)
	w.text(":")
	data, err = json.Marshal(core.GenesisAccount{Code: code, Balance: total, Nonce: 1})
	if err != nil {
		return err
	}
	w.text(string(data[:len(data)-1]))
	w.text(`,"storage":{`)
	if w.err != nil {
		return w.err
	}
	first := true
	if err := walkStorage(&s.metadata, genesis.Config.ChainID, s.Walk, func(key common.Hash, value common.StorageValue64) error {
		if !first {
			w.text(",")
		}
		first = false
		w.value(key)
		w.text(":")
		w.value(value)
		return w.err
	}); err != nil {
		return err
	}
	w.text("}}}}\n")
	return w.err
}

func validateBaseGenesis(genesis *core.Genesis, contract common.Address) error {
	if genesis == nil || genesis.Config == nil {
		return errors.New("genesis has no chain configuration")
	}
	if genesis.Number != 0 {
		return errors.New("base genesis block number must be zero")
	}
	if _, exists := genesis.Alloc[contract]; exists {
		return errors.New("genesis already allocates the migration contract address")
	}
	return validTarget(genesis.Config.ChainID, contract)
}
