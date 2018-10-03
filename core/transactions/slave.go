package transactions

import (
	"bytes"
	"encoding/binary"

	"github.com/theQRL/go-qrl/core/addressstate"
	"github.com/theQRL/go-qrl/misc"
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

func (tx *SlaveTransaction) GetHashableBytes() goqrllib.UcharVector {
	tmp := new(bytes.Buffer)
	tmp.Write(tx.MasterAddr())
	binary.Write(tmp, binary.BigEndian, uint64(tx.Fee()))

	for i := 0; i < len(tx.SlavePKs()); i++ {
		tmp.Write(tx.SlavePKs()[i])
		binary.Write(tmp, binary.BigEndian, tx.AccessTypes()[i])
	}

	tmptxhash := misc.UcharVector{}
	tmptxhash.AddBytes(tmp.Bytes())
	tmptxhash.New(goqrllib.Sha2_256(tmptxhash.GetData()))

	return tmptxhash.GetData()
}

func (tx *SlaveTransaction) validateCustom() bool {
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
		if 0 < accessType || accessType > 1 {
			tx.log.Warn("Invalid Access typ %s", accessType)
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

	if tx.Fee() < 0 {
		tx.log.Warn("[SlaveTransaction] State validation failed for %s because: Negative Send", string(tx.Txhash()))
		return false
	}

	if balance < tx.Fee() {
		tx.log.Warn("[SlaveTransaction] State validation failed for %s because: Insufficient funds", string(tx.Txhash()))
		tx.log.Warn("Balance: %s, Amount: %s", balance, tx.Fee())
		return false
	}

	if addrFromPkState.OTSKeyReuse(tx.OtsKey()) {
		tx.log.Warn("[SlaveTransaction] State validation failed for %s because: OTS Public key re-use detected", string(tx.Txhash()))
		return false
	}

	return true
}

func (tx *SlaveTransaction) ApplyStateChanges(addressesState map[string]*addressstate.AddressState) {
	tx.applyStateChangesForPK(addressesState)

	if addrState, ok := addressesState[string(tx.AddrFrom())]; ok {
		addrState.SubtractBalance(tx.Fee())
		for i := 0; i < len(tx.SlavePKs()) ; i++ {
			addrState.AddSlavePKSAccessType(tx.SlavePKs()[i], tx.AccessTypes()[i])
		}
		addrState.AppendTransactionHash(tx.Txhash())
	}
}

func (tx *SlaveTransaction) RevertStateChanges(addressesState map[string]*addressstate.AddressState) {
	tx.revertStateChangesForPK(addressesState)

	if addrState, ok := addressesState[string(tx.AddrFrom())]; ok {
		addrState.AddBalance(tx.Fee())
		for i := 0; i < len(tx.SlavePKs()) ; i++ {
			addrState.RemoveSlavePKSAccessType(tx.SlavePKs()[i])
		}
		addrState.RemoveTransactionHash(tx.Txhash())
	}
}

func (tx *SlaveTransaction) SetAffectedAddress(addressesState map[string]*addressstate.AddressState) {
	addressesState[string(tx.AddrFrom())] = &addressstate.AddressState{}
	addressesState[string(tx.PK())] = &addressstate.AddressState{}
}
