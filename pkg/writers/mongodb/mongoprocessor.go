package mongodb

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"github.com/theQRL/go-qrl/pkg/config"
	"github.com/theQRL/go-qrl/pkg/core/block"
	"github.com/theQRL/go-qrl/pkg/core/chain"
	"github.com/theQRL/go-qrl/pkg/generated"
	"github.com/theQRL/go-qrl/pkg/log"
	"github.com/theQRL/go-qrl/pkg/misc"
	"github.com/theQRL/go-qrl/pkg/writers/mongodb/transactionaction"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/x/bsonx"
	"reflect"
	"sync"
	"time"
)

type MongoProcessor struct {
	client   *mongo.Client
	ctx      context.Context
	database *mongo.Database
	config   *config.Config
	chain    *chain.Chain
	log      log.LoggerInterface

	lastBlock *Block
	Exit      chan struct{}
	LoopWG    sync.WaitGroup

	bulkBlocks              []mongo.WriteModel
	bulkAccounts            []mongo.WriteModel
	bulkPaginatedAccountTxs []mongo.WriteModel

	bulkTransactions    []mongo.WriteModel
	bulkCoinBaseTx      []mongo.WriteModel
	bulkTransferTx      []mongo.WriteModel
	bulkTokenTx         []mongo.WriteModel
	bulkTransferTokenTx []mongo.WriteModel
	bulkMessageTx       []mongo.WriteModel
	bulkSlaveTx         []mongo.WriteModel

	blocksCollection              *mongo.Collection
	accountsCollection            *mongo.Collection
	paginatedAccountTxsCollection *mongo.Collection

	transactionsCollection    *mongo.Collection
	coinBaseTxCollection      *mongo.Collection
	transferTxCollection      *mongo.Collection
	tokenTxCollection         *mongo.Collection
	transferTokenTxCollection *mongo.Collection
	messageTxCollection       *mongo.Collection
	slaveTxCollection         *mongo.Collection

	bulkUnconfirmedTransactions    []mongo.WriteModel
	bulkUnconfirmedTransferTx      []mongo.WriteModel
	bulkUnconfirmedTokenTx         []mongo.WriteModel
	bulkUnconfirmedTransferTokenTx []mongo.WriteModel
	bulkUnconfirmedMessageTx       []mongo.WriteModel
	bulkUnconfirmedSlaveTx         []mongo.WriteModel

	unconfirmedTransactionsCollection    *mongo.Collection
	unconfirmedTransferTxCollection      *mongo.Collection
	unconfirmedTokenTxCollection         *mongo.Collection
	unconfirmedTransferTokenTxCollection *mongo.Collection
	unconfirmedMessageTxCollection       *mongo.Collection
	unconfirmedSlaveTxCollection         *mongo.Collection

	chanTransactionAction         chan *transactionaction.TransactionAction
	unconfirmedTransactionsHashes map[string]bool
}

func (m *MongoProcessor) BlockProcessor(b *block.Block) error {
	session, err := m.client.StartSession(options.Session())
	if err != nil {
		m.log.Info("[BlockProcessor] Failed to Create Session", "err", err.Error())
		return err
	}
	err = session.StartTransaction(options.Transaction())
	if err != nil {
		m.log.Info("[BlockProcessor] Failed to Start Transaction", "err", err.Error())
		return err
	}
	mongoBlock := &Block{}
	mongoBlock.BlockFromPBData(b.PBData())
	operation := mongo.NewInsertOneModel()
	operation.SetDocument(mongoBlock)
	m.bulkBlocks = append(m.bulkBlocks, operation)
	accounts := make(map[string]*Account)
	accountTxHashes := make(map[string][]*TransactionHashType)
	for _, tx := range b.Transactions() {
		m.TransactionProcessor(tx, b.BlockNumber(), accounts, accountTxHashes)
	}
	err = m.PaginatedAccountTxsProcessor(accounts, accountTxHashes)
	if err != nil {
		m.log.Info("Error in BlockProcessor", "Error", err.Error())
		return err
	}
	m.AccountProcessor(accounts)

	err = m.WriteAll()
	if err != nil {
		m.log.Error("[BlockProcessor] Error while WriteAll",
			"Error", err)
		err2 := session.AbortTransaction(m.ctx)
		if err2 != nil {
			m.log.Info("[BlockProcessor] Failed to Abort Transaction",
				"Error", err2.Error())
			return err2
		}
		return err
	}
	err = session.CommitTransaction(m.ctx)
	if err != nil {
		m.log.Info("[BlockProcessor] Failed to Commit Transaction",
			"Error", err.Error())
		return err
	}
	m.lastBlock = mongoBlock

	return nil
}

func (m *MongoProcessor) TransactionProcessor(tx *generated.Transaction, blockNumber uint64, accounts map[string]*Account, paginatedAccountTxs map[string][]*TransactionHashType) {
	mongoTx, txDetails := ProtoToTransaction(tx, blockNumber)
	operation := mongo.NewInsertOneModel()
	operation.SetDocument(mongoTx)
	m.bulkTransactions = append(m.bulkTransactions, operation)

	operation = mongo.NewInsertOneModel()
	operation.SetDocument(txDetails)

	switch tx.TransactionType.(type) {
	case *generated.Transaction_Coinbase:
		m.bulkCoinBaseTx = append(m.bulkCoinBaseTx, operation)
	case *generated.Transaction_Transfer_:
		m.bulkTransferTx = append(m.bulkTransferTx, operation)
	case *generated.Transaction_Token_:
		m.bulkTokenTx = append(m.bulkTokenTx, operation)
	case *generated.Transaction_TransferToken_:
		m.bulkTransferTokenTx = append(m.bulkTransferTokenTx, operation)
	case *generated.Transaction_Message_:
		m.bulkMessageTx = append(m.bulkMessageTx, operation)
	case *generated.Transaction_Slave_:
		m.bulkSlaveTx = append(m.bulkSlaveTx, operation)
	}
	mongoTx.Apply(m, accounts, txDetails, int64(blockNumber), paginatedAccountTxs)

}

func (m *MongoProcessor) PaginatedAccountTxsProcessor(accounts map[string]*Account, accountTxHashes map[string][]*TransactionHashType) error {
	var arrayPaginatedAccountTxs []*PaginatedAccountTxs
	for qAddress, arrayTransactionHashType := range accountTxHashes {
		address := misc.Qaddress2Bin(qAddress)

		currentPage := accounts[qAddress].Pages

		key := GetPaginatedAccountTxsKey(address, currentPage)
		paginatedAccountTxs := &PaginatedAccountTxs{}
		paginatedAccountTxs.Key = key

		singleResult := m.paginatedAccountTxsCollection.FindOne(m.ctx, bson.D{{"key", key}})
		_ = singleResult.Decode(paginatedAccountTxs)

		for _, transactionHashType := range arrayTransactionHashType {
			if uint64(len(paginatedAccountTxs.TransactionHashes)) == m.config.User.MongoProcessorConfig.ItemsPerPage {
				arrayPaginatedAccountTxs = append(arrayPaginatedAccountTxs, paginatedAccountTxs)
				accounts[qAddress].Pages++
				currentPage = accounts[qAddress].Pages
				paginatedAccountTxs = &PaginatedAccountTxs{}
				paginatedAccountTxs.Key = GetPaginatedAccountTxsKey(address, currentPage)
			}
			paginatedAccountTxs.TransactionHashes = append(paginatedAccountTxs.TransactionHashes, transactionHashType.TransactionHash)
			paginatedAccountTxs.TransactionTypes = append(paginatedAccountTxs.TransactionTypes, transactionHashType.Type)
		}
		if len(paginatedAccountTxs.TransactionHashes) > 0 {
			arrayPaginatedAccountTxs = append(arrayPaginatedAccountTxs, paginatedAccountTxs)
		}
	}

	for _, paginatedAccountTxs := range arrayPaginatedAccountTxs {
		operation := mongo.NewUpdateOneModel()
		operation.SetUpsert(true)
		operation.SetFilter(bsonx.Doc{{"key", bsonx.Binary(0, paginatedAccountTxs.Key)}})
		operation.SetUpdate(paginatedAccountTxs)
		m.bulkPaginatedAccountTxs = append(m.bulkPaginatedAccountTxs, operation)
	}

	return nil
}

func (m *MongoProcessor) UnconfirmedTransactionProcessor(tx *generated.Transaction, seenTimestamp uint64) {
	mongoTx, txDetails := ProtoToUnconfirmedTransaction(tx, seenTimestamp)
	operation := mongo.NewInsertOneModel()
	operation.SetDocument(mongoTx)
	m.bulkUnconfirmedTransactions = append(m.bulkUnconfirmedTransactions, operation)

	operation = mongo.NewInsertOneModel()
	operation.SetDocument(txDetails)
	switch tx.TransactionType.(type) {
	case *generated.Transaction_Transfer_:
		m.bulkUnconfirmedTransferTx = append(m.bulkUnconfirmedTransferTx, operation)
	case *generated.Transaction_Token_:
		m.bulkUnconfirmedTokenTx = append(m.bulkUnconfirmedTokenTx, operation)
	case *generated.Transaction_TransferToken_:
		m.bulkUnconfirmedTransferTokenTx = append(m.bulkUnconfirmedTransferTokenTx, operation)
	case *generated.Transaction_Message_:
		m.bulkUnconfirmedMessageTx = append(m.bulkUnconfirmedMessageTx, operation)
	case *generated.Transaction_Slave_:
		m.bulkUnconfirmedSlaveTx = append(m.bulkUnconfirmedSlaveTx, operation)
	}
	mongoTx.Apply(m, txDetails)
}

func (m *MongoProcessor) RemoveUnconfirmedTxn(tx *generated.Transaction) {
	txHash := tx.TransactionHash
	_, err := m.unconfirmedTransactionsCollection.DeleteOne(m.ctx, bsonx.Doc{{"transaction_hash", bsonx.Binary(0, txHash)}})
	if err != nil {
		m.log.Error("Error while removing Unconfirmed Transaction",
			"Error", err)
	}

	switch tx.TransactionType.(type) {
	case *generated.Transaction_Transfer_:
		_, err := m.unconfirmedTransferTxCollection.DeleteOne(m.ctx, bsonx.Doc{{"transaction_hash", bsonx.Binary(0, txHash)}})
		if err != nil {
			m.log.Error("Error while removing Unconfirmed TransferTxn",
				"Error", err)
		}
	case *generated.Transaction_Token_:
		_, err := m.unconfirmedTokenTxCollection.DeleteOne(m.ctx, bsonx.Doc{{"transaction_hash", bsonx.Binary(0, txHash)}})
		if err != nil {
			m.log.Error("Error while removing Unconfirmed TokenTxn",
				"Error", err)
		}
	case *generated.Transaction_TransferToken_:
		_, err := m.unconfirmedTransferTokenTxCollection.DeleteOne(m.ctx, bsonx.Doc{{"transaction_hash", bsonx.Binary(0, txHash)}})
		if err != nil {
			m.log.Error("Error while removing Unconfirmed TransferTokenTxn",
				"Error", err)
		}
	case *generated.Transaction_Message_:
		_, err := m.unconfirmedMessageTxCollection.DeleteOne(m.ctx, bsonx.Doc{{"transaction_hash", bsonx.Binary(0, txHash)}})
		if err != nil {
			m.log.Error("Error while removing Unconfirmed MessageTxn",
				"Error", err)
		}
	case *generated.Transaction_Slave_:
		_, err := m.unconfirmedSlaveTxCollection.DeleteOne(m.ctx, bsonx.Doc{{"transaction_hash", bsonx.Binary(0, txHash)}})
		if err != nil {
			m.log.Error("Error while removing Unconfirmed SlaveTxn",
				"Error", err)
		}
	}
}

func (m *MongoProcessor) AccountProcessor(accounts map[string]*Account) {
	for _, account := range accounts {
		operation := mongo.NewUpdateOneModel()
		operation.SetUpsert(true)
		operation.SetFilter(bsonx.Doc{{"address", bsonx.Binary(0, account.Address)}})
		operation.SetUpdate(account)
		m.bulkAccounts = append(m.bulkAccounts, operation)
	}
}

func (m *MongoProcessor) WriteAll() error {
	if len(m.bulkBlocks) > 0 {
		if err := m.WriteBlocks(); err != nil {
			return err
		}
	}
	if len(m.bulkTransactions) > 0 {
		if err := m.WriteTransactions(); err != nil {
			return err
		}
	}
	if len(m.bulkPaginatedAccountTxs) > 0 {
		if err := m.WritePaginatedAccountTxs(); err != nil {
			return err
		}
	}
	if len(m.bulkAccounts) > 0 {
		if err := m.WriteAccounts(); err != nil {
			return err
		}
	}
	m.EmptyBulks()
	return nil
}

func (m *MongoProcessor) EmptyBulks() {
	m.bulkBlocks = m.bulkBlocks[:0]
	m.bulkAccounts = m.bulkAccounts[:0]

	m.bulkTransactions = m.bulkTransactions[:0]
	m.bulkCoinBaseTx = m.bulkCoinBaseTx[:0]
	m.bulkTransferTx = m.bulkTransferTx[:0]
	m.bulkTokenTx = m.bulkTokenTx[:0]
	m.bulkTransferTokenTx = m.bulkTransferTokenTx[:0]
	m.bulkMessageTx = m.bulkMessageTx[:0]
	m.bulkSlaveTx = m.bulkSlaveTx[:0]
	m.bulkPaginatedAccountTxs = m.bulkPaginatedAccountTxs[:0]
}

func (m *MongoProcessor) WriteBlocks() error {
	_, err := m.blocksCollection.BulkWrite(m.ctx, m.bulkBlocks)
	if err != nil {
		// TODO: Do something
		return err
	}
	return nil
}

func (m *MongoProcessor) WriteTransactions() error {
	_, err := m.transactionsCollection.BulkWrite(m.ctx, m.bulkTransactions)
	if err != nil {
		// TODO: Do something
		return err
	}
	if len(m.bulkCoinBaseTx) > 0 {
		_, err = m.coinBaseTxCollection.BulkWrite(m.ctx, m.bulkCoinBaseTx)
		if err != nil {
			// TODO: Do something
			return err
		}
	}
	if len(m.bulkTransferTx) > 0 {
		_, err = m.transferTxCollection.BulkWrite(m.ctx, m.bulkTransferTx)
		if err != nil {
			// TODO: Do something
			return err
		}
	}
	if len(m.bulkTokenTx) > 0 {
		_, err = m.tokenTxCollection.BulkWrite(m.ctx, m.bulkTokenTx)
		if err != nil {
			// TODO: Do something
			return err
		}
	}
	if len(m.bulkTransferTokenTx) > 0 {
		_, err = m.transferTokenTxCollection.BulkWrite(m.ctx, m.bulkTransferTokenTx)
		if err != nil {
			// TODO: Do something
			return err
		}
	}
	if len(m.bulkMessageTx) > 0 {
		_, err = m.messageTxCollection.BulkWrite(m.ctx, m.bulkMessageTx)
		if err != nil {
			// TODO: Do something
			return err
		}
	}
	if len(m.bulkSlaveTx) > 0 {
		_, err = m.slaveTxCollection.BulkWrite(m.ctx, m.bulkSlaveTx)
		if err != nil {
			// TODO: Do something
			return err
		}
	}
	return nil
}

func (m *MongoProcessor) WritePaginatedAccountTxs() error {
	_, err := m.paginatedAccountTxsCollection.BulkWrite(m.ctx, m.bulkPaginatedAccountTxs)
	if err != nil {
		// TODO: Do something
		return err
	}
	return nil
}

func (m *MongoProcessor) WriteUnconfirmedTransactions() error {
	_, err := m.unconfirmedTransactionsCollection.BulkWrite(m.ctx, m.bulkUnconfirmedTransactions)
	if err != nil {
		// TODO: Do something
		return err
	}
	if len(m.bulkUnconfirmedTransferTx) > 0 {
		_, err = m.unconfirmedTransferTxCollection.BulkWrite(m.ctx, m.bulkUnconfirmedTransferTx)
		if err != nil {
			// TODO: Do something
			return err
		}
	}
	if len(m.bulkUnconfirmedTokenTx) > 0 {
		_, err = m.unconfirmedTokenTxCollection.BulkWrite(m.ctx, m.bulkUnconfirmedTokenTx)
		if err != nil {
			// TODO: Do something
			return err
		}
	}
	if len(m.bulkUnconfirmedTransferTokenTx) > 0 {
		_, err = m.unconfirmedTransferTokenTxCollection.BulkWrite(m.ctx, m.bulkUnconfirmedTransferTokenTx)
		if err != nil {
			// TODO: Do something
			return err
		}
	}
	if len(m.bulkUnconfirmedMessageTx) > 0 {
		_, err = m.unconfirmedMessageTxCollection.BulkWrite(m.ctx, m.bulkUnconfirmedMessageTx)
		if err != nil {
			// TODO: Do something
			return err
		}
	}
	if len(m.bulkUnconfirmedSlaveTx) > 0 {
		_, err = m.unconfirmedSlaveTxCollection.BulkWrite(m.ctx, m.bulkUnconfirmedSlaveTx)
		if err != nil {
			// TODO: Do something
			return err
		}
	}

	return nil
}

func (m *MongoProcessor) EmptyUnconfirmedTxBulks() {
	m.bulkUnconfirmedTransactions = m.bulkUnconfirmedTransactions[:0]
	m.bulkUnconfirmedTransferTx = m.bulkUnconfirmedTransferTx[:0]
	m.bulkUnconfirmedTokenTx = m.bulkUnconfirmedTokenTx[:0]
	m.bulkUnconfirmedTransferTokenTx = m.bulkUnconfirmedTransferTokenTx[:0]
	m.bulkUnconfirmedMessageTx = m.bulkUnconfirmedMessageTx[:0]
	m.bulkUnconfirmedSlaveTx = m.bulkUnconfirmedSlaveTx[:0]
}

func (m *MongoProcessor) WriteAccounts() error {
	_, err := m.accountsCollection.BulkWrite(m.ctx, m.bulkAccounts)
	if err != nil {
		// TODO: Do something
		return err
	}
	return nil
}

func (m *MongoProcessor) IsDataBaseExists(dbName string) (bool, error) {
	databaseNames, err := m.client.ListDatabaseNames(m.ctx, bsonx.Doc{})
	if err != nil {
		return false, err
	}
	for i := range databaseNames {
		if databaseNames[i] == dbName {
			return true, nil
		}
	}
	return false, nil
}

func (m *MongoProcessor) IsCollectionExists(collectionName string) (bool, error) {
	cursor, err := m.database.ListCollections(m.ctx, bsonx.Doc{})
	if err != nil {
		return false, err
	}
	for cursor.Next(m.ctx) {
		next := &bsonx.Doc{}
		err := cursor.Decode(next)
		if err != nil {
			return false, err
		}
		//_, err = next.LookupErr(collectionName)
		elem, err := next.LookupErr("name")
		if err != nil {
			return false, nil
		}

		elemName := elem.StringValue()
		if elemName == collectionName {
			return true, nil
		}
	}
	return false, nil
}

func (m *MongoProcessor) CreateBlocksIndexes(found bool) error {
	m.blocksCollection = m.database.Collection("blocks")
	if found {
		return nil
	}
	_, err := m.blocksCollection.Indexes().CreateMany(context.Background(),
		[]mongo.IndexModel{
			{Keys: bson.M{"block_number": int32(-1)}},
			{Keys: bson.M{"header_hash": int32(1)}},
		})
	if err != nil {
		m.log.Error("Error while modeling index for blocks",
			"Error", err)
		return err
	}
	return nil
}

func (m *MongoProcessor) CreateTransactionsIndexes(found bool) error {
	m.transactionsCollection = m.database.Collection("txs")
	if found {
		return nil
	}
	_, err := m.transactionsCollection.Indexes().CreateMany(context.Background(),
		[]mongo.IndexModel{
			{Keys: bson.M{"master_address": int32(1)}},
			{Keys: bson.M{"address_from": int32(1)}},
			{Keys: bson.M{"transaction_hash": int32(1)}},
			{Keys: bson.M{"transaction_type": int32(1)}},
			{Keys: bson.M{"block_number": int32(-1)}},
		})

	if err != nil {
		m.log.Error("Error while modeling index for transactions",
			"Error", err)
		return err
	}
	return nil
}

func (m *MongoProcessor) CreateUnconfirmedTransactionsIndexes() error {
	m.unconfirmedTransactionsCollection = m.database.Collection("unconfirmed_txs")

	_, err := m.unconfirmedTransactionsCollection.Indexes().CreateMany(context.Background(),
		[]mongo.IndexModel{
			{Keys: bson.M{"master_address": int32(1)}},
			{Keys: bson.M{"address_from": int32(1)}},
			{Keys: bson.M{"transaction_hash": int32(1)}},
			{Keys: bson.M{"transaction_type": int32(1)}},
			{Keys: bson.M{"seen_timestamp": int32(-1)}},
		})

	if err != nil {
		m.log.Error("Error while modeling index for unconfirmedTransactions",
			"Error", err)
		return err
	}
	return nil
}

func (m *MongoProcessor) CreateCoinBaseTxsIndexes(found bool) error {
	m.coinBaseTxCollection = m.database.Collection("coin_base_txs")
	if found {
		return nil
	}
	_, err := m.coinBaseTxCollection.Indexes().CreateMany(context.Background(),
		[]mongo.IndexModel{
			{Keys: bson.M{"transaction_hash": int32(1)}},
			{Keys: bson.M{"address_to": int32(1)}},
		})

	if err != nil {
		m.log.Error("Error while modeling index for coinBase",
			"Error", err)
		return err
	}
	return nil
}

func (m *MongoProcessor) CreateTransferTxsIndexes(found bool) error {
	m.transferTxCollection = m.database.Collection("transfer_txs")
	if found {
		return nil
	}
	_, err := m.transferTxCollection.Indexes().CreateMany(context.Background(),
		[]mongo.IndexModel{
			{Keys: bson.M{"transaction_hash": int32(1)}},
			{Keys: bson.M{"addresses_to": int32(1)}},
		})

	if err != nil {
		m.log.Error("Error while modeling index for transferTx",
			"Error", err)
		return err
	}
	return nil
}

func (m *MongoProcessor) CreateUnconfirmedTransferTxsIndexes() error {
	m.unconfirmedTransferTxCollection = m.database.Collection("unconfirmed_transfer_txs")

	_, err := m.unconfirmedTransferTxCollection.Indexes().CreateMany(context.Background(),
		[]mongo.IndexModel{
			{Keys: bson.M{"transaction_hash": int32(1)}},
			{Keys: bson.M{"addresses_to": int32(1)}},
		})

	if err != nil {
		m.log.Error("Error while modeling index for unconfirmedTransferTx",
			"Error", err)
		return err
	}
	return nil
}

func (m *MongoProcessor) CreateTokenTxsIndexes(found bool) error {
	m.tokenTxCollection = m.database.Collection("token_txs")
	if found {
		return nil
	}
	_, err := m.tokenTxCollection.Indexes().CreateMany(context.Background(),
		[]mongo.IndexModel{
			{Keys: bson.M{"transaction_hash": int32(1)}},
			{Keys: bson.M{"symbol": int32(1)}},
			{Keys: bson.M{"symbol_str": int32(1)}},
			{Keys: bson.M{"name": int32(1)}},
			{Keys: bson.M{"name_str": int32(1)}},
			{Keys: bson.M{"addresses_to": int32(1)}},
		})

	if err != nil {
		m.log.Error("Error while modeling index for tokenTx",
			"Error", err)
		return err
	}
	return nil
}

func (m *MongoProcessor) CreateUnconfirmedTokenTxsIndexes() error {
	m.unconfirmedTokenTxCollection = m.database.Collection("unconfirmed_token_txs")

	_, err := m.unconfirmedTokenTxCollection.Indexes().CreateMany(context.Background(),
		[]mongo.IndexModel{
			{Keys: bson.M{"transaction_hash": int32(1)}},
			{Keys: bson.M{"symbol": int32(1)}},
			{Keys: bson.M{"symbol_str": int32(1)}},
			{Keys: bson.M{"name": int32(1)}},
			{Keys: bson.M{"name_str": int32(1)}},
			{Keys: bson.M{"addresses_to": int32(1)}},
		})

	if err != nil {
		m.log.Error("Error while modeling index for unconfirmedTokenTx",
			"Error", err)
		return err
	}
	return nil
}

func (m *MongoProcessor) CreateTransferTokenTxsIndexes(found bool) error {
	m.transferTokenTxCollection = m.database.Collection("transfer_token_txs")
	if found {
		return nil
	}
	_, err := m.transferTokenTxCollection.Indexes().CreateMany(context.Background(),
		[]mongo.IndexModel{
			{Keys: bson.M{"transaction_hash": int32(1)}},
			{Keys: bson.M{"token_txn_hash": int32(1)}},
			{Keys: bson.M{"addresses_to": int32(1)}},
		})

	if err != nil {
		m.log.Error("Error while modeling index for transferTokenTx",
			"Error", err)
		return err
	}
	return nil
}

func (m *MongoProcessor) CreateUnconfirmedTransferTokenTxsIndexes() error {
	m.unconfirmedTransferTokenTxCollection = m.database.Collection("unconfirmed_transfer_token_txs")

	_, err := m.unconfirmedTransferTokenTxCollection.Indexes().CreateMany(context.Background(),
		[]mongo.IndexModel{
			{Keys: bson.M{"transaction_hash": int32(1)}},
			{Keys: bson.M{"token_txn_hash": int32(1)}},
			{Keys: bson.M{"addresses_to": int32(1)}},
		})

	if err != nil {
		m.log.Error("Error while modeling index for unconfirmedTransferTokenTx",
			"Error", err)
		return err
	}
	return nil
}

func (m *MongoProcessor) CreateMessageTxsIndexes(found bool) error {
	m.messageTxCollection = m.database.Collection("message_txs")
	if found {
		return nil
	}
	_, err := m.messageTxCollection.Indexes().CreateMany(context.Background(),
		[]mongo.IndexModel{
			{Keys: bson.M{"transaction_hash": int32(1)}},
		})

	if err != nil {
		m.log.Error("Error while modeling index for messageTx",
			"Error", err)
		return err
	}
	return nil
}

func (m *MongoProcessor) CreateUnconfirmedMessageTxsIndexes() error {
	m.unconfirmedMessageTxCollection = m.database.Collection("unconfirmed_message_txs")

	_, err := m.unconfirmedMessageTxCollection.Indexes().CreateMany(context.Background(),
		[]mongo.IndexModel{
			{Keys: bson.M{"transaction_hash": int32(1)}},
		})

	if err != nil {
		m.log.Error("Error while modeling index for unconfirmedMessageTx",
			"Error", err)
		return err
	}
	return nil
}

func (m *MongoProcessor) CreateSlaveTxsIndexes(found bool) error {
	m.slaveTxCollection = m.database.Collection("slave_txs")
	if found {
		return nil
	}
	_, err := m.slaveTxCollection.Indexes().CreateMany(context.Background(),
		[]mongo.IndexModel{
			{Keys: bson.M{"transaction_hash": int32(1)}},
		})

	if err != nil {
		m.log.Error("Error while modeling index for slaveTx",
			"Error", err)
		return err
	}
	return nil
}

func (m *MongoProcessor) CreateUnconfirmedSlaveTxsIndexes() error {
	m.unconfirmedSlaveTxCollection = m.database.Collection("unconfirmed_slave_txs")

	_, err := m.unconfirmedSlaveTxCollection.Indexes().CreateMany(context.Background(),
		[]mongo.IndexModel{
			{Keys: bson.M{"transaction_hash": int32(1)}},
		})

	if err != nil {
		m.log.Error("Error while modeling index for UnconfirmedSlaveTx",
			"Error", err)
		return err
	}
	return nil
}

func (m *MongoProcessor) CreateAccountsIndexes(found bool) error {
	m.accountsCollection = m.database.Collection("accounts")
	if found {
		return nil
	}
	_, err := m.accountsCollection.Indexes().CreateMany(context.Background(),
		[]mongo.IndexModel{
			{Keys: bson.M{"address": int32(1)}},
		})

	if err != nil {
		m.log.Error("Error while modeling index for accounts",
			"Error", err)
		return err
	}

	return nil
}

func (m *MongoProcessor) CreatePaginatedAccountTxs(found bool) error {
	m.paginatedAccountTxsCollection = m.database.Collection("paginated_account_txs")
	if found {
		return nil
	}
	_, err := m.paginatedAccountTxsCollection.Indexes().CreateMany(context.Background(),
		[]mongo.IndexModel{
			{Keys: bson.M{"key": int32(1)}},
		})

	if err != nil {
		m.log.Error("Error while modeling index for paginated_account_txs",
			"Error", err)
		return err
	}

	return nil
}

func (m *MongoProcessor) CreateCleanUnconfirmedTransactionCollections() error {
	collectionsLists := map[string] interface{} {
		"unconfirmed_txs": m.CreateUnconfirmedTransactionsIndexes,
		"unconfirmed_transfer_txs": m.CreateUnconfirmedTransferTxsIndexes,
		"unconfirmed_token_txs": m.CreateUnconfirmedTokenTxsIndexes,
		"unconfirmed_transfer_token_txs": m.CreateUnconfirmedTransferTokenTxsIndexes,
		"unconfirmed_message_txs": m.CreateUnconfirmedMessageTxsIndexes,
		"unconfirmed_slave_txs": m.CreateUnconfirmedSlaveTxsIndexes,
	}
	for collectionName, indexCreatorFunc := range collectionsLists {
		found, err := m.IsCollectionExists(collectionName)
		if err != nil {
			return err
		}
		if found {
			collection := m.database.Collection(collectionName)
			err := collection.Drop(m.ctx)
			if err != nil {
				m.log.Error("Error while removing collection",
					"Collection Name", collectionName,
					"Error", err)
				return err
			}
		}
		err = indexCreatorFunc.(func() error)()
		if err != nil {
			return err
		}
	}
	return nil
}

func (m *MongoProcessor) CreateIndexes() error {
	collectionsLists := map[string] interface{} {
		"blocks": m.CreateBlocksIndexes,
		"txs": m.CreateTransactionsIndexes,
		"coin_base_txs": m.CreateCoinBaseTxsIndexes,
		"transfer_txs": m.CreateTransferTxsIndexes,
		"token_txs": m.CreateTokenTxsIndexes,
		"transfer_token_txs": m.CreateTransferTokenTxsIndexes,
		"message_txs": m.CreateMessageTxsIndexes,
		"slave_txs": m.CreateSlaveTxsIndexes,
		"accounts": m.CreateAccountsIndexes,
		"paginated_account_txs": m.CreatePaginatedAccountTxs,
	}
	blocksCollectionFound := true
	for collectionName, indexCreatorFunc := range collectionsLists {
		found, err := m.IsCollectionExists(collectionName)
		if err != nil {
			return err
		}
		if collectionName == "blocks" {
			blocksCollectionFound = found
		}
		err = indexCreatorFunc.(func(bool) error)(found)
		if err != nil {
			return err
		}
	}

	if !blocksCollectionFound {
		b, err := m.chain.GetBlockByNumber(0)
		if err != nil {
			m.log.Error("[MongoProcessor.Sync]Error while Getting BlockNumber",
				"BlockNumber", 0,
				"Error", err)
		}
		coinBaseAddress := m.config.Dev.Genesis.CoinbaseAddress
		account := CreateAccount(coinBaseAddress, 0)
		account.AddBalance(m.config.Dev.Genesis.MaxCoinSupply)
		m.AccountProcessor(map[string]*Account{misc.Bin2Qaddress(coinBaseAddress):account})
		_ = m.WriteAll()
		err = m.BlockProcessor(b)
		if err != nil {
			m.log.Info("Error while CreatingIndexes")
			return nil
		}
	} else {
		b, err := m.GetLastBlockFromDB()
		if err != nil {
			return err
		}
		m.lastBlock = b
	}
	return m.CreateCleanUnconfirmedTransactionCollections()
}

func (m *MongoProcessor) IsAccountProcessed(blockNumber int64) bool {
	accounts := make(map[string]*Account)
	LoadAccount(m, config.GetConfig().Dev.Genesis.CoinbaseAddress, accounts, blockNumber)
	return accounts[misc.Bin2Qaddress(config.GetConfig().Dev.Genesis.CoinbaseAddress)].BlockNumber != blockNumber
}

func (m *MongoProcessor) RemoveBlock(b *block.Block) error {
	accounts := make(map[string]*Account)
	txHashes := make(map[string]*Transaction)
	accountTxHashes := make(map[string][]*TransactionHashType)

	for i := len(b.Transactions()) - 1; i >= 0 ; i-- {
		m.RevertTransaction(b.Transactions()[i], b.BlockNumber(), accounts, accountTxHashes)
	}

	err := m.RevertPaginatedAccountTxs(accounts, accountTxHashes)
	if err != nil {
		m.log.Error("[RemoveBlock] Error while Reverting PaginatedAccountTxs",
			"Error", err)
		return err
	}
	m.AccountProcessor(accounts)
	err = m.WriteAll()
	if err != nil {
		m.log.Error("[RemoveBlock] Error while Updating Accounts",
			"Error", err)
		return err
	}

	for txHash, tx := range txHashes {
		switch tx.TransactionType {
		case 0:
			_, err := m.coinBaseTxCollection.DeleteOne(m.ctx, bsonx.Doc{{"transaction_hash", bsonx.Binary(0, misc.HStr2Bin(txHash))}})
			if err != nil {
				m.log.Error("Error while removing CoinBaseTxn",
					"Error", err)
				return err
			}
		case 1:
			_, err := m.transferTxCollection.DeleteOne(m.ctx, bsonx.Doc{{"transaction_hash", bsonx.Binary(0, misc.HStr2Bin(txHash))}})
			if err != nil {
				m.log.Error("Error while removing TransferTxn",
					"Error", err)
				return err
			}
		case 2:
			_, err := m.tokenTxCollection.DeleteOne(m.ctx, bsonx.Doc{{"transaction_hash", bsonx.Binary(0, misc.HStr2Bin(txHash))}})
			if err != nil {
				m.log.Error("Error while removing TokenTxn",
					"Error", err)
				return err
			}
		case 3:
			_, err := m.transferTokenTxCollection.DeleteOne(m.ctx, bsonx.Doc{{"transaction_hash", bsonx.Binary(0, misc.HStr2Bin(txHash))}})
			if err != nil {
				m.log.Error("Error while removing TransferTokenTxn",
					"Error", err)
				return err
			}
		case 4:
			_, err := m.messageTxCollection.DeleteOne(m.ctx, bsonx.Doc{{"transaction_hash", bsonx.Binary(0, misc.HStr2Bin(txHash))}})
			if err != nil {
				m.log.Error("Error while removing MessageTxn",
					"Error", err)
				return err
			}
		case 5:
			_, err := m.slaveTxCollection.DeleteOne(m.ctx, bsonx.Doc{{"transaction_hash", bsonx.Binary(0, misc.HStr2Bin(txHash))}})
			if err != nil {
				m.log.Error("Error while removing SlaveTxn",
					"Error", err)
				return err
			}
		}
	}
	_, err = m.transactionsCollection.DeleteMany(m.ctx, bsonx.Doc{{"block_number", bsonx.Int64(int64(b.BlockNumber()))}})
	if err != nil {
		m.log.Error("Error while removing transaction",
			"Error", err)
		return err
	}

	_, err = m.blocksCollection.DeleteOne(m.ctx, bsonx.Doc{{"block_number", bsonx.Int64(int64(b.BlockNumber()))}})
	if err != nil {
		m.log.Error("Error while removing block",
			"Error", err)
		return err
	}
	return nil
}

func (m *MongoProcessor) RevertTransaction(tx *generated.Transaction, blockNumber uint64, accounts map[string]*Account, paginatedAccountTxs map[string][]*TransactionHashType) {
	mongoTx, txDetails := ProtoToTransaction(tx, blockNumber)
	operation := mongo.NewDeleteOneModel()
	operation.SetFilter(mongoTx) // TODO: Confirm if this works
	m.bulkTransactions = append(m.bulkTransactions, operation)

	operation = mongo.NewDeleteOneModel()
	operation.SetFilter(txDetails)

	switch tx.TransactionType.(type) {
	case *generated.Transaction_Coinbase:
		m.bulkCoinBaseTx = append(m.bulkCoinBaseTx, operation)
	case *generated.Transaction_Transfer_:
		m.bulkTransferTx = append(m.bulkTransferTx, operation)
	case *generated.Transaction_Token_:
		m.bulkTokenTx = append(m.bulkTokenTx, operation)
	case *generated.Transaction_TransferToken_:
		m.bulkTransferTokenTx = append(m.bulkTransferTokenTx, operation)
	case *generated.Transaction_Message_:
		m.bulkMessageTx = append(m.bulkMessageTx, operation)
	case *generated.Transaction_Slave_:
		m.bulkSlaveTx = append(m.bulkSlaveTx, operation)
	}
	mongoTx.Revert(m, accounts, txDetails, int64(blockNumber), paginatedAccountTxs)
}

func (m *MongoProcessor) RevertCoinBaseTransaction(tx *Transaction, accounts map[string]*Account, blockNumber int64, paginatedAccountTxs map[string][]*TransactionHashType) error {
	singleResult := m.coinBaseTxCollection.FindOne(m.ctx, bson.D{{"transaction_hash", tx.TransactionHash}})
	coinBaseTx := &CoinBaseTransaction{}
	err := singleResult.Decode(coinBaseTx)
	if err != nil {
		m.log.Error("Error while Decoding CoinBase Transaction",
			"Error", err)
		return err
	}
	tx.Revert(m, accounts, coinBaseTx, blockNumber, paginatedAccountTxs)
	return nil
}

func (m *MongoProcessor) RevertTransferTransaction(tx *Transaction, accounts map[string]*Account, blockNumber int64, paginatedAccountTxs map[string][]*TransactionHashType) error {
	singleResult := m.transferTxCollection.FindOne(m.ctx, bson.D{{"transaction_hash", tx.TransactionHash}})
	transferTx := &TransferTransaction{}
	err := singleResult.Decode(transferTx)
	if err != nil {
		m.log.Error("Error while Decoding Transfer Transaction",
			"Error", err)
		return err
	}
	tx.Revert(m, accounts, transferTx, blockNumber, paginatedAccountTxs)
	return nil
}

func (m *MongoProcessor) RevertTokenTransaction(tx *Transaction, accounts map[string]*Account, blockNumber int64, paginatedAccountTxs map[string][]*TransactionHashType) error {
	singleResult := m.tokenTxCollection.FindOne(m.ctx, bson.D{{"transaction_hash", tx.TransactionHash}})
	tokenTx := &TokenTransaction{}
	err := singleResult.Decode(tokenTx)
	if err != nil {
		m.log.Error("Error while Decoding Token Transaction",
			"Error", err)
		return err
	}
	tx.Revert(m, accounts, tokenTx, blockNumber, paginatedAccountTxs)
	return nil
}

func (m *MongoProcessor) RevertTransferTokenTransaction(tx *Transaction, accounts map[string]*Account, blockNumber int64, paginatedAccountTxs map[string][]*TransactionHashType) error {
	singleResult := m.transferTokenTxCollection.FindOne(m.ctx, bson.D{{"transaction_hash", tx.TransactionHash}})
	transferTokenTx := &TransferTokenTransaction{}
	err := singleResult.Decode(transferTokenTx)
	if err != nil {
		m.log.Error("Error while Decoding Transfer Token Transaction",
			"Error", err)
		return err
	}
	tx.Revert(m, accounts, transferTokenTx, blockNumber, paginatedAccountTxs)
	return nil
}

func (m *MongoProcessor) RevertMessageTransaction(tx *Transaction, accounts map[string]*Account, blockNumber int64, paginatedAccountTxs map[string][]*TransactionHashType) error {
	singleResult := m.messageTxCollection.FindOne(m.ctx, bson.D{{"transaction_hash", tx.TransactionHash}})
	messageTx := &MessageTransaction{}
	err := singleResult.Decode(messageTx)
	if err != nil {
		m.log.Error("Error while Decoding Message Transaction",
			"Error", err)
		return err
	}
	tx.Revert(m, accounts, messageTx, blockNumber, paginatedAccountTxs)
	return nil
}

func (m *MongoProcessor) RevertSlaveTransaction(tx *Transaction, accounts map[string]*Account, blockNumber int64, paginatedAccountTxs map[string][]*TransactionHashType) error {
	singleResult := m.slaveTxCollection.FindOne(m.ctx, bson.D{{"transaction_hash", tx.TransactionHash}})
	slaveTx := &SlaveTransaction{}
	err := singleResult.Decode(slaveTx)
	if err != nil {
		m.log.Error("Error while Decoding Slave Transaction",
			"Error", err)
		return err
	}
	tx.Revert(m, accounts, slaveTx, blockNumber, paginatedAccountTxs)
	return nil
}

func (m *MongoProcessor) RevertPaginatedAccountTxs(accounts map[string]*Account, accountTxHashes map[string][]*TransactionHashType) error {
	var arrayPaginatedAccountTxs []*PaginatedAccountTxs
	for qAddress, arrayTransactionHashType := range accountTxHashes {
		address := misc.Qaddress2Bin(qAddress)

		currentPage := accounts[qAddress].Pages

		paginatedAccountTxs := &PaginatedAccountTxs{}
		paginatedAccountTxs.Key = GetPaginatedAccountTxsKey(address, currentPage)

		singleResult := m.paginatedAccountTxsCollection.FindOne(m.ctx, bson.D{{"key", paginatedAccountTxs.Key}})

		err := singleResult.Decode(paginatedAccountTxs)
		if err != nil {
			fmt.Println("Error while Decoding Paginated", err)
			return err
		}

		for _, transactionHashType := range arrayTransactionHashType {
			lenTransactionHashes := len(paginatedAccountTxs.TransactionHashes)
			if !reflect.DeepEqual(transactionHashType.TransactionHash, paginatedAccountTxs.TransactionHashes[lenTransactionHashes - 1]) {
				fmt.Println("Expected Hash", misc.Bin2HStr(transactionHashType.TransactionHash))
				fmt.Println("Found Hash", misc.Bin2HStr(paginatedAccountTxs.TransactionHashes[lenTransactionHashes - 1]))
				return errors.New("Transaction Hash mismatch during fork recovery")
			}
			paginatedAccountTxs.TransactionHashes = paginatedAccountTxs.TransactionHashes[:lenTransactionHashes - 1]
			paginatedAccountTxs.TransactionTypes = paginatedAccountTxs.TransactionTypes[:lenTransactionHashes - 1]
			if len(paginatedAccountTxs.TransactionHashes) == 0 {
				arrayPaginatedAccountTxs = append(arrayPaginatedAccountTxs, paginatedAccountTxs)
				if accounts[qAddress].Pages > 0 {
					accounts[qAddress].Pages--
					currentPage = accounts[qAddress].Pages
					paginatedAccountTxs = &PaginatedAccountTxs{}
					paginatedAccountTxs.Key = GetPaginatedAccountTxsKey(address, currentPage)
					singleResult = m.paginatedAccountTxsCollection.FindOne(m.ctx, bson.D{{"key", paginatedAccountTxs.Key}})
					err := singleResult.Decode(paginatedAccountTxs)
					if err != nil {
						fmt.Println("Error while Decoding PaginatedAccountTxs", err)
						return err
					}
				}
			}
		}
		if len(paginatedAccountTxs.TransactionHashes) > 0 {
			arrayPaginatedAccountTxs = append(arrayPaginatedAccountTxs, paginatedAccountTxs)
		}
	}

	for _, paginatedAccountTxs := range arrayPaginatedAccountTxs {
		operation := mongo.NewUpdateOneModel()
		operation.SetUpsert(true)
		operation.SetFilter(bsonx.Doc{{"key", bsonx.Binary(0, paginatedAccountTxs.Key)}})
		operation.SetUpdate(paginatedAccountTxs)
		m.bulkPaginatedAccountTxs = append(m.bulkPaginatedAccountTxs, operation)
	}

	return nil
}

func (m *MongoProcessor) GetLastBlockFromDB() (*Block, error) {
	o := &options.FindOneOptions{}
	o.Sort = bson.D{{"block_number", -1}}
	result := m.blocksCollection.FindOne(m.ctx, bson.D{{}}, o)
	b := &Block{}
	err := result.Decode(b)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func (m *MongoProcessor) ForkRecovery() error {
	for {
		b, err := m.GetLastBlockFromDB()
		if err != nil {
			m.log.Error("[ForkRecovery] Error while Retrieving last block",
				"Error", err)
			return err
		}
		b2, err := m.chain.GetBlockByNumber(uint64(b.BlockNumber))
		if err == nil {
			if reflect.DeepEqual(b2.HeaderHash(), b.HeaderHash) {
				m.lastBlock = b
				return nil // Fork Recovery Finished
			}
		}
		session, err := m.client.StartSession(options.Session())
		if err != nil {
			m.log.Info("[ForkRecovery] Failed to Create Session", "err", err.Error())
			return err
		}
		err = session.StartTransaction(options.Transaction())
		if err != nil {
			m.log.Info("[ForkRecovery] Failed to Start Transaction", "err", err.Error())
			return err
		}
		oldBlock, err := m.chain.GetBlock(b.HeaderHash)
		if err != nil {
			m.log.Info("[ForkRecovery] Failed to GetBlock",
				"headerhash", misc.Bin2HStr(b.HeaderHash),
				"err", err.Error())
			return err
		}
		err = m.RemoveBlock(oldBlock)
		if err != nil {
			err2 := session.AbortTransaction(m.ctx)
			if err2 != nil {
				m.log.Info("[ForkRecovery] Failed to Abort Transaction", "err", err2.Error())
				return err2
			}
			return err
		}
		err = session.CommitTransaction(m.ctx)
		if err != nil {
			m.log.Info("[ForkRecovery] Failed to Commit Transaction", "err", err.Error())
			return err
		}
	}
}

func (m *MongoProcessor) UpdateUnconfirmedTransactions() {
	m.LoopWG.Add(1)
	defer m.LoopWG.Done()

	for {
		select {
		case ta := <- m.chanTransactionAction:
			txHash := misc.Bin2HStr(ta.Transaction.TransactionHash)
			switch ta.IsAdd {
			case true:
				m.UnconfirmedTransactionProcessor(ta.Transaction, ta.Timestamp)
				err := m.WriteUnconfirmedTransactions()
				if err != nil {
					m.log.Error("Error while writing Unconfirmed Transactions",
						"Txhash", txHash,
						"Error", err)
					continue
				}
				m.EmptyUnconfirmedTxBulks()
				m.unconfirmedTransactionsHashes[txHash] = true
			case false:
				if _, ok := m.unconfirmedTransactionsHashes[txHash]; !ok {
					continue
				}
				m.RemoveUnconfirmedTxn(ta.Transaction)
				delete(m.unconfirmedTransactionsHashes, txHash)
			}
		case <- m.Exit:
			return
		}
	}
}

func (m *MongoProcessor) Sync() error {
	for {
		lastBlock := m.chain.GetLastBlock()
		lastBlockNumber := int64(lastBlock.BlockNumber())
		if lastBlockNumber == m.lastBlock.BlockNumber {
			return nil
		} else if lastBlockNumber > m.lastBlock.BlockNumber {
			b, _ := m.chain.GetBlockByNumber(uint64(m.lastBlock.BlockNumber))
			if !reflect.DeepEqual(b.HeaderHash(), m.lastBlock.HeaderHash) {
				err := m.ForkRecovery()
				if err != nil {
					return err
				}
			}
			blockNumber := uint64(m.lastBlock.BlockNumber) + 1
			b, err := m.chain.GetBlockByNumber(blockNumber)
			if err != nil {
				m.log.Error("[Sync] Error while Getting BlockNumber",
					"BlockNumber", blockNumber,
					"Error", err)
			}
			if !reflect.DeepEqual(b.PrevHeaderHash(), m.lastBlock.HeaderHash) {
				err := m.ForkRecovery()
				if err != nil {
					return err
				}
			}
			err = m.BlockProcessor(b)
			if err != nil {
				m.log.Error("[Sync] Error in BlockProcessor")
				return err
			}

		} else {
			return m.ForkRecovery()
		}
	}
}

func (m *MongoProcessor) Run() {
	m.LoopWG.Add(1)
	defer m.LoopWG.Done()

	go m.UpdateUnconfirmedTransactions()  // Listen for unconfirmed transactions

	for {
		select {
		case <- time.After(15 * time.Second):
			m.Sync()
		case <- m.Exit:
			return
		}
	}
}

func CreateMongoProcessor(dbName string, chain *chain.Chain) (*MongoProcessor, error) {
	m := &MongoProcessor{}
	m.log = log.GetLogger()
	m.Exit = make(chan struct{}, 2)
	m.chanTransactionAction = make(chan *transactionaction.TransactionAction, 100)
	m.unconfirmedTransactionsHashes = make(map [string]bool)
	m.chain = chain
	m.config = config.GetConfig()

	host := m.config.User.MongoProcessorConfig.Host
	port := m.config.User.MongoProcessorConfig.Port
	username := m.config.User.MongoProcessorConfig.Username
	password := m.config.User.MongoProcessorConfig.Password

	m.ctx, _ = context.WithTimeout(context.Background(), 60*time.Second)
	mongoURL := fmt.Sprintf("mongodb://%s:%d", host, port)
	if len(username) > 0 {
		mongoURL = fmt.Sprintf(
			"mongodb://%s:%s@%s:%d/%s",
			username, password, host, port, dbName)
	}
	clientOptions := options.Client().ApplyURI(mongoURL)
	client, err := mongo.NewClient(clientOptions)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	err = client.Connect(m.ctx)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	m.ctx = context.TODO()
	m.client = client
	m.database = m.client.Database(dbName)

	err = m.CreateIndexes()
	if err != nil {
		return nil, err
	}

	chain.GetTransactionPool().SetChanTransactionAction(m.chanTransactionAction)

	return m, nil
}
