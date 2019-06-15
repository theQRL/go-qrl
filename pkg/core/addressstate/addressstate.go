package addressstate

import (
	"encoding/base64"
	"github.com/theQRL/go-qrl/pkg/log"
	"reflect"

	"github.com/golang/protobuf/proto"

	c "github.com/theQRL/go-qrl/pkg/config"
	"github.com/theQRL/go-qrl/pkg/generated"
	"github.com/theQRL/go-qrl/pkg/misc"
	"github.com/theQRL/qrllib/goqrllib/goqrllib"
)

type AddressStateInterface interface {
	PBData() *generated.AddressState

	Address() []byte

	Nonce() uint64

	Balance() uint64

	SetBalance(balance uint64)

	AddBalance(balance uint64)

	SubtractBalance(balance uint64)

	OTSBitfield() [][]byte

	OTSCounter() uint64

	TransactionHashes() [][]byte

	LatticePKList() []*generated.LatticePK

	SlavePKSAccessType() map[string]uint32

	UpdateTokenBalance(tokenTxHash []byte, balance uint64, subtract bool)

	GetTokenBalance(tokenTxHash []byte) uint64

	IsTokenExists(tokenTxHash []byte) bool

	AddSlavePKSAccessType(slavePK []byte, accessType uint32)

	RemoveSlavePKSAccessType(slavePK []byte)

	IncreaseNonce()

	DecreaseNonce()

	GetSlavePermission(slavePK []byte) (uint16, bool)

	GetDefault(address []byte) *AddressState

	OTSKeyReuse(otsKeyIndex uint64) bool

	SetOTSKey(otsKeyIndex uint64)

	IsValidAddress(address []byte) bool

	Serialize() string
}

type PlainAddressState struct {
	Address           string   `json:"address" bson:"address"`
	Balance           uint64   `json:"balance" bson:"balance"`
	Nonce             uint64   `json:"nonce" bson:"nonce"`
	OtsBitfield       []string `json:"otsBitField" bson:"otsBitField"`
	TransactionHashes []string `json:"transactionHashes" bson:"transactionHashes"`
	OtsCounter        uint64   `json:"otsCounter" bson:"otsCounter"`
}

type PlainBalance struct {
	Balance string `json:"balance" bson:"balance"`
}

type NextUnusedOTS struct {
	UnusedOTSIndex uint64 `json:"unusedOtsIndex" bson:"unusedOtsIndex"`
	Found          bool   `json:"found" bson:"found"`
}

type IsUnusedOTSIndex struct {
	IsUnusedOTSIndex bool `json:"isUnusedOtsIndex" bson:"isUnusedOtsIndex"`
}

func (a *PlainAddressState) AddressStateFromPBData(a2 *generated.AddressState) {
	a.Address = misc.Bin2Qaddress(a2.Address)
	a.Balance = a2.Balance
	a.Nonce = a2.Nonce
	for i := 0; i < int(c.GetConfig().Dev.OtsBitFieldSize); i++ {
		a.OtsBitfield = append(a.OtsBitfield, base64.StdEncoding.EncodeToString(a2.OtsBitfield[i]))
	}

	for _, txHashes := range a2.TransactionHashes {
		a.TransactionHashes = append(a.TransactionHashes, misc.Bin2HStr(txHashes))
	}

	a.OtsCounter = a2.OtsCounter
}

type AddressState struct {
	data *generated.AddressState
}

func (a *AddressState) PBData() *generated.AddressState {
	return a.data
}

func (a *AddressState) Address() []byte {
	return a.data.Address
}

func (a *AddressState) Height() uint64 {
	return uint64(a.data.Address[1] << 1)
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

func (a *AddressState) SubtractBalance(balance uint64) {
	a.data.Balance -= balance
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
		if reflect.DeepEqual(hash, hash1) {
			a.data.TransactionHashes = append(a.data.TransactionHashes[:index], a.data.TransactionHashes[index+1:]...)
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

func (a *AddressState) UpdateTokenBalance(tokenTxHash []byte, balance uint64, subtract bool) {
	strTokenTxHash := misc.Bin2HStr(tokenTxHash)
	if a.data.Tokens == nil {
		a.data.Tokens = make(map[string]uint64)
	}
	if subtract {
		a.data.Tokens[strTokenTxHash] -= balance
	} else {
		a.data.Tokens[strTokenTxHash] += balance
	}

	if a.data.Tokens[strTokenTxHash] == 0 {
		delete(a.data.Tokens, strTokenTxHash)
	}
}

func (a *AddressState) GetTokenBalance(tokenTxHash []byte) uint64 {
	strTokenTxHash := misc.Bin2HStr(tokenTxHash)
	if balance, ok := a.data.Tokens[strTokenTxHash]; ok {
		return balance
	}
	return 0
}

func (a *AddressState) IsTokenExists(tokenTxHash []byte) bool {
	strTokenTxHash := misc.Bin2HStr(tokenTxHash)
	_, ok := a.data.Tokens[strTokenTxHash]
	return ok
}

func (a *AddressState) AddSlavePKSAccessType(slavePK []byte, accessType uint32) {
	if a.data.SlavePksAccessType == nil {
		a.data.SlavePksAccessType = make(map[string]uint32)
	}
	a.data.SlavePksAccessType[string(slavePK)] = accessType
}

func (a *AddressState) RemoveSlavePKSAccessType(slavePK []byte) {
	delete(a.data.SlavePksAccessType, string(slavePK))
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

func (a *AddressState) GetUnusedOTSIndex(startFrom uint64) (uint64, bool) {
	maxOTSIndex := a.Height()
	for ; startFrom < maxOTSIndex; startFrom++ {
		if !a.OTSKeyReuse(startFrom) {
			return startFrom, true
		}
	}
	return 0, false
}

func (a *AddressState) OTSKeyReuse(otsKeyIndex uint64) bool {
	if otsKeyIndex < uint64(c.GetConfig().Dev.MaxOTSTracking) {
		offset := otsKeyIndex >> 3
		relative := otsKeyIndex % 8
		if (a.data.OtsBitfield[offset][0]>>relative)&1 == 1 {
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
	if otsKeyIndex < c.GetConfig().Dev.MaxOTSTracking {
		offset := otsKeyIndex >> 3
		relative := otsKeyIndex % 8
		bitfield := a.data.OtsBitfield[offset]
		a.data.OtsBitfield[offset][0] = bitfield[0] | (1 << relative)
	} else {
		a.data.OtsCounter = otsKeyIndex
	}
}

func IsValidAddress(address []byte) bool {
	// Warning: Never pass this validation True for Coinbase Address
	if goqrllib.QRLHelperAddressIsValid(misc.BytesToUCharVector(address)) {
		return true
	}
	return false
}

func CreateAddressState(address []byte, nonce uint64, balance uint64, otsBitfield [][]byte, tokens map[string]uint64, slavePksAccessType map[string]uint32, otsCounter uint64) *AddressState {
	a := &AddressState{&generated.AddressState{}}
	a.data.Address = address
	a.data.Nonce = nonce
	a.data.Balance = balance
	a.data.OtsBitfield = otsBitfield
	for i := 0; i < int(c.GetConfig().Dev.OtsBitFieldSize); i++ {
		if a.data.OtsBitfield[i] == nil {
			a.data.OtsBitfield[i] = make([]byte, 8)
			for j := 0; j < 8; j++ {
				a.data.OtsBitfield[i][j] = otsBitfield[i][j]
			}
		}
	}
	a.data.OtsCounter = otsCounter

	a.data.Tokens = make(map[string]uint64)
	for tokenTxhash, token := range tokens {
		a.UpdateTokenBalance([]byte(tokenTxhash), token, false)
	}

	a.data.SlavePksAccessType = make(map[string]uint32)
	for slavePK, accessType := range slavePksAccessType {
		a.AddSlavePKSAccessType([]byte(slavePK), accessType)
	}

	return a
}

func GetDefaultAddressState(address []byte) *AddressState {
	config := c.GetConfig()
	otsBitfield := make([][]byte, config.Dev.OtsBitFieldSize)

	var tokens map[string]uint64
	var slavePksAccessType map[string]uint32
	return CreateAddressState(address, uint64(config.Dev.DefaultNonce), config.Dev.DefaultAccountBalance, otsBitfield, tokens, slavePksAccessType, 0)
}

func (a *AddressState) Serialize() ([]byte, error) {
	data, err := proto.Marshal(a.data)
	if err != nil {
		l := log.GetLogger()
		l.Info("Error while serializing data",
			"error", err.Error())
		return nil, err
	}
	return data, err
}

func DeSerializeAddressState(data []byte) (*AddressState, error) {
	a := &AddressState{&generated.AddressState{}}

	if err := proto.Unmarshal(data, a.data); err != nil {
		return nil, err
	}

	return a, nil
}
