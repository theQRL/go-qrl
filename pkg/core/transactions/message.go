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

	tmptxhash := goqrllib.Sha2_256(misc.BytesToUCharVector(tmp.Bytes()))

	return misc.UCharVectorToBytes(tmptxhash)
}

func (tx *MessageTransaction) validateCustom() bool {
	lenMessageHash := len(tx.MessageHash())
	if lenMessageHash > 80 || lenMessageHash == 0 {
		tx.log.Warn("Message length must be greater than 0 and less than 81")
		tx.log.Warn("Found message length %s", len(tx.MessageHash()))
		return false
	}

	return true
}

func (tx *MessageTransaction) ValidateExtended(addrFromState *addressstate.AddressState, addrFromPKState *addressstate.AddressState) bool {
	if !tx.ValidateSlave(addrFromState, addrFromPKState) {
		return false
	}

	balance := addrFromState.Balance()

	if tx.Fee() < 0 {
		tx.log.Warn("State validation failed for %s because: Negative txn fee", string(tx.Txhash()))
	}

	if balance < tx.Fee() {
		tx.log.Warn("State validation failed for %s because: Insufficient funds",
			"TxHash", misc.Bin2HStr(tx.Txhash()),
			"Balance", balance,
			"Fee", tx.Fee())
		return false
	}

	if addrFromPKState.OTSKeyReuse(tx.OtsKey()) {
		tx.log.Warn("State validation failed",
			"Txhash", misc.Bin2HStr(tx.Txhash()),
			"Ots Key", tx.OtsKey())
		return false
	}

	return true
}

func (tx *MessageTransaction) Validate(verifySignature bool) bool {
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

func (tx *MessageTransaction) ApplyStateChanges(addressesState map[string]*addressstate.AddressState) {
	if addrState, ok := addressesState[misc.Bin2Qaddress(tx.AddrFrom())]; ok {
		addrState.SubtractBalance(tx.Fee())
		// Disabled Tracking of Transaction Hash into AddressState
		//addrState.AppendTransactionHash(tx.Txhash())
	}

	tx.applyStateChangesForPK(addressesState)
}

func (tx *MessageTransaction) RevertStateChanges(addressesState map[string]*addressstate.AddressState) {
	if addrState, ok := addressesState[misc.Bin2Qaddress(tx.AddrFrom())]; ok {
		addrState.AddBalance(tx.Fee())
		// Disabled Tracking of Transaction Hash into AddressState
		//addrState.RemoveTransactionHash(tx.Txhash())
	}

	tx.revertStateChangesForPK(addressesState)
}

func (tx *MessageTransaction) SetAffectedAddress(addressesState map[string]*addressstate.AddressState) {
	addressesState[misc.Bin2Qaddress(tx.AddrFrom())] = nil
	addressesState[misc.PK2Qaddress(tx.PK())] = nil
}

func CreateMessageTransaction(message []byte, fee uint64, xmssPK []byte, masterAddr []byte) *MessageTransaction {
	tx := &MessageTransaction{}
	tx.config = config.GetConfig()
	tx.log = log.GetLogger()

	tx.data = &generated.Transaction{}
	tx.data.TransactionType = &generated.Transaction_Message_{Message: &generated.Transaction_Message{}}

	if masterAddr != nil {
		tx.data.MasterAddr = masterAddr
	}

	tx.data.GetMessage().MessageHash = message
	tx.data.Fee = fee
	tx.data.PublicKey = xmssPK

	if !tx.Validate(false) {
		return nil
	}

	return tx
}
