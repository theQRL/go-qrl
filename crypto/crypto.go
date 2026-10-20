// Copyright 2014 The go-ethereum Authors
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

package crypto

import (
	"bufio"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"math/big"
	"os"

	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/common/math"
	"github.com/theQRL/go-qrl/rlp"
)

// SignatureLength indicates the byte length required to carry a signature with recovery id.
const SignatureLength = 64 + 1 // 64 bytes ECDSA signature + 1 byte recovery id

// RecoveryIDOffset points to the byte offset within the signature that contains the recovery id.
const RecoveryIDOffset = 64

// DigestLength sets the signature digest exact length
const DigestLength = 32

var secp256k1N = S256().Params().N

var errInvalidPubkey = errors.New("invalid secp256k1 public key")

// EllipticCurve contains curve operations.
type EllipticCurve interface {
	elliptic.Curve

	// Point marshaling/unmarshaing.
	Marshal(x, y *big.Int) []byte
	Unmarshal(data []byte) (x, y *big.Int)
}

// KeccakState wraps sha3.state. In addition to the usual hash methods, it also supports
// Read to get a variable amount of data from the hash state. Read is faster than Sum
// because it doesn't copy the internal state, but also modifies the internal state.
type KeccakState interface {
	hash.Hash
	Read([]byte) (int, error)
}

// HashData hashes the provided data using the KeccakState and returns a 32 byte hash
func HashData(kh KeccakState, data []byte) (h common.Hash) {
	kh.Reset()
	kh.Write(data)
	kh.Read(h[:])
	return h
}

// CreateAddress creates a qrl address given the bytes and the nonce
func CreateAddress(b common.Address, nonce uint64) common.Address {
	data, _ := rlp.EncodeToBytes([]any{b, nonce})
	return keccakToAddress(data)
}

// CreateAddress2 creates a qrl address for a CREATE2 invocation given the
// sender address, a 64-byte salt and the keccak256 hash of the init code.
//
// The salt is 512 bits wide to match the VM stack word. Callers that hold a
// narrower value should left-pad to 64 bytes.
func CreateAddress2(b common.Address, salt [64]byte, inithash []byte) common.Address {
	return keccakToAddress([]byte{0xff}, b.Bytes(), salt[:], inithash)
}

// addressDomain prefixes every Keccak-512 input used to derive a QRL
// address. It isolates address hashes from any other Keccak-512 use in the
// ecosystem so collisions in one protocol can't be carried into another.
var addressDomain = []byte("QRL-ADDR-v1")

// keccakToAddress derives a 64-byte address as
//
//	address = Keccak-512(addressDomain || data...)
//
// Using the full 64-byte digest yields 2^256 collision resistance under the
// birthday bound, matching SHAKE256's maximum security strength.
func keccakToAddress(data ...[]byte) common.Address {
	parts := make([][]byte, 0, len(data)+1)
	parts = append(parts, addressDomain)
	parts = append(parts, data...)
	h := Keccak512(parts...)
	return common.BytesToAddress(h[len(h)-common.AddressLength:])
}

// ToECDSA creates a private key with the given D value.
func ToECDSA(d []byte) (*ecdsa.PrivateKey, error) {
	priv := new(ecdsa.PrivateKey)
	priv.PublicKey.Curve = S256()
	if 8*len(d) != priv.Params().BitSize {
		return nil, fmt.Errorf("invalid length, need %d bits", priv.Params().BitSize)
	}
	priv.D = new(big.Int).SetBytes(d)

	// The priv.D must < N
	if priv.D.Cmp(secp256k1N) >= 0 {
		return nil, errors.New("invalid private key, >=N")
	}
	// The priv.D must not be zero or negative.
	if priv.D.Sign() <= 0 {
		return nil, errors.New("invalid private key, zero or negative")
	}

	priv.PublicKey.X, priv.PublicKey.Y = S256().ScalarBaseMult(d)
	if priv.PublicKey.X == nil {
		return nil, errors.New("invalid private key")
	}
	return priv, nil
}

// FromECDSA exports a private key into a binary dump.
func FromECDSA(priv *ecdsa.PrivateKey) []byte {
	if priv == nil {
		return nil
	}
	return math.PaddedBigBytes(priv.D, priv.Params().BitSize/8)
}

// UnmarshalPubkey converts bytes to a secp256k1 public key.
func UnmarshalPubkey(pub []byte) (*ecdsa.PublicKey, error) {
	x, y := S256().Unmarshal(pub)
	if x == nil {
		return nil, errInvalidPubkey
	}
	if !S256().IsOnCurve(x, y) {
		return nil, errInvalidPubkey
	}
	return &ecdsa.PublicKey{Curve: S256(), X: x, Y: y}, nil
}

// FromECDSAPub converts a secp256k1 public key to bytes.
// Note: it does not use the curve from pub, instead it always
// encodes using secp256k1.
func FromECDSAPub(pub *ecdsa.PublicKey) []byte {
	if pub == nil || pub.X == nil || pub.Y == nil {
		return nil
	}
	return S256().Marshal(pub.X, pub.Y)
}

// HexToECDSA parses a secp256k1 private key.
func HexToECDSA(hexkey string) (*ecdsa.PrivateKey, error) {
	b, err := hex.DecodeString(hexkey)
	if byteErr, ok := err.(hex.InvalidByteError); ok {
		return nil, fmt.Errorf("invalid hex character %q in private key", byte(byteErr))
	} else if err != nil {
		return nil, errors.New("invalid hex data for private key")
	}
	return ToECDSA(b)
}

// LoadECDSA loads a secp256k1 private key from the given file.
func LoadECDSA(file string) (*ecdsa.PrivateKey, error) {
	fd, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer fd.Close()

	r := bufio.NewReader(fd)
	buf := make([]byte, 64)
	n, err := common.ReadASCII(buf, r)
	if err != nil {
		return nil, err
	} else if n != len(buf) {
		return nil, errors.New("key file too short, want 64 hex characters")
	}
	if err := common.CheckKeyFileEnd(r); err != nil {
		return nil, err
	}

	return HexToECDSA(string(buf))
}

// SaveECDSA saves a secp256k1 private key to the given file with
// restrictive permissions. The key data is saved hex-encoded.
func SaveECDSA(file string, key *ecdsa.PrivateKey) error {
	k := hex.EncodeToString(FromECDSA(key))
	return os.WriteFile(file, []byte(k), 0600)
}

// GenerateKey generates a new private key.
func GenerateKey() (*ecdsa.PrivateKey, error) {
	return ecdsa.GenerateKey(S256(), rand.Reader)
}

func zeroBytes(bytes []byte) {
	clear(bytes)
}
