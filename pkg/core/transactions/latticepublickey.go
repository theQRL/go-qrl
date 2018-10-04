package transactions

import (
	"bytes"
	"encoding/binary"
	"reflect"

	"github.com/theQRL/go-qrl/pkg/core/addressstate"
	"github.com/theQRL/go-qrl/pkg/generated"
	"github.com/theQRL/go-qrl/pkg/misc"
	"github.com/theQRL/qrllib/goqrllib/goqrllib"
)

type LatticePublicKey struct {
	Transaction
}


func (tx *LatticePublicKey) KyberPk() []byte {
	return tx.data.GetLatticePK().KyberPk
}

func (tx *LatticePublicKey) DilithiumPk() []byte {
	return tx.data.GetLatticePK().DilithiumPk
}

func (tx *LatticePublicKey) GetHashableBytes() []byte {
	tmp := new(bytes.Buffer)
	tmp.Write(tx.MasterAddr())
	binary.Write(tmp, binary.BigEndian, uint64(tx.Fee()))
	tmp.Write(tx.KyberPk())
	tmp.Write(tx.DilithiumPk())

	tmptxhash := misc.UcharVector{}
	tmptxhash.AddBytes(tmp.Bytes())
	tmptxhash.New(goqrllib.Sha2_256(tmptxhash.GetData()))

	return tmptxhash.GetBytes()
}

func (tx *LatticePublicKey) validateCustom() bool {
	// FIXME: This is missing
	return true
}

func (tx *LatticePublicKey) ValidateExtended(addrFromState *addressstate.AddressState, addrFromPKState *addressstate.AddressState) bool {
	if !tx.ValidateSlave(addrFromState, addrFromPKState) {
		return false
	}

	balance := addrFromState.Balance()

	if tx.Fee() < 0 {
		tx.log.Warn("Lattice Txn: State validation failed %s : Negative fee %s", string(tx.Txhash()), tx.Fee())
		return false
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

func (tx *LatticePublicKey) ApplyStateChanges(addressesState map[string]*addressstate.AddressState) {
	if addrState, ok := addressesState[string(tx.AddrFrom())]; ok {
		addrState.SubtractBalance(tx.Fee())
		addrState.AppendTransactionHash(tx.Txhash())
	}

	tx.applyStateChangesForPK(addressesState)
}

func (tx *LatticePublicKey) RevertStateChanges(addressesState map[string]*addressstate.AddressState) {
	if addrState, ok := addressesState[string(tx.AddrFrom())]; ok {
		addrState.AddBalance(tx.Fee())
		addrState.RemoveTransactionHash(tx.Txhash())
	}

	tx.revertStateChangesForPK(addressesState)
}

func (tx *LatticePublicKey) SetAffectedAddress(addressesState map[string]*addressstate.AddressState) {
	addressesState[string(tx.AddrFrom())] = &addressstate.AddressState{}
	addressesState[string(tx.PK())] = &addressstate.AddressState{}
}

func (tx *LatticePublicKey) AddLatticePK(a *addressstate.AddressState) {
	latticePK := &generated.LatticePK{
		Txhash: tx.Txhash(),
		DilithiumPk: tx.DilithiumPk(),
		KyberPk: tx.KyberPk(),
	}

	a.PBData().LatticePKList = append(a.PBData().LatticePKList, latticePK)
}

func (tx *LatticePublicKey) RemoveLatticePK(a *addressstate.AddressState) {
	for i, latticePK := range a.PBData().LatticePKList {
		if reflect.DeepEqual(latticePK.Txhash, tx.Txhash()) {
			a.PBData().LatticePKList = append(a.PBData().LatticePKList[0:i], a.PBData().LatticePKList[i+1:]...)
		}
	}
}

func CreateLatticeTransaction(messageHash []byte, fee uint64, xmssPK []byte, masterAddr []byte) *LatticePublicKey {
	tx := &LatticePublicKey{}

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
