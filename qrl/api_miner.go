// Copyright 2023 The go-ethereum Authors
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

package qrl

import (
	"math/big"

	"github.com/theQRL/go-qrl/common/hexutil"
)

// MinerAPI provides an API to control the miner.
type MinerAPI struct {
	q *QRL
}

// NewMinerAPI create a new MinerAPI instance.
func NewMinerAPI(q *QRL) *MinerAPI {
	return &MinerAPI{q}
}

// SetExtra sets the extra data string that is included when this miner mines a block.
func (api *MinerAPI) SetExtra(extra string) (bool, error) {
	if err := api.q.Miner().SetExtra([]byte(extra)); err != nil {
		return false, err
	}
	return true, nil
}

// SetGasPrice sets the minimum accepted gas price for the miner.
func (api *MinerAPI) SetGasPrice(gasPrice hexutil.Big) bool {
	api.q.lock.Lock()
	api.q.gasPrice = (*big.Int)(&gasPrice)
	api.q.lock.Unlock()

	api.q.txPool.SetGasTip((*big.Int)(&gasPrice))
	api.q.Miner().SetGasTip((*big.Int)(&gasPrice))
	return true
}

// SetGasLimit sets the gaslimit to target towards during mining.
func (api *MinerAPI) SetGasLimit(gasLimit hexutil.Uint64) bool {
	api.q.Miner().SetGasCeil(uint64(gasLimit))
	return true
}
