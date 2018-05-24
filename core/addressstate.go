package core

import (
	"github.com/cyyber/go-qrl/generated"
	"github.com/cyyber/go-qrl/core/transactions"
	"github.com/theQRL/qrllib/goqrllib"
	"github.com/cyyber/go-qrl/misc"
	"github.com/golang/protobuf/proto"
	"reflect"
)

type AddressStateInterface interface {

	PBData() *generated.AddressState

	Address() []byte

	Nonce() uint64

	Balance() uint64

	SetBalance(balance uint64)

	OTSBitfield() [][]byte

	OTSCounter() uint64

	TransactionHashes() [][]byte

	LatticePKList() []*generated.LatticePK

	SlavePKSAccessType() map[string]uint32

	UpdateTokenBalance(tokenTxHash []byte, balance uint64)

	GetTokenBalance(tokenTxHash []byte) uint64

	IsTokenExists(tokenTxHash []byte) bool

	AddSlavePKSAccessType(slavePK []byte, accessType uint32)

	RemoveSlavePKSAccessType(slavePK []byte)

	AddLatticePK(latticeTx *transactions.LatticePublicKey)

	RemoveLatticePK(latticeTx *transactions.LatticePublicKey)

	IncreaseNonce()

	DecreaseNonce()

	GetSlavePermission(slavePK []byte) (uint16, bool)

	GetDefault(address []byte) *AddressState

	OTSKeyReuse(otsKeyIndex uint16) bool

	SetOTSKey(otsKeyIndex uint16)

	// UnsetOTSKey(otsKeyIndex uint16, state) State Object

	IsValidAddress(address []byte) bool

	Serialize() string
}

type AddressState struct {
	data *generated.AddressState
	config *Config
}

func (a *AddressState) PBData() *generated.AddressState {
	return a.data
}

func (a *AddressState) Address() []byte {
	return a.data.Address
}

func (a *AddressState) Nonce() uint64 {
	return a.data.Nonce
}

func (a *AddressState) Balance() uint64 {
	return a.data.Balance
}

func (a *AddressState) SetBalance(balance uint64) {
	a.data.Balance = balance
}

func (a *AddressState) AddBalance(balance uint64) {
	a.data.Balance += balance
}

func (a *AddressState) OTSBitfield() [][]byte {
	return a.data.OtsBitfield
}

func (a *AddressState) OTSCounter() uint64 {
	return a.data.OtsCounter
}

func (a *AddressState) TransactionHashes() [][]byte {
	return a.data.TransactionHashes
}

func (a *AddressState) AppendTransactionHash(hash []byte) {
	a.data.TransactionHashes = append(a.data.TransactionHashes, hash)
}

func (a *AddressState) RemoveTransactionHash(hash []byte) {
	for index, hash1 := range a.data.TransactionHashes {
		if reflect.DeepEqual(index, hash1) {
			a.data.TransactionHashes = append(a.data.TransactionHashes[:index], a.data.TransactionHashes[index+1:])
			//TODO: Fix remove code
			return
		}
	}
}

func (a *AddressState) LatticePKList() []*generated.LatticePK {
	return a.data.LatticePKList
}

func (a *AddressState) SlavePKSAccessType() map[string]uint32 {
	return a.data.SlavePksAccessType
}

func (a *AddressState) UpdateTokenBalance(tokenTxHash []byte, balance uint64) {
	strTokenTxHash := goqrllib.Bin2hstr(tokenTxHash)
	a.data.Tokens[strTokenTxHash] += balance
	if a.data.Tokens[strTokenTxHash] == 0 {
		delete(a.data.Tokens, strTokenTxHash)
	}
}

func (a *AddressState) GetTokenBalance(tokenTxHash []byte) uint64 {
	strTokenTxHash := goqrllib.Bin2hstr(tokenTxHash)
	if balance, ok := a.data.Tokens[strTokenTxHash]; ok {
		return balance
	}
	return 0
}

func (a *AddressState) IsTokenExists(tokenTxHash []byte) bool {
	strTokenTxHash := goqrllib.Bin2hstr(tokenTxHash)
	_, ok := a.data.Tokens[strTokenTxHash]
	return ok
}

func (a *AddressState) AddSlavePKSAccessType(slavePK []byte, accessType uint32) {
	a.data.SlavePksAccessType[string(slavePK)] = accessType
}

func (a *AddressState) RemoveSlavePKSAccessType(slavePK []byte) {
	delete(a.data.SlavePksAccessType, string(slavePK))
}

func (a *AddressState) AddLatticePK(latticeTx *transactions.LatticePublicKey) {
	latticePK := &generated.LatticePK{
		Txhash: latticeTx.txhash,
		DilithiumPk: latticeTx.dilithiumPK,
		KyberPk: latticeTx.kyberPK,
	}

	a.data.LatticePKList = append(a.data.LatticePKList, latticePK)
}

func (a *AddressState) RemoveLatticePK(latticeTx *transactions.LatticePublicKey) {
	for _, index := range a.data.LatticePKList {
		// TODO check if deletion possible while iterating
	}
}

func (a *AddressState) IncreaseNonce() {
	a.data.Nonce++
}

func (a *AddressState) DecreaseNonce() {
	a.data.Nonce--
}

func (a *AddressState) GetSlavePermission(slavePK []byte) (uint32, bool) {
	value, ok := a.data.SlavePksAccessType[string(slavePK)]
	return value, ok
}

func (a *AddressState) GetDefault(address []byte) *AddressState {
	return a
}

func (a *AddressState) OTSKeyReuse(otsKeyIndex uint16) bool {
	if otsKeyIndex < a.config.Dev.MaxOTSTracking {
		offset := otsKeyIndex >> 3
		relative := otsKeyIndex % 8
		if (a.data.OtsBitfield[offset][0] >> relative) & 1 == 1 {
			return true
		}
	} else {
		if uint64(otsKeyIndex) <= a.data.OtsCounter {
			return true
		}
	}

	return false
}

func (a *AddressState) SetOTSKey(otsKeyIndex uint64) {
	if otsKeyIndex < uint64(a.config.Dev.MaxOTSTracking) {
		offset := otsKeyIndex >> 3
		relative := otsKeyIndex % 8
		bitfield := a.data.OtsBitfield[offset]
		a.data.OtsBitfield[offset][0] = bitfield[0] | (1 << relative)
	} else {
		a.data.OtsCounter = otsKeyIndex
	}
}

//func (a *AddressState) UnsetOTSKey(otsKeyIndex uint64, state) {
//    // TODO when state is ready
//}

func IsValidAddress(address []byte) bool {
	if !goqrllib.QRLHelperAddressIsValid(misc.BytesToUCharVector(address)) {
		return true
	}
	return false
}

func (a *AddressState) Serialize() string {
	return proto.CompactTextString(a.data)
}

func Create(address []byte, nonce uint64, balance uint64) *AddressState {
	a := &AddressState{}
	a.data.Address = address
	a.data.Nonce = nonce
	a.data.Balance = balance
	//make([512][]byte, a.data.OtsBitfield)
	return a
}

func GetDefault(address []byte) *AddressState {
	c := Config{}
	return Create(address, uint64(c.Dev.DefaultNonce), c.Dev.DefaultAccountBalance)
}
