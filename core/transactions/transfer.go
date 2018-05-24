package transactions

import (
	"github.com/theQRL/qrllib/goqrllib"
	"github.com/cyyber/go-qrl/misc"
	"encoding/binary"
	"bytes"
	"github.com/cyyber/go-qrl/core"
	"reflect"
)

type TransferTransaction struct {
	Transaction
}

func (tx *TransferTransaction) AddrsTo() [][]byte {
	return tx.data.GetTransfer().AddrsTo
}

func (tx *TransferTransaction) Amounts() []uint64 {
	return tx.data.GetTransfer().Amounts
}

func (tx *TransferTransaction) TotalAmounts() uint64 {
	totalAmount := uint64(0)
	for amount := range tx.Amounts() {
		totalAmount += uint64(amount)
	}
	return totalAmount
}

func (tx *TransferTransaction) GetHashableBytes() goqrllib.UcharVector {
	tmp := new(bytes.Buffer)
	tmp.Write(tx.MasterAddr())
	binary.Write(tmp, binary.BigEndian, uint64(tx.Fee()))
	for i := 0; i < len(tx.AddrsTo()); i++ {
		tmp.Write(tx.AddrsTo()[i])
		binary.Write(tmp, binary.BigEndian, tx.Amounts()[i])
	}

	tmptxhash := misc.UcharVector{}
	tmptxhash.AddBytes(tmp.Bytes())
	tmptxhash.New(goqrllib.Sha2_256(tmptxhash.GetData()))

	return tmptxhash.GetData()
}

func (tx *TransferTransaction) validateCustom() bool {
	for _, amount := range tx.Amounts() {
		if amount == 0 {
			tx.log.Warn("Amount cannot be 0", tx.Amounts())
			tx.log.Warn("Invalid TransferTransaction")
			return false
		}
	}

	if tx.Fee() < 0 {
		tx.log.Warn("TransferTransaction [%s] Invalid Fee = %d", goqrllib.Bin2hstr(tx.Txhash()), tx.Fee)
		return false
	}

	if len(tx.AddrsTo()) > int(tx.config.Dev.Transaction.MultiOutputLimit) {
		tx.log.Warn("[TransferTransaction] Number of addresses exceeds max limit'")
		tx.log.Warn(">> Length of addrsTo %s", len(tx.AddrsTo()))
		tx.log.Warn(">> Length of amounts %s", len(tx.Amounts()))
		return false
	}

	if len(tx.AddrsTo()) != len(tx.Amounts()) {
		tx.log.Warn("[TransferTransaction] Mismatch number of addresses to & amounts")
		tx.log.Warn(">> Length of addrsTo %s", len(tx.AddrsTo()))
		tx.log.Warn(">> Length of amounts %s", len(tx.Amounts()))
		return false
	}

	if !core.IsValidAddress(tx.AddrFrom()) {
		tx.log.Warn("[TransferTransaction] Invalid address addr_from: %s", tx.AddrFrom())
		return false
	}

	for _, addrTo := range tx.AddrsTo() {
		if !core.IsValidAddress(addrTo) {
			tx.log.Warn("[TransferTransaction] Invalid address addr_to: %s", tx.AddrsTo())
			return false
		}
	}

	return true
}

func (tx *TransferTransaction) validateExtended(
	addrFromState *core.AddressState,
	addrFromPkState *core.AddressState) bool {
	if !tx.ValidateSlave(addrFromState, addrFromPkState) {
		return false
	}

	balance := addrFromState.Balance()
	totalAmount := tx.TotalAmounts()

	if balance < totalAmount + tx.Fee() {
		tx.log.Warn("State validation failed for %s because: Insufficient funds", goqrllib.Bin2hstr(tx.Txhash()))
		tx.log.Warn("balance: %s, fee: %s, amount: %s", balance, tx.Fee(), totalAmount)
		return false
	}

	if addrFromPkState.OTSKeyReuse(tx.OtsKey()) {
		tx.log.Warn("State validation failed for %s because: OTS Public key re-use detected",
			goqrllib.Bin2hstr(tx.Txhash()))
		return false
	}

	return true
}

func (tx *TransferTransaction) ApplyStateChanges(addressesState map[string]core.AddressState) {
	tx.applyStateChangesForPK(addressesState)

	if addrState, ok := addressesState[string(tx.AddrFrom())]; ok {
		total := tx.TotalAmounts() + tx.Fee()
		addrState.AddBalance(total * -1)
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

func (tx *TransferTransaction) RevertStateChanges(addressesState map[string]core.AddressState, state) {
	tx.revertStateChangesForPK(addressesState, state)

	//TODO: Fix when State is ready
	if addrState, ok := addressesState[string(tx.AddrFrom())]; ok {
		total := tx.TotalAmounts() + tx.Fee()
		addrState.AddBalance(total)
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

func (tx *TransferTransaction) SetAffectedAddress(addressesState map[string]core.AddressState) {
	addressesState[string(tx.AddrFrom())] = core.AddressState{}
	addressesState[string(tx.PK())] = core.AddressState{}

	for _, element := range tx.AddrsTo() {
		addressesState[string(element)] = core.AddressState{}
	}
}

func Create(addrsTo [][]byte, amounts []uint64, fee uint64, xmssPK []byte, masterAddr []byte) *TransferTransaction {
	tx := &TransferTransaction{}

	if masterAddr != nil {
		tx.data.MasterAddr = masterAddr
	}

	tx.data.PublicKey = xmssPK

	tx.data.Fee = fee

	tx.data.GetTransfer().AddrsTo = addrsTo

	tx.data.GetTransfer().Amounts = amounts

	return tx
}
