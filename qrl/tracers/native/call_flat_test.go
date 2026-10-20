// Copyright 2024 The go-ethereum Authors
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

package native_test

import (
	"errors"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/core/vm"
	"github.com/theQRL/go-qrl/params"
	"github.com/theQRL/go-qrl/qrl/tracers"
)

func TestCallFlatStop(t *testing.T) {
	tracer, err := tracers.DefaultDirectory.New("flatCallTracer", &tracers.Context{}, nil)
	require.NoError(t, err)

	env := vm.NewQRVM(
		vm.BlockContext{BlockNumber: big.NewInt(0)},
		vm.TxContext{},
		nil,
		params.MainnetChainConfig,
		vm.Config{},
	)
	tracer.CaptureTxStart(0)
	tracer.CaptureStart(env, common.Address{}, common.Address{}, false, nil, 0, big.NewInt(0))

	stopError := errors.New("stop error")
	tracer.Stop(stopError)
	tracer.CaptureTxEnd(0)

	_, tracerError := tracer.GetResult()
	require.Equal(t, stopError, tracerError)
}
