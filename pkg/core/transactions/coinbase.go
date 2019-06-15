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

type PlainCoinBaseTransaction struct {
	Fee             uint64 `json:"fee"`
	PublicKey       string `json:"publicKey"`
	Signature       string `json:"signature"`
	Nonce           uint64 `json:"nonce"`
	TransactionHash string `json:"transactionHash"`
	TransactionType string `json:"transactionType"`

	AddressTo string `json:"addressTo"`
	Amount    uint64 `json:"amount"`
}

func (t *PlainCoinBaseTransaction) TransactionFromPBData(tx *generated.Transaction) {
	t.Fee = tx.Fee
	t.PublicKey = misc.Bin2HStr(tx.PublicKey)
	t.Signature = misc.Bin2HStr(tx.Signature)
	t.Nonce = tx.Nonce
	t.TransactionHash = misc.Bin2HStr(tx.TransactionHash)
	t.TransactionType = "coinbase"
	t.AddressTo = misc.Bin2Qaddress(tx.GetCoinbase().AddrTo)
	t.Amount = tx.GetCoinbase().Amount
}

type CoinBase struct {
	Transaction
}

func (tx *CoinBase) AddrTo() []byte {
	return tx.data.GetCoinbase().AddrTo
}

func (tx *CoinBase) Amount() uint64 {
	return tx.data.GetCoinbase().GetAmount()
}

func (tx *CoinBase) GetHashableBytes() []byte {
	tmp := new(bytes.Buffer)
	tmp.Write(tx.MasterAddr())
	tmp.Write(tx.AddrTo())
	binary.Write(tmp, binary.BigEndian, uint64(tx.Nonce()))
	binary.Write(tmp, binary.BigEndian, uint64(tx.Amount()))

	tmptxhash := misc.NewUCharVector()
	tmptxhash.AddBytes(tmp.Bytes())
	tmptxhash.New(goqrllib.Sha2_256(tmptxhash.GetData()))

	return tmptxhash.GetBytes()
}

func (tx *CoinBase) UpdateMiningAddress(miningAddress []byte) {
	tx.data.GetCoinbase().AddrTo = miningAddress
	tx.data.TransactionHash = tx.GetHashableBytes()
}

func (tx *CoinBase) validateCustom() bool {
	if !reflect.DeepEqual(tx.MasterAddr(), tx.config.Dev.Genesis.CoinbaseAddress) {
		tx.log.Warn("Master address doesnt match with coinbase_address")
		return false
	}

	if tx.Fee() != 0 {
		tx.log.Warn("Fee for coinbase transaction should be 0")
		return false
	}

	return true
}

func (tx *CoinBase) ValidateExtendedCoinbase(blockNumber uint64) bool {
	if !reflect.DeepEqual(tx.MasterAddr(), tx.config.Dev.Genesis.CoinbaseAddress) {
		tx.log.Warn(
			"Master address doesnt match with coinbase_address",
			"MasterAddr", tx.MasterAddr(),
				"Expected MasterAddr", tx.config.Dev.Genesis.CoinbaseAddress)
		return false
	}

	if !addressstate.IsValidAddress(tx.AddrTo()) {
		tx.log.Warn(
			"Invalid address",
			"addr_from", misc.Bin2HStr(tx.MasterAddr()),
			"addr_to", misc.Bin2HStr(tx.AddrTo()))
		return false
	}

	if tx.Nonce() != blockNumber+1 {
		tx.log.Warn(
			"Nonce doesnt match",
			"nonce", tx.Nonce(),
			"blockNumber", blockNumber)
		return false
	}

	return tx.validateCustom()
}

func (tx *CoinBase) Validate(verifySignature bool) bool {
	if !tx.validateCustom() {
		tx.log.Warn("Custom validation failed")
		return false
	}

	expectedTransactionHash := tx.GetHashableBytes()
	if !reflect.DeepEqual(expectedTransactionHash, tx.Txhash()) {
		tx.log.Warn("CoinBase TransactionHash mismatch")
		return false
	}

	return true
}

func (tx *CoinBase) ApplyStateChanges(addressesState map[string]*addressstate.AddressState) {
	strAddrTo := misc.Bin2Qaddress(tx.AddrTo())
	if addrState, ok := addressesState[strAddrTo]; ok {
		addrState.AddBalance(tx.Amount())
		// Disabled Tracking of Transaction Hash into AddressState
		addrState.AppendTransactionHash(tx.Txhash())
	}

	strAddrFrom := misc.Bin2Qaddress(tx.config.Dev.Genesis.CoinbaseAddress)

	if addrState, ok := addressesState[strAddrFrom]; ok {
		masterQAddr := misc.Bin2Qaddress(tx.MasterAddr())
		addressesState[masterQAddr].SubtractBalance(tx.Amount())
		// Disabled Tracking of Transaction Hash into AddressState
		addressesState[masterQAddr].AppendTransactionHash(tx.Txhash())
		addrState.IncreaseNonce()
	}
}

func (tx *CoinBase) RevertStateChanges(addressesState map[string]*addressstate.AddressState) {
	strAddrTo := misc.Bin2Qaddress(tx.AddrTo())
	if addrState, ok := addressesState[strAddrTo]; ok {
		addrState.SubtractBalance(tx.Amount())
		// Disabled Tracking of Transaction Hash into AddressState
		addrState.RemoveTransactionHash(tx.Txhash())
	}

	strAddrFrom := misc.Bin2Qaddress(tx.config.Dev.Genesis.CoinbaseAddress)

	if addrState, ok := addressesState[strAddrFrom]; ok {
		masterQAddr := misc.Bin2Qaddress(tx.MasterAddr())
		addressesState[masterQAddr].AddBalance(tx.Amount())
		// Disabled Tracking of Transaction Hash into AddressState
		addressesState[masterQAddr].RemoveTransactionHash(tx.Txhash())
		addrState.DecreaseNonce()
	}
}

func (tx *CoinBase) SetAffectedAddress(addressesState map[string]*addressstate.AddressState) {
	addressesState[misc.Bin2Qaddress(tx.MasterAddr())] = nil
	addressesState[misc.Bin2Qaddress(tx.AddrTo())] = nil
}

func CreateCoinBase(minerAddress []byte, blockNumber uint64, amount uint64) *CoinBase {
	tx := &CoinBase{}
	tx.config = config.GetConfig()
	tx.log = log.GetLogger()

	tx.data = &generated.Transaction{}
	tx.data.TransactionType = &generated.Transaction_Coinbase{Coinbase: &generated.Transaction_CoinBase{}}

	tx.data.MasterAddr = tx.config.Dev.Genesis.CoinbaseAddress
	tx.data.GetCoinbase().AddrTo = minerAddress
	tx.data.GetCoinbase().Amount = amount
	tx.data.Nonce = blockNumber + 1
	tx.data.TransactionHash = tx.GetHashableBytes()

	if !tx.Validate(false) {
		return nil
	}

	return tx
}
