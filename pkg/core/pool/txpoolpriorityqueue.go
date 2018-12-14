package pool

import (
	"errors"
	"github.com/theQRL/go-qrl/pkg/config"
	"github.com/theQRL/go-qrl/pkg/core/block"
	"github.com/theQRL/go-qrl/pkg/core/transactions"
	"reflect"
)

type PriorityQueue []*TransactionInfo

func (pq PriorityQueue) Full() bool {
	return uint64(pq.Len()) > config.GetConfig().User.TransactionPool.TransactionPoolSize
}

func (pq PriorityQueue) Len() int {
	return len(pq)
}

func (pq PriorityQueue) Less(i, j int) bool {
	return pq[i].priority > pq[j].priority
}

func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *PriorityQueue) Push(newTi *TransactionInfo) error {
	if pq != nil {
		for _, ti := range *pq {
			if reflect.DeepEqual(ti.tx.Txhash(), newTi.tx.Txhash()) {
				return errors.New("transaction already exists in pool")
			}
			if reflect.DeepEqual(ti.tx.PK(), newTi.tx.PK()) {
				if ti.tx.OtsKey() == newTi.tx.OtsKey() {
					return errors.New("a transaction already exists signed with same ots key")
				}
			}
		}
	}

	n := len(*pq)
	transactionInfo := newTi
	transactionInfo.index = n
	*pq = append(*pq, transactionInfo)
	return nil
}

func (pq *PriorityQueue) Pop() *TransactionInfo {
	old := *pq
	n := len(old)
	if n == 0 {
		return nil
	}
	transactionInfo := old[n-1]
	transactionInfo.index = -1 // for safety
	*pq = old[:n-1]
	return transactionInfo
}

func (pq *PriorityQueue) Remove(tx transactions.TransactionInterface) {
	if pq != nil {
		for index, ti := range *pq {
			if reflect.DeepEqual(ti.tx.Txhash(), tx.Txhash()) {
				pq.removeByIndex(index)
				return
			}
		}
	}
}

func (pq *PriorityQueue) removeByIndex(index int) {
	*pq = append((*pq)[:index], (*pq)[index+1:]...)
}

func (pq *PriorityQueue) RemoveTxInBlock(block *block.Block, maxOTSTracking uint64) {
	if pq == nil {
		return
	}
	for _, protoTX := range block.Transactions()[1:] {
		tx := transactions.ProtoToTransaction(protoTX)
		if tx.OtsKey() < maxOTSTracking {
			pq.Remove(tx)
		} else {
			for index, ti := range *pq {
				if reflect.DeepEqual(tx.PK(), ti.tx.PK()) {
					if ti.tx.OtsKey() <= tx.OtsKey() {
						pq.removeByIndex(index)
					}
				}
			}
		}
	}
}