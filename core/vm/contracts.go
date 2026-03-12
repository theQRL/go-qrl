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
	"github.com/theQRL/go-qrl/params"
)

// PrecompiledContract is the basic interface for native Go contracts. The implementation
// requires a deterministic gas count based on the input size of the Run method of the
// contract.
type PrecompiledContract interface {
	RequiredGas(input []byte) uint64  // RequiredPrice calculates the contract gas use
	Run(input []byte) ([]byte, error) // Run runs the precompiled contract
}

// PrecompiledContractsZond contains the default set of pre-compiled QRL
// contracts used in the Zond release.
var PrecompiledContractsZond = map[common.Address]PrecompiledContract{
	common.BytesToAddress([]byte{1}): &depositroot{},
	common.BytesToAddress([]byte{2}): &sha256hash{},
	common.BytesToAddress([]byte{4}): &dataCopy{},
	common.BytesToAddress([]byte{5}): &bigModExp{},
}

var (
	PrecompiledAddressesZond []common.Address
)

func init() {
	for k := range PrecompiledContractsZond {
		PrecompiledAddressesZond = append(PrecompiledAddressesZond, k)
	}
}

// ActivePrecompiles returns the precompiles enabled with the current configuration.
func ActivePrecompiles(rules params.Rules) []common.Address {
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

// TODO(now.youtrack.cloud/issue/TGZ-5)
func (c *depositroot) RequiredGas(input []byte) uint64 {
	// NOTE(rgeraldes): number of sha256 ops below does not include the number of zero
	// hashes done; these are calculated during the prysmaticlabs/fastssz lib init
	// 238 sha256 ops + 64 bytes input per op
	// return uint64(64+31)/32*params.Sha256PerWordGas + params.Sha256BaseGas
	return (2*params.Sha256PerWordGas + params.Sha256BaseGas) * 238
}

func (c *depositroot) Run(input []byte) ([]byte, error) {
	var (
		pkBytes     = getData(input, 0, 2592)    // 2592 bytes
		credsBytes  = getData(input, 2592, 32)   // 32 bytes
		amountBytes = getData(input, 2624, 8)    // 8 bytes
		sigBytes    = getData(input, 2632, 4627) // 4627 bytes
	)

	var amountUint uint64
	buf := bytes.NewReader(amountBytes)
	err := binary.Read(buf, binary.LittleEndian, &amountUint)
	if err != nil {
		return nil, err
	}

	data := &depositdata{
		PublicKey:             pkBytes,
		WithdrawalCredentials: credsBytes,
		Amount:                amountUint,
		Signature:             sigBytes,
	}
	h, err := data.HashTreeRoot()
	if err != nil {
		return nil, pkgerrors.Wrap(err, "could not hash tree root deposit data item")
	}

	return h[:], nil
}

type depositdata struct {
	PublicKey             []byte
	WithdrawalCredentials []byte
	Amount                uint64
	Signature             []byte
}

// HashTreeRoot ssz hashes the Deposit_Data object
func (d *depositdata) HashTreeRoot() ([32]byte, error) {
	return ssz.HashWithDefaultHasher(d)
}

// HashTreeRootWith ssz hashes the Deposit_Data object with a hasher
func (d *depositdata) HashTreeRootWith(hh *ssz.Hasher) (err error) {
	indx := hh.Index()

	// Field (0) 'Pubkey'
	if size := len(d.PublicKey); size != 2592 {
		err = ssz.ErrBytesLengthFn("--.Pubkey", size, 2592)
		return
	}
	hh.PutBytes(d.PublicKey)

	// Field (1) 'WithdrawalCredentials'
	if size := len(d.WithdrawalCredentials); size != 32 {
		err = ssz.ErrBytesLengthFn("--.WithdrawalCredentials", size, 32)
		return
	}
	hh.PutBytes(d.WithdrawalCredentials)

	// Field (2) 'Amount'
	hh.PutUint64(d.Amount)

	// Field (3) 'Signature'
	if size := len(d.Signature); size != 4627 {
		err = ssz.ErrBytesLengthFn("--.Signature", size, 4627)
		return
	}
	hh.PutBytes(d.Signature)

	if ssz.EnableVectorizedHTR {
		hh.MerkleizeVectorizedHTR(indx)
	} else {
		hh.Merkleize(indx)
	}
	return
}

// SHA256 implemented as a native contract.
type sha256hash struct{}

// RequiredGas returns the gas required to execute the pre-compiled contract.
//
// This method does not require any overflow checking as the input size gas costs
// required for anything significant is so high it's impossible to pay for.
func (c *sha256hash) RequiredGas(input []byte) uint64 {
	return uint64(len(input)+31)/32*params.Sha256PerWordGas + params.Sha256BaseGas
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
	return uint64(len(input)+31)/32*params.IdentityPerWordGas + params.IdentityBaseGas
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
