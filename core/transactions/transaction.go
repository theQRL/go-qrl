package transactions

import (
	"encoding/binary"
	"reflect"
	"github.com/golang/protobuf/proto"
	"github.com/theQRL/qrllib/goqrllib"
	"github.com/cyyber/go-qrl/generated"
	"github.com/cyyber/go-qrl/misc"
	"github.com/cyyber/go-qrl/crypto"
	"github.com/golang/protobuf/jsonpb"
	"github.com/cyyber/go-qrl/log"
	"github.com/cyyber/go-qrl/core"
	"github.com/cyyber/go-qrl/core/pool"
)

type TransactionInterface interface {

	Size() int

	PBData() *generated.Transaction

	Type()

	Fee() uint64

	Nonce() uint64

	MasterAddr() []byte

	AddrFrom() []byte

	OtsKey() uint16

	GetOtsFromSignature(signature []byte) uint64

	PK() []byte

	Signature() []byte

	FromPBdata(pbdata *generated.Transaction) //Set return type

	GetSlave() []byte

	Txhash() []byte

	UpdateTxhash(hashableBytes goqrllib.UcharVector)

	GetHashableBytes() goqrllib.UcharVector

	Sign(xmss crypto.XMSS, message goqrllib.UcharVector)

	ApplyStateChanges(addressesState map[string]core.AddressState)

	RevertStateChanges(addressesState map[string]core.AddressState, state *core.State)

	applyStateChangesForPK(addressesState map[string]core.AddressState)

	revertStateChangesForPK(addressesState map[string]core.AddressState, state *core.State)

	SetAffectedAddress(addressesState map[string]core.AddressState)

	validateCustom() bool

	ValidateXMSS(hashableBytes goqrllib.UcharVector) bool

	ValidateSlave(addrFromState *core.AddressState, addrFromPKState *core.AddressState) bool

	FromJSON(jsonData string) *Transaction

	JSON() (string, error)

}

type Transaction struct {
	log    log.Logger
	data   *generated.Transaction
	config core.Config
}

func (tx *Transaction) Size() int {
	return proto.Size(tx.data)
}

func (tx *Transaction) PBData() *generated.Transaction {
	return tx.data
}

func (tx *Transaction) Type() {
	// TODO
	//tx.data.transactionType.(type)
}

func (tx *Transaction) Fee() uint64 {
	return tx.data.Fee
}

func (tx *Transaction) Nonce() uint64 {
	return tx.data.Nonce
}

func (tx *Transaction) MasterAddr() []byte {
	return tx.data.MasterAddr
}

func (tx *Transaction) AddrFrom() []byte {
	if tx.MasterAddr() != nil {
		return tx.MasterAddr()
	}

	pk := tx.PK()
	upk := misc.UcharVector{}
	upk.AddBytes(pk)
	upk.New(goqrllib.QRLHelperGetAddress(upk.GetData()))

	return upk.GetBytes()

}

func (tx *Transaction) OtsKey() uint16 {
	return binary.BigEndian.Uint16(tx.data.Signature[0:8])
}

func (tx *Transaction) GetOtsFromSignature(signature []byte) uint64 {
	return binary.BigEndian.Uint64(signature[0:8])
}

func (tx *Transaction) PK() []byte {
	return tx.data.PublicKey
}

func (tx *Transaction) Signature() []byte {
	return tx.data.Signature
}

func (tx *Transaction) FromPBdata(pbdata *generated.Transaction) {
	tx.data = pbdata
}

func (tx *Transaction) GetSlave() []byte {
	pk := tx.PK()
	upk := misc.UcharVector{}
	upk.AddBytes(pk)
	upk.New(goqrllib.QRLHelperGetAddress(upk.GetData()))

	if reflect.DeepEqual(upk.GetBytes(), tx.AddrFrom()) {
		return upk.GetBytes()
	}

	return nil
}

func (tx *Transaction) Txhash() []byte {
	return tx.data.TransactionHash
}

func (tx *Transaction) UpdateTxhash(hashableBytes goqrllib.UcharVector) {
	tmp := misc.UcharVector{}
	tmp.New(hashableBytes)
	tmp.AddBytes(tx.Signature())
	tmp.AddBytes(tx.PK())
	tx.data.TransactionHash = tmp.GetBytes()
}

func (tx *Transaction) GetHashableBytes() goqrllib.UcharVector {
	//TODO When State is ready
}

func (tx *Transaction) Sign(xmss crypto.XMSS, message goqrllib.UcharVector) {
	tx.data.Signature = xmss.Sign(message)
}

func (tx *Transaction) applyStateChangesForPK(addressesState map[string]core.AddressState) {
	addrFromPK := misc.UCharVectorToString(goqrllib.QRLHelperGetAddress(misc.BytesToUCharVector(tx.PK())))
	if _, ok := addressesState[addrFromPK]; ok {
		if string(tx.AddrFrom()) != addrFromPK {
			addressesState[addrFromPK].AppendTransactionHash(tx.Txhash())
		}
		addressesState[addrFromPK].IncreaseNonce()
		addressesState[addrFromPK].SetOTSKey(uint64(tx.OtsKey()))
	}
}

func (tx *Transaction) revertStateChangesForPK(addressesState map[string]core.AddressState, state *core.State) {
	addrFromPK := misc.UCharVectorToString(goqrllib.QRLHelperGetAddress(misc.BytesToUCharVector(tx.PK())))
	if _, ok := addressesState[addrFromPK]; ok {
		if string(tx.AddrFrom()) != addrFromPK {
			addressesState[addrFromPK].RemoveTransactionHash(tx.Txhash())
		}
		addressesState[addrFromPK].DecreaseNonce()
		addressesState[addrFromPK].UnsetOTSKey(tx.OtsKey(), state) //TODO: Fix when state is ready
	}
}

func (tx *Transaction) ApplyStateChanges(addressesState map[string]core.AddressState) {
	panic("Not Implemented")
}

func (tx *Transaction) RevertStateChanges(addressesState map[string]core.AddressState, state *core.State) {
	panic("Not Implemented")
}

func (tx *Transaction) SetAffectedAddress(addressesState map[string]core.AddressState) {
	addressesState[string(tx.AddrFrom())] = core.AddressState{}
	addressesState[string(tx.PK())] = core.AddressState{}
}

func (tx *Transaction) validateCustom() bool {
	panic("Not Implemented")
	return false
}

func (tx *Transaction) ValidateXMSS(hashableBytes goqrllib.UcharVector) bool {
	if !goqrllib.XmssFastVerify(hashableBytes,
		misc.BytesToUCharVector(tx.Signature()),
		misc.BytesToUCharVector(tx.PK())) {
		tx.log.Warn("XMSS Verification Failed")
		return false
	}
	return true
}

func (tx *Transaction) ValidateSlave(addrFromState *core.AddressState, addrFromPKState *core.AddressState) bool {
	addrFromPK := misc.UCharVectorToString(goqrllib.QRLHelperGetAddress(misc.BytesToUCharVector(tx.PK())))

	if string(tx.MasterAddr()) == addrFromPK {
		tx.log.Warn("Matching master_addr field and address from PK")
		return false
	}

	accessType, ok := addrFromPKState.GetSlavePermission(tx.PK())

	if !ok {
		tx.log.Warn("Public key and address don't match")
		return false
	}

	if accessType != 0 {
		tx.log.Warn("Access Type ", accessType)
		tx.log.Warn("Slave Address doesnt have sufficient permission")
		return false
	}

	return true
}

func (tx *Transaction) FromJSON(jsonData string) *Transaction {
	tx.data = &generated.Transaction{}
	jsonpb.UnmarshalString(jsonData, tx.data)
	return tx
}

func (tx *Transaction) JSON() (string, error) {
	ma := jsonpb.Marshaler{}
	return ma.MarshalToString(tx.data)
}

func ProtoToTransaction(protoTX *generated.Transaction) TransactionInterface {
	var tx TransactionInterface
	switch _ := protoTX.TransactionType.(type) {
	case *generated.Transaction_Transfer_:
		tx = &TransferTransaction{}
	case *generated.Transaction_Coinbase:
		tx = &CoinBase{}
	case *generated.Transaction_Token_:
		tx = &TokenTransaction{}
	case *generated.Transaction_TransferToken_:
		tx = &TransferTokenTransaction{}
	case *generated.Transaction_Message_:
		tx = &MessageTransaction{}
	}

	if tx != nil {
		tx.data = protoTX
	}

	return tx
}