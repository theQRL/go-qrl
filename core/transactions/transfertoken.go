package transactions

import (
	"bytes"
	"encoding/binary"
	"reflect"

	"github.com/theQRL/go-qrl/core/addressstate"
	"github.com/theQRL/go-qrl/misc"
	"github.com/theQRL/qrllib/goqrllib/goqrllib"
)

type TransferTokenTransaction struct {
	Transaction
}

func (tx *TransferTokenTransaction) TokenTxhash() []byte {
	return tx.data.GetTransferToken().TokenTxhash
}

func (tx *TransferTokenTransaction) AddrsTo() [][]byte {
	return tx.data.GetTransferToken().AddrsTo
}

func (tx *TransferTokenTransaction) Amounts() []uint64 {
	return tx.data.GetTransferToken().Amounts
}

func (tx *TransferTokenTransaction) TotalAmount() uint64 {
	var totalAmount uint64

	for _, amount := range tx.Amounts() {
		totalAmount += amount
	}

	return totalAmount
}

func (tx *TransferTokenTransaction) GetHashableBytes() []byte {
	tmp := new(bytes.Buffer)
	tmp.Write(tx.MasterAddr())
	binary.Write(tmp, binary.BigEndian, uint64(tx.Fee()))
	tmp.Write(tx.TokenTxhash())

	for i := 0; i < len(tx.AddrsTo()); i++ {
		tmp.Write(tx.AddrsTo()[i])
		binary.Write(tmp, binary.BigEndian, tx.Amounts()[i])
	}

	tmptxhash := misc.UcharVector{}
	tmptxhash.AddBytes(tmp.Bytes())
	tmptxhash.New(goqrllib.Sha2_256(tmptxhash.GetData()))

	return tmptxhash.GetBytes()
}

func (tx *TransferTokenTransaction) validateCustom() bool {
	for _, amount := range tx.Amounts() {
		if amount <= 0 {
			tx.log.Warn("[TransferTokenTransaction] Amount cannot be 0 or negative", tx.Amounts())
			return false
		}

		if tx.Fee() < 0 {
			tx.log.Warn("[TransferTokenTransaction] Invalid Fee = %d", string(tx.Txhash()), tx.Fee())
			return false
		}

		if len(tx.AddrsTo()) > int(tx.config.Dev.Transaction.MultiOutputLimit) {
			tx.log.Warn("[TransferTokenTransaction] Number of addresses or amounts exceeds max limit")
			tx.log.Warn("Number of addresses %s ", len(tx.AddrsTo()))
			tx.log.Warn("Number of Amounts %s ", len(tx.Amounts()))
			return false
		}

		if len(tx.AddrsTo()) != len(tx.Amounts()) {
			tx.log.Warn("[TransferTokenTransaction] Invalid address addr_from: %s", tx.AddrFrom())
			return false
		}

		for _, addrTo := range tx.AddrsTo() {
			if !addressstate.IsValidAddress(addrTo) {
				tx.log.Warn("[TransferTokenTransaction] Invalid address addr_to: %s", addrTo)
				return false
			}
		}

		return true
	}
	return false
}

func (tx *TransferTokenTransaction) ValidateExtended(addrFromState *addressstate.AddressState, addrFromPkState *addressstate.AddressState) bool {
	if !tx.ValidateSlave(addrFromState, addrFromPkState) {
		return false
	}

	balance := addrFromState.Balance()
	totalAmount := tx.TotalAmount()

	if balance < tx.Fee() {
		tx.log.Warn("[TransferTokenTransaction] State validation failed for %s because: Insufficient funds", goqrllib.Bin2hstr(tx.Txhash()))
		tx.log.Warn("balance: %s, fee: %s", balance, tx.Fee())
		return false
	}

	if !addrFromState.IsTokenExists(tx.TokenTxhash()) {
		tx.log.Warn("%s doesnt own any such token %s", tx.AddrFrom(), string(tx.TokenTxhash()))
		return false
	}

	tokenBalance := addrFromState.GetTokenBalance(tx.TokenTxhash())
	if tokenBalance < totalAmount {
		tx.log.Warn("Insufficient amount of token")
		tx.log.Warn("Token Balance : %s, Sent Token Amount: %s", tokenBalance, totalAmount)
		return false
	}

	if addrFromPkState.OTSKeyReuse(tx.OtsKey()) {
		tx.log.Warn("[TransferTokenTransaction] State validation failed for %s because: OTS Public key re-use detected", string(tx.Txhash()))
		return false
	}

	return true
}

func (tx *TransferTokenTransaction) ApplyStateChanges(addressesState map[string]*addressstate.AddressState) {
	tx.applyStateChangesForPK(addressesState)

	if addrState, ok := addressesState[string(tx.AddrFrom())]; ok {
		addrState.AddBalance(tx.Fee() * -1)
		addrState.AppendTransactionHash(tx.Txhash())
	}

	addrsTo := tx.AddrsTo()
	amounts := tx.Amounts()
	for index := range addrsTo {
		addrTo := addrsTo[index]
		amount := amounts[index]

		if addrState, ok := addressesState[string(addrTo)]; ok {
			addrState.AddBalance(amount)
			if !reflect.DeepEqual(addrTo, tx.AddrFrom()) {
				addrState.AppendTransactionHash(tx.Txhash())
			}
		}
	}
}

func (tx *TransferTokenTransaction) RevertStateChanges(addressesState map[string]*addressstate.AddressState) {
	tx.revertStateChangesForPK(addressesState)

	if addrState, ok := addressesState[string(tx.AddrFrom())]; ok {
		addrState.AddBalance(tx.Fee())
		addrState.RemoveTransactionHash(tx.Txhash())
	}

	addrsTo := tx.AddrsTo()
	amounts := tx.Amounts()
	for index := range addrsTo {
		addrTo := addrsTo[index]
		amount := amounts[index]

		if addrState, ok := addressesState[string(addrTo)]; ok {
			addrState.AddBalance(amount * -1)
			if !reflect.DeepEqual(addrTo, tx.AddrFrom()) {
				addrState.RemoveTransactionHash(tx.Txhash())
			}
		}
	}
}

func (tx *TransferTokenTransaction) SetAffectedAddress(addressesState map[string]*addressstate.AddressState) {
	addressesState[string(tx.AddrFrom())] = &addressstate.AddressState{}
	addressesState[string(tx.PK())] = &addressstate.AddressState{}

	for _, element := range tx.AddrsTo() {
		addressesState[string(element)] = &addressstate.AddressState{}
	}
}

func CreateTransferToken(tokenTxhash []byte, addrsTo [][]byte, amounts []uint64, fee uint64, xmssPK []byte, masterAddr []byte) *TransferTokenTransaction{
	tx := &TransferTokenTransaction{}

	tx.data.MasterAddr = masterAddr
	tx.data.PublicKey = xmssPK
	tx.data.Fee = fee

	transferTx := tx.data.GetTransferToken()
	transferTx.TokenTxhash = tokenTxhash
	transferTx.AddrsTo = addrsTo
	transferTx.Amounts = amounts

	if !tx.Validate(misc.BytesToUCharVector(tx.GetHashableBytes()), false) {
		return nil
	}

	return tx
}
