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

package qrl

import (
	"context"
	"errors"
	"math/big"
	"time"

	qrl "github.com/theQRL/go-qrl"
	"github.com/theQRL/go-qrl/accounts"
	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/consensus"
	"github.com/theQRL/go-qrl/core"
	"github.com/theQRL/go-qrl/core/bloombits"
	"github.com/theQRL/go-qrl/core/rawdb"
	"github.com/theQRL/go-qrl/core/state"
	"github.com/theQRL/go-qrl/core/txpool"
	"github.com/theQRL/go-qrl/core/types"
	"github.com/theQRL/go-qrl/core/vm"
	"github.com/theQRL/go-qrl/event"
	"github.com/theQRL/go-qrl/params"
	"github.com/theQRL/go-qrl/qrl/gasprice"
	"github.com/theQRL/go-qrl/qrl/tracers"
	"github.com/theQRL/go-qrl/qrldb"
	"github.com/theQRL/go-qrl/rpc"
)

// QRLAPIBackend implements qrlapi.Backend for full nodes
type QRLAPIBackend struct {
	extRPCEnabled bool
	qrl           *QRL
	gpo           *gasprice.Oracle
}

// ChainConfig returns the active chain configuration.
func (b *QRLAPIBackend) ChainConfig() *params.ChainConfig {
	return b.qrl.blockchain.Config()
}

func (b *QRLAPIBackend) CurrentBlock() *types.Header {
	return b.qrl.blockchain.CurrentBlock()
}

func (b *QRLAPIBackend) SetHead(number uint64) {
	b.qrl.handler.downloader.Cancel()
	b.qrl.blockchain.SetHead(number)
}

func (b *QRLAPIBackend) HeaderByNumber(ctx context.Context, number rpc.BlockNumber) (*types.Header, error) {
	// Pending block is only known by the miner
	if number == rpc.PendingBlockNumber {
		block, _, _ := b.qrl.miner.Pending()
		if block == nil {
			return nil, errors.New("pending block is not available")
		}
		return block.Header(), nil
	}
	// Otherwise resolve and return the block
	if number == rpc.LatestBlockNumber {
		return b.qrl.blockchain.CurrentBlock(), nil
	}
	if number == rpc.FinalizedBlockNumber {
		block := b.qrl.blockchain.CurrentFinalBlock()
		if block == nil {
			return nil, errors.New("finalized block not found")
		}
		return block, nil
	}
	if number == rpc.SafeBlockNumber {
		block := b.qrl.blockchain.CurrentSafeBlock()
		if block == nil {
			return nil, errors.New("safe block not found")
		}
		return block, nil
	}
	return b.qrl.blockchain.GetHeaderByNumber(uint64(number)), nil
}

func (b *QRLAPIBackend) HeaderByNumberOrHash(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (*types.Header, error) {
	if blockNr, ok := blockNrOrHash.Number(); ok {
		return b.HeaderByNumber(ctx, blockNr)
	}
	if hash, ok := blockNrOrHash.Hash(); ok {
		header := b.qrl.blockchain.GetHeaderByHash(hash)
		if header == nil {
			return nil, errors.New("header for hash not found")
		}
		if blockNrOrHash.RequireCanonical && b.qrl.blockchain.GetCanonicalHash(header.Number.Uint64()) != hash {
			return nil, errors.New("hash is not currently canonical")
		}
		return header, nil
	}
	return nil, errors.New("invalid arguments; neither block nor hash specified")
}

func (b *QRLAPIBackend) HeaderByHash(ctx context.Context, hash common.Hash) (*types.Header, error) {
	return b.qrl.blockchain.GetHeaderByHash(hash), nil
}

func (b *QRLAPIBackend) BlockByNumber(ctx context.Context, number rpc.BlockNumber) (*types.Block, error) {
	// Pending block is only known by the miner
	if number == rpc.PendingBlockNumber {
		block, _, _ := b.qrl.miner.Pending()
		if block == nil {
			return nil, errors.New("pending block is not available")
		}
		return block, nil
	}
	// Otherwise resolve and return the block
	if number == rpc.LatestBlockNumber {
		header := b.qrl.blockchain.CurrentBlock()
		return b.qrl.blockchain.GetBlock(header.Hash(), header.Number.Uint64()), nil
	}
	if number == rpc.FinalizedBlockNumber {
		header := b.qrl.blockchain.CurrentFinalBlock()
		if header == nil {
			return nil, errors.New("finalized block not found")
		}
		return b.qrl.blockchain.GetBlock(header.Hash(), header.Number.Uint64()), nil
	}
	if number == rpc.SafeBlockNumber {
		header := b.qrl.blockchain.CurrentSafeBlock()
		if header == nil {
			return nil, errors.New("safe block not found")
		}
		return b.qrl.blockchain.GetBlock(header.Hash(), header.Number.Uint64()), nil
	}
	return b.qrl.blockchain.GetBlockByNumber(uint64(number)), nil
}

func (b *QRLAPIBackend) BlockByHash(ctx context.Context, hash common.Hash) (*types.Block, error) {
	return b.qrl.blockchain.GetBlockByHash(hash), nil
}

// GetBody returns body of a block. It does not resolve special block numbers.
func (b *QRLAPIBackend) GetBody(ctx context.Context, hash common.Hash, number rpc.BlockNumber) (*types.Body, error) {
	if number < 0 || hash == (common.Hash{}) {
		return nil, errors.New("invalid arguments; expect hash and no special block numbers")
	}
	if body := b.qrl.blockchain.GetBody(hash); body != nil {
		return body, nil
	}
	return nil, errors.New("block body not found")
}

func (b *QRLAPIBackend) BlockByNumberOrHash(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (*types.Block, error) {
	if blockNr, ok := blockNrOrHash.Number(); ok {
		return b.BlockByNumber(ctx, blockNr)
	}
	if hash, ok := blockNrOrHash.Hash(); ok {
		header := b.qrl.blockchain.GetHeaderByHash(hash)
		if header == nil {
			return nil, errors.New("header for hash not found")
		}
		if blockNrOrHash.RequireCanonical && b.qrl.blockchain.GetCanonicalHash(header.Number.Uint64()) != hash {
			return nil, errors.New("hash is not currently canonical")
		}
		block := b.qrl.blockchain.GetBlock(hash, header.Number.Uint64())
		if block == nil {
			return nil, errors.New("header found, but block body is missing")
		}
		return block, nil
	}
	return nil, errors.New("invalid arguments; neither block nor hash specified")
}

func (b *QRLAPIBackend) Pending() (*types.Block, types.Receipts, *state.StateDB) {
	return b.qrl.miner.Pending()
}

func (b *QRLAPIBackend) StateAndHeaderByNumber(ctx context.Context, number rpc.BlockNumber) (*state.StateDB, *types.Header, error) {
	// Pending state is only known by the miner
	if number == rpc.PendingBlockNumber {
		block, _, state := b.qrl.miner.Pending()
		if block == nil || state == nil {
			return nil, nil, errors.New("pending state is not available")
		}
		return state, block.Header(), nil
	}
	// Otherwise resolve the block number and return its state
	header, err := b.HeaderByNumber(ctx, number)
	if err != nil {
		return nil, nil, err
	}
	if header == nil {
		return nil, nil, errors.New("header not found")
	}
	stateDb, err := b.qrl.BlockChain().StateAt(header.Root)
	if err != nil {
		return nil, nil, err
	}
	return stateDb, header, nil
}

func (b *QRLAPIBackend) StateAndHeaderByNumberOrHash(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (*state.StateDB, *types.Header, error) {
	if blockNr, ok := blockNrOrHash.Number(); ok {
		return b.StateAndHeaderByNumber(ctx, blockNr)
	}
	if hash, ok := blockNrOrHash.Hash(); ok {
		header, err := b.HeaderByHash(ctx, hash)
		if err != nil {
			return nil, nil, err
		}
		if header == nil {
			return nil, nil, errors.New("header for hash not found")
		}
		if blockNrOrHash.RequireCanonical && b.qrl.blockchain.GetCanonicalHash(header.Number.Uint64()) != hash {
			return nil, nil, errors.New("hash is not currently canonical")
		}
		stateDb, err := b.qrl.BlockChain().StateAt(header.Root)
		if err != nil {
			return nil, nil, err
		}
		return stateDb, header, nil
	}
	return nil, nil, errors.New("invalid arguments; neither block nor hash specified")
}

func (b *QRLAPIBackend) GetReceipts(ctx context.Context, hash common.Hash) (types.Receipts, error) {
	return b.qrl.blockchain.GetReceiptsByHash(hash), nil
}

func (b *QRLAPIBackend) GetLogs(ctx context.Context, hash common.Hash, number uint64) ([][]*types.Log, error) {
	return rawdb.ReadLogs(b.qrl.chainDb, hash, number), nil
}

func (b *QRLAPIBackend) GetQRVM(ctx context.Context, msg *core.Message, state *state.StateDB, header *types.Header, vmConfig *vm.Config, blockCtx *vm.BlockContext) *vm.QRVM {
	if vmConfig == nil {
		vmConfig = b.qrl.blockchain.GetVMConfig()
	}
	txContext := core.NewQRVMTxContext(msg)
	var context vm.BlockContext
	if blockCtx != nil {
		context = *blockCtx
	} else {
		context = core.NewQRVMBlockContext(header, b.qrl.BlockChain(), nil)
	}
	return vm.NewQRVM(context, txContext, state, b.ChainConfig(), *vmConfig)
}

func (b *QRLAPIBackend) SubscribeRemovedLogsEvent(ch chan<- core.RemovedLogsEvent) event.Subscription {
	return b.qrl.BlockChain().SubscribeRemovedLogsEvent(ch)
}

func (b *QRLAPIBackend) SubscribeChainEvent(ch chan<- core.ChainEvent) event.Subscription {
	return b.qrl.BlockChain().SubscribeChainEvent(ch)
}

func (b *QRLAPIBackend) SubscribeChainHeadEvent(ch chan<- core.ChainHeadEvent) event.Subscription {
	return b.qrl.BlockChain().SubscribeChainHeadEvent(ch)
}

func (b *QRLAPIBackend) SubscribeChainSideEvent(ch chan<- core.ChainSideEvent) event.Subscription {
	return b.qrl.BlockChain().SubscribeChainSideEvent(ch)
}

func (b *QRLAPIBackend) SubscribeLogsEvent(ch chan<- []*types.Log) event.Subscription {
	return b.qrl.BlockChain().SubscribeLogsEvent(ch)
}

func (b *QRLAPIBackend) SendTx(ctx context.Context, signedTx *types.Transaction) error {
	return b.qrl.txPool.Add([]*types.Transaction{signedTx}, true, false)[0]
}

func (b *QRLAPIBackend) GetPoolTransactions() (types.Transactions, error) {
	pending := b.qrl.txPool.Pending(txpool.PendingFilter{})
	var txs types.Transactions
	for _, batch := range pending {
		for _, lazy := range batch {
			if tx := lazy.Resolve(); tx != nil {
				txs = append(txs, tx)
			}
		}
	}
	return txs, nil
}

func (b *QRLAPIBackend) GetPoolTransaction(hash common.Hash) *types.Transaction {
	return b.qrl.txPool.Get(hash)
}

func (b *QRLAPIBackend) GetTransaction(ctx context.Context, txHash common.Hash) (*types.Transaction, common.Hash, uint64, uint64, error) {
	tx, blockHash, blockNumber, index := rawdb.ReadTransaction(b.qrl.ChainDb(), txHash)
	return tx, blockHash, blockNumber, index, nil
}

func (b *QRLAPIBackend) GetPoolNonce(ctx context.Context, addr common.Address) (uint64, error) {
	return b.qrl.txPool.Nonce(addr), nil
}

func (b *QRLAPIBackend) Stats() (runnable int, blocked int) {
	return b.qrl.txPool.Stats()
}

func (b *QRLAPIBackend) TxPoolContent() (map[common.Address][]*types.Transaction, map[common.Address][]*types.Transaction) {
	return b.qrl.txPool.Content()
}

func (b *QRLAPIBackend) TxPoolContentFrom(addr common.Address) ([]*types.Transaction, []*types.Transaction) {
	return b.qrl.txPool.ContentFrom(addr)
}

func (b *QRLAPIBackend) TxPool() *txpool.TxPool {
	return b.qrl.txPool
}

func (b *QRLAPIBackend) SubscribeNewTxsEvent(ch chan<- core.NewTxsEvent) event.Subscription {
	return b.qrl.txPool.SubscribeTransactions(ch)
}

func (b *QRLAPIBackend) SyncProgress() qrl.SyncProgress {
	return b.qrl.Downloader().Progress()
}

func (b *QRLAPIBackend) SuggestGasTipCap(ctx context.Context) (*big.Int, error) {
	return b.gpo.SuggestTipCap(ctx)
}

func (b *QRLAPIBackend) FeeHistory(ctx context.Context, blockCount uint64, lastBlock rpc.BlockNumber, rewardPercentiles []float64) (firstBlock *big.Int, reward [][]*big.Int, baseFee []*big.Int, gasUsedRatio []float64, err error) {
	return b.gpo.FeeHistory(ctx, blockCount, lastBlock, rewardPercentiles)
}

func (b *QRLAPIBackend) ChainDb() qrldb.Database {
	return b.qrl.ChainDb()
}

func (b *QRLAPIBackend) EventMux() *event.TypeMux {
	return b.qrl.EventMux()
}

func (b *QRLAPIBackend) AccountManager() *accounts.Manager {
	return b.qrl.AccountManager()
}

func (b *QRLAPIBackend) ExtRPCEnabled() bool {
	return b.extRPCEnabled
}

func (b *QRLAPIBackend) RPCGasCap() uint64 {
	return b.qrl.config.RPCGasCap
}

func (b *QRLAPIBackend) RPCQRVMTimeout() time.Duration {
	return b.qrl.config.RPCQRVMTimeout
}

func (b *QRLAPIBackend) RPCTxFeeCap() float64 {
	return b.qrl.config.RPCTxFeeCap
}

func (b *QRLAPIBackend) BloomStatus() (uint64, uint64) {
	sections, _, _ := b.qrl.bloomIndexer.Sections()
	return params.BloomBitsBlocks, sections
}

func (b *QRLAPIBackend) ServiceFilter(ctx context.Context, session *bloombits.MatcherSession) {
	for range bloomFilterThreads {
		go session.Multiplex(bloomRetrievalBatch, bloomRetrievalWait, b.qrl.bloomRequests)
	}
}

func (b *QRLAPIBackend) Engine() consensus.Engine {
	return b.qrl.engine
}

func (b *QRLAPIBackend) CurrentHeader() *types.Header {
	return b.qrl.blockchain.CurrentHeader()
}

func (b *QRLAPIBackend) StateAtBlock(ctx context.Context, block *types.Block, reexec uint64, base *state.StateDB, readOnly bool, preferDisk bool) (*state.StateDB, tracers.StateReleaseFunc, error) {
	return b.qrl.stateAtBlock(ctx, block, reexec, base, readOnly, preferDisk)
}

func (b *QRLAPIBackend) StateAtTransaction(ctx context.Context, block *types.Block, txIndex int, reexec uint64) (*core.Message, vm.BlockContext, *state.StateDB, tracers.StateReleaseFunc, error) {
	return b.qrl.stateAtTransaction(ctx, block, txIndex, reexec)
}
