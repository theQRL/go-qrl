package transactions

import (
	"bytes"
	"encoding/binary"

	"github.com/theQRL/go-qrl/core"
	"github.com/theQRL/go-qrl/misc"
	"github.com/theQRL/qrllib/goqrllib/goqrllib"
)

type MessageTransaction struct {
	Transaction
}

func (tx *MessageTransaction) MessageHash() []byte {
	return tx.data.GetMessage().MessageHash
}

func (tx *MessageTransaction) GetHashableBytes() []byte {
	tmp := new(bytes.Buffer)
	tmp.Write(tx.MasterAddr())
	binary.Write(tmp, binary.BigEndian, uint64(tx.Fee()))
	tmp.Write(tx.MessageHash())

	tmptxhash := misc.UcharVector{}
	tmptxhash.AddBytes(tmp.Bytes())
	tmptxhash.New(goqrllib.Sha2_256(tmptxhash.GetData()))

	return tmptxhash.GetBytes()
}

func (tx *MessageTransaction) validateCustom() bool {
	lenMessageHash := len(tx.MessageHash())
	if  lenMessageHash > 80 || lenMessageHash == 0 {
		tx.log.Warn("Message length must be greater than 0 and less than 81")
		tx.log.Warn("Found message length %s", len(tx.MessageHash()))
		return false
	}

	return true
}

func (tx *MessageTransaction) ValidateExtended(addrFromState *core.AddressState, addrFromPKState *core.AddressState) bool {
	if !tx.ValidateSlave(addrFromState, addrFromPKState) {
		return false
	}

	balance := addrFromState.Balance()

	if tx.Fee() < 0 {
		tx.log.Warn("State validation failed for %s because: Negative txn fee", string(tx.Txhash()))
	}

	if balance < tx.Fee() {
		tx.log.Warn("State validation failed for %s because: Insufficient funds", string(tx.Txhash()))
		tx.log.Warn("Balance: %s, Fee: %s", balance, tx.Fee())
		return false
	}

	if addrFromPKState.OTSKeyReuse(tx.OtsKey()) {
		tx.log.Warn("State validation failed for %s because: OTS Public key re-use detected", string(tx.Txhash()))
		return false
	}

	return true
}

func (tx *MessageTransaction) ApplyStateChanges(addressesState map[string]*core.AddressState) {
	if addrState, ok := addressesState[string(tx.AddrFrom())]; ok {
		addrState.AddBalance(tx.Fee() * -1)
		addrState.AppendTransactionHash(tx.Txhash())
	}

	tx.applyStateChangesForPK(addressesState)
}

func (tx *MessageTransaction) RevertStateChanges(addressesState map[string]*core.AddressState, state *core.State) {
	if addrState, ok := addressesState[string(tx.AddrFrom())]; ok {
		addrState.AddBalance(tx.Fee())
		addrState.RemoveTransactionHash(tx.Txhash())
	}

	tx.revertStateChangesForPK(addressesState, state)
}

func (tx *MessageTransaction) SetAffectedAddress(addressesState map[string]*core.AddressState) {
	addressesState[string(tx.AddrFrom())] = &core.AddressState{}
	addressesState[string(tx.PK())] = &core.AddressState{}
}

func CreateMessageTransaction(messageHash []byte, fee uint64, xmssPK []byte, masterAddr []byte) *MessageTransaction {
	tx := &MessageTransaction{}

	if masterAddr != nil {
		tx.data.MasterAddr = masterAddr
	}

	tx.data.GetMessage().MessageHash = messageHash
	tx.data.Fee = fee

	tx.data.PublicKey = xmssPK

	if !tx.Validate(misc.BytesToUCharVector(tx.GetHashableBytes()), false) {
		return nil
	}

	return tx
}
