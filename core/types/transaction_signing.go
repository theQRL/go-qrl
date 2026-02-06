// Copyright 2016 The go-ethereum Authors
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

package types

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/theQRL/go-qrllib/wallet/common/wallettype"
	"github.com/theQRL/go-zond/common"
	"github.com/theQRL/go-zond/crypto/pqcrypto"
	"github.com/theQRL/go-zond/crypto/pqcrypto/wallet"
	"github.com/theQRL/go-zond/params"
)

var ErrInvalidChainId = errors.New("invalid chain id for signer")

// sigCache is used to cache the derived sender and contains
// the signer used to derive it.
type sigCache struct {
	signer Signer
	from   common.Address
}

// MakeSigner returns a Signer based on the given chain config and block number.
func MakeSigner(config *params.ChainConfig) Signer {
	return NewShanghaiSigner(config.ChainID)
}

// LatestSigner returns the 'most permissive' Signer available for the given chain
// configuration. Specifically, this enables support of all types of transacrions
// when their respective forks are scheduled to occur at any block number (or time)
// in the chain config.
//
// Use this in transaction-handling code where the current block number is unknown. If you
// have the current block number available, use MakeSigner instead.
func LatestSigner(config *params.ChainConfig) Signer {
	return NewShanghaiSigner(config.ChainID)
}

// LatestSignerForChainID returns the 'most permissive' Signer available. Specifically,
// this enables support for EIP-155 replay protection and all implemented EIP-2718
// transaction types if chainID is non-nil.
//
// Use this in transaction-handling code where the current block number and fork
// configuration are unknown. If you have a ChainConfig, use LatestSigner instead.
// If you have a ChainConfig and know the current block number, use MakeSigner instead.
func LatestSignerForChainID(chainID *big.Int) Signer {
	return NewShanghaiSigner(chainID)
}

// SignTx signs the transaction using the given ML-DSA-87 signer and wallet.
func SignTx(tx *Transaction, s Signer, w wallet.Wallet) (*Transaction, error) {
	// Check that chain ID of tx matches the signer. We also accept ID zero here,
	// because it indicates that the chain ID was not specified in the tx.
	// NOTE(rgeraldes24): chain ID is filled in in the WithSignatureAndPublicKey method
	// below if its not specified in the transaction
	if tx.ChainId().Sign() != 0 && tx.ChainId().Cmp(s.ChainID()) != 0 {
		return nil, fmt.Errorf("%w: have %d want %d", ErrInvalidChainId, tx.ChainId(), s.ChainID())
	}

	desc := w.GetDescriptor().ToBytes()
	extraParams := []byte{}
	h := s.Hash(tx, desc, extraParams)
	sig, err := pqcrypto.Sign(h[:], w)
	if err != nil {
		return nil, err
	}
	pk := w.GetPK()
	return tx.WithAuthValues(s, sig[:], pk[:], desc, extraParams)
}

// SignNewTx creates a transaction and signs it.
func SignNewTx(w wallet.Wallet, s Signer, txdata TxData) (*Transaction, error) {
	tx := NewTx(txdata)
	descBytes := w.GetDescriptor().ToBytes()
	extraParams := []byte{}
	h := s.Hash(tx, descBytes, extraParams)
	sig, err := pqcrypto.Sign(h[:], w)
	if err != nil {
		return nil, err
	}
	pk := w.GetPK()
	return tx.WithAuthValues(s, sig, pk[:], descBytes, extraParams)
}

// MustSignNewTx creates a transaction and signs it.
// This panics if the transaction cannot be signed.
func MustSignNewTx(d wallet.Wallet, s Signer, txdata TxData) *Transaction {
	tx, err := SignNewTx(d, s, txdata)
	if err != nil {
		panic(err)
	}
	return tx
}

// Sender returns the address derived from the public key and an error
// if it failed deriving or upon an incorrect signature.
//
// Sender may cache the address, allowing it to be used regardless of
// signing method. The cache is invalidated if the cached signer does
// not match the signer used in the current call.
func Sender(signer Signer, tx *Transaction) (common.Address, error) {
	if sc := tx.from.Load(); sc != nil {
		sigCache := sc.(sigCache)
		// If the signer used to derive from in a previous
		// call is not the same as used current, invalidate
		// the cache.
		if sigCache.signer.Equal(signer) {
			return sigCache.from, nil
		}
	}

	addr, err := signer.Sender(tx)
	if err != nil {
		return common.Address{}, err
	}
	tx.from.Store(sigCache{signer: signer, from: addr})
	return addr, nil
}

// Signer encapsulates transaction signature handling. The name of this type is slightly
// misleading because Signers don't actually sign, they're just for validating and
// processing of signatures.
//
// Note that this interface is not a stable API and may change at any time to accommodate
// new protocol rules.
type Signer interface {
	// Sender returns the sender address of the transaction.
	Sender(tx *Transaction) (common.Address, error)

	// AuthValues returns the raw signature, publicKey, descriptor and params values
	// corresponding to the given signature.
	AuthValues(tx *Transaction, sig, pk, desc, extraParams []byte) (signature, publicKey, descriptor, params []byte, err error)
	ChainID() *big.Int

	// Hash returns 'signature hash', i.e. the transaction hash that is signed by the
	// private key. This hash does not uniquely identify the transaction.
	Hash(tx *Transaction, descriptor []byte, extraParams []byte) common.Hash

	// Equal returns true if the given signer is the same as the receiver.
	Equal(Signer) bool
}

type ShanghaiSigner struct {
	ChainId *big.Int
}

// NewShangaiSigner returns a signer that accepts
// - EIP-1559 dynamic fee transactions
// - EIP-2930 access list transactions,
// - EIP-155 replay protected transactions
func NewShanghaiSigner(chainId *big.Int) Signer {
	return ShanghaiSigner{chainId}
}

func (s ShanghaiSigner) ChainID() *big.Int {
	return s.ChainId
}

func (s ShanghaiSigner) Sender(tx *Transaction) (common.Address, error) {
	if tx.ChainId().Cmp(s.ChainId) != 0 {
		return common.Address{}, fmt.Errorf("%w: have %d want %d", ErrInvalidChainId, tx.ChainId(), s.ChainId)
	}

	sig, pk, desc, params := tx.RawSignatureValue(), tx.RawPublicKeyValue(), tx.Descriptor(), tx.ExtraParams()

	msg := s.Hash(tx, desc, params)

	pqcryptodesc, err := pqcrypto.BytesToDescriptor(desc)
	if err != nil {
		return common.Address{}, err
	}

	ok := false
	switch pqcryptodesc.Type() {
	case byte(wallettype.ML_DSA_87):
		ok, err = pqcrypto.MLDSA87VerifySignature(sig, msg.Bytes(), pk)
		if err != nil {
			return common.Address{}, err
		}
	default:
		return common.Address{}, fmt.Errorf("unsupported wallet type in descriptor: %v", pqcryptodesc.Type())
	}
	if !ok {
		return common.Address{}, fmt.Errorf("%w: verification failed", pqcrypto.ErrBadSignature)
	}

	return pqcrypto.PublicKeyAndDescriptorToAddress(tx.RawPublicKeyValue(), pqcryptodesc)
}

func (s ShanghaiSigner) Equal(s2 Signer) bool {
	x, ok := s2.(ShanghaiSigner)
	return ok && x.ChainId.Cmp(s.ChainId) == 0
}

func (s ShanghaiSigner) AuthValues(tx *Transaction, sig, pk, desc, extraParams []byte) ([]byte, []byte, []byte, []byte, error) {
	// Check that chain ID of tx matches the signer. We also accept ID zero here,
	// because it indicates that the chain ID was not specified in the tx.
	chainID := tx.inner.chainID()
	if chainID.Sign() != 0 && chainID.Cmp(s.ChainId) != 0 {
		return nil, nil, nil, nil, fmt.Errorf("%w: have %d want %d", ErrInvalidChainId, chainID, s.ChainId)
	}
	return decodeSignature(sig), decodePublicKey(pk), decodeDescriptor(desc), decodeExtraParams(extraParams), nil
}

// Hash returns the hash to be signed by the sender.
// It does not uniquely identify the transaction.
// Hash returns the hash to be signed by the sender.
// It does not uniquely identify the transaction.
func (s ShanghaiSigner) Hash(tx *Transaction, descriptor []byte, extraParams []byte) common.Hash {
	switch tx.Type() {
	case DynamicFeeTxType:
		return prefixedRlpHash(
			tx.Type(),
			[]any{
				s.ChainId,
				tx.Nonce(),
				tx.GasTipCap(),
				tx.GasFeeCap(),
				tx.Gas(),
				tx.To(),
				tx.Value(),
				tx.Data(),
				tx.AccessList(),
				descriptor,
				extraParams,
			})
	default:
		// This _should_ not happen, but in case someone sends in a bad
		// json struct via RPC, it's probably more prudent to return an
		// empty hash instead of killing the node with a panic
		//panic("Unsupported transaction type: %d", tx.typ)
		return common.Hash{}
	}
}

func decodeSignature(sig []byte) (signature []byte) {
	if len(sig) != pqcrypto.MLDSA87SignatureLength {
		panic(fmt.Sprintf("wrong size for ml-dsa-87 signature: got %d, want %d", len(sig), pqcrypto.MLDSA87SignatureLength))
	}
	signature = make([]byte, pqcrypto.MLDSA87SignatureLength)
	copy(signature, sig)
	return signature
}

func decodePublicKey(pk []byte) (publicKey []byte) {
	if len(pk) != pqcrypto.MLDSA87PublicKeyLength {
		panic(fmt.Sprintf("wrong size for ml-dsa-87 publickey: got %d, want %d", len(pk), pqcrypto.MLDSA87PublicKeyLength))
	}
	publicKey = make([]byte, pqcrypto.MLDSA87PublicKeyLength)
	copy(publicKey, pk)
	return publicKey
}

func decodeDescriptor(d []byte) (descriptor []byte) {
	if len(d) != pqcrypto.DescriptorSize {
		panic(fmt.Sprintf("wrong size for descriptor: got %d, want %d", len(d), pqcrypto.DescriptorSize))
	}
	descriptor = make([]byte, pqcrypto.DescriptorSize)
	copy(descriptor, d)
	return descriptor
}

func decodeExtraParams(d []byte) []byte {
	extraParams := make([]byte, len(d))
	copy(extraParams, d)
	return extraParams
}
