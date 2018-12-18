package pool

import (
	"errors"
	"sync"

	c "github.com/theQRL/go-qrl/pkg/config"
	"github.com/theQRL/go-qrl/pkg/core/block"
	"github.com/theQRL/go-qrl/pkg/core/transactions"
	"github.com/theQRL/go-qrl/pkg/ntp"
)

type TransactionPool struct {
	lock   sync.Mutex
	txPool *PriorityQueue
	config *c.Config
	ntp    ntp.NTPInterface
}

func (t *TransactionPool) isFull() bool {
	return t.txPool.Full()
}

func (t *TransactionPool) Contains(tx transactions.TransactionInterface) bool {
	return t.txPool.Contains(tx)
}

func (t *TransactionPool) Add(tx transactions.TransactionInterface, blockNumber uint64, timestamp uint64) error {
	t.lock.Lock()
	defer t.lock.Unlock()


	if t.isFull() {
		return errors.New("transaction pool is full")
	}

	if timestamp == 0 {
		timestamp = t.ntp.Time()
	}

	ti := CreateTransactionInfo(tx, blockNumber, timestamp)

	return t.txPool.Push(ti)
}

func (t *TransactionPool) Pop() *TransactionInfo {
	t.lock.Lock()
	defer t.lock.Unlock()

	return t.txPool.Pop()
}

func (t *TransactionPool) Remove(tx transactions.TransactionInterface) {
	t.lock.Lock()
	defer t.lock.Unlock()

	t.txPool.Remove(tx)
}

func (t *TransactionPool) RemoveTxInBlock(block *block.Block) {
	t.lock.Lock()
	defer t.lock.Unlock()

	t.txPool.RemoveTxInBlock(block, t.config.Dev.MaxOTSTracking)
}

func (t *TransactionPool) AddTxFromBlock(block *block.Block, currentBlockHeight uint64) error {
	for _, protoTX := range block.Transactions()[1:] {
		err := t.Add(transactions.ProtoToTransaction(protoTX), currentBlockHeight, t.ntp.Time())
		if err != nil {
			return err
		}
	}
	return nil
}

func (t *TransactionPool) CheckStale(currentBlockHeight uint64) error {
	t.lock.Lock()
	defer t.lock.Unlock()

	// TODO: CheckStale transaction in progress
	//for e := t.txPool.Front(); e != nil; e = e.Next() {
	//	ti := e.Value.(*TransactionInfo)
	//	if ti.IsStale(currentBlockHeight) {
	//		/*
	//			TODO: Add Code for State validation of stale txn
	//			In case of state validation fails, removes the 00
	//			transaction from pool
	//		*/
	//		ti.UpdateBlockNumber(currentBlockHeight)
	//		// TODO: Broadcast txn to other peers
	//	}
	//}
	return nil
}

func CreateTransactionPool() *TransactionPool {
	t := &TransactionPool{
		config: c.GetConfig(),
		ntp: ntp.GetNTP(),
		txPool: &PriorityQueue{},
	}
	return t
}