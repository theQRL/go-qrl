package pool

import (
	"container/list"
	"github.com/cyyber/go-qrl/core"
	"github.com/cyyber/go-qrl/core/transactions"
	"github.com/cyyber/go-qrl/misc"
	"errors"
	"reflect"
)

type TransactionPool struct {
	txPool list.List
	config *core.Config
	ntp *misc.NTP
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

	if timestamp == 0 {
		ts, err := t.ntp.Time()
		if err != nil {
			return err
		}
		timestamp = ts
	}

	ti := CreateTransactionInfo(tx, blockNumber, timestamp)

	t.txPool.PushBack(ti)

	return nil
}

func (t *TransactionPool) Remove(tx transactions.TransactionInterface) {
	for e := t.txPool.Front(); e != nil; e = e.Next() {
		tx2 := e.Value.(TransactionInfo)
		if reflect.DeepEqual(tx2.tx.Txhash(), tx.Txhash()) {
			t.txPool.Remove(e)
			break
		}
	}
}

func (t *TransactionPool) RemoveTxInBlock(block core.Block) {
	for _, protoTX := range block.Transactions() {
		tx := transactions.ProtoToTransaction(protoTX)
		if tx.OtsKey() < t.config.Dev.MaxOTSTracking {
			t.Remove(tx)
		} else {
			for e := t.txPool.Front(); e != nil; {
				tmp := e
				e := e.Next()

				tx2 := e.Value.(TransactionInfo)
				if reflect.DeepEqual(tx.PK(), tx2.tx.PK()) {
					if tx2.tx.OtsKey() <= tx.OtsKey() {
						t.txPool.Remove(tmp)
					}
				}
			}
		}
	}
}

func (t *TransactionPool) AddTxFromBlock(block core.Block, currentBlockHeight uint64) error {
	for _, protoTX := range block.Transactions() {
		time, err := t.ntp.Time()
		if err != nil {
			return err
		}

		err = t.Add(transactions.ProtoToTransaction(protoTX), currentBlockHeight, time)
		if err != nil {
			return err
		}
	}
}

func (t *TransactionPool) CheckStale(currentBlockHeight uint64) error {
	for e := t.txPool.Front(); e != nil; e = e.Next() {
		tx := e.Value.(TransactionInfo)
		if tx.IsStale(currentBlockHeight) {
			tx.blockNumber = currentBlockHeight
			// TODO: Broadcast txn to other peers
		}
	}
}