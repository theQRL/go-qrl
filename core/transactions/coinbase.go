package transactions

import (
	"bytes"
	"encoding/binary"
	"reflect"

	"github.com/theQRL/go-qrl/core/addressstate"
	"github.com/theQRL/go-qrl/generated"
	"github.com/theQRL/go-qrl/misc"
	"github.com/theQRL/qrllib/goqrllib/goqrllib"
)

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

	tmptxhash := misc.UcharVector{}
	tmptxhash.AddBytes(tmp.Bytes())
	tmptxhash.New(goqrllib.Sha2_256(tmptxhash.GetData()))

	return tmptxhash.GetBytes()
}

func (tx *CoinBase) UpdateMiningAddress(miningAddress []byte) {
	tx.data.GetCoinbase().AddrTo = miningAddress
	tx.data.TransactionHash = tx.GetHashableBytes()
}

func (tx *CoinBase) validateCustom() bool {
	if tx.Fee() != 0 {
		tx.log.Warn("Fee for coinbase transaction should be 0")
		return false
	}

	return true
}

func (tx *CoinBase) ValidateExtended(blockNumber uint64) bool {
	if reflect.DeepEqual(tx.MasterAddr(), tx.config.Dev.Genesis.CoinbaseAddress) {
		tx.log.Warn("Master address doesnt match with coinbase_address")
		tx.log.Warn(string(tx.MasterAddr()), tx.config.Dev.Genesis.CoinbaseAddress)
		return false
	}

	if !(addressstate.IsValidAddress(tx.MasterAddr()) && addressstate.IsValidAddress(tx.AddrTo())) {
		tx.log.Warn("Invalid address addr_from: %s addr_to: %s", tx.MasterAddr(), tx.AddrTo())
		return false
	}

	return tx.validateCustom()
}

func (tx *CoinBase) ApplyStateChanges(addressesState map[string]*addressstate.AddressState) {
	strAddrTo := string(tx.AddrTo())
	if addrState, ok := addressesState[strAddrTo]; ok {
		addrState.AddBalance(tx.Amount())
		addrState.AppendTransactionHash(tx.Txhash())
	}

	strAddrFrom := string(tx.config.Dev.Genesis.CoinbaseAddress)

	if addrState, ok := addressesState[strAddrFrom]; ok {
		addressesState[string(tx.MasterAddr())].AddBalance(tx.Amount() * -1)
		addressesState[string(tx.MasterAddr())].AppendTransactionHash(tx.Txhash())
		addrState.IncreaseNonce()
	}
}

func (tx *CoinBase) RevertStateChanges(addressesState map[string]*addressstate.AddressState) {
	strAddrTo := string(tx.AddrTo())
	if addrState, ok := addressesState[strAddrTo]; ok {
		addrState.AddBalance(tx.Amount() * -1)
		addrState.RemoveTransactionHash(tx.Txhash())
	}

	strAddrFrom := string(tx.config.Dev.Genesis.CoinbaseAddress)

	if addrState, ok := addressesState[strAddrFrom]; ok {
		addressesState[string(tx.MasterAddr())].AddBalance(tx.Amount())
		addressesState[string(tx.MasterAddr())].RemoveTransactionHash(tx.Txhash())
		addrState.DecreaseNonce()
	}
}

func (tx *CoinBase) SetAffectedAddress(addressesState map[string]*addressstate.AddressState) {
	addressesState[string(tx.AddrFrom())] = &addressstate.AddressState{}
	addressesState[string(tx.PK())] = &addressstate.AddressState{}

	addressesState[string(tx.MasterAddr())] = &addressstate.AddressState{}
	addressesState[string(tx.AddrTo())] = &addressstate.AddressState{}
}

func (tx *CoinBase) FromPBData(transaction *generated.Transaction) *CoinBase {
	tx.data = transaction
	return tx
}

func CreateCoinBase(minerAddress []byte, blockNumber uint64, amount uint64) *CoinBase {
	tx := &CoinBase{}
	tx.data.MasterAddr = tx.config.Dev.Genesis.CoinbaseAddress
	tx.data.GetCoinbase().AddrTo = minerAddress
	tx.data.GetCoinbase().Amount = amount
	tx.data.Nonce = blockNumber + 1
	tx.data.TransactionHash = tx.GetHashableBytes()

	if !tx.Validate(misc.BytesToUCharVector(tx.GetHashableBytes()), false) {
		return nil
	}

	return tx
}
