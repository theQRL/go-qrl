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

package vm

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	gomath "math"
	"math/big"

	pkgerrors "github.com/pkg/errors"
	ssz "github.com/prysmaticlabs/fastssz"
	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/common/math"
	"github.com/theQRL/go-qrl/crypto/pqcrypto"
	"github.com/theQRL/go-qrl/params"
	cryptomldsa87 "github.com/theQRL/go-qrllib/crypto/ml_dsa_87"
	cryptoxmss "github.com/theQRL/go-qrllib/crypto/xmss"
)

// PrecompiledContract is the basic interface for native Go contracts. The implementation
// requires a deterministic gas count based on the input size of the Run method of the
// contract.
type PrecompiledContract interface {
	RequiredGas(input []byte) uint64  // RequiredPrice calculates the contract gas use
	Run(input []byte) ([]byte, error) // Run runs the precompiled contract
}

// trueWord is returned when a precompile verification succeeds.
var trueWord = common.LeftPadBytes([]byte{1}, WordBytes)

// PrecompiledContractsZond contains the default set of pre-compiled QRL
// contracts used in the Zond release.
var PrecompiledContractsZond = map[common.Address]PrecompiledContract{
	common.BytesToAddress([]byte{1}): &depositroot{},
	common.BytesToAddress([]byte{2}): &sha256hash{},
	common.BytesToAddress([]byte{3}): &mldsa87Verify{},
	common.BytesToAddress([]byte{4}): &dataCopy{},
	common.BytesToAddress([]byte{5}): &bigModExp{},
	common.BytesToAddress([]byte{7}): &xmssVerify{},
}

var (
	PrecompiledAddressesZond []common.Address
)

func init() {
	for k := range PrecompiledContractsZond {
		PrecompiledAddressesZond = append(PrecompiledAddressesZond, k)
	}
}

// ActivePrecompiles returns the precompiles enabled from Zond genesis. The
// current protocol has no fork-dependent precompile sets.
func ActivePrecompiles() []common.Address {
	return PrecompiledAddressesZond
}

// RunPrecompiledContract runs and evaluates the output of a precompiled contract.
// It returns
// - the returned bytes,
// - the _remaining_ gas,
// - any error that occurred
func RunPrecompiledContract(p PrecompiledContract, input []byte, suppliedGas uint64) (ret []byte, remainingGas uint64, err error) {
	gasCost := p.RequiredGas(input)
	if suppliedGas < gasCost {
		return nil, 0, ErrOutOfGas
	}
	suppliedGas -= gasCost
	output, err := p.Run(input)
	return output, suppliedGas, err
}

type depositroot struct{}

// The depositroot precompile input is the DepositData fields concatenated
// without padding, in SSZ field order:
//
//	pubkey(2592) || withdrawal_recipient(64) || amount(8, little-endian) ||
//	randao_commitment(32) || signature(4627)
//
// randao_commitment is the top layer of the validator's RANDAO hash onion
// (sha256 chain); the beacon chain stores it in the validator record and
// checks each RANDAO reveal against it.
const (
	depositPublicKeyLength           = pqcrypto.MLDSA87PublicKeyLength
	depositWithdrawalRecipientLength = common.AddressLength
	depositAmountLength              = 8
	depositRandaoCommitmentLength    = common.HashLength
	depositSignatureLength           = pqcrypto.MLDSA87SignatureLength
	depositPublicKeyOffset           = 0
	depositWithdrawalRecipientOffset = depositPublicKeyOffset + depositPublicKeyLength
	depositAmountOffset              = depositWithdrawalRecipientOffset + depositWithdrawalRecipientLength
	depositRandaoCommitmentOffset    = depositAmountOffset + depositAmountLength
	depositSignatureOffset           = depositRandaoCommitmentOffset + depositRandaoCommitmentLength
	depositInputLength               = depositSignatureOffset + depositSignatureLength
)

func (c *depositroot) RequiredGas(input []byte) uint64 {
	return params.DepositrootGas
}

func (c *depositroot) Run(input []byte) ([]byte, error) {
	var (
		pkBytes                  = getData(input, depositPublicKeyOffset, depositPublicKeyLength)
		withdrawalRecipientBytes = getData(input, depositWithdrawalRecipientOffset, depositWithdrawalRecipientLength)
		amountBytes              = getData(input, depositAmountOffset, depositAmountLength)
		randaoCommitmentBytes    = getData(input, depositRandaoCommitmentOffset, depositRandaoCommitmentLength)
		sigBytes                 = getData(input, depositSignatureOffset, depositSignatureLength)
	)

	var amountUint uint64
	buf := bytes.NewReader(amountBytes)
	err := binary.Read(buf, binary.LittleEndian, &amountUint)
	if err != nil {
		return nil, err
	}

	data := &depositdata{
		PublicKey:           pkBytes,
		WithdrawalRecipient: withdrawalRecipientBytes,
		Amount:              amountUint,
		RandaoCommitment:    randaoCommitmentBytes,
		Signature:           sigBytes,
	}
	h, err := data.HashTreeRoot()
	if err != nil {
		return nil, pkgerrors.Wrap(err, "could not hash tree root deposit data item")
	}

	return h[:], nil
}

const (
	mldsa87VerifyDigestOffset        = 0
	mldsa87VerifyPublicKeyOffset     = mldsa87VerifyDigestOffset + common.HashLength
	mldsa87VerifySignatureOffset     = mldsa87VerifyPublicKeyOffset + cryptomldsa87.CRYPTO_PUBLIC_KEY_BYTES
	mldsa87VerifyContextLengthOffset = mldsa87VerifySignatureOffset + cryptomldsa87.CRYPTO_BYTES
	mldsa87VerifyContextOffset       = mldsa87VerifyContextLengthOffset + 1
	mldsa87VerifyMinInputLength      = mldsa87VerifyContextOffset
	mldsa87VerifyMaxContextLength    = 255
)

// mldsa87Verify verifies an ML-DSA-87 signature over a fixed-size digest using
// the supplied public key and context.
type mldsa87Verify struct{}

func (*mldsa87Verify) RequiredGas([]byte) uint64 {
	return params.MLDSA87VerifyGas
}

func (*mldsa87Verify) Run(input []byte) ([]byte, error) {
	if len(input) < mldsa87VerifyMinInputLength {
		return nil, nil
	}

	context := input[mldsa87VerifyContextOffset:]
	if len(context) != int(input[mldsa87VerifyContextLengthOffset]) {
		return nil, nil
	}

	digest := input[mldsa87VerifyDigestOffset:mldsa87VerifyPublicKeyOffset]
	publicKeyBytes := input[mldsa87VerifyPublicKeyOffset:mldsa87VerifySignatureOffset]

	// The key comes straight from calldata, so it never passes through the
	// wallet-layer constructor. ParsePublicKey is the only way to obtain a
	// key Verify accepts, and it rejects a weak key (one under which the
	// FIPS 204 primitive, which by spec does not validate keys, would accept
	// a signature anyone can compute), so the check holds by construction.
	publicKey, err := cryptomldsa87.ParsePublicKey(publicKeyBytes)
	if err != nil {
		return nil, nil
	}

	signatureBytes := input[mldsa87VerifySignatureOffset:mldsa87VerifyContextLengthOffset]
	signature := [cryptomldsa87.CRYPTO_BYTES]byte(signatureBytes)

	if !cryptomldsa87.Verify(context, digest, signature, publicKey) {
		return nil, nil
	}

	return trueWord, nil
}

// xmssVerify verifies an XMSS signature produced by a legacy QRL wallet with
// the semantics of qrllib's XmssBase::verify (theQRL/qrllib, WOTS+ w = 16), so
// that signatures of keys generated by those wallets can be checked from a
// contract (Hyperion's xmssverify builtin).
//
// The input is the packed encoding
//
//	message_length(4, big-endian) || message || signature || extended_pk(67)
//
// where extended_pk is the qrllib extended public key (3-byte QRL descriptor ||
// 32-byte root || 32-byte public seed) and signature has 2180 + 32 * height
// bytes (4-byte leaf index || 32-byte randomness R || 67 * 32-byte WOTS+
// signature || height * 32-byte authentication path). The key is the trailing
// 67 bytes and the message is delimited by its length prefix, so the signature
// is whatever lies between them. The output is trueWord when the signature
// verifies and empty otherwise; malformed input yields the empty output,
// exactly where qrllib returns false or rejects an argument.
type xmssVerify struct{}

const (
	xmssVerifyMessageLengthSize = 4
	xmssVerifyDigestSize        = 32
	xmssVerifyDescriptorSize    = 3
	// xmssVerifyExtendedPKSize is qrllib's extended public key: descriptor || root || public seed.
	xmssVerifyExtendedPKSize = xmssVerifyDescriptorSize + 2*xmssVerifyDigestSize
	xmssVerifyMinInputLength = xmssVerifyMessageLengthSize + xmssVerifyExtendedPKSize
	xmssVerifyIndexSize      = 4
	xmssVerifyWOTSChains     = 67 // WOTS+ chains for n = 32, w = 16
	xmssVerifyWOTSSteps      = 15 // at most w - 1 chaining steps per chain
	xmssVerifyLTreeNodes     = 66 // nodes hashed while compressing the 67 chains into one leaf
	// xmssVerifySignatureBaseSize is the signature without its authentication path:
	// leaf index || R || WOTS+ signature.
	xmssVerifySignatureBaseSize = xmssVerifyIndexSize + xmssVerifyDigestSize + xmssVerifyWOTSChains*xmssVerifyDigestSize
	xmssVerifyMinHeight         = 4
	xmssVerifyMaxHeight         = cryptoxmss.MaxHeight

	// Hash invocations of one verification, charged by RequiredGas: the chaining
	// function hash_f is three hashes (two PRFs and the keyed hash) and the tree
	// node function hash_h is four (three PRFs and the keyed hash), plus one H_msg
	// over the message, whose input starts with toByte(type) || R || root || index.
	xmssVerifyFixedHashes          = xmssVerifyWOTSChains*xmssVerifyWOTSSteps*3 + xmssVerifyLTreeNodes*4
	xmssVerifyHashesPerLevel       = 4
	xmssVerifyMessageHashKeyLength = 4 * xmssVerifyDigestSize
)

// RequiredGas prices every hash the verification performs like a SHA256
// precompile call over its at most 160-byte (three-word) input, for the worst
// case number of WOTS+ chaining steps, plus the message hash. The tree height is
// read from the key descriptor (the maximum height if it is malformed) and the
// message length from the prefix, capped by the input actually supplied.
func (*xmssVerify) RequiredGas(input []byte) uint64 {
	height := uint64(xmssVerifyMaxHeight)
	if len(input) >= xmssVerifyExtendedPKSize {
		descriptorHeight := uint64(input[len(input)-xmssVerifyExtendedPKSize+1]&0x0f) << 1
		if descriptorHeight >= xmssVerifyMinHeight && descriptorHeight <= xmssVerifyMaxHeight {
			height = descriptorHeight
		}
	}
	var messageLength uint64
	if len(input) >= xmssVerifyMessageLengthSize {
		messageLength = min(uint64(binary.BigEndian.Uint32(input)), uint64(len(input)-xmssVerifyMessageLengthSize))
	}
	hashes := uint64(xmssVerifyFixedHashes) + xmssVerifyHashesPerLevel*height
	return params.XMSSVerifyHashGas*hashes +
		params.Sha256BaseGas + params.Sha256PerWordGas*toWordSize(xmssVerifyMessageHashKeyLength+messageLength)
}

func (*xmssVerify) Run(input []byte) ([]byte, error) {
	if len(input) < xmssVerifyMinInputLength {
		return nil, nil
	}
	messageEnd := uint64(xmssVerifyMessageLengthSize) + uint64(binary.BigEndian.Uint32(input))
	keyStart := uint64(len(input) - xmssVerifyExtendedPKSize)
	if messageEnd > keyStart {
		return nil, nil
	}
	if !verifyLegacyXMSS(input[xmssVerifyMessageLengthSize:messageEnd], input[messageEnd:keyStart], input[keyStart:]) {
		return nil, nil
	}
	return trueWord, nil
}

// verifyLegacyXMSS mirrors qrllib's XmssBase::verify together with the argument
// validation it performs (QRLDescriptor::fromExtendedPK, XmssValidation and
// xmss_Verifysig): every input qrllib rejects, by throwing or by returning false,
// is false here. The checks are spelled out instead of delegating to go-qrllib's
// wallet layer, which accepts a non-zero reserved descriptor byte, any address
// format, tree height 2 and a leaf index beyond the tree, all of which qrllib
// refuses.
func verifyLegacyXMSS(message, signature, extendedPK []byte) bool {
	if len(extendedPK) != xmssVerifyExtendedPKSize {
		return false
	}
	if len(signature) > xmssVerifySignatureBaseSize+xmssVerifyMaxHeight*xmssVerifyDigestSize {
		return false
	}

	// QRL descriptor: hash function in the low and signature type in the high
	// nibble of byte 0, height / 2 in the low and address format in the high
	// nibble of byte 1, and a reserved zero byte 2.
	if extendedPK[2] != 0 {
		return false
	}
	hashFunction, err := cryptoxmss.ToHashFunction(extendedPK[0] & 0x0f)
	if err != nil {
		return false
	}
	if extendedPK[1]>>4 != 0 { // SHA256_2X is the only address format
		return false
	}
	// Signature type 0 is XMSS; type 1 is a QRL multi-sig address, which has no
	// tree behind it and nothing to verify. Everything else is unknown.
	if extendedPK[0]>>4 != 0 {
		return false
	}
	height := int(extendedPK[1]&0x0f) << 1
	if height < xmssVerifyMinHeight || height > xmssVerifyMaxHeight {
		return false
	}

	// The signature size determines the height, which has to agree with the descriptor.
	if len(signature) < xmssVerifySignatureBaseSize || (len(signature)-xmssVerifyIndexSize)%xmssVerifyDigestSize != 0 {
		return false
	}
	if (len(signature)-xmssVerifySignatureBaseSize)/xmssVerifyDigestSize != height {
		return false
	}

	// The leaf index has to address a leaf of the tree.
	if index := binary.BigEndian.Uint32(signature); index >= uint32(1)<<height {
		return false
	}

	return cryptoxmss.Verify(hashFunction, message, signature, extendedPK[xmssVerifyDescriptorSize:])
}

// depositdata mirrors the beacon chain's DepositData SSZ container
// (qrysm proto Deposit_Data): field order and sizes must match exactly, since
// the root produced here is compared against the root the depositor signed.
type depositdata struct {
	PublicKey           []byte
	WithdrawalRecipient []byte
	Amount              uint64
	RandaoCommitment    []byte
	Signature           []byte
}

// HashTreeRoot ssz hashes the Deposit_Data object
func (d *depositdata) HashTreeRoot() ([32]byte, error) {
	return ssz.HashWithDefaultHasher(d)
}

// HashTreeRootWith ssz hashes the Deposit_Data object with a hasher
func (d *depositdata) HashTreeRootWith(hh *ssz.Hasher) (err error) {
	indx := hh.Index()

	// Field (0) 'Pubkey'
	if size := len(d.PublicKey); size != depositPublicKeyLength {
		err = ssz.ErrBytesLengthFn("--.Pubkey", size, depositPublicKeyLength)
		return
	}
	hh.PutBytes(d.PublicKey)

	// Field (1) 'WithdrawalRecipient'
	if size := len(d.WithdrawalRecipient); size != depositWithdrawalRecipientLength {
		err = ssz.ErrBytesLengthFn("--.WithdrawalRecipient", size, depositWithdrawalRecipientLength)
		return
	}
	hh.PutBytes(d.WithdrawalRecipient)

	// Field (2) 'Amount'
	hh.PutUint64(d.Amount)

	// Field (3) 'RandaoCommitment'
	if size := len(d.RandaoCommitment); size != depositRandaoCommitmentLength {
		err = ssz.ErrBytesLengthFn("--.RandaoCommitment", size, depositRandaoCommitmentLength)
		return
	}
	hh.PutBytes(d.RandaoCommitment)

	// Field (4) 'Signature'
	if size := len(d.Signature); size != depositSignatureLength {
		err = ssz.ErrBytesLengthFn("--.Signature", size, depositSignatureLength)
		return
	}
	hh.PutBytes(d.Signature)

	hh.Merkleize(indx)
	return
}

// SHA256 implemented as a native contract.
type sha256hash struct{}

// RequiredGas returns the gas required to execute the pre-compiled contract.
//
// This method does not require any overflow checking as the input size gas costs
// required for anything significant is so high it's impossible to pay for.
func (c *sha256hash) RequiredGas(input []byte) uint64 {
	return toWordSize(uint64(len(input)))*params.Sha256PerWordGas + params.Sha256BaseGas
}
func (c *sha256hash) Run(input []byte) ([]byte, error) {
	h := sha256.Sum256(input)
	return h[:], nil
}

// data copy implemented as a native contract.
type dataCopy struct{}

// RequiredGas returns the gas required to execute the pre-compiled contract.
//
// This method does not require any overflow checking as the input size gas costs
// required for anything significant is so high it's impossible to pay for.
func (c *dataCopy) RequiredGas(input []byte) uint64 {
	return toWordSize(uint64(len(input)))*params.IdentityPerWordGas + params.IdentityBaseGas
}
func (c *dataCopy) Run(in []byte) ([]byte, error) {
	return common.CopyBytes(in), nil
}

// bigModExp implements a native big integer exponential modular operation.
type bigModExp struct{}

var (
	big0  = big.NewInt(0)
	big1  = big.NewInt(1)
	big3  = big.NewInt(3)
	big7  = big.NewInt(7)
	big8  = big.NewInt(8)
	big32 = big.NewInt(32)
)

// RequiredGas returns the gas required to execute the pre-compiled contract.
func (c *bigModExp) RequiredGas(input []byte) uint64 {
	var (
		baseLen = new(big.Int).SetBytes(getData(input, 0, 32))
		expLen  = new(big.Int).SetBytes(getData(input, 32, 32))
		modLen  = new(big.Int).SetBytes(getData(input, 64, 32))
	)
	if len(input) > 96 {
		input = input[96:]
	} else {
		input = input[:0]
	}
	// Retrieve the head 32 bytes of exp for the adjusted exponent length
	var expHead *big.Int
	if big.NewInt(int64(len(input))).Cmp(baseLen) <= 0 {
		expHead = new(big.Int)
	} else {
		if expLen.Cmp(big32) > 0 {
			expHead = new(big.Int).SetBytes(getData(input, baseLen.Uint64(), 32))
		} else {
			expHead = new(big.Int).SetBytes(getData(input, baseLen.Uint64(), expLen.Uint64()))
		}
	}
	// Calculate the adjusted exponent length
	var msb int
	if bitlen := expHead.BitLen(); bitlen > 0 {
		msb = bitlen - 1
	}
	adjExpLen := new(big.Int)
	if expLen.Cmp(big32) > 0 {
		adjExpLen.Sub(expLen, big32)
		adjExpLen.Mul(big8, adjExpLen)
	}
	adjExpLen.Add(adjExpLen, big.NewInt(int64(msb)))
	// Calculate the gas cost of the operation
	gas := new(big.Int).Set(math.BigMax(modLen, baseLen))

	// EIP-2565 has three changes
	// 1. Different multComplexity (inlined here)
	// in EIP-2565 (https://eips.ethereum.org/EIPS/eip-2565):
	//
	// def mult_complexity(x):
	//    ceiling(x/8)^2
	//
	//where is x is max(length_of_MODULUS, length_of_BASE)
	gas = gas.Add(gas, big7)
	gas = gas.Div(gas, big8)
	gas.Mul(gas, gas)

	gas.Mul(gas, math.BigMax(adjExpLen, big1))
	// 2. Different divisor (`GQUADDIVISOR`) (3)
	gas.Div(gas, big3)
	if gas.BitLen() > 64 {
		return gomath.MaxUint64
	}
	// 3. Minimum price of 200 gas
	if gas.Uint64() < 200 {
		return 200
	}
	return gas.Uint64()
}

func (c *bigModExp) Run(input []byte) ([]byte, error) {
	var (
		baseLen = new(big.Int).SetBytes(getData(input, 0, 32)).Uint64()
		expLen  = new(big.Int).SetBytes(getData(input, 32, 32)).Uint64()
		modLen  = new(big.Int).SetBytes(getData(input, 64, 32)).Uint64()
	)
	if len(input) > 96 {
		input = input[96:]
	} else {
		input = input[:0]
	}
	// Handle a special case when both the base and mod length is zero
	if baseLen == 0 && modLen == 0 {
		return []byte{}, nil
	}
	// Retrieve the operands and execute the exponentiation
	var (
		base = new(big.Int).SetBytes(getData(input, 0, baseLen))
		exp  = new(big.Int).SetBytes(getData(input, baseLen, expLen))
		mod  = new(big.Int).SetBytes(getData(input, baseLen+expLen, modLen))
		v    []byte
	)
	switch {
	case mod.BitLen() == 0:
		// Modulo 0 is undefined, return zero
		return common.LeftPadBytes([]byte{}, int(modLen)), nil
	case base.BitLen() == 1: // a bit length of 1 means it's 1 (or -1).
		//If base == 1, then we can just return base % mod (if mod >= 1, which it is)
		v = base.Mod(base, mod).Bytes()
	default:
		v = base.Exp(base, exp, mod).Bytes()
	}
	return common.LeftPadBytes(v, int(modLen)), nil
}
