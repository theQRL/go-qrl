package pool

import (
	"errors"
	"github.com/theQRL/go-qrl/pkg/config"
	"github.com/theQRL/go-qrl/pkg/core/block"
	"github.com/theQRL/go-qrl/pkg/core/state"
	"github.com/theQRL/go-qrl/pkg/core/transactions"
	"github.com/theQRL/go-qrl/pkg/log"
	"github.com/theQRL/go-qrl/pkg/misc"
	"github.com/theQRL/go-qrl/pkg/ntp"
	"github.com/theQRL/go-qrl/pkg/p2p/messages"
	"github.com/theQRL/go-qrl/pkg/writers/mongodb/transactionaction"
	"sync"
	"time"
)

type TransactionPool struct {
	lock                     sync.Mutex
	txPool                   *PriorityQueue
	log                      log.LoggerInterface
	config                   *config.Config
	ntp                      ntp.NTPInterface
	registerAndBroadcastChan chan *messages.RegisterMessage
	chanTransactionAction    chan *transactionaction.TransactionAction
}

func (t *TransactionPool) SetRegisterAndBroadcastChan(c chan *messages.RegisterMessage) {
	t.registerAndBroadcastChan = c
}

func (t *TransactionPool) SetChanTransactionAction(ta chan *transactionaction.TransactionAction) {
	t.chanTransactionAction = ta
}

func (t *TransactionPool) isFull() bool {
	return t.txPool.Full()
}

func (t *TransactionPool) IsFull() bool {
	t.lock.Lock()
	defer t.lock.Unlock()

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

	err := t.txPool.Push(ti)
	if err != nil {
		return err
	}
	if t.chanTransactionAction != nil {
		ta := &transactionaction.TransactionAction{
			Transaction: tx.PBData(),
			Timestamp:timestamp,
			IsAdd:true,
		}
		select {
		case t.chanTransactionAction <- ta:
		case <- time.After(1 * time.Second):
			t.log.Warn("[Add] Chan Timeout for mongodb unconfirmed transaction")
		}
	}
	return nil
}

func (t *TransactionPool) Pop() *TransactionInfo {
	t.lock.Lock()
	defer t.lock.Unlock()

	return t.txPool.Pop()
}

func (t *TransactionPool) Remove(tx transactions.TransactionInterface) bool {
	t.lock.Lock()
	defer t.lock.Unlock()

	if t.txPool.Remove(tx) {
		if t.chanTransactionAction != nil {
			ta := &transactionaction.TransactionAction{
				Transaction: tx.PBData(),
				IsAdd:false,
			}
			select {
			case t.chanTransactionAction <- ta:
			case <- time.After(1 * time.Second):
				t.log.Warn("[Remove] Chan Timeout for mongodb unconfirmed transaction")
			}
		}
		return true
	}
	return false
}

func (t *TransactionPool) RemoveTxInBlock(block *block.Block) {
	t.lock.Lock()
	defer t.lock.Unlock()

	t.txPool.RemoveTxInBlock(block, t.config.Dev.MaxOTSTracking)
	for _, tx := range block.Transactions() {
		if t.chanTransactionAction != nil {
			ta := &transactionaction.TransactionAction{
				Transaction: tx,
				IsAdd:false,
			}
			select {
			case t.chanTransactionAction <- ta:
			case <- time.After(1 * time.Second):
				t.log.Warn("[RemoveTxInBlock] Chan Timeout for mongodb unconfirmed transaction")
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

func (t *TransactionPool) CheckStale(currentBlockHeight uint64, state *state.State) error {
	t.lock.Lock()
	defer t.lock.Unlock()

	txPoolLength := len(*t.txPool)
	for i := 0; i < txPoolLength; i++ {
		txInfo := (*t.txPool)[i]
		if txInfo.IsStale(currentBlockHeight) {
			tx := txInfo.Transaction()
			addrFromState, err := state.GetAddressState(tx.AddrFrom())
			if err != nil {
				t.log.Error("Error while getting AddressState",
					"Txhash", tx.Txhash(),
					"Address", misc.Bin2Qaddress(tx.AddrFrom()),
					"Error", err.Error())
				return err
			}
			addrFromPKState := addrFromState
			addrFromPK := tx.GetSlave()
			if addrFromPK != nil {
				addrFromPKState, err = state.GetAddressState(addrFromPK)
				if err != nil {
					t.log.Error("Error while getting AddressState",
						"Txhash", tx.Txhash(),
						"Address", misc.Bin2Qaddress(tx.GetSlave()),
						"Error", err.Error())
					return err
				}
			}
			if !tx.ValidateExtended(addrFromState, addrFromPKState) {
				t.txPool.removeByIndex(i)
				i -= 1
				txPoolLength -= 1
				continue
			}
			// TODO: Chan to Re-Broadcast Txn
			//txInfo.UpdateBlockNumber(currentBlockHeight)
			//msg := &generated.Message{
			//	Msg:&generated.LegacyMessage_T{
			//		Block:b.PBData(),
			//	},
			//	MessageType:generated.LegacyMessage_BK,
			//}
			//
			//registerMessage := &messages.RegisterMessage{
			//	MsgHash:misc.Bin2HStr(b.HeaderHash()),
			//	Msg:msg,
			//}
			//select {
			//case t.registerAndBroadcastChan <- nil:
			//}

		}
	}
	return nil
}

func CreateTransactionPool() *TransactionPool {
	t := &TransactionPool{
		log: log.GetLogger(),
		config: config.GetConfig(),
		ntp: ntp.GetNTP(),
		txPool: &PriorityQueue{},
	}
	return t
}