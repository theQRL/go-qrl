package pool

import (
	"github.com/cyyber/go-qrl/core/transactions"
	c "github.com/cyyber/go-qrl/config"
)

type TransactionInfo struct {
	tx transactions.TransactionInterface
	blockNumber uint64
	timestamp uint64
	config *c.Config
}

func (t *TransactionInfo) Transaction() transactions.TransactionInterface {
	return t.tx
}

func (t *TransactionInfo) BlockNumber() uint64 {
	return t.blockNumber
}

func (t *TransactionInfo) Timestamp() uint64 {
	return t.timestamp
}

func (t *TransactionInfo) IsStale(currentBlockHeight uint64) bool {
	if currentBlockHeight > t.blockNumber + t.config.User.TransactionPool.StaleTransactionThreshold {
		return true
	}

	// If chain recovered from a fork where chain height is reduced
	// then update block_number of the transactions in pool
	if currentBlockHeight < t.blockNumber {
		t.blockNumber = currentBlockHeight
	}

	return false
}

func CreateTransactionInfo(tx transactions.TransactionInterface, blockNumber uint64, timestamp uint64) *TransactionInfo {
	t := &TransactionInfo{}
	t.tx = tx
	t.blockNumber = blockNumber
	t.timestamp = timestamp

	return t
}
