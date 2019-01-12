package transactions

import (
	"bytes"
	"encoding/binary"
	"reflect"

	"github.com/theQRL/go-qrl/pkg/config"
	"github.com/theQRL/go-qrl/pkg/core/addressstate"
	"github.com/theQRL/go-qrl/pkg/generated"
	"github.com/theQRL/go-qrl/pkg/log"
	"github.com/theQRL/go-qrl/pkg/misc"
	"github.com/theQRL/qrllib/goqrllib/goqrllib"
)

type PlainTransferTransaction struct {
	MasterAddress string  `json:"master_address"`
	Fee uint64  `json:"fee"`
	PublicKey string  `json:"public_key"`
	Signature string  `json:"signature"`
	Nonce uint64  `json:"nonce"`
	TransactionHash string  `json:"transaction_hash"`
	TransactionType string  `json:"transaction_type"`

	AddressesTo []string  `json:"addresses_to"`
	Amounts []uint64  `json:"amounts"`
}

func (t *PlainTransferTransaction) TransactionFromPBData(tx *generated.Transaction) {
	if tx.MasterAddr != nil {
		t.MasterAddress = misc.Bin2Qaddress(tx.MasterAddr)
	}
	t.Fee = tx.Fee
	t.PublicKey = misc.Bin2HStr(tx.PublicKey)
	t.Signature = misc.Bin2HStr(tx.Signature)
	t.Nonce = tx.Nonce
	t.TransactionHash = misc.Bin2HStr(tx.TransactionHash)
	t.TransactionType = "transfer"
	t.AddressesTo = misc.Bin2QAddresses(tx.GetTransfer().AddrsTo)
	t.Amounts = tx.GetTransfer().Amounts
}

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
	for _, amount := range tx.Amounts() {
		totalAmount += uint64(amount)
	}
	return totalAmount
}

func (tx *TransferTransaction) GetHashableBytes() []byte {
	tmp := new(bytes.Buffer)
	tmp.Write(tx.MasterAddr())
	binary.Write(tmp, binary.BigEndian, uint64(tx.Fee()))
	for i := 0; i < len(tx.AddrsTo()); i++ {
		tmp.Write(tx.AddrsTo()[i])
		binary.Write(tmp, binary.BigEndian, tx.Amounts()[i])
	}

	tmptxhash := goqrllib.Sha2_256(misc.BytesToUCharVector(tmp.Bytes()))

	return misc.UCharVectorToBytes(tmptxhash)
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
		tx.log.Warn("TransferTransaction [%s] Invalid Fee = %d", misc.Bin2HStr(tx.Txhash()), tx.Fee)
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

	if !addressstate.IsValidAddress(tx.AddrFrom()) {
		tx.log.Warn("[TransferTransaction] Invalid address addr_from: %s", tx.AddrFrom())
		return false
	}

	for _, addrTo := range tx.AddrsTo() {
		if !addressstate.IsValidAddress(addrTo) {
			tx.log.Warn("[TransferTransaction] Invalid address addr_to: %s", tx.AddrsTo())
			return false
		}
	}

	return true
}

func (tx *TransferTransaction) ValidateExtended(
	addrFromState *addressstate.AddressState,
	addrFromPkState *addressstate.AddressState) bool {
	if !tx.ValidateSlave(addrFromState, addrFromPkState) {
		return false
	}

	balance := addrFromState.Balance()
	totalAmount := tx.TotalAmounts()

	if balance < totalAmount+tx.Fee() {
		tx.log.Warn("State validation failed because of Insufficient funds",
			"txhash", misc.Bin2HStr(tx.Txhash()),
			"balance", balance,
			"fee", tx.Fee())
		return false
	}

	if addrFromPkState.OTSKeyReuse(tx.OtsKey()) {
		tx.log.Warn("State validation failed because of OTS Public key re-use detected",
			"txhash", misc.Bin2HStr(tx.Txhash()))
		return false
	}

	return true
}

func (tx *TransferTransaction) Validate(verifySignature bool) bool {
	if !tx.validateCustom() {
		tx.log.Warn("Custom validation failed")
		return false
	}

	if reflect.DeepEqual(tx.config.Dev.Genesis.CoinbaseAddress, tx.PK()) || reflect.DeepEqual(tx.config.Dev.Genesis.CoinbaseAddress, tx.MasterAddr()) {
		tx.log.Warn("Coinbase Address only allowed to do Coinbase Transaction")
		return false
	}

	expectedTransactionHash := tx.GenerateTxHash(tx.GetHashableBytes())

	if verifySignature && !reflect.DeepEqual(expectedTransactionHash, tx.Txhash()) {
		tx.log.Warn("Invalid Transaction hash",
			"Expected Transaction hash", misc.Bin2HStr(expectedTransactionHash),
			"Found Transaction hash", misc.Bin2HStr(tx.Txhash()))
		return false
	}

	if verifySignature {
		if !goqrllib.XmssFastVerify(misc.BytesToUCharVector(tx.GetHashableBytes()),
			misc.BytesToUCharVector(tx.Signature()),
			misc.BytesToUCharVector(tx.PK())) {
			tx.log.Warn("XMSS Verification Failed")
			return false
		}
	}
	return true
}

func (tx *TransferTransaction) ApplyStateChanges(addressesState map[string]*addressstate.AddressState) {
	tx.applyStateChangesForPK(addressesState)

	if addrState, ok := addressesState[misc.Bin2Qaddress(tx.AddrFrom())]; ok {
		total := tx.TotalAmounts() + tx.Fee()
		addrState.SubtractBalance(total)
		// Disabled Tracking of Transaction Hash into AddressState
		//addrState.AppendTransactionHash(tx.Txhash())
	}

	addrsTo := tx.AddrsTo()
	amounts := tx.Amounts()
	for index := range addrsTo {
		addrTo := addrsTo[index]
		amount := amounts[index]

		if addrState, ok := addressesState[misc.Bin2Qaddress(addrTo)]; ok {
			addrState.AddBalance(amount)
			// Disabled Tracking of Transaction Hash into AddressState
			//if !reflect.DeepEqual(addrTo, tx.AddrFrom()) {
			//	addrState.AppendTransactionHash(tx.Txhash())
			//}
		}
	}
}

func (tx *TransferTransaction) RevertStateChanges(addressesState map[string]*addressstate.AddressState) {
	tx.revertStateChangesForPK(addressesState)

	//TODO: Fix when State is ready
	if addrState, ok := addressesState[misc.Bin2Qaddress(tx.AddrFrom())]; ok {
		total := tx.TotalAmounts() + tx.Fee()
		addrState.AddBalance(total)
		// Disabled Tracking of Transaction Hash into AddressState
		//addrState.RemoveTransactionHash(tx.Txhash())
	}

	addrsTo := tx.AddrsTo()
	amounts := tx.Amounts()
	for index := range addrsTo {
		addrTo := addrsTo[index]
		amount := amounts[index]

		if addrState, ok := addressesState[misc.Bin2Qaddress(addrTo)]; ok {
			addrState.SubtractBalance(amount)
			// Disabled Tracking of Transaction Hash into AddressState
			//if !reflect.DeepEqual(addrTo, tx.AddrFrom()) {
			//	addrState.RemoveTransactionHash(tx.Txhash())
			//}
		}
	}
}

func (tx *TransferTransaction) SetAffectedAddress(addressesState map[string]*addressstate.AddressState) {
	addressesState[misc.Bin2Qaddress(tx.AddrFrom())] = &addressstate.AddressState{}
	addressesState[misc.PK2Qaddress(tx.PK())] = &addressstate.AddressState{}

	for _, element := range tx.AddrsTo() {
		addressesState[misc.Bin2Qaddress(element)] = &addressstate.AddressState{}
	}
}

func CreateTransferTransaction(addrsTo [][]byte, amounts []uint64, fee uint64, xmssPK []byte, masterAddr []byte) *TransferTransaction {
	tx := &TransferTransaction{}
	tx.config = config.GetConfig()
	tx.log = log.GetLogger()

	tx.data = &generated.Transaction{}
	tx.data.TransactionType = &generated.Transaction_Transfer_{Transfer: &generated.Transaction_Transfer{}}

	if masterAddr != nil {
		tx.data.MasterAddr = masterAddr
	}

	tx.data.PublicKey = xmssPK
	tx.data.Fee = fee
	tx.data.GetTransfer().AddrsTo = addrsTo
	tx.data.GetTransfer().Amounts = amounts

	if !tx.Validate(false) {
		return nil
	}

	return tx
}
