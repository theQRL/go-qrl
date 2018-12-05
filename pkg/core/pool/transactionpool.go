package pool

import (
	"container/list"
	"errors"
	"reflect"

	c "github.com/theQRL/go-qrl/pkg/config"
	"github.com/theQRL/go-qrl/pkg/core/block"
	"github.com/theQRL/go-qrl/pkg/core/transactions"
	"github.com/theQRL/go-qrl/pkg/ntp"
)

type TransactionPool struct {
	txPool list.List
	config *c.Config
	ntp    *ntp.NTP
}

func (t *TransactionPool) IsFull() bool {
	if t.txPool.Len() >= int(t.config.User.TransactionPool.TransactionPoolSize) {
		return true
	}

	return false
}

func (t *TransactionPool) Add(tx transactions.TransactionInterface, blockNumber uint64, timestamp uint64) error {
	if t.IsFull() {
		return errors.New("transaction pool is full")
	}

	for e := t.txPool.Front(); e != nil; e = e.Next() {
		ti := e.Value.(*TransactionInfo)
		if reflect.DeepEqual(ti.tx.Txhash(), tx.Txhash()) {
			return errors.New("transaction already exists in pool")
		}
		if reflect.DeepEqual(ti.tx.PK(), tx.PK()) {
			if ti.tx.OtsKey() == tx.OtsKey() {
				return errors.New("a transaction already exists signed with same ots key")
			}
		}
	}

	if timestamp == 0 {
		timestamp = t.ntp.Time()
	}

	ti := CreateTransactionInfo(tx, blockNumber, timestamp)

	t.txPool.PushBack(ti)

	return nil
}

func (t *TransactionPool) Remove(tx transactions.TransactionInterface) {
	for e := t.txPool.Front(); e != nil; e = e.Next() {
		ti := e.Value.(*TransactionInfo)
		if reflect.DeepEqual(ti.tx.Txhash(), tx.Txhash()) {
			t.txPool.Remove(e)
			break
		}
	}
}

func (t *TransactionPool) RemoveTxInBlock(block *block.Block) {
	for _, protoTX := range block.Transactions()[1:] {
		tx := transactions.ProtoToTransaction(protoTX)
		if tx.OtsKey() < t.config.Dev.MaxOTSTracking {
			t.Remove(tx)
		} else {
			for e := t.txPool.Front(); e != nil; {
				tmp := e
				e := e.Next()

				ti := e.Value.(*TransactionInfo)
				if reflect.DeepEqual(tx.PK(), ti.tx.PK()) {
					if ti.tx.OtsKey() <= tx.OtsKey() {
						t.txPool.Remove(tmp)
					}
				}
			}
		}
	}
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
	for e := t.txPool.Front(); e != nil; e = e.Next() {
		ti := e.Value.(*TransactionInfo)
		if ti.IsStale(currentBlockHeight) {
			/*
				TODO: Add Code for State validation of stale txn
				In case of state validation fails, removes the transaction from pool
			*/
			ti.UpdateBlockNumber(currentBlockHeight)
			// TODO: Broadcast txn to other peers
		}
	}
	return nil
}

func CreateTransactionPool() *TransactionPool {
	t := &TransactionPool{
		config: c.GetConfig(),
		ntp: ntp.GetNTP(),
	}
	return t
}