package transactions

import (
	"bytes"
	"encoding/binary"
	"github.com/theQRL/go-qrl/pkg/config"
	"github.com/theQRL/go-qrl/pkg/generated"
	"github.com/theQRL/go-qrl/pkg/log"
	"reflect"

	"github.com/theQRL/go-qrl/pkg/core/addressstate"
	"github.com/theQRL/go-qrl/pkg/misc"
	"github.com/theQRL/qrllib/goqrllib/goqrllib"
)

type SlaveTransaction struct {
	Transaction
}

func (tx *SlaveTransaction) SlavePKs() [][]byte {
	return tx.data.GetSlave().SlavePks
}

func (tx *SlaveTransaction) AccessTypes() []uint32 {
	return tx.data.GetSlave().AccessTypes
}

func (tx *SlaveTransaction) GetHashableBytes() []byte {
	tmp := new(bytes.Buffer)
	tmp.Write(tx.MasterAddr())
	binary.Write(tmp, binary.BigEndian, uint64(tx.Fee()))

	for i := 0; i < len(tx.SlavePKs()); i++ {
		tmp.Write(tx.SlavePKs()[i])
		binary.Write(tmp, binary.BigEndian, tx.AccessTypes()[i])
	}

	tmptxhash := goqrllib.Sha2_256(misc.BytesToUCharVector(tmp.Bytes()))

	return misc.UCharVectorToBytes(tmptxhash)
}

func (tx *SlaveTransaction) validateCustom() bool {
	if tx.Fee() < 0 {
		tx.log.Warn("[SlaveTransaction] State validation failed for %s because: Negative Send", misc.Bin2HStr(tx.Txhash()))
		return false
	}

	if len(tx.SlavePKs()) > int(tx.config.Dev.Transaction.MultiOutputLimit) {
		tx.log.Warn("Number of slave_pks exceeds limit")
		tx.log.Warn("Slave pks len %s", len(tx.SlavePKs()))
		tx.log.Warn("Access types len %s", len(tx.AccessTypes()))
		return false
	}

	if len(tx.SlavePKs()) != len(tx.AccessTypes()) {
		tx.log.Warn("Number of slave pks are not equal to the number of access types provided")
		tx.log.Warn("Slave pks len %s", len(tx.SlavePKs()))
		tx.log.Warn("Access types len %s", len(tx.AccessTypes()))
		return false
	}

	for _, accessType := range tx.AccessTypes() {
		if accessType < uint32(0)  || accessType > uint32(2) {
			tx.log.Warn("Invalid Access type %s", accessType)
			return false
		}
	}

	return true
}

func (tx *SlaveTransaction) ValidateExtended(addrFromState *addressstate.AddressState, addrFromPkState *addressstate.AddressState) bool {
	if !tx.ValidateSlave(addrFromState, addrFromPkState) {
		return false
	}

	balance := addrFromState.Balance()

	if balance < tx.Fee() {
		tx.log.Warn("[SlaveTransaction] State validation failed for %s because: Insufficient funds", misc.Bin2HStr(tx.Txhash()))
		tx.log.Warn("Balance: %s, Amount: %s", balance, tx.Fee())
		return false
	}

	if addrFromPkState.OTSKeyReuse(tx.OtsKey()) {
		tx.log.Warn("[SlaveTransaction] State validation failed for %s because: OTS Public key re-use detected", misc.Bin2HStr(tx.Txhash()))
		return false
	}

	return true
}

func (tx *SlaveTransaction) Validate(verifySignature bool) bool {
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

func (tx *SlaveTransaction) ApplyStateChanges(addressesState map[string]*addressstate.AddressState) {
	tx.applyStateChangesForPK(addressesState)

	if addrState, ok := addressesState[misc.Bin2Qaddress(tx.AddrFrom())]; ok {
		addrState.SubtractBalance(tx.Fee())
		for i := 0; i < len(tx.SlavePKs()) ; i++ {
			addrState.AddSlavePKSAccessType(tx.SlavePKs()[i], tx.AccessTypes()[i])
		}
		addrState.AppendTransactionHash(tx.Txhash())
	}
}

func (tx *SlaveTransaction) RevertStateChanges(addressesState map[string]*addressstate.AddressState) {
	tx.revertStateChangesForPK(addressesState)

	if addrState, ok := addressesState[misc.Bin2Qaddress(tx.AddrFrom())]; ok {
		addrState.AddBalance(tx.Fee())
		for i := 0; i < len(tx.SlavePKs()) ; i++ {
			addrState.RemoveSlavePKSAccessType(tx.SlavePKs()[i])
		}
		addrState.RemoveTransactionHash(tx.Txhash())
	}
}

func (tx *SlaveTransaction) SetAffectedAddress(addressesState map[string]*addressstate.AddressState) {
	addressesState[misc.Bin2Qaddress(tx.AddrFrom())] = &addressstate.AddressState{}
	addressesState[misc.PK2Qaddress(tx.PK())] = &addressstate.AddressState{}
}

func CreateSlaveTransaction(slavePKs [][]byte, accessTypes []uint32, fee uint64, xmssPK []byte, masterAddr []byte) *SlaveTransaction {
	tx := &SlaveTransaction{}
	tx.config = config.GetConfig()
	tx.log = log.GetLogger()

	tx.data = &generated.Transaction{}
	tx.data.TransactionType = &generated.Transaction_Slave_{Slave: &generated.Transaction_Slave{}}

	if masterAddr != nil {
		tx.data.MasterAddr = masterAddr
	}

	tx.data.PublicKey = xmssPK
	tx.data.Fee = fee
	tx.data.GetSlave().SlavePks = slavePKs
	tx.data.GetSlave().AccessTypes = accessTypes

	if !tx.Validate(false) {
		return nil
	}

	return tx
}
