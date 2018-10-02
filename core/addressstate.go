package core

import (
	"errors"
	"reflect"

	"github.com/golang/protobuf/proto"

	c "github.com/theQRL/go-qrl/config"
	"github.com/theQRL/go-qrl/core/transactions"
	"github.com/theQRL/go-qrl/generated"
	"github.com/theQRL/go-qrl/misc"

	"github.com/theQRL/qrllib/goqrllib/goqrllib"
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

	UnsetOTSKey(otsKeyIndex uint16, state *State)

	IsValidAddress(address []byte) bool

	Serialize() string
}

type AddressState struct {
	data *generated.AddressState
	config *c.Config
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

func (a *AddressState) OtsBitfield() [][]byte {
	return a.data.OtsBitfield
}

func (a *AddressState) OtsCounter() uint64 {
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
			a.data.TransactionHashes = append(a.data.TransactionHashes[:index], a.data.TransactionHashes[index+1:]...)
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
		Txhash: latticeTx.Txhash(),
		DilithiumPk: latticeTx.DilithiumPk(),
		KyberPk: latticeTx.KyberPk(),
	}

	a.data.LatticePKList = append(a.data.LatticePKList, latticePK)
}

func (a *AddressState) RemoveLatticePK(latticeTx *transactions.LatticePublicKey) {
	for i, latticePK := range a.data.LatticePKList {
		if reflect.DeepEqual(latticePK.Txhash, latticeTx.Txhash()) {
			a.data.LatticePKList = append(a.data.LatticePKList[0:i], a.data.LatticePKList[i+1:]...)
		}
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

func (a *AddressState) UnsetOTSKey(otsKeyIndex uint64, state *State) error {
	if otsKeyIndex < uint64(a.config.Dev.MaxOTSTracking) {
		offset := otsKeyIndex >> 3
		relative := otsKeyIndex % 8
		bitfield := a.data.OtsBitfield[offset]
		a.data.OtsBitfield[offset][0] = bitfield[0] & ^(1 << relative)
		return nil
	} else {
		a.data.OtsCounter = 0
		hashes := a.TransactionHashes()
		for i := len(hashes); i >= 0 ; i-- {
			tm, err := state.GetTxMetadata(hashes[i])
			if err != nil {
				return err
			}
			tx := transactions.ProtoToTransaction(tm.Transaction)
			if tx.OtsKey() >= a.config.Dev.MaxOTSTracking {
				a.data.OtsCounter = uint64(tx.OtsKey())
				return nil
			}
		}
	}
	return errors.New("OTS key didn't change")
}

func IsValidAddress(address []byte) bool {
	// Warning: Never pass this validation True for Coinbase Address
	if !goqrllib.QRLHelperAddressIsValid(misc.BytesToUCharVector(address)) {
		return true
	}
	return false
}

func CreateAddressState(address []byte, nonce uint64, balance uint64, otsBitfield [c.Config{}.Dev.OtsBitFieldSize][8]byte, tokens map[string]uint64, slavePksAccessType map[string]uint32, otsCounter uint64) *AddressState {
	a := &AddressState{}
	a.data.Address = address
	a.data.Nonce = nonce
	a.data.Balance = balance
	a.data.OtsBitfield = make([][]byte, c.Config{}.Dev.OtsBitFieldSize)
	for i := 0; i < int(c.Config{}.Dev.OtsBitFieldSize); i++ {
		a.data.OtsBitfield[i] = make([]byte, 8)
		for j := 0; j < 8; j++ {
			a.data.OtsBitfield[i][j] = otsBitfield[i][j]
		}
	}
	a.data.OtsCounter = otsCounter

	for tokenTxhash, token := range tokens {
		a.UpdateTokenBalance([]byte(tokenTxhash), token)
	}

	for slavePK, accessType := range slavePksAccessType {
		a.AddSlavePKSAccessType([]byte(slavePK), accessType)
	}

	return a
}

func GetDefaultAddressState(address []byte) *AddressState {
	config := c.Config{}
	var otsBitfield [config.Dev.OtsBitFieldSize][8]byte
	var tokens map[string]uint64
	var slavePksAccessType map[string]uint32
	return CreateAddressState(address, uint64(config.Dev.DefaultNonce), config.Dev.DefaultAccountBalance, otsBitfield, tokens, slavePksAccessType, 0)
}

func (a *AddressState) Serialize() ([]byte, error) {
	return proto.Marshal(a.data)
}

func DeSerializeAddressState(data []byte) (*AddressState, error) {
	a := &AddressState{}

	if err := proto.Unmarshal(data, a.data); err != nil {
		return a, err
	}

	return a, nil
}
