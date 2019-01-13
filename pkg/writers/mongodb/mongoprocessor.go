package mongodb

import (
	"context"
	"fmt"
	"github.com/mongodb/mongo-go-driver/bson"
	"github.com/mongodb/mongo-go-driver/mongo"
	"github.com/mongodb/mongo-go-driver/mongo/options"
	"github.com/mongodb/mongo-go-driver/x/bsonx"
	"github.com/theQRL/go-qrl/pkg/config"
	"github.com/theQRL/go-qrl/pkg/core/block"
	"github.com/theQRL/go-qrl/pkg/core/chain"
	"github.com/theQRL/go-qrl/pkg/generated"
	"github.com/theQRL/go-qrl/pkg/log"
	"github.com/theQRL/go-qrl/pkg/misc"
	"reflect"
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

	bulkBlocks   []mongo.WriteModel
	bulkAccounts []mongo.WriteModel

	bulkTransactions    []mongo.WriteModel
	bulkCoinBaseTx      []mongo.WriteModel
	bulkTransferTx      []mongo.WriteModel
	bulkTokenTx         []mongo.WriteModel
	bulkTransferTokenTx []mongo.WriteModel
	bulkMessageTx       []mongo.WriteModel
	bulkSlaveTx         []mongo.WriteModel

	blocksCollection   *mongo.Collection
	accountsCollection *mongo.Collection

	transactionsCollection    *mongo.Collection
	coinBaseTxCollection      *mongo.Collection
	transferTxCollection      *mongo.Collection
	tokenTxCollection         *mongo.Collection
	transferTokenTxCollection *mongo.Collection
	messageTxCollection       *mongo.Collection
	slaveTxCollection         *mongo.Collection
}

func (m *MongoProcessor) BlockProcessor(b *block.Block) {
	mongoBlock := &Block{}
	mongoBlock.BlockFromPBData(b.PBData())
	operation := mongo.NewInsertOneModel()
	operation.Document(mongoBlock)
	m.bulkBlocks = append(m.bulkBlocks, operation)
	accounts := make(map[string]*Account)
	for _, tx := range b.Transactions() {
		m.TransactionProcessor(tx, b.BlockNumber(), accounts)
	}
	m.AccountProcessor(accounts)
	m.lastBlock = mongoBlock
}

func (m *MongoProcessor) TransactionProcessor(tx *generated.Transaction, blockNumber uint64, accounts map[string]*Account) {
	mongoTx, txDetails := ProtoToTransaction(tx, blockNumber)
	operation := mongo.NewInsertOneModel()
	operation.Document(mongoTx)
	m.bulkTransactions = append(m.bulkTransactions, operation)

	operation = mongo.NewInsertOneModel()
	operation.Document(txDetails)
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
	mongoTx.Apply(m, accounts, txDetails, int64(blockNumber))
}

func (m *MongoProcessor) AccountProcessor(accounts map[string]*Account) {
	for _, account := range accounts {
		operation := mongo.NewUpdateOneModel()

		operation.Upsert(true)
		operation.Filter(bsonx.Doc{{"address", bsonx.Binary(0, account.Address)}})
		operation.Update(account)
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

func (m *MongoProcessor) CreateIndexes() error {
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

	_, err = m.transactionsCollection.Indexes().CreateMany(context.Background(),
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

	_, err = m.coinBaseTxCollection.Indexes().CreateMany(context.Background(),
		[]mongo.IndexModel{
			{Keys: bson.M{"transaction_hash": int32(1)}},
			{Keys: bson.M{"address_to": int32(1)}},
		})

	if err != nil {
		m.log.Error("Error while modeling index for coinBase",
			"Error", err)
		return err
	}

	_, err = m.transferTxCollection.Indexes().CreateMany(context.Background(),
		[]mongo.IndexModel{
			{Keys: bson.M{"transaction_hash": int32(1)}},
			{Keys: bson.M{"addresses_to": int32(1)}},
		})

	if err != nil {
		m.log.Error("Error while modeling index for transferTx",
			"Error", err)
		return err
	}

	_, err = m.tokenTxCollection.Indexes().CreateMany(context.Background(),
		[]mongo.IndexModel{
			{Keys: bson.M{"transaction_hash": int32(1)}},
			{Keys: bson.M{"symbol": int32(1)}},
			{Keys: bson.M{"addresses_to": int32(1)}},
		})

	if err != nil {
		m.log.Error("Error while modeling index for tokenTx",
			"Error", err)
		return err
	}

	_, err = m.transferTokenTxCollection.Indexes().CreateMany(context.Background(),
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

	_, err = m.messageTxCollection.Indexes().CreateMany(context.Background(),
		[]mongo.IndexModel{
			{Keys: bson.M{"transaction_hash": int32(1)}},
		})

	if err != nil {
		m.log.Error("Error while modeling index for messageTx",
			"Error", err)
		return err
	}

	_, err = m.slaveTxCollection.Indexes().CreateMany(context.Background(),
		[]mongo.IndexModel{
			{Keys: bson.M{"transaction_hash": int32(1)}},
		})

	if err != nil {
		m.log.Error("Error while modeling index for slaveTx",
			"Error", err)
		return err
	}

	_, err = m.accountsCollection.Indexes().CreateMany(context.Background(),
		[]mongo.IndexModel{
			{Keys: bson.M{"address": int32(1)}},
		})

	if err != nil {
		m.log.Error("Error while modeling index for slaveTx",
			"Error", err)
		return err
	}

	return nil

}

func (m *MongoProcessor) IsAccountProcessed(blockNumber int64) bool {
	accounts := make(map[string]*Account)
	LoadAccount(m, config.GetConfig().Dev.Genesis.CoinbaseAddress, accounts, blockNumber)
	return accounts[misc.Bin2Qaddress(config.GetConfig().Dev.Genesis.CoinbaseAddress)].BlockNumber != blockNumber
}

func (m *MongoProcessor) RemoveBlock(b *Block) {
	accounts := make(map[string]*Account)
	txHashes := make(map[string]int8)
	m.RevertTransaction(b.BlockNumber, accounts, txHashes)

	if !m.IsAccountProcessed(b.BlockNumber) {
		m.AccountProcessor(accounts)
	}
	err := m.WriteAll()
	if err != nil {
		m.log.Error("[RemoveBlock] Error while Updating Accounts",
			"Error", err)
	}

	for txHash, txType := range txHashes {
		switch txType {
		case 0:
			_, err := m.coinBaseTxCollection.DeleteOne(m.ctx, bsonx.Doc{{"transaction_hash", bsonx.Binary(0, misc.HStr2Bin(txHash))}})
			if err != nil {
				m.log.Error("Error while removing CoinBaseTxn",
					"Error", err)
			}
		case 1:
			_, err := m.transferTxCollection.DeleteOne(m.ctx, bsonx.Doc{{"transaction_hash", bsonx.Binary(0, misc.HStr2Bin(txHash))}})
			if err != nil {
				m.log.Error("Error while removing TransferTxn",
					"Error", err)
			}
		case 2:
			_, err := m.tokenTxCollection.DeleteOne(m.ctx, bsonx.Doc{{"transaction_hash", bsonx.Binary(0, misc.HStr2Bin(txHash))}})
			if err != nil {
				m.log.Error("Error while removing TokenTxn",
					"Error", err)
			}
		case 3:
			_, err := m.transferTokenTxCollection.DeleteOne(m.ctx, bsonx.Doc{{"transaction_hash", bsonx.Binary(0, misc.HStr2Bin(txHash))}})
			if err != nil {
				m.log.Error("Error while removing TransferTokenTxn",
					"Error", err)
			}
		case 4:
			_, err := m.messageTxCollection.DeleteOne(m.ctx, bsonx.Doc{{"transaction_hash", bsonx.Binary(0, misc.HStr2Bin(txHash))}})
			if err != nil {
				m.log.Error("Error while removing MessageTxn",
					"Error", err)
			}
		case 5:
			_, err := m.slaveTxCollection.DeleteOne(m.ctx, bsonx.Doc{{"transaction_hash", bsonx.Binary(0, misc.HStr2Bin(txHash))}})
			if err != nil {
				m.log.Error("Error while removing SlaveTxn",
					"Error", err)
			}
		}
	}
	_, err = m.transactionsCollection.DeleteMany(m.ctx, bsonx.Doc{{"block_number", bsonx.Int64(b.BlockNumber)}})
	if err != nil {
		m.log.Error("Error while removing transaction",
			"Error", err)
	}

	_, err = m.blocksCollection.DeleteOne(m.ctx, bsonx.Doc{{"block_number", bsonx.Int64(b.BlockNumber)}})
	if err != nil {
		m.log.Error("Error while removing block",
			"Error", err)
	}
}

func (m *MongoProcessor) RevertTransaction(blockNumber int64, accounts map[string]*Account, txHashes map[string]int8) {
	cursor, err := m.transactionsCollection.Find(m.ctx, bson.D{{"block_number", blockNumber}})
	if err != nil {
		//TODO: Do Something
		m.log.Error("Error while Finding Transaction",
			"Error", err)
	}
	tx := &Transaction{}
	for cursor.Next(m.ctx) {
		err := cursor.Decode(tx)
		if err != nil {
			//TODO: Do Something
			m.log.Error("Error while Decoding Transaction",
				"Error", err)
		}
		txHashes[misc.Bin2HStr(tx.TransactionHash)] = tx.TransactionType
		switch tx.TransactionType {
		case 0:
			m.RevertCoinBaseTransaction(tx, accounts, blockNumber)
		case 1:
			m.RevertTransferTransaction(tx, accounts, blockNumber)
		case 2:
			m.RevertTokenTransaction(tx, accounts, blockNumber)
		case 3:
			m.RevertTransferTokenTransaction(tx, accounts, blockNumber)
		case 4:
			m.RevertMessageTransaction(tx, accounts, blockNumber)
		case 5:
			m.RevertSlaveTransaction(tx, accounts, blockNumber)
		default:
			fmt.Println("Error no type found")
		}
	}
}

func (m *MongoProcessor) RevertCoinBaseTransaction(tx *Transaction, accounts map[string]*Account, blockNumber int64) {
	cursor, err := m.coinBaseTxCollection.Find(m.ctx, bson.D{{"transaction_hash", tx.TransactionHash}})
	if err != nil {
		//TODO: Do Something
		m.log.Error("Error while Finding CoinBase Transaction",
			"Error", err)
	}
	coinBaseTx := &CoinBaseTransaction{}
	for cursor.Next(m.ctx) {
		err := cursor.Decode(coinBaseTx)
		if err != nil {
			//TODO: Do Something
			m.log.Error("Error while Decoding CoinBase Transaction",
				"Error", err)
		}
		tx.Revert(m, accounts, coinBaseTx, blockNumber)
	}
}

func (m *MongoProcessor) RevertTransferTransaction(tx *Transaction, accounts map[string]*Account, blockNumber int64) {
	cursor, err := m.coinBaseTxCollection.Find(m.ctx, bson.D{{"transaction_hash", tx.TransactionHash}})
	if err != nil {
		//TODO: Do Something
		m.log.Error("Error while Finding Transfer Transaction",
			"Error", err)
	}
	transferTx := &TransferTransaction{}
	for cursor.Next(m.ctx) {
		err := cursor.Decode(transferTx)
		if err != nil {
			//TODO: Do Something
			m.log.Error("Error while Decoding Transfer Transaction",
				"Error", err)
		}
		tx.Revert(m, accounts, transferTx, blockNumber)
	}
}

func (m *MongoProcessor) RevertTokenTransaction(tx *Transaction, accounts map[string]*Account, blockNumber int64) {
	cursor, err := m.tokenTxCollection.Find(m.ctx, bson.D{{"transaction_hash", tx.TransactionHash}})
	if err != nil {
		//TODO: Do Something
		m.log.Error("Error while Finding Token Transaction",
			"Error", err)
	}
	tokenTx := &TokenTransaction{}
	for cursor.Next(m.ctx) {
		err := cursor.Decode(tokenTx)
		if err != nil {
			//TODO: Do Something
			m.log.Error("Error while Decoding Token Transaction",
				"Error", err)
		}
		tx.Revert(m, accounts, tokenTx, blockNumber)
	}
}

func (m *MongoProcessor) RevertTransferTokenTransaction(tx *Transaction, accounts map[string]*Account, blockNumber int64) {
	cursor, err := m.transferTokenTxCollection.Find(m.ctx, bson.D{{"transaction_hash", tx.TransactionHash}})
	if err != nil {
		//TODO: Do Something
		m.log.Error("Error while Finding Transfer Token Transaction",
			"Error", err)
	}
	transferTokenTx := &TransferTokenTransaction{}
	for cursor.Next(m.ctx) {
		err := cursor.Decode(transferTokenTx)
		if err != nil {
			//TODO: Do Something
			m.log.Error("Error while Decoding Transfer Token Transaction",
				"Error", err)
		}
		tx.Revert(m, accounts, transferTokenTx, blockNumber)
	}
}

func (m *MongoProcessor) RevertMessageTransaction(tx *Transaction, accounts map[string]*Account, blockNumber int64) {
	cursor, err := m.messageTxCollection.Find(m.ctx, bson.D{{"transaction_hash", tx.TransactionHash}})
	if err != nil {
		//TODO: Do Something
		m.log.Error("Error while Finding Message Transaction",
			"Error", err)
	}
	messageTx := &MessageTransaction{}
	for cursor.Next(m.ctx) {
		err := cursor.Decode(messageTx)
		if err != nil {
			//TODO: Do Something
			m.log.Error("Error while Decoding Message Transaction",
				"Error", err)
		}
		tx.Revert(m, accounts, messageTx, blockNumber)
	}
}

func (m *MongoProcessor) RevertSlaveTransaction(tx *Transaction, accounts map[string]*Account, blockNumber int64) {
	cursor, err := m.messageTxCollection.Find(m.ctx, bson.D{{"transaction_hash", tx.TransactionHash}})
	if err != nil {
		//TODO: Do Something
		m.log.Error("Error while Finding Slave Transaction",
			"Error", err)
	}
	slaveTx := &SlaveTransaction{}
	for cursor.Next(m.ctx) {
		err := cursor.Decode(slaveTx)
		if err != nil {
			//TODO: Do Something
			m.log.Error("Error while Decoding Slave Transaction",
				"Error", err)
		}
		tx.Revert(m, accounts, slaveTx, blockNumber)
	}
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

func (m *MongoProcessor) ForkRecovery() {
	for {
		b, err := m.GetLastBlockFromDB()
		if err != nil {
			m.log.Error("[ForkRecovery] Error while Retrieving last block",
				"Error", err)
		}
		b2, err := m.chain.GetBlockByNumber(uint64(b.BlockNumber))
		if err == nil {
			if reflect.DeepEqual(b2.HeaderHash(), b.HeaderHash) {
				m.lastBlock = b
				return // Fork Recovery Finished
			}
		}
		m.RemoveBlock(b)
	}
}

func (m *MongoProcessor) Sync() {
	for {
		lastBlock := m.chain.GetLastBlock()
		lastBlockNumber := int64(lastBlock.BlockNumber())
		if lastBlockNumber == m.lastBlock.BlockNumber {
			return
		} else if lastBlockNumber > m.lastBlock.BlockNumber {
			b, _ := m.chain.GetBlockByNumber(uint64(m.lastBlock.BlockNumber))
			if !reflect.DeepEqual(b.HeaderHash(), m.lastBlock.HeaderHash) {
				m.ForkRecovery()
			}
			blockNumber := uint64(m.lastBlock.BlockNumber) + 1
			b, err := m.chain.GetBlockByNumber(blockNumber)
			if err != nil {
				m.log.Error("[MongoProcessor.Sync]Error while Getting BlockNumber",
					"BlockNumber", blockNumber,
					"Error", err)
			}
			if !reflect.DeepEqual(b.PrevHeaderHash(), m.lastBlock.HeaderHash) {
				m.ForkRecovery()
			}
			m.BlockProcessor(b)
			err = m.WriteAll()
			if err != nil {
				m.log.Error("[Sync] Error while WriteAll",
					"Error", err)
			}
		} else {
			m.ForkRecovery()
		}
	}
}

func (m *MongoProcessor) Run() {
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
	m.Exit = make(chan struct{})

	m.chain = chain
	m.config = config.GetConfig()
	host := m.config.User.MongoProcessorConfig.Host
	port := m.config.User.MongoProcessorConfig.Port

	m.ctx, _ = context.WithTimeout(context.Background(), 60*time.Second)
	client, err := mongo.Connect(m.ctx, fmt.Sprintf("mongodb://%s:%d", host, port))
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	m.ctx = context.TODO()
	m.client = client
	found, err := m.IsDataBaseExists(dbName)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	m.database = client.Database(dbName)
	m.blocksCollection = m.database.Collection("blocks")
	m.accountsCollection = m.database.Collection("accounts")
	m.transactionsCollection = m.database.Collection("txs")
	m.coinBaseTxCollection = m.database.Collection("coin_base_txs")
	m.transferTxCollection = m.database.Collection("transfer_txs")
	m.tokenTxCollection = m.database.Collection("token_txs")
	m.transferTokenTxCollection = m.database.Collection("transfer_token_txs")
	m.messageTxCollection = m.database.Collection("message_txs")
	m.slaveTxCollection = m.database.Collection("slave_txs")
	if !found {
		fmt.Println("Index being created")
		err := m.CreateIndexes()
		if err != nil {
			m.log.Error("Error CreateIndexes", "Error", err)
		}

		b, err := m.chain.GetBlockByNumber(0)
		if err != nil {
			m.log.Error("[MongoProcessor.Sync]Error while Getting BlockNumber",
				"BlockNumber", b.BlockNumber(),
				"Error", err)
		}
		coinBaseAddress := m.config.Dev.Genesis.CoinbaseAddress
		account := CreateAccount(coinBaseAddress, 0)
		account.AddBalance(m.config.Dev.Genesis.MaxCoinSupply)
		m.AccountProcessor(map[string]*Account{misc.Bin2Qaddress(coinBaseAddress):account})
		_ = m.WriteAll()
		m.BlockProcessor(b)
		_ = m.WriteAll()
	} else {
		b, err := m.GetLastBlockFromDB()
		if err != nil {
			return nil, err
		}
		m.lastBlock = b
	}
	return m, nil
}
