// Copyright 2015 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package common

import (
	"bytes"
	"database/sql/driver"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"math/rand"
	"reflect"
	"strconv"

	"github.com/theQRL/go-qrl/common/hexutil"
	"golang.org/x/crypto/sha3"
)

// Lengths of hashes and addresses in bytes.
const (
	// HashLength is the expected length of the hash
	HashLength = 32
	// AddressLength is the expected length of the address
	AddressLength = 64
	// LogTopicLength is the width of a log topic, in bytes.
	LogTopicLength = 64

	// StorageValue64Length is the width of a persistent storage slot value, in
	// bytes. It matches the VM stack word so that a 512-bit value pushed by a
	// contract round-trips through SSTORE/SLOAD without truncation.
	StorageValue64Length = 64
)

var (
	hashT    = reflect.TypeFor[Hash]()
	addressT = reflect.TypeFor[Address]()

	// MaxAddress represents the maximum possible address value.
	MaxAddress = MustParseAddress("Qffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff")

	// MaxHash represents the maximum possible hash value.
	MaxHash = HexToHash("0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff")

	ErrInvalidAddress = errors.New("invalid address")
)

// Hash represents the 32 byte Keccak256 hash of arbitrary data.
type Hash [HashLength]byte

// BytesToHash sets b to hash.
// If b is larger than len(h), b will be cropped from the left.
func BytesToHash(b []byte) Hash {
	var h Hash
	h.SetBytes(b)
	return h
}

// BigToHash sets byte representation of b to hash.
// If b is larger than len(h), b will be cropped from the left.
func BigToHash(b *big.Int) Hash { return BytesToHash(b.Bytes()) }

// HexToHash sets byte representation of s to hash.
// If b is larger than len(h), b will be cropped from the left.
func HexToHash(s string) Hash { return BytesToHash(FromHex(s)) }

// Cmp compares two hashes.
func (h Hash) Cmp(other Hash) int {
	return bytes.Compare(h[:], other[:])
}

// Bytes gets the byte representation of the underlying hash.
func (h Hash) Bytes() []byte { return h[:] }

// Big converts a hash to a big integer.
func (h Hash) Big() *big.Int { return new(big.Int).SetBytes(h[:]) }

// Hex converts a hash to a hex string.
func (h Hash) Hex() string { return hexutil.Encode(h[:]) }

// TerminalString implements log.TerminalStringer, formatting a string for console
// output during logging.
func (h Hash) TerminalString() string {
	return fmt.Sprintf("%x..%x", h[:3], h[29:])
}

// String implements the stringer interface and is used also by the logger when
// doing full logging into a file.
func (h Hash) String() string {
	return h.Hex()
}

// Format implements fmt.Formatter.
// Hash supports the %v, %s, %q, %x, %X and %d format verbs.
func (h Hash) Format(s fmt.State, c rune) {
	hexb := make([]byte, 2+len(h)*2)
	copy(hexb, "0x")
	hex.Encode(hexb[2:], h[:])

	switch c {
	case 'x', 'X':
		if !s.Flag('#') {
			hexb = hexb[2:]
		}
		if c == 'X' {
			hexb = bytes.ToUpper(hexb)
		}
		fallthrough
	case 'v', 's':
		s.Write(hexb)
	case 'q':
		q := []byte{'"'}
		s.Write(q)
		s.Write(hexb)
		s.Write(q)
	case 'd':
		fmt.Fprint(s, ([len(h)]byte)(h))
	default:
		fmt.Fprintf(s, "%%!%c(hash=%x)", c, h)
	}
}

// UnmarshalText parses a hash in hex syntax.
func (h *Hash) UnmarshalText(input []byte) error {
	return hexutil.UnmarshalFixedText("Hash", input, h[:])
}

// UnmarshalJSON parses a hash in hex syntax.
func (h *Hash) UnmarshalJSON(input []byte) error {
	return hexutil.UnmarshalFixedJSON(hashT, input, h[:])
}

// MarshalText returns the hex representation of h.
func (h Hash) MarshalText() ([]byte, error) {
	return hexutil.Bytes(h[:]).MarshalText()
}

// SetBytes sets the hash to the value of b.
// If b is larger than len(h), b will be cropped from the left.
func (h *Hash) SetBytes(b []byte) {
	if len(b) > len(h) {
		b = b[len(b)-HashLength:]
	}

	copy(h[HashLength-len(b):], b)
}

// Generate implements testing/quick.Generator.
func (h Hash) Generate(rand *rand.Rand, size int) reflect.Value {
	m := rand.Intn(len(h))
	for i := len(h) - 1; i > m; i-- {
		h[i] = byte(rand.Uint32())
	}
	return reflect.ValueOf(h)
}

// Scan implements Scanner for database/sql.
func (h *Hash) Scan(src any) error {
	srcB, ok := src.([]byte)
	if !ok {
		return fmt.Errorf("can't scan %T into Hash", src)
	}
	if len(srcB) != HashLength {
		return fmt.Errorf("can't scan []byte of len %d into Hash, want %d", len(srcB), HashLength)
	}
	copy(h[:], srcB)
	return nil
}

// Value implements valuer for database/sql.
func (h Hash) Value() (driver.Value, error) {
	return h[:], nil
}

// ImplementsGraphQLType returns true if Hash implements the specified GraphQL type.
func (Hash) ImplementsGraphQLType(name string) bool { return name == "Bytes32" }

// UnmarshalGraphQL unmarshals the provided GraphQL query data.
func (h *Hash) UnmarshalGraphQL(input any) error {
	var err error
	switch input := input.(type) {
	case string:
		err = h.UnmarshalText([]byte(input))
	default:
		err = fmt.Errorf("unexpected type %T for Hash", input)
	}
	return err
}

// UnprefixedHash allows marshaling a Hash without 0x prefix.
type UnprefixedHash Hash

// UnmarshalText decodes the hash from hex. The 0x prefix is optional.
func (h *UnprefixedHash) UnmarshalText(input []byte) error {
	return hexutil.UnmarshalFixedUnprefixedText("UnprefixedHash", input, h[:])
}

// MarshalText encodes the hash as hex.
func (h UnprefixedHash) MarshalText() ([]byte, error) {
	return []byte(hex.EncodeToString(h[:])), nil
}

/////////// LogTopic

// LogTopic is the 64-byte topic of a contract event. Topics hold ABI-encoded
// indexed event arguments; 64-byte addresses and 512-bit VM words use the full
// width.
type LogTopic [LogTopicLength]byte

// BytesToRightAlignedLogTopic copies b into a LogTopic, right-aligned.
//
// QRVM LOG{0..4} opcodes push a uint512 stack word and serialize it big-endian
// via Bytes64(). A value of N bytes sits in the LOW N bytes of that big-endian
// encoding — the high (LogTopicLength-N) bytes are zero padding. Mirroring that
// layout here keeps raw topic values comparable to on-chain topics.
//
// Inputs longer than LogTopicLength are cropped from the left, keeping the
// last LogTopicLength bytes.
func BytesToRightAlignedLogTopic(b []byte) LogTopic {
	var t LogTopic
	if len(b) > len(t) {
		b = b[len(b)-LogTopicLength:]
	}
	copy(t[LogTopicLength-len(b):], b)
	return t
}

// BytesToLeftAlignedLogTopic copies b into a LogTopic, left-aligned.
//
// ABI bytesN values, including bytes32 Keccak hashes used as event signature
// topics and indexed dynamic-value topics, occupy the HIGH bytes of a VM64 ABI
// word: bytes || zero-padding.
//
// Inputs longer than LogTopicLength are cropped from the right, keeping the
// first LogTopicLength bytes.
func BytesToLeftAlignedLogTopic(b []byte) LogTopic {
	var t LogTopic
	copy(t[:], b)
	return t
}

// ParseLogTopic parses s as a complete serialized topic: a 0x-prefixed hex
// string decoding to exactly LogTopicLength bytes, preserved verbatim without
// re-alignment. Use the alignment helpers to build topics from shorter ABI
// values.
func ParseLogTopic(s string) (LogTopic, error) {
	var t LogTopic
	b, err := hexutil.Decode(s)
	if err != nil {
		return t, err
	}
	if len(b) != LogTopicLength {
		return t, fmt.Errorf("hex has invalid length %d after decoding; expected %d for topic", len(b), LogTopicLength)
	}
	copy(t[:], b)
	return t, nil
}

// MustParseLogTopic calls ParseLogTopic and panics on error.
// It is intended for tests and package-level initializations with hard-coded strings.
func MustParseLogTopic(s string) LogTopic {
	t, err := ParseLogTopic(s)
	if err != nil {
		panic(fmt.Errorf("invalid log topic %q: %w", s, err))
	}
	return t
}

// HashToLogTopic left-aligns a 32-byte hash in a LogTopic (hash || zero-padding),
// since Hyperion treats Keccak hashes as ABI bytes32 values; see
// BytesToLeftAlignedLogTopic for the alignment rationale.
func HashToLogTopic(h Hash) LogTopic {
	return BytesToLeftAlignedLogTopic(h[:])
}

// AddressToLogTopic converts a full-width address to a LogTopic.
//
// VM64 addresses are LogTopicLength bytes and fill the ABI word exactly. The
// conversion (rather than a copy) makes that width match a compile-time
// invariant: this line stops building if the two ever diverge.
func AddressToLogTopic(a Address) LogTopic {
	return LogTopic(a)
}

// Bytes returns a slice view of the topic.
func (t LogTopic) Bytes() []byte { return t[:] }

// Hex returns t as a 0x-prefixed lowercase hex string.
func (t LogTopic) Hex() string { return hexutil.Encode(t[:]) }

// Big returns t interpreted as a big-endian unsigned integer.
func (t LogTopic) Big() *big.Int { return new(big.Int).SetBytes(t[:]) }

// String implements fmt.Stringer.
func (t LogTopic) String() string { return t.Hex() }

// IsZero reports whether t is the zero topic.
func (t LogTopic) IsZero() bool {
	for _, b := range t {
		if b != 0 {
			return false
		}
	}
	return true
}

// MarshalText encodes t as a 0x-prefixed hex string.
func (t LogTopic) MarshalText() ([]byte, error) {
	return hexutil.Bytes(t[:]).MarshalText()
}

// UnmarshalText decodes t from a 0x-prefixed hex string.
func (t *LogTopic) UnmarshalText(input []byte) error {
	return hexutil.UnmarshalFixedText("LogTopic", input, t[:])
}

// UnmarshalJSON decodes t from a JSON-quoted hex string.
func (t *LogTopic) UnmarshalJSON(input []byte) error {
	return hexutil.UnmarshalFixedJSON(reflect.TypeFor[LogTopic](), input, t[:])
}

// ImplementsGraphQLType reports whether LogTopic satisfies the Bytes64 scalar.
func (LogTopic) ImplementsGraphQLType(name string) bool { return name == "Bytes64" }

// UnmarshalGraphQL decodes t from a 0x-prefixed hex string supplied by a GraphQL query.
func (t *LogTopic) UnmarshalGraphQL(input any) error {
	s, ok := input.(string)
	if !ok {
		return fmt.Errorf("unexpected type %T for Bytes64", input)
	}
	return t.UnmarshalText([]byte(s))
}

/////////// StorageValue64

// StorageValue64 is the 64-byte value of a persistent storage slot. Slot keys
// remain 32 bytes (they are Keccak-256 hashes produced by contracts) but a
// value can hold a full 512-bit VM word — most importantly the 64-byte
// address type, which does not fit in 32 bytes.
type StorageValue64 [StorageValue64Length]byte

// BytesToStorageValue64 copies b into a StorageValue64, right-aligned. If b is
// longer than StorageValue64Length it is cropped from the left.
func BytesToStorageValue64(b []byte) StorageValue64 {
	var v StorageValue64
	v.SetBytes(b)
	return v
}

// HexToStorageValue64 parses a hex string (with or without 0x prefix) into a
// StorageValue64.
func HexToStorageValue64(s string) StorageValue64 { return BytesToStorageValue64(FromHex(s)) }

// Bytes returns a copy of v's bytes.
func (v StorageValue64) Bytes() []byte { return v[:] }

// Hex returns v as a 0x-prefixed lowercase hex string.
func (v StorageValue64) Hex() string { return hexutil.Encode(v[:]) }

// Big returns v interpreted as a big-endian unsigned integer.
func (v StorageValue64) Big() *big.Int { return new(big.Int).SetBytes(v[:]) }

// String implements fmt.Stringer.
func (v StorageValue64) String() string { return v.Hex() }

// IsZero reports whether v is the zero value.
func (v StorageValue64) IsZero() bool {
	for _, b := range v {
		if b != 0 {
			return false
		}
	}
	return true
}

// SetBytes copies b into v, right-aligned. If b is longer than
// StorageValue64Length the most-significant bytes are dropped.
func (v *StorageValue64) SetBytes(b []byte) {
	if len(b) > len(v) {
		b = b[len(b)-StorageValue64Length:]
	}
	// Zero first to avoid leaving stale bytes in the MSB region.
	for i := range v {
		v[i] = 0
	}
	copy(v[StorageValue64Length-len(b):], b)
}

// MarshalText encodes v as a 0x-prefixed hex string.
func (v StorageValue64) MarshalText() ([]byte, error) {
	return hexutil.Bytes(v[:]).MarshalText()
}

// UnmarshalText decodes v from a 0x-prefixed hex string.
func (v *StorageValue64) UnmarshalText(input []byte) error {
	return hexutil.UnmarshalFixedText("StorageValue64", input, v[:])
}

// UnmarshalJSON decodes v from a JSON-quoted hex string.
func (v *StorageValue64) UnmarshalJSON(input []byte) error {
	return hexutil.UnmarshalFixedJSON(reflect.TypeFor[StorageValue64](), input, v[:])
}

// ImplementsGraphQLType reports whether StorageValue64 satisfies the Bytes64 scalar.
func (StorageValue64) ImplementsGraphQLType(name string) bool { return name == "Bytes64" }

// UnmarshalGraphQL decodes v from a 0x-prefixed hex string supplied by a GraphQL query.
func (v *StorageValue64) UnmarshalGraphQL(input any) error {
	s, ok := input.(string)
	if !ok {
		return fmt.Errorf("unexpected type %T for Bytes64", input)
	}
	return v.UnmarshalText([]byte(s))
}

/////////// Address

// Address represents the 64 byte address of a QRL account.
type Address [AddressLength]byte

// BytesToAddress returns Address with value b.
// If b is larger than len(h), b will be cropped from the left.
func BytesToAddress(b []byte) Address {
	var a Address
	a.SetBytes(b)
	return a
}

// BigToAddress returns Address with byte values of b.
// If b is larger than len(h), b will be cropped from the left.
func BigToAddress(b *big.Int) Address { return BytesToAddress(b.Bytes()) }

// NewAddressFromString returns Address with byte values of s.
func NewAddressFromString(hexaddr string) (Address, error) {
	if !IsAddress(hexaddr) {
		return Address{}, ErrInvalidAddress
	}
	rawAddr, _ := hex.DecodeString(hexaddr[1:])
	return BytesToAddress(rawAddr), nil
}

// MustParseAddress calls NewAddressFromString and panics on error.
// It is intended for tests and package-level initializations with hard-coded strings.
func MustParseAddress(hexaddr string) Address {
	addr, err := NewAddressFromString(hexaddr)
	if err != nil {
		panic(fmt.Errorf("invalid QRL address %q: %w", hexaddr, err))
	}
	return addr
}

// IsAddress verifies whether a string can represent a valid hex-encoded
// QRL address or not.
func IsAddress(s string) bool {
	if !hasQPrefix(s) {
		return false
	}
	s = s[1:]

	return len(s) == 2*AddressLength && isHex(s)
}

// IsHexEncodedHash verifies whether a string can represent a valid hex-encoded
// 32-byte hash, with or without 0x prefix.
func IsHexEncodedHash(s string) bool {
	if has0xPrefix(s) {
		s = s[2:]
	}
	return len(s) == 2*HashLength && isHex(s)
}

// Cmp compares two addresses.
func (a Address) Cmp(other Address) int {
	return bytes.Compare(a[:], other[:])
}

// Bytes gets the string representation of the underlying address.
func (a Address) Bytes() []byte { return a[:] }

// Big converts an address to a big integer.
func (a Address) Big() *big.Int { return new(big.Int).SetBytes(a[:]) }

// Hex returns the canonical QIP-55 mixed-case representation of the address.
func (a Address) Hex() string {
	return qip55AddressHex(a[:])
}

// String implements fmt.Stringer.
func (a Address) String() string {
	return a.Hex()
}

func qip55AddressHex(addr []byte) string {
	lower := make([]byte, 1+len(addr)*2)
	copy(lower[:1], hexutil.PrefixQ)
	hex.Encode(lower[1:], addr)

	var checksum [AddressLength]byte
	h := sha3.NewShake256()
	_, _ = h.Write(lower[1:])
	_, _ = h.Read(checksum[:])

	out := make([]byte, len(lower))
	copy(out, lower)
	for i := 1; i < len(out); i++ {
		c := out[i]
		if c < 'a' || c > 'f' {
			continue
		}
		nibble := checksum[(i-1)/2]
		if (i-1)%2 == 0 {
			nibble >>= 4
		} else {
			nibble &= 0x0f
		}
		if nibble >= 8 {
			out[i] = c - ('a' - 'A')
		}
	}
	return string(out)
}

func (a Address) hex() []byte {
	var buf [len(a)*2 + 1]byte
	copy(buf[:1], hexutil.PrefixQ)
	hex.Encode(buf[1:], a[:])
	return buf[:]
}

// Format implements fmt.Formatter.
// Address supports the %v, %s, %q, %x, %X and %d format verbs.
func (a Address) Format(s fmt.State, c rune) {
	switch c {
	case 'v', 's':
		s.Write([]byte(a.Hex()))
	case 'q':
		fmt.Fprintf(s, "%q", a.Hex())
	case 'x', 'X':
		hex := a.hex()
		if !s.Flag('#') {
			hex = hex[1:]
		}
		if c == 'X' {
			hex = bytes.ToUpper(hex)
		}
		s.Write(hex)
	case 'd':
		fmt.Fprint(s, ([len(a)]byte)(a))
	default:
		fmt.Fprintf(s, "%%!%c(address=%x)", c, a)
	}
}

// SetBytes sets the address to the value of b.
// If b is larger than len(a), b will be cropped from the left.
func (a *Address) SetBytes(b []byte) {
	if len(b) > len(a) {
		b = b[len(b)-AddressLength:]
	}
	clear(a[:])
	copy(a[AddressLength-len(b):], b)
}

// MarshalText returns the canonical QIP-55 representation of a.
func (a Address) MarshalText() ([]byte, error) {
	return []byte(a.Hex()), nil
}

// UnmarshalText parses a hash in hex syntax.
func (a *Address) UnmarshalText(input []byte) error {
	return hexutil.UnmarshalFixedTextQ("Address", input, a[:])
}

// UnmarshalJSON parses a address in hex syntax.
func (a *Address) UnmarshalJSON(input []byte) error {
	return hexutil.UnmarshalFixedJSONQ(addressT, input, a[:])
}

// Scan implements Scanner for database/sql.
func (a *Address) Scan(src any) error {
	srcB, ok := src.([]byte)
	if !ok {
		return fmt.Errorf("can't scan %T into Address", src)
	}
	if len(srcB) != AddressLength {
		return fmt.Errorf("can't scan []byte of len %d into Address, want %d", len(srcB), AddressLength)
	}
	copy(a[:], srcB)
	return nil
}

// Value implements valuer for database/sql.
func (a Address) Value() (driver.Value, error) {
	return a[:], nil
}

// ImplementsGraphQLType returns true if Address implements the specified GraphQL type.
func (a Address) ImplementsGraphQLType(name string) bool { return name == "Address" }

// UnmarshalGraphQL unmarshals the provided GraphQL query data.
func (a *Address) UnmarshalGraphQL(input any) error {
	var err error
	switch input := input.(type) {
	case string:
		err = a.UnmarshalText([]byte(input))
	default:
		err = fmt.Errorf("unexpected type %T for Address", input)
	}
	return err
}

// MixedcaseAddress retains the original string and can validate its QIP-55
// checksum casing.
type MixedcaseAddress struct {
	addr     Address
	original string
}

// NewMixedcaseAddress constructor (mainly for testing)
func NewMixedcaseAddress(addr Address) MixedcaseAddress {
	return MixedcaseAddress{addr: addr, original: addr.Hex()}
}

// NewMixedcaseAddressFromString is mainly meant for unit-testing
func NewMixedcaseAddressFromString(hexaddr string) (*MixedcaseAddress, error) {
	if !IsAddress(hexaddr) {
		return nil, ErrInvalidAddress
	}
	rawAddr, _ := hex.DecodeString(hexaddr[1:])
	return &MixedcaseAddress{addr: BytesToAddress(rawAddr), original: hexaddr}, nil
}

// MustParseMixedcaseAddress calls NewMixedcaseAddressFromString and panics on
// error. It is intended for tests and package-level initializations with
// hard-coded strings.
func MustParseMixedcaseAddress(hexaddr string) *MixedcaseAddress {
	addr, err := NewMixedcaseAddressFromString(hexaddr)
	if err != nil {
		panic(fmt.Errorf("invalid QRL mixed-case address %q: %w", hexaddr, err))
	}
	return addr
}

// UnmarshalJSON parses MixedcaseAddress
func (ma *MixedcaseAddress) UnmarshalJSON(input []byte) error {
	if err := hexutil.UnmarshalFixedJSONQ(addressT, input, ma.addr[:]); err != nil {
		return err
	}
	return json.Unmarshal(input, &ma.original)
}

// MarshalJSON marshals the original value
func (ma MixedcaseAddress) MarshalJSON() ([]byte, error) {
	return json.Marshal(ma.original)
}

// Address returns the address
func (ma *MixedcaseAddress) Address() Address {
	return ma.addr
}

// String implements fmt.Stringer
func (ma *MixedcaseAddress) String() string {
	return ma.original
}

// ValidChecksum returns true if the original string matches the canonical
// QIP-55 checksum casing for the underlying address.
func (ma *MixedcaseAddress) ValidChecksum() bool {
	if ma == nil || !IsAddress(ma.original) {
		return false
	}
	return ma.original == ma.addr.Hex()
}

// Original returns the mixed-case input string
func (ma *MixedcaseAddress) Original() string {
	return ma.original
}

type Decimal uint64

func isString(input []byte) bool {
	return len(input) >= 2 && input[0] == '"' && input[len(input)-1] == '"'
}

// UnmarshalJSON parses a hash in hex syntax.
func (d *Decimal) UnmarshalJSON(input []byte) error {
	if !isString(input) {
		return &json.UnmarshalTypeError{Value: "non-string", Type: reflect.TypeFor[uint64]()}
	}
	if i, err := strconv.ParseInt(string(input[1:len(input)-1]), 10, 64); err == nil {
		*d = Decimal(i)
		return nil
	} else {
		return err
	}
}

type PrettyBytes []byte

// TerminalString implements log.TerminalStringer, formatting a string for console
// output during logging.
func (b PrettyBytes) TerminalString() string {
	if len(b) < 7 {
		return fmt.Sprintf("%x", b)
	}
	return fmt.Sprintf("%#x...%x (%dB)", b[:3], b[len(b)-3:], len(b))
}
