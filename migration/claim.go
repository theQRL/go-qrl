package migration

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"strings"

	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/common/hexutil"
	"github.com/theQRL/go-qrl/core/vm"
)

// Signature contains public information only. This tooling never receives a
// wallet seed or private key and never chooses or advances a wallet's OTS index.
type Signature struct {
	PublicKey string `json:"public_key"`
	Signature string `json:"signature"`
}

func HexBytes(s string) ([]byte, error) {
	s = strings.TrimPrefix(s, "0x")
	if len(s)%2 != 0 {
		return nil, errors.New("hex input has odd length")
	}
	return hex.DecodeString(s)
}

func (a Account) OTSUsed(index uint32) bool {
	i := sort.Search(len(a.UsedOTS), func(i int) bool { return a.UsedOTS[i].Last >= index })
	return i < len(a.UsedOTS) && a.UsedOTS[i].First <= index
}

type ClaimTransaction struct {
	ChainID   string         `json:"chain_id"`
	To        common.Address `json:"to"`
	Value     string         `json:"value"`
	Data      hexutil.Bytes  `json:"data"`
	Source    string         `json:"source"`
	Recipient common.Address `json:"recipient"`
	Amount    string         `json:"amount_base_units"`
}

// PrepareClaim verifies each supplied signature against the frozen snapshot and
// builds calldata. The contract independently rechecks it against on-chain state,
// including OTS indices consumed since genesis. A relayer can pay transaction gas.
func PrepareClaim(s *Snapshot, chainID *big.Int, contract common.Address, source string, recipient common.Address, signatures []Signature) (*ClaimTransaction, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	return prepareClaim(s, func(address string) (Account, error) { return memoryAccount(s, address) }, chainID, contract, source, recipient, signatures)
}

func (s *SnapshotStore) PrepareClaim(chainID *big.Int, contract common.Address, source string, recipient common.Address, signatures []Signature) (*ClaimTransaction, error) {
	return prepareClaim(&s.metadata, s.Account, chainID, contract, source, recipient, signatures)
}

func memoryAccount(s *Snapshot, address string) (Account, error) {
	i := sort.Search(len(s.Accounts), func(i int) bool { return s.Accounts[i].Address >= address })
	if i == len(s.Accounts) || s.Accounts[i].Address != address {
		return Account{}, fmt.Errorf("account %s is absent from snapshot", address)
	}
	return s.Accounts[i], nil
}

func prepareClaim(s *Snapshot, lookup func(string) (Account, error), chainID *big.Int, contract common.Address, source string, recipient common.Address, signatures []Signature) (*ClaimTransaction, error) {
	allocation, err := lookup(source)
	if err != nil {
		return nil, err
	}
	message, err := claimMessage(s, allocation, chainID, contract, source, recipient)
	if err != nil {
		return nil, err
	}
	weights := make(map[string]uint32)
	for _, owner := range allocation.Signatories {
		weights[owner.Address] = owner.Weight
	}
	type verified struct {
		id      *big.Int
		pk, sig []byte
	}
	var entries []verified
	seen := make(map[string]bool)
	var votes uint64
	for _, supplied := range signatures {
		pk, err := HexBytes(supplied.PublicKey)
		if err != nil {
			return nil, err
		}
		address, err := AddressFromPublicKey(pk)
		if err != nil {
			return nil, err
		}
		name := "Q" + hex.EncodeToString(address)
		if seen[name] {
			return nil, errors.New("duplicate signatory")
		}
		seen[name] = true
		owner, err := lookup(name)
		if err != nil || owner.Kind != "xmss" {
			return nil, errors.New("XMSS signer is absent from snapshot")
		}
		if allocation.Kind == "xmss" && name != source {
			return nil, errors.New("public key does not own source address")
		}
		if allocation.Kind == "multisig" && weights[name] == 0 {
			return nil, errors.New("public key is not a multisig signatory")
		}
		sig, err := HexBytes(supplied.Signature)
		if err != nil {
			return nil, err
		}
		if len(sig) < 4 {
			return nil, errors.New("signature is too short")
		}
		index := binary.BigEndian.Uint32(sig)
		if uint64(index) >= uint64(1)<<owner.Height || owner.OTSUsed(index) {
			return nil, fmt.Errorf("%s: OTS index %d is unavailable in snapshot", name, index)
		}
		input := binary.BigEndian.AppendUint32(nil, uint32(len(message)))
		input = append(input, message...)
		input = append(input, sig...)
		input = append(input, pk...)
		precompile := vm.PrecompiledContractsZond[common.BytesToAddress([]byte{7})]
		result, _, err := vm.RunPrecompiledContract(precompile, input, precompile.RequiredGas(input))
		if err != nil || len(result) != vm.WordBytes || new(big.Int).SetBytes(result).Cmp(big.NewInt(1)) != 0 {
			return nil, fmt.Errorf("%s: invalid signature for this exact claim", name)
		}
		votes += uint64(weights[name])
		entries = append(entries, verified{AccountID(address), pk, sig})
	}
	if allocation.Kind == "xmss" && len(entries) != 1 {
		return nil, errors.New("XMSS claim requires exactly one signature")
	}
	if allocation.Kind == "multisig" && (len(entries) == 0 || votes < uint64(allocation.Threshold)) {
		return nil, errors.New("insufficient multisig signature weight")
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].id.Cmp(entries[j].id) < 0 })
	_, contractABI, _, err := Contract()
	if err != nil {
		return nil, err
	}
	var data []byte
	if allocation.Kind == "xmss" {
		data, err = contractABI.Pack("claim", entries[0].pk, entries[0].sig, recipient)
	} else {
		var keys, sigs [][]byte
		for _, entry := range entries {
			keys = append(keys, entry.pk)
			sigs = append(sigs, entry.sig)
		}
		address, _ := LegacyAddress(source)
		data, err = contractABI.Pack("claimMultisig", address, keys, sigs, recipient)
	}
	if err != nil {
		return nil, err
	}
	return &ClaimTransaction{chainID.String(), contract, "0x0", data, source, recipient, allocation.BalanceBaseUnits}, nil
}
