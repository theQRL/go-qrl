package mongodb

import (
	"fmt"
	"github.com/theQRL/go-qrl/pkg/generated"
	"github.com/theQRL/go-qrl/pkg/misc"
	"github.com/theQRL/gq00/pkg/config"
	"reflect"
)

type Transaction struct {
	MasterAddress   []byte `json:"master_address" bson:"master_address"`
	Fee             int64  `json:"fee" bson:"fee"`
	AddressFrom     []byte `json:"address_from" bson:"address_from"`
	PublicKey       []byte `json:"public_key" bson:"public_key"`
	Signature       []byte `json:"signature" bson:"signature"`
	Nonce           int64  `json:"nonce" bson:"nonce"`
	TransactionHash []byte `json:"transaction_hash" bson:"transaction_hash"`
	TransactionType int8   `json:"transaction_type" bson:"transaction_type"`
	BlockNumber     int64  `json:"transaction_type" bson:"block_number"`
}

func (t *Transaction) TransactionFromPBData(tx *generated.Transaction, blockNumber uint64, txType int8) {
	t.MasterAddress = tx.MasterAddr
	t.TransactionType = txType
	if t.TransactionType > 0 {
		t.Fee = int64(tx.Fee)
		t.AddressFrom = misc.PK2BinAddress(tx.PublicKey)
		t.PublicKey = tx.PublicKey
		t.Signature = tx.Signature
	} else {
		t.AddressFrom = config.GetConfig().Dev.Genesis.CoinbaseAddress
	}
	t.Nonce = int64(tx.Nonce)
	t.TransactionHash = tx.TransactionHash
	t.BlockNumber = int64(blockNumber)
}

func (t *Transaction) Apply(m *MongoProcessor, accounts map[string]*Account, txDetails TransactionInterface, blockNumber int64) {
	var qAddress string
	if t.MasterAddress != nil {
		LoadAccount(m, t.MasterAddress, accounts, blockNumber)
		qAddress = misc.Bin2Qaddress(t.MasterAddress)
	} else {
		LoadAccount(m, t.AddressFrom, accounts, blockNumber)
		qAddress = misc.Bin2Qaddress(t.AddressFrom)
	}
	accounts[qAddress].SubBalance(uint64(t.Fee))
	accounts[qAddress].BlockNumber = blockNumber

	LoadAccount(m, t.AddressFrom, accounts, blockNumber)
	if t.TransactionType > 0 {
		qAddress = misc.Bin2Qaddress(t.AddressFrom)
		accounts[qAddress].SetOTSKey(misc.OTSKeyFromSig(t.Signature))
	}
	accounts[qAddress].Nonce = t.Nonce
	accounts[qAddress].BlockNumber = blockNumber
	txDetails.Apply(m, accounts, t.MasterAddress, t.AddressFrom, blockNumber)
}

func (t *Transaction) Revert(m *MongoProcessor, accounts map[string]*Account, txDetails TransactionInterface, blockNumber int64) {
	var qAddress string
	if t.MasterAddress != nil {
		//TODO: Load Account
		LoadAccount(m, t.MasterAddress, accounts, blockNumber)
		qAddress = misc.Bin2Qaddress(t.MasterAddress)
	} else {
		LoadAccount(m, t.AddressFrom, accounts, blockNumber)
		qAddress = misc.Bin2Qaddress(t.AddressFrom)
	}
	accounts[qAddress].AddBalance(uint64(t.Fee))
	accounts[qAddress].BlockNumber = blockNumber - 1

	LoadAccount(m, t.AddressFrom, accounts, blockNumber)
	if t.TransactionType > 0 {
		qAddress = misc.Bin2Qaddress(t.AddressFrom)
		accounts[qAddress].ResetOTSKey(misc.OTSKeyFromSig(t.Signature))
	}
	accounts[qAddress].Nonce = t.Nonce - 1
	accounts[qAddress].BlockNumber = blockNumber - 1
	txDetails.Revert(m, accounts, t.MasterAddress, t.AddressFrom, blockNumber)
}

func (t *Transaction) IsEqualPBData(tx *generated.Transaction, blockNumber uint64, txType int8) bool {
	tx2 := &Transaction{}
	tx2.TransactionFromPBData(tx, blockNumber, txType)
	return t.IsEqual(tx2)
}

func (t *Transaction) IsEqual(tx *Transaction) bool {
	if !reflect.DeepEqual(t.MasterAddress, tx.MasterAddress) {
		fmt.Println("Mismatch Master Address")
		return false
	}
	if t.Fee != tx.Fee {
		fmt.Println("Mismatch Fee", t.Fee, tx.Fee)
		return false
	}
	if !reflect.DeepEqual(t.PublicKey, tx.PublicKey) {
		fmt.Println("Mismatch Public Key")
		return false
	}
	if !reflect.DeepEqual(t.Signature, tx.Signature) {
		fmt.Println("Mismatch Signature")
		return false
	}
	if t.Nonce != tx.Nonce {
		fmt.Println("Mismatch Nonce")
		return false
	}
	if !reflect.DeepEqual(t.TransactionHash, tx.TransactionHash) {
		fmt.Println("Mismatch Transaction Hash")
		return false
	}
	if t.BlockNumber != tx.BlockNumber {
		fmt.Println("Mismatch BlockNumber")
		return false
	}
	return true
}

type TransferTransaction struct {
	TransactionHash []byte   `json:"transaction_hash" bson:"transaction_hash"`
	AddressesTo     [][]byte `json:"addresses_to" bson:"addresses_to"`
	Amounts         []int64 `json:"amounts" bson:"amounts"`
}

func (t *TransferTransaction) TransactionFromPBData(tx *generated.Transaction) {
	t.TransactionHash = tx.TransactionHash
	t.AddressesTo = tx.GetTransfer().AddrsTo
	for _, amount := range tx.GetTransfer().Amounts {
		t.Amounts = append(t.Amounts, int64(amount))
	}
}

func (t *TransferTransaction) IsEqualPBData(tx *generated.Transaction) bool {
	tx2 := &TransferTransaction{}
	tx2.TransactionFromPBData(tx)
	return t.IsEqual(tx2)
}

func (t *TransferTransaction) IsEqual(tx *TransferTransaction) bool {
	if !reflect.DeepEqual(t.TransactionHash, tx.TransactionHash) {
		return false
	}
	if !reflect.DeepEqual(t.AddressesTo, tx.AddressesTo) {
		return false
	}
	if !reflect.DeepEqual(t.Amounts, tx.Amounts) {
		return false
	}
	return true
}

func (t *TransferTransaction) Apply(m *MongoProcessor, accounts map[string]*Account, masterAddress []byte, addressFrom []byte, blockNumber int64) {
	var qAddress string
	var total int64
	for i := range t.AddressesTo {
		LoadAccount(m, t.AddressesTo[i], accounts, blockNumber)
		qAddress = misc.Bin2Qaddress(t.AddressesTo[i])
		accounts[qAddress].AddBalance(uint64(t.Amounts[i]))
		accounts[qAddress].BlockNumber = blockNumber
		total += t.Amounts[i]
	}
	if masterAddress != nil {
		qAddress = misc.Bin2Qaddress(masterAddress)
	} else {
		qAddress = misc.Bin2Qaddress(addressFrom)
	}
	accounts[qAddress].SubBalance(uint64(total))
	accounts[qAddress].BlockNumber = blockNumber
}

func (t *TransferTransaction) Revert(m *MongoProcessor, accounts map[string]*Account, masterAddress []byte, addressFrom []byte, blockNumber int64) {
	var qAddress string
	var total int64
	for i := range t.AddressesTo {
		LoadAccount(m, t.AddressesTo[i], accounts, blockNumber)
		qAddress = misc.Bin2Qaddress(t.AddressesTo[i])
		accounts[qAddress].SubBalance(uint64(t.Amounts[i]))
		accounts[qAddress].BlockNumber = blockNumber - 1
		total += t.Amounts[i]
	}
	if masterAddress != nil {
		qAddress = misc.Bin2Qaddress(masterAddress)
	} else {
		qAddress = misc.Bin2Qaddress(addressFrom)
	}
	accounts[qAddress].AddBalance(uint64(total))
	accounts[qAddress].BlockNumber = blockNumber - 1
}


type CoinBaseTransaction struct {
	TransactionHash []byte `json:"transaction_hash" bson:"transaction_hash"`
	AddressTo       []byte `json:"address_to" bson:"address_to"`
	Amount          int64 `json:"amount" bson:"amount"`
}

func (t *CoinBaseTransaction) TransactionFromPBData(tx *generated.Transaction) {
	t.TransactionHash = tx.TransactionHash
	t.AddressTo = tx.GetCoinbase().AddrTo
	t.Amount = int64(tx.GetCoinbase().Amount)
}

func (t *CoinBaseTransaction) IsEqualPBData(tx *generated.Transaction) bool {
	tx2 := &CoinBaseTransaction{}
	tx2.TransactionFromPBData(tx)
	return t.IsEqual(tx2)
}

func (t *CoinBaseTransaction) IsEqual(tx *CoinBaseTransaction) bool {
	if !reflect.DeepEqual(t.TransactionHash, tx.TransactionHash) {
		return false
	}
	if !reflect.DeepEqual(t.AddressTo, tx.AddressTo) {
		return false
	}
	if !reflect.DeepEqual(t.Amount, tx.Amount) {
		return false
	}
	return true
}

func (t *CoinBaseTransaction) Apply(m *MongoProcessor, accounts map[string]*Account, masterAddress []byte, addressFrom []byte, blockNumber int64) {
	var qAddress string

	qAddress = misc.Bin2Qaddress(masterAddress)
	accounts[qAddress].SubBalance(uint64(t.Amount))
	accounts[qAddress].BlockNumber = blockNumber

	LoadAccount(m, t.AddressTo, accounts, blockNumber)
	qAddress = misc.Bin2Qaddress(t.AddressTo)
	accounts[qAddress].AddBalance(uint64(t.Amount))
	accounts[qAddress].BlockNumber = blockNumber
}

func (t *CoinBaseTransaction) Revert(m *MongoProcessor, accounts map[string]*Account, masterAddress []byte, addressFrom []byte, blockNumber int64) {
	var qAddress string

	qAddress = misc.Bin2Qaddress(masterAddress)
	accounts[qAddress].AddBalance(uint64(t.Amount))
	accounts[qAddress].BlockNumber = blockNumber - 1

	LoadAccount(m, t.AddressTo, accounts, blockNumber)
	qAddress = misc.Bin2Qaddress(t.AddressTo)
	accounts[qAddress].SubBalance(uint64(t.Amount))
	accounts[qAddress].BlockNumber = blockNumber - 1
}

type MessageTransaction struct {
	TransactionHash []byte `json:"transaction_hash" bson:"transaction_hash"`
	MessageHash     []byte `json:"message_hash" bson:"message_hash"`
}

func (t *MessageTransaction) TransactionFromPBData(tx *generated.Transaction) {
	t.TransactionHash = tx.TransactionHash
	t.MessageHash = tx.GetMessage().MessageHash
}

func (t *MessageTransaction) IsEqualPBData(tx *generated.Transaction) bool {
	tx2 := &MessageTransaction{}
	tx2.TransactionFromPBData(tx)
	return t.IsEqual(tx2)
}

func (t *MessageTransaction) IsEqual(tx *MessageTransaction) bool {
	if !reflect.DeepEqual(t.TransactionHash, tx.TransactionHash) {
		return false
	}
	if !reflect.DeepEqual(t.MessageHash, tx.MessageHash) {
		return false
	}
	return true
}

func (t *MessageTransaction) Apply(m *MongoProcessor, accounts map[string]*Account, masterAddress []byte, addressFrom []byte, blockNumber int64) {
}

func (t *MessageTransaction) Revert(m *MongoProcessor, accounts map[string]*Account, masterAddress []byte, addressFrom []byte, blockNumber int64) {
}

type SlaveTransaction struct {
	TransactionHash []byte   `json:"transaction_hash" bson:"transaction_hash"`
	SlavePKs        [][]byte `json:"slave_public_keys" bson:"slave_public_keys"`
	AccessTypes     []int32  `json:"access_types" bson:"access_types"`
}

func (t *SlaveTransaction) TransactionFromPBData(tx *generated.Transaction) {
	slave := tx.GetSlave()
	t.TransactionHash = tx.TransactionHash
	t.SlavePKs = slave.SlavePks
	for _, accessTypes := range tx.GetSlave().AccessTypes {
		t.AccessTypes = append(t.AccessTypes, int32(accessTypes))
	}
}

func (t *SlaveTransaction) IsEqualPBData(tx *generated.Transaction) bool {
	tx2 := &SlaveTransaction{}
	tx2.TransactionFromPBData(tx)
	return t.IsEqual(tx2)
}

func (t *SlaveTransaction) IsEqual(tx *SlaveTransaction) bool {
	if !reflect.DeepEqual(t.TransactionHash, tx.TransactionHash) {
		return false
	}
	if !reflect.DeepEqual(t.SlavePKs, tx.SlavePKs) {
		return false
	}
	if !reflect.DeepEqual(t.AccessTypes, tx.AccessTypes) {
		return false
	}
	return true
}

func (t *SlaveTransaction) Apply(m *MongoProcessor, accounts map[string]*Account, masterAddress []byte, addressFrom []byte, blockNumber int64) {
}

func (t *SlaveTransaction) Revert(m *MongoProcessor, accounts map[string]*Account, masterAddress []byte, addressFrom []byte, blockNumber int64) {
}

type TokenTransaction struct {
	TransactionHash []byte   `json:"transaction_hash" bson:"transaction_hash"`
	Symbol          []byte   `json:"symbol" bson:"symbol"`
	Name            []byte   `json:"name" bson:"name"`
	Owner           []byte   `json:"owner" bson:"owner"`
	Decimals        int64   `json:"decimals" bson:"decimals"`
	AddressesTo     [][]byte `json:"addresses_to" bson:"addresses_to"`
	Amounts         []int64 `json:"amounts" bson:"amounts"`
}

func (t *TokenTransaction) TransactionFromPBData(tx *generated.Transaction) {
	token := tx.GetToken()
	t.TransactionHash = tx.TransactionHash
	t.Symbol = token.Symbol
	t.Name = token.Name
	t.Owner = token.Owner
	t.Decimals = int64(token.Decimals)
	for _, initialBalance := range token.InitialBalances {
		t.AddressesTo = append(t.AddressesTo, initialBalance.Address)
		t.Amounts = append(t.Amounts, int64(initialBalance.Amount))
	}
}

func (t *TokenTransaction) IsEqualPBData(tx *generated.Transaction) bool {
	tx2 := &TokenTransaction{}
	tx2.TransactionFromPBData(tx)
	return t.IsEqual(tx2)
}

func (t *TokenTransaction) IsEqual(tx *TokenTransaction) bool {
	if !reflect.DeepEqual(t.TransactionHash, tx.TransactionHash) {
		return false
	}
	if !reflect.DeepEqual(t.Symbol, tx.Symbol) {
		return false
	}
	if !reflect.DeepEqual(t.Name, tx.Name) {
		return false
	}
	if !reflect.DeepEqual(t.Owner, tx.Owner) {
		return false
	}
	if t.Decimals != tx.Decimals {
		return false
	}
	if !reflect.DeepEqual(t.AddressesTo, tx.AddressesTo) {
		return false
	}
	if !reflect.DeepEqual(t.Amounts, tx.Amounts) {
		return false
	}
	return true
}

func (t *TokenTransaction) Apply(m *MongoProcessor, accounts map[string]*Account, masterAddress []byte, addressFrom []byte, blockNumber int64) {
}

func (t *TokenTransaction) Revert(m *MongoProcessor, accounts map[string]*Account, masterAddress []byte, addressFrom []byte, blockNumber int64) {
}

type TransferTokenTransaction struct {
	TransactionHash []byte   `json:"transaction_hash" bson:"transaction_hash"`
	TokenTxnHash    []byte   `json:"token_txn_hash" bson:"token_txn_hash"`
	AddressesTo     [][]byte `json:"addresses_to" bson:"addresses_to"`
	Amounts         []int64  `json:"amounts" bson:"amounts"`
}

func (t *TransferTokenTransaction) TransactionFromPBData(tx *generated.Transaction) {
	transferToken := tx.GetTransferToken()
	t.TransactionHash = tx.TransactionHash
	t.AddressesTo = transferToken.AddrsTo
	for _, amount := range tx.GetTransferToken().Amounts {
		t.Amounts = append(t.Amounts, int64(amount))
	}
	t.TokenTxnHash = transferToken.TokenTxhash
}

func (t *TransferTokenTransaction) IsEqualPBData(tx *generated.Transaction) bool {
	tx2 := &TransferTokenTransaction{}
	tx2.TransactionFromPBData(tx)
	return t.IsEqual(tx2)
}

func (t *TransferTokenTransaction) IsEqual(tx *TransferTokenTransaction) bool {
	if !reflect.DeepEqual(t.TransactionHash, tx.TransactionHash) {
		return false
	}
	if !reflect.DeepEqual(t.TokenTxnHash, tx.TokenTxnHash) {
		return false
	}
	if !reflect.DeepEqual(t.AddressesTo, tx.AddressesTo) {
		return false
	}
	if !reflect.DeepEqual(t.Amounts, tx.Amounts) {
		return false
	}
	return true
}

func (t *TransferTokenTransaction) Apply(m *MongoProcessor, accounts map[string]*Account, masterAddress []byte, addressFrom []byte, blockNumber int64) {
}

func (t *TransferTokenTransaction) Revert(m *MongoProcessor, accounts map[string]*Account, masterAddress []byte, addressFrom []byte, blockNumber int64) {
}

type TransactionInterface interface {
	TransactionFromPBData(tx *generated.Transaction)
	Apply(m *MongoProcessor, accounts map[string]*Account, masterAddress []byte, addressFrom []byte, blockNumber int64)
	Revert(m *MongoProcessor, accounts map[string]*Account, masterAddress []byte, addressFrom []byte, blockNumber int64)
}

func ProtoToTransaction(tx *generated.Transaction, blockNumber uint64) (*Transaction, TransactionInterface){
	t := &Transaction{}

	var t2 TransactionInterface
	var txType int8
	switch tx.TransactionType.(type) {
	case *generated.Transaction_Coinbase:
		txType = 0
		t2 = &CoinBaseTransaction{}
		t2.TransactionFromPBData(tx)
	case *generated.Transaction_Transfer_:
		txType = 1
		t2 = &TransferTransaction{}
		t2.TransactionFromPBData(tx)
	case *generated.Transaction_Token_:
		txType = 2
		t2 = &TokenTransaction{}
		t2.TransactionFromPBData(tx)
	case *generated.Transaction_TransferToken_:
		txType = 3
		t2 = &TransferTokenTransaction{}
		t2.TransactionFromPBData(tx)
	case *generated.Transaction_Message_:
		txType = 4
		t2 = &MessageTransaction{}
		t2.TransactionFromPBData(tx)
	case *generated.Transaction_Slave_:
		txType = 5
		t2 = &SlaveTransaction{}
		t2.TransactionFromPBData(tx)
	default:
		fmt.Println("Not matched with any txn type")
	}
	t.TransactionFromPBData(tx, blockNumber, txType)
	return t, t2
}
