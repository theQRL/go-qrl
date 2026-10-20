// Package migration exports the frozen legacy QRL state and builds a genesis
// allocation for XMSS-authenticated redemption on QRVM.
package migration

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
)

const (
	Format             = "qrl-xmss-migration-v1"
	MainnetGenesisHash = "2a1c4a9433f1de36f8b99c7c5aceb7bd2eb39e1ead648ea58227d399ad84c724"
	BalanceScale       = 1_000_000_000
)

// OTSRange includes both endpoints. Ranges are ordered, disjoint and coalesced.
type OTSRange struct {
	First uint32 `json:"first"`
	Last  uint32 `json:"last"`
}

type Signatory struct {
	Address string `json:"address"`
	Weight  uint32 `json:"weight"`
}

type Account struct {
	Address          string      `json:"address"`
	Kind             string      `json:"kind"`
	BalanceShor      string      `json:"balance_shor"`
	Quanta           string      `json:"quanta"`
	BalanceBaseUnits string      `json:"balance_base_units"`
	Height           uint32      `json:"height,omitempty"`
	UsedOTSCount     uint64      `json:"used_ots_count,omitempty"`
	UsedOTS          []OTSRange  `json:"used_ots_ranges,omitempty"`
	Signatories      []Signatory `json:"signatories,omitempty"`
	Threshold        uint32      `json:"threshold,omitempty"`
	// Legacy consistency hint; not needed in the exported manifest because the
	// complete bitmap is exported and committed instead.
	fullyUsedPages uint64
}

type Snapshot struct {
	Format                string `json:"format"`
	Network               string `json:"network"`
	StateVersion          uint32 `json:"state_version"`
	Height                uint64 `json:"height"`
	BlockHash             string `json:"block_hash"`
	GenesisHash           string `json:"genesis_hash"`
	TotalBalanceShor      string `json:"total_balance_shor"`
	TotalBalanceBaseUnits string `json:"total_balance_base_units"`
	// Legacy issuance does not decrease when protocol rules clear balances.
	// Report the difference without recreating those absent wallet funds.
	RecordedCoinSupplyShor string `json:"recorded_coin_supply_shor"`
	SupplyGapShor          string `json:"supply_gap_shor"`
	// The 32-byte zero coinbase account is unmined supply, not a wallet. It is
	// accounted for explicitly but never made into a redeemable entitlement.
	UnminedReserveShor string    `json:"unmined_reserve_shor"`
	Accounts           []Account `json:"accounts"`
	Commitment         string    `json:"commitment"`
}

func parseAmount(s string) (*big.Int, error) {
	n, ok := new(big.Int).SetString(s, 10)
	if !ok || n.Sign() < 0 || n.String() != s {
		return nil, fmt.Errorf("invalid canonical unsigned amount %q", s)
	}
	return n, nil
}

func newAccount(address []byte, balance uint64) Account {
	n := new(big.Int).SetUint64(balance)
	return Account{Address: "Q" + hex.EncodeToString(address), BalanceShor: n.String(),
		Quanta:           new(big.Rat).SetFrac(n, big.NewInt(1e9)).FloatString(9),
		BalanceBaseUnits: new(big.Int).Mul(n, big.NewInt(BalanceScale)).String()}
}

func LegacyAddress(text string) ([]byte, error) {
	if len(text) != 79 || text[0] != 'Q' {
		return nil, errors.New("legacy address must be Q followed by 78 lowercase hex digits")
	}
	b, err := hex.DecodeString(text[1:])
	if err != nil || hex.EncodeToString(b) != text[1:] {
		return nil, errors.New("noncanonical legacy address")
	}
	if err := validateAddress(b); err != nil {
		return nil, err
	}
	return b, nil
}

func validateAddress(b []byte) error {
	if len(b) != 39 {
		return errors.New("legacy address must contain 39 bytes")
	}
	h := sha256.Sum256(b[:35])
	if !bytes.Equal(h[28:], b[35:]) {
		return errors.New("invalid legacy address checksum")
	}
	if bytes.Equal(b[:3], []byte{0x11, 0, 0}) {
		return nil
	}
	height := uint32(b[1]&15) * 2
	if b[0] > 2 || b[1]>>4 != 0 || b[2] != 0 || height < 4 || height > 30 {
		return fmt.Errorf("unsupported legacy address descriptor %x", b[:3])
	}
	return nil
}

func AddressFromPublicKey(pk []byte) ([]byte, error) {
	if len(pk) != 67 {
		return nil, errors.New("XMSS extended public key must contain 67 bytes")
	}
	h := sha256.Sum256(pk)
	b := append(append([]byte{}, pk[:3]...), h[:]...)
	checksum := sha256.Sum256(b)
	b = append(b, checksum[28:]...)
	if err := validateAddress(b); err != nil {
		return nil, err
	}
	if b[0]>>4 != 0 {
		return nil, errors.New("not an XMSS public key")
	}
	return b, nil
}

func AccountID(address []byte) *big.Int {
	h := sha256.Sum256(address)
	return new(big.Int).SetBytes(h[:])
}

func (s Snapshot) digest() string {
	h := sha256.New()
	_ = writeCanonicalSnapshot(h, &s, s.Accounts == nil, func(visit func(Account) error) error {
		for _, a := range s.Accounts {
			if err := visit(a); err != nil {
				return err
			}
		}
		return nil
	}, "")
	return hex.EncodeToString(h.Sum(nil))
}

func (s *Snapshot) seal() { s.Commitment = s.digest() }

// Validate fails closed on ambiguous state, missing multisig signers, altered
// manifests and unit conversion errors before any genesis funding is created.
func (s *Snapshot) Validate() error {
	if s.Commitment != s.digest() {
		return errors.New("snapshot commitment mismatch")
	}
	total := new(big.Int)
	for i, a := range s.Accounts {
		if i > 0 && s.Accounts[i-1].Address >= a.Address {
			return errors.New("accounts must be sorted without duplicates")
		}
		if err := validateAccount(a); err != nil {
			return err
		}
		balance, _ := parseAmount(a.BalanceShor)
		total.Add(total, balance)
	}
	for _, a := range s.Accounts {
		if err := validateSignatories(a, func(address string) (Account, error) {
			return memoryAccount(s, address)
		}); err != nil {
			return err
		}
	}
	return s.validateTotals(total)
}

func (s *Snapshot) validateTotals(total *big.Int) error {
	if s.Format != Format || s.Network != "qrl-mainnet" || s.StateVersion != 3 || s.GenesisHash != MainnetGenesisHash {
		return errors.New("unsupported snapshot format, network, genesis, or state version")
	}
	if b, err := hex.DecodeString(s.BlockHash); err != nil || len(b) != 32 || hex.EncodeToString(b) != s.BlockHash {
		return errors.New("invalid snapshot block hash")
	}
	if reserve, err := parseAmount(s.UnminedReserveShor); err != nil || !reserve.IsUint64() {
		return errors.New("invalid unmined reserve")
	}
	if total.String() != s.TotalBalanceShor || new(big.Int).Mul(total, big.NewInt(BalanceScale)).String() != s.TotalBalanceBaseUnits {
		return errors.New("snapshot balance totals do not reconcile")
	}
	supply, err := parseAmount(s.RecordedCoinSupplyShor)
	if err != nil || !supply.IsUint64() || supply.Cmp(total) < 0 {
		return errors.New("invalid recorded coin supply")
	}
	if new(big.Int).Sub(supply, total).String() != s.SupplyGapShor {
		return errors.New("snapshot supply gap does not reconcile")
	}
	return nil
}

func validateAccount(a Account) error {
	addr, err := LegacyAddress(a.Address)
	if err != nil {
		return fmt.Errorf("%s: %w", a.Address, err)
	}
	balance, err := parseAmount(a.BalanceShor)
	if err != nil || !balance.IsUint64() {
		return fmt.Errorf("%s: invalid legacy balance", a.Address)
	}
	converted := new(big.Int).Mul(balance, big.NewInt(BalanceScale))
	if a.BalanceBaseUnits != converted.String() || a.Quanta != new(big.Rat).SetFrac(balance, big.NewInt(1e9)).FloatString(9) {
		return fmt.Errorf("%s: incorrect balance conversion", a.Address)
	}
	switch a.Kind {
	case "xmss":
		if addr[0]>>4 != 0 || a.Height != uint32(addr[1]&15)*2 || len(a.Signatories) != 0 || a.Threshold != 0 {
			return errors.New("invalid XMSS account metadata")
		}
		var count uint64
		for j, r := range a.UsedOTS {
			if r.First > r.Last || uint64(r.Last) >= uint64(1)<<a.Height || (j > 0 && uint64(r.First) <= uint64(a.UsedOTS[j-1].Last)+1) {
				return errors.New("invalid or noncanonical OTS ranges")
			}
			count += uint64(r.Last) - uint64(r.First) + 1
		}
		if count != a.UsedOTSCount {
			return errors.New("OTS count does not match bitfield")
		}
	case "multisig":
		if !bytes.Equal(addr[:3], []byte{0x11, 0, 0}) || a.Height != 0 || a.UsedOTSCount != 0 || len(a.UsedOTS) != 0 || len(a.Signatories) == 0 {
			return errors.New("invalid multisig metadata")
		}
	default:
		return fmt.Errorf("unsupported account kind %q", a.Kind)
	}
	return nil
}

func validateSignatories(a Account, lookup func(string) (Account, error)) error {
	var votes uint64
	seen := make(map[string]bool)
	for _, signer := range a.Signatories {
		owner, err := lookup(signer.Address)
		if err != nil {
			return fmt.Errorf("read multisig signatory %s: %w", signer.Address, err)
		}
		if owner.Kind != "xmss" || signer.Weight == 0 || seen[signer.Address] {
			return errors.New("invalid, duplicate, or missing multisig signatory")
		}
		seen[signer.Address] = true
		votes += uint64(signer.Weight)
	}
	if a.Kind == "multisig" && votes < uint64(a.Threshold) {
		return errors.New("unreachable multisig threshold")
	}
	return nil
}
