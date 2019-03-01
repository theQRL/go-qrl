package mongodb

import (
	"fmt"
	"github.com/theQRL/go-qrl/pkg/config"
	"github.com/theQRL/go-qrl/pkg/generated"
	"github.com/theQRL/go-qrl/pkg/misc"
	"reflect"
)

type UnconfirmedTransaction struct {
	MasterAddress   []byte `json:"master_address" bson:"master_address"`
	Fee             int64  `json:"fee" bson:"fee"`
	AddressFrom     []byte `json:"address_from" bson:"address_from"`
	PublicKey       []byte `json:"public_key" bson:"public_key"`
	Signature       []byte `json:"signature" bson:"signature"`
	Nonce           int64  `json:"nonce" bson:"nonce"`
	TransactionHash []byte `json:"transaction_hash" bson:"transaction_hash"`
	TransactionType int8   `json:"transaction_type" bson:"transaction_type"`
	SeenTimestamp   int64  `json:"seen_timestamp" bson:"seen_timestamp"`
}

func (t *UnconfirmedTransaction) TransactionFromPBData(tx *generated.Transaction, seenTimestamp uint64, txType int8) {
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
	t.SeenTimestamp = int64(seenTimestamp)
}

func (t *UnconfirmedTransaction) Apply(m *MongoProcessor, txDetails UnconfirmedTransactionInterface) {
}

func (t *UnconfirmedTransaction) Revert(m *MongoProcessor, txDetails UnconfirmedTransactionInterface) {
}

func (t *UnconfirmedTransaction) IsEqualPBData(tx *generated.Transaction, seenTimestamp uint64, txType int8) bool {
	tx2 := &UnconfirmedTransaction{}
	tx2.TransactionFromPBData(tx, seenTimestamp, txType)
	return t.IsEqual(tx2)
}

func (t *UnconfirmedTransaction) IsEqual(tx *UnconfirmedTransaction) bool {
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
	if t.SeenTimestamp != tx.SeenTimestamp {
		fmt.Println("Mismatch Seen Timestamp")
		return false
	}
	return true
}

type UnconfirmedTransferTransaction struct {
	TransactionHash []byte   `json:"transaction_hash" bson:"transaction_hash"`
	AddressesTo     [][]byte `json:"addresses_to" bson:"addresses_to"`
	Amounts         []int64  `json:"amounts" bson:"amounts"`
}

func (t *UnconfirmedTransferTransaction) TransactionFromPBData(tx *generated.Transaction) {
	t.TransactionHash = tx.TransactionHash
	t.AddressesTo = tx.GetTransfer().AddrsTo
	for _, amount := range tx.GetTransfer().Amounts {
		t.Amounts = append(t.Amounts, int64(amount))
	}
}

func (t *UnconfirmedTransferTransaction) IsEqualPBData(tx *generated.Transaction) bool {
	tx2 := &UnconfirmedTransferTransaction{}
	tx2.TransactionFromPBData(tx)
	return t.IsEqual(tx2)
}

func (t *UnconfirmedTransferTransaction) IsEqual(tx *UnconfirmedTransferTransaction) bool {
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

func (t *UnconfirmedTransferTransaction) Apply(m *MongoProcessor, masterAddress []byte, addressFrom []byte) {
}

func (t *UnconfirmedTransferTransaction) Revert(m *MongoProcessor, masterAddress []byte, addressFrom []byte) {
}

type UnconfirmedMessageTransaction struct {
	TransactionHash []byte `json:"transaction_hash" bson:"transaction_hash"`
	MessageHash     []byte `json:"message_hash" bson:"message_hash"`
}

func (t *UnconfirmedMessageTransaction) TransactionFromPBData(tx *generated.Transaction) {
	t.TransactionHash = tx.TransactionHash
	t.MessageHash = tx.GetMessage().MessageHash
}

func (t *UnconfirmedMessageTransaction) IsEqualPBData(tx *generated.Transaction) bool {
	tx2 := &UnconfirmedMessageTransaction{}
	tx2.TransactionFromPBData(tx)
	return t.IsEqual(tx2)
}

func (t *UnconfirmedMessageTransaction) IsEqual(tx *UnconfirmedMessageTransaction) bool {
	if !reflect.DeepEqual(t.TransactionHash, tx.TransactionHash) {
		return false
	}
	if !reflect.DeepEqual(t.MessageHash, tx.MessageHash) {
		return false
	}
	return true
}

func (t *UnconfirmedMessageTransaction) Apply(m *MongoProcessor, masterAddress []byte, addressFrom []byte) {
}

func (t *UnconfirmedMessageTransaction) Revert(m *MongoProcessor, masterAddress []byte, addressFrom []byte) {
}

type UnconfirmedSlaveTransaction struct {
	TransactionHash []byte   `json:"transaction_hash" bson:"transaction_hash"`
	SlavePKs        [][]byte `json:"slave_public_keys" bson:"slave_public_keys"`
	AccessTypes     []int32  `json:"access_types" bson:"access_types"`
}

func (t *UnconfirmedSlaveTransaction) TransactionFromPBData(tx *generated.Transaction) {
	slave := tx.GetSlave()
	t.TransactionHash = tx.TransactionHash
	t.SlavePKs = slave.SlavePks
	for _, accessTypes := range tx.GetSlave().AccessTypes {
		t.AccessTypes = append(t.AccessTypes, int32(accessTypes))
	}
}

func (t *UnconfirmedSlaveTransaction) IsEqualPBData(tx *generated.Transaction) bool {
	tx2 := &UnconfirmedSlaveTransaction{}
	tx2.TransactionFromPBData(tx)
	return t.IsEqual(tx2)
}

func (t *UnconfirmedSlaveTransaction) IsEqual(tx *UnconfirmedSlaveTransaction) bool {
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

func (t *UnconfirmedSlaveTransaction) Apply(m *MongoProcessor, masterAddress []byte, addressFrom []byte) {
}

func (t *UnconfirmedSlaveTransaction) Revert(m *MongoProcessor, masterAddress []byte, addressFrom []byte) {
}

type UnconfirmedTokenTransaction struct {
	TransactionHash []byte   `json:"transaction_hash" bson:"transaction_hash"`
	Symbol          []byte   `json:"symbol" bson:"symbol"`
	SymbolStr		string	 `json:"symbol" bson:"symbol_str"`
	Name            []byte   `json:"name" bson:"name"`
	NameStr			string	 `json:"symbol" bson:"name_str"`
	Owner           []byte   `json:"owner" bson:"owner"`
	Decimals        int64    `json:"decimals" bson:"decimals"`
	AddressesTo     [][]byte `json:"addresses_to" bson:"addresses_to"`
	Amounts         []int64  `json:"amounts" bson:"amounts"`
}

func (t *UnconfirmedTokenTransaction) TransactionFromPBData(tx *generated.Transaction) {
	token := tx.GetToken()
	t.TransactionHash = tx.TransactionHash
	t.Symbol = token.Symbol
	t.SymbolStr = misc.BytesToString(token.Symbol)

	t.Name = token.Name
	t.NameStr = misc.BytesToString(token.Name)

	t.Owner = token.Owner
	t.Decimals = int64(token.Decimals)
	for _, initialBalance := range token.InitialBalances {
		t.AddressesTo = append(t.AddressesTo, initialBalance.Address)
		t.Amounts = append(t.Amounts, int64(initialBalance.Amount))
	}
}

func (t *UnconfirmedTokenTransaction) IsEqualPBData(tx *generated.Transaction) bool {
	tx2 := &UnconfirmedTokenTransaction{}
	tx2.TransactionFromPBData(tx)
	return t.IsEqual(tx2)
}

func (t *UnconfirmedTokenTransaction) IsEqual(tx *UnconfirmedTokenTransaction) bool {
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

func (t *UnconfirmedTokenTransaction) Apply(m *MongoProcessor, masterAddress []byte, addressFrom []byte) {
}

func (t *UnconfirmedTokenTransaction) Revert(m *MongoProcessor, masterAddress []byte, addressFrom []byte) {
}

type UnconfirmedTransferTokenTransaction struct {
	TransactionHash []byte   `json:"transaction_hash" bson:"transaction_hash"`
	TokenTxnHash    []byte   `json:"token_txn_hash" bson:"token_txn_hash"`
	AddressesTo     [][]byte `json:"addresses_to" bson:"addresses_to"`
	Amounts         []int64  `json:"amounts" bson:"amounts"`
}

func (t *UnconfirmedTransferTokenTransaction) TransactionFromPBData(tx *generated.Transaction) {
	transferToken := tx.GetTransferToken()
	t.TransactionHash = tx.TransactionHash
	t.AddressesTo = transferToken.AddrsTo
	for _, amount := range tx.GetTransferToken().Amounts {
		t.Amounts = append(t.Amounts, int64(amount))
	}
	t.TokenTxnHash = transferToken.TokenTxhash
}

func (t *UnconfirmedTransferTokenTransaction) IsEqualPBData(tx *generated.Transaction) bool {
	tx2 := &UnconfirmedTransferTokenTransaction{}
	tx2.TransactionFromPBData(tx)
	return t.IsEqual(tx2)
}

func (t *UnconfirmedTransferTokenTransaction) IsEqual(tx *UnconfirmedTransferTokenTransaction) bool {
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

func (t *UnconfirmedTransferTokenTransaction) Apply(m *MongoProcessor, masterAddress []byte, addressFrom []byte) {
}

func (t *UnconfirmedTransferTokenTransaction) Revert(m *MongoProcessor, masterAddress []byte, addressFrom []byte) {
}

type UnconfirmedTransactionInterface interface {
	TransactionFromPBData(tx *generated.Transaction)
	Apply(m *MongoProcessor, masterAddress []byte, addressFrom []byte)
	Revert(m *MongoProcessor, masterAddress []byte, addressFrom []byte)
}

func ProtoToUnconfirmedTransaction(tx *generated.Transaction, seenTimestamp uint64) (*UnconfirmedTransaction, UnconfirmedTransactionInterface){
	t := &UnconfirmedTransaction{}

	var t2 UnconfirmedTransactionInterface
	var txType int8
	switch tx.TransactionType.(type) {
	case *generated.Transaction_Transfer_:
		txType = 1
		t2 = &UnconfirmedTransferTransaction{}
		t2.TransactionFromPBData(tx)
	case *generated.Transaction_Token_:
		txType = 2
		t2 = &UnconfirmedTokenTransaction{}
		t2.TransactionFromPBData(tx)
	case *generated.Transaction_TransferToken_:
		txType = 3
		t2 = &UnconfirmedTransferTokenTransaction{}
		t2.TransactionFromPBData(tx)
	case *generated.Transaction_Message_:
		txType = 4
		t2 = &UnconfirmedMessageTransaction{}
		t2.TransactionFromPBData(tx)
	case *generated.Transaction_Slave_:
		txType = 5
		t2 = &UnconfirmedSlaveTransaction{}
		t2.TransactionFromPBData(tx)
	default:
		fmt.Println("Not matched with any unconfirmed txn type")
	}
	t.TransactionFromPBData(tx, seenTimestamp, txType)
	return t, t2
}
