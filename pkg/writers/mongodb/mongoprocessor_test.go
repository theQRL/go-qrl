package mongodb

import (
	"github.com/stretchr/testify/assert"
	"github.com/theQRL/go-qrl/pkg/config"
	"github.com/theQRL/go-qrl/pkg/core/block"
	"github.com/theQRL/go-qrl/pkg/core/chain"
	"github.com/theQRL/go-qrl/pkg/core/state"
	"github.com/theQRL/go-qrl/pkg/core/transactions"
	"github.com/theQRL/go-qrl/pkg/misc"
	"github.com/theQRL/go-qrl/pkg/ntp"
	"github.com/theQRL/go-qrl/pkg/pow"
	"github.com/theQRL/go-qrl/test/genesis"
	"github.com/theQRL/go-qrl/test/helper"
	"go.mongodb.org/mongo-driver/bson"
	"io/ioutil"
	"os"
	"testing"
)

type TestMongoProcessor struct {
	m       *MongoProcessor
	dt      pow.DifficultyTrackerInterface
	tempDir string
}

func CreateTestMongoProcessor() *TestMongoProcessor {
	tempDir, err := ioutil.TempDir("", "")

	conf := config.GetConfig()
	conf.User.MongoProcessorConfig.DBName = "EXPLORER_BETA"
	conf.Dev.Genesis.GenesisTimestamp = 1528402558
	conf.Dev.Genesis.GenesisPrevHeadehash = []byte("Thirst of Quantas")
	conf.Dev.Genesis.GenesisDifficulty = 5000
	conf.User.SetDataDir(tempDir)

	// TODO: Replace state with some temporary directory
	s, err := state.CreateState()
	c := chain.CreateChain(s)
	if err != nil {
		panic("Error while Creating Chain for Test")
	}
	dt := &pow.MockDifficultyTracker{}
	dt.SetGetReturnValue(2)
	c.SetDifficultyTracker(dt)
	genesisBlock, err := test.CreateGenesisBlock()
	if err != nil {
		panic("Error while Creating Genesis Block")
	}
	err = c.Load(genesisBlock)
	if err != nil {
		panic("Error loading Chain")
	}
	ntp.GetNTP() // Initialize NTP

	m := &TestMongoProcessor{}
	m.m, err = CreateMongoProcessor(conf.User.MongoProcessorConfig.DBName, c)
	if err != nil {
		return nil
	}
	m.dt = dt
	m.tempDir = tempDir
	return m
}

func (m *TestMongoProcessor) Clean() error {
	err := os.RemoveAll(m.tempDir)
	if err != nil {
		return err
	}
	return m.m.database.Drop(m.m.ctx)
}

func TestMongoProcessor_TransactionProcessor(t *testing.T) {
	m := CreateTestMongoProcessor()
	defer m.Clean()
	aliceXMSS := helper.GetAliceXMSS(4)
	bobXMSS := helper.GetBobXMSS(4)
	bytesAddrsTo := misc.StringAddressToBytesArray([]string{aliceXMSS.QAddress()})

	tx1 := transactions.CreateTransferTransaction(
		bytesAddrsTo,
		[]uint64{10},
		18446744073709551615, // Max Uint64 Value, will result into storage of -1 in MongoDB
		misc.UCharVectorToBytes(bobXMSS.PK()),
		misc.UCharVectorToBytes(bobXMSS.Address()),
	)
	tx1.Sign(bobXMSS, misc.BytesToUCharVector(tx1.GetHashableBytes()))
	accounts := make(map[string]*Account)
	m.m.TransactionProcessor(tx1.PBData(), 1, accounts)
	assert.Nil(t, m.m.WriteAll())

	cursor, err := m.m.transactionsCollection.Find(m.m.ctx, bson.D{{"transaction_hash", tx1.Txhash()}})
	assert.Nil(t, err)
	//
	// TODO: REcheck this part
	mongoTransaction, mongoTransferTx := ProtoToTransaction(tx1.PBData(), 1)
	for cursor.Next(m.m.ctx) {
		err := cursor.Decode(mongoTransaction)
		assert.Nil(t, err)
		err = cursor.Decode(mongoTransferTx)
		assert.Nil(t, err)
		break
	}
	assert.True(t, mongoTransaction.IsEqualPBData(tx1.PBData(), 1, 1))
	assert.True(t, mongoTransferTx.(*TransferTransaction).IsEqualPBData(tx1.PBData()))
	assert.Len(t, m.m.bulkTransactions, 0)
	assert.Len(t, m.m.bulkTransferTx, 0)
}

func TestMongoProcessor_IsCollectionExists(t *testing.T) {
	m := CreateTestMongoProcessor()
	defer m.Clean()

	found, err := m.m.IsCollectionExists("blocks")
	assert.Nil(t, err)
	assert.True(t, found)
	m.Clean()

	found, err = m.m.IsCollectionExists("blocks")
	assert.Nil(t, err)
	assert.False(t, found)
}

func TestMongoProcessor_Sync(t *testing.T) {
	m := CreateTestMongoProcessor()
	defer m.Clean()
	m.m.Sync()
	assert.Equal(t,
		"331f564613b106c27f9d57687270a5d722e9c590107902b78f00ebd38ddd0e2c",
		misc.Bin2HStr(m.m.lastBlock.HeaderHash))
	assert.Equal(t, int64(0), m.m.lastBlock.BlockNumber)

	cursor, err := m.m.transactionsCollection.Find(m.m.ctx, bson.D{{"block_number", 0}})
	assert.Nil(t, err)

	count := 0
	for cursor.Next(m.m.ctx) {
		count++
	}
	assert.Equal(t, count, 8)

	result := m.m.accountsCollection.FindOne(m.m.ctx, bson.D{{"address", m.m.config.Dev.Genesis.CoinbaseAddress}})
	account := &Account{}
	assert.Nil(t, result.Decode(account))
	assert.Equal(t, int64(40000000000000000), account.Balance)
}

func TestMongoProcessor_BlockProcessor(t *testing.T) {
	m := CreateTestMongoProcessor()
	defer m.Clean()
	m.m.Sync()
	genesisBlock := m.m.chain.GetLastBlock()

	bobXMSS := helper.GetBobXMSS(6)
	aliceXMSS := helper.GetAliceXMSS(6)
	slaveTx := transactions.CreateSlaveTransaction([][]byte{misc.UCharVectorToBytes(bobXMSS.PK())}, []uint32{0}, 0, misc.UCharVectorToBytes(aliceXMSS.PK()), nil)
	slaveTx.Sign(aliceXMSS, misc.BytesToUCharVector(slaveTx.GetHashableBytes()))
	slaveTx.PBData().Nonce = 1

	assert.True(t, slaveTx.Validate(true))
	n := ntp.GetMockedNTP()

	// Block 1
	n.SetTimestamp(1615270948)
	block1 := block.CreateBlock(
		misc.UCharVectorToBytes(aliceXMSS.Address()),
		1,
		genesisBlock.HeaderHash(),
		genesisBlock.Timestamp(),
		[]transactions.TransactionInterface {slaveTx},
		1615270948)
	block1.SetDifficultyTracker(m.dt)
	block1.SetNonces(2, 0)
	parentMetadata, err := m.m.chain.GetBlockMetaData(genesisBlock.HeaderHash())
	assert.Nil(t, err)
	measurement, err := m.m.chain.GetMeasurement(uint32(block1.Timestamp()), block1.PrevHeaderHash(), parentMetadata)
	assert.Nil(t, err)
	assert.True(t, block1.Validate(nil, genesisBlock, parentMetadata, measurement, nil))
	assert.True(t, m.m.chain.AddBlock(block1))
	assert.True(t, block1.IsEqual(m.m.chain.GetLastBlock()))

	// Check Bob is Alice's slave.
	aliceState, err := m.m.chain.GetAddressState(misc.UCharVectorToBytes(aliceXMSS.Address()))
	assert.Equal(t, len(aliceState.SlavePKSAccessType()), 1)
	assert.Contains(t, aliceState.SlavePKSAccessType(), misc.UCharVectorToString(bobXMSS.PK()))

	// Check MongoDB
	m.m.Sync()
	assert.Equal(t, m.m.lastBlock.HeaderHash, block1.HeaderHash())
	result := m.m.accountsCollection.FindOne(m.m.ctx, bson.D{{"address", m.m.config.Dev.Genesis.CoinbaseAddress}})
	account := &Account{}
	assert.Nil(t, result.Decode(account))
	assert.Equal(t, int64(39999993343650538), account.Balance)

	// Fork Block
	n.SetTimestamp(1715270948)
	forkBlock := block.CreateBlock(
		misc.UCharVectorToBytes(bobXMSS.Address()),
		1,
		genesisBlock.HeaderHash(),
		genesisBlock.Timestamp(),
		[]transactions.TransactionInterface {},
		1715270948)
	forkBlock.SetDifficultyTracker(m.dt)
	forkBlock.SetNonces(4, 0)
	parentMetadata, err = m.m.chain.GetBlockMetaData(genesisBlock.HeaderHash())
	assert.Nil(t, err)
	measurement, err = m.m.chain.GetMeasurement(uint32(forkBlock.Timestamp()), forkBlock.PrevHeaderHash(), parentMetadata)
	assert.Nil(t, err)
	assert.True(t, forkBlock.Validate(nil, genesisBlock, parentMetadata, measurement, nil))
	assert.True(t, m.m.chain.AddBlock(forkBlock))
	assert.True(t, block1.IsEqual(m.m.chain.GetLastBlock()))

	forkBlockFromState, err := m.m.chain.GetBlock(forkBlock.HeaderHash())
	assert.Nil(t, err)
	assert.True(t, forkBlock.IsEqual(forkBlockFromState))

	// Check MongoDB
	m.m.Sync()
	assert.Equal(t, m.m.lastBlock.HeaderHash, block1.HeaderHash())
	result = m.m.accountsCollection.FindOne(m.m.ctx, bson.D{{"address", m.m.config.Dev.Genesis.CoinbaseAddress}})
	account = &Account{}
	assert.Nil(t, result.Decode(account))
	assert.Equal(t, int64(39999993343650538), account.Balance)

	// Fork Block 2
	n.SetTimestamp(1815270948)
	forkBlock2 := block.CreateBlock(
		misc.UCharVectorToBytes(bobXMSS.Address()),
		2,
		forkBlock.HeaderHash(),
		forkBlock.Timestamp(),
		[]transactions.TransactionInterface {},
		1815270948)
	forkBlock2.SetDifficultyTracker(m.dt)
	forkBlock2.SetNonces(1, 0)
	parentMetadata, err = m.m.chain.GetBlockMetaData(forkBlock.HeaderHash())
	assert.Nil(t, err)
	measurement, err = m.m.chain.GetMeasurement(uint32(forkBlock2.Timestamp()), forkBlock2.PrevHeaderHash(), parentMetadata)
	assert.Nil(t, err)
	assert.True(t, forkBlock2.Validate(nil, forkBlock, parentMetadata, measurement, nil))
	assert.True(t, m.m.chain.AddBlock(forkBlock2))
	assert.True(t, forkBlock2.IsEqual(m.m.chain.GetLastBlock()))

	forkBlock2FromState, err := m.m.chain.GetBlock(forkBlock2.HeaderHash())
	assert.Nil(t, err)
	assert.True(t, forkBlock2FromState.IsEqual(forkBlock2))

	// Check MongoDB
	m.m.Sync()
	assert.Equal(t, m.m.lastBlock.HeaderHash, forkBlock2.HeaderHash())
	result = m.m.accountsCollection.FindOne(m.m.ctx, bson.D{{"address", m.m.config.Dev.Genesis.CoinbaseAddress}})
	account = &Account{}
	assert.Nil(t, result.Decode(account))
	assert.Equal(t, int64(39999986687302185), account.Balance)

	// Bob should not be Alice's slave after fork recovery as the slave txn no more belong to mainchain
	aliceState, err = m.m.chain.GetAddressState(misc.UCharVectorToBytes(aliceXMSS.Address()))
	assert.Equal(t, len(aliceState.SlavePKSAccessType()), 0)
	assert.NotContains(t, aliceState.SlavePKSAccessType(), misc.UCharVectorToString(bobXMSS.PK()))

	// Test if IsCollectionExists works
	found, err := m.m.IsCollectionExists("blocks")
	assert.True(t, found)

	// Test if CreateIndexes retrieves last block from db, if lastBlock is unknown
	m.m.lastBlock = nil
	err = m.m.CreateIndexes()
	assert.Equal(t, m.m.lastBlock.BlockNumber, int64(2))
}

func CreateBlock(minerAddress []byte, blockNumber uint64, prevBlock *block.Block, timestamp uint64) *block.Block {
	return block.CreateBlock(
		minerAddress,
		blockNumber,
		prevBlock.HeaderHash(),
		prevBlock.Timestamp(),
		[]transactions.TransactionInterface {},
		timestamp)

}

func TestMongoProcessor_ForkRecovery(t *testing.T) {
	m := CreateTestMongoProcessor()
	defer m.Clean()
	m.m.Sync()

	genesisBlock := m.m.chain.GetLastBlock()

	aliceXMSS := helper.GetAliceXMSS(6)
	aliceAddressBytes := misc.UCharVectorToBytes(aliceXMSS.Address())
	block1 := CreateBlock(aliceAddressBytes, 1, genesisBlock, 1815270948)
	block2 := CreateBlock(aliceAddressBytes, 2, block1, 1815270948)
	block3 := CreateBlock(aliceAddressBytes, 3, block2, 1815270948)

	bobXMSS := helper.GetBobXMSS(6)
	bobAddressBytes := misc.UCharVectorToBytes(bobXMSS.Address())
	forkBlock2 := CreateBlock(bobAddressBytes, 2, block1, 1815270948)
	forkBlock3 := CreateBlock(bobAddressBytes, 3, forkBlock2, 1815270948)
	forkBlock4 := CreateBlock(bobAddressBytes, 3, forkBlock3, 1815270948)

	m.m.chain.AddBlock(block1)
	m.m.chain.AddBlock(block2)
	m.m.chain.AddBlock(block3)

	m.m.chain.AddBlock(forkBlock2)
	assert.True(t, block3.IsEqual(m.m.chain.GetLastBlock()))

	m.m.chain.AddBlock(forkBlock3)
	assert.True(t, block3.IsEqual(m.m.chain.GetLastBlock()))

	m.m.chain.AddBlock(forkBlock4)
	assert.True(t, forkBlock4.IsEqual(m.m.chain.GetLastBlock()))

	m.m.Sync()
	assert.Equal(t, m.m.lastBlock.HeaderHash, forkBlock4.HeaderHash())
	result := m.m.accountsCollection.FindOne(m.m.ctx, bson.D{{"address", m.m.config.Dev.Genesis.CoinbaseAddress}})
	account := &Account{}
	assert.Nil(t, result.Decode(account))
	assert.Equal(t, int64(39999980030954939), account.Balance)

	result = m.m.accountsCollection.FindOne(m.m.ctx, bson.D{{"address", bobAddressBytes}})
	account = &Account{}
	assert.Nil(t, result.Decode(account))
	assert.Equal(t, int64(13312695599), account.Balance)

	result = m.m.accountsCollection.FindOne(m.m.ctx, bson.D{{"address", aliceAddressBytes}})
	account = &Account{}
	assert.Nil(t, result.Decode(account))
	assert.Equal(t, int64(6656349462), account.Balance)
}