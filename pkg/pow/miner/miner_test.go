package miner

import (
	"github.com/stretchr/testify/assert"
	"github.com/theQRL/go-qrl/pkg/config"
	"github.com/theQRL/go-qrl/pkg/core/chain"
	"github.com/theQRL/go-qrl/pkg/core/state"
	"github.com/theQRL/go-qrl/pkg/core/transactions"
	"github.com/theQRL/go-qrl/pkg/misc"
	"github.com/theQRL/go-qrl/pkg/ntp"
	"github.com/theQRL/go-qrl/pkg/p2p/messages"
	"github.com/theQRL/go-qrl/pkg/pow"
	"github.com/theQRL/go-qrl/test/genesis"
	"github.com/theQRL/go-qrl/test/helper"
	"github.com/theQRL/qryptonight/goqryptonight"
	"io/ioutil"
	"os"
	"testing"
	"time"
)

type TestMiner struct {
	m *Miner
	s *state.State // This could be used to mock AddressState
}

func NewTestMiner() *TestMiner {
	tempDir, err := ioutil.TempDir("", "")

	conf := config.GetConfig()
	conf.User.Miner.MiningAddress = "Q01050086390643df86ff4810b62bdb3e4539040046b5e3956eff2476c5c79807e78ecda2994a91"
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

	m := CreateMiner(c, make(chan *messages.RegisterMessage, 100))

	return &TestMiner{m:m, s:s}
}

func CleanUp(dir string) {
	os.RemoveAll(dir)
}

func TestCreateMiner(t *testing.T) {
	m := NewTestMiner()
	defer CleanUp(m.m.config.User.QrlDir)

	assert.NotNil(t, m)
}

func TestMiner_CreateBlock(t *testing.T) {
	/*
	CreateBlock with empty transaction pool
	 */
	m := NewTestMiner()
	defer CleanUp(m.m.config.User.QrlDir)

	aliceXMSS := helper.GetAliceXMSS(4)
	minerAddress := misc.UCharVectorToBytes(aliceXMSS.Address())
	parentBlock := m.m.chain.GetLastBlock()
	miningNonce := uint32(0)

	b, err := m.m.CreateBlock(
		minerAddress,
		parentBlock,
		miningNonce,
		m.m.chain.GetTransactionPool(),
	)

	assert.NotNil(t, b)
	assert.Nil(t, err)
}

func TestMiner_CreateBlock2(t *testing.T) {
	/*
	A transfer transaction added into transaction pool that fails validation due to insufficient balance.
	This transaction should not be included in the block as well as should have been removed from the txPool.
	 */
	m := NewTestMiner()
	defer CleanUp(m.m.config.User.QrlDir)

	bobXMSS := helper.GetBobXMSS(4)

	aliceXMSS := helper.GetAliceXMSS(4)
	minerAddress := misc.UCharVectorToBytes(aliceXMSS.Address())
	parentBlock := m.m.chain.GetLastBlock()
	miningNonce := uint32(0)
	txPool := m.m.chain.GetTransactionPool()

	bytesAddrsTo := misc.StringAddressToBytesArray([]string{aliceXMSS.QAddress()})
	tx1 := transactions.CreateTransferTransaction(
			bytesAddrsTo,
			[]uint64{10},
			20,
			misc.UCharVectorToBytes(bobXMSS.PK()),
			nil,
		)
	tx1.Sign(bobXMSS, misc.BytesToUCharVector(tx1.GetHashableBytes()))
	err := txPool.Add(tx1, parentBlock.BlockNumber(), parentBlock.Timestamp())
	assert.Nil(t, err)
	assert.True(t, txPool.Contains(tx1))

	b, err := m.m.CreateBlock(
		minerAddress,
		parentBlock,
		miningNonce,
		txPool,
	)

	assert.NotNil(t, b)
	assert.Nil(t, err)
	assert.Len(t, b.Transactions(), 1)
	assert.False(t, txPool.Contains(tx1))
}

func TestMiner_CreateBlock3(t *testing.T) {
	/*
	A transfer transaction(tx1) added into transaction pool that fails validation due to insufficient balance.
	tx1 should not be included in the block as well as should have been removed from the txPool.
	 */
	m := NewTestMiner()
	defer CleanUp(m.m.config.User.QrlDir)

	bobXMSS := helper.GetBobXMSS(4)
	err := m.s.MockAddressState(misc.UCharVectorToBytes(bobXMSS.Address()), 0, 100)
	assert.Nil(t, err)

	aliceXMSS := helper.GetAliceXMSS(4)
	minerAddress := misc.UCharVectorToBytes(aliceXMSS.Address())
	parentBlock := m.m.chain.GetLastBlock()
	miningNonce := uint32(0)
	txPool := m.m.chain.GetTransactionPool()

	bytesAddrsTo := misc.StringAddressToBytesArray([]string{aliceXMSS.QAddress()})
	tx1 := transactions.CreateTransferTransaction(
		bytesAddrsTo,
		[]uint64{10},
		20,
		misc.UCharVectorToBytes(bobXMSS.PK()),
		nil,
	)
	tx1.Sign(bobXMSS, misc.BytesToUCharVector(tx1.GetHashableBytes()))
	err = txPool.Add(tx1, parentBlock.BlockNumber(), parentBlock.Timestamp())
	assert.Nil(t, err)
	assert.True(t, txPool.Contains(tx1))

	b, err := m.m.CreateBlock(
		minerAddress,
		parentBlock,
		miningNonce,
		txPool,
	)

	assert.NotNil(t, b)
	assert.Nil(t, err)
	assert.Len(t, b.Transactions(), 2)
	assert.Equal(t, b.Transactions()[1], tx1.PBData())
	assert.True(t, txPool.Contains(tx1))
}

func TestMiner_CreateBlock4(t *testing.T) {
	/*
	A transfer transaction(tx1) added into transaction pool that fails validation due to insufficient balance.
	While another transfer transaction(tx2) has been added into the block that successfully passes the validation.
	tx1 should not be included in the block as well as should have been removed from the txPool.
	tx2 should be included in the block and should persist into txPool.
	 */
	m := NewTestMiner()
	defer CleanUp(m.m.config.User.QrlDir)

	bobXMSS := helper.GetBobXMSS(4)
	err := m.s.MockAddressState(misc.UCharVectorToBytes(bobXMSS.Address()), 0, 30)
	assert.Nil(t, err)

	aliceXMSS := helper.GetAliceXMSS(4)
	minerAddress := misc.UCharVectorToBytes(aliceXMSS.Address())
	parentBlock := m.m.chain.GetLastBlock()
	miningNonce := uint32(0)
	txPool := m.m.chain.GetTransactionPool()

	bytesAddrsTo := misc.StringAddressToBytesArray([]string{aliceXMSS.QAddress()})
	tx1 := transactions.CreateTransferTransaction(
		bytesAddrsTo,
		[]uint64{100},
		20,
		misc.UCharVectorToBytes(bobXMSS.PK()),
		nil,
	)
	tx1.Sign(bobXMSS, misc.BytesToUCharVector(tx1.GetHashableBytes()))
	err = txPool.Add(tx1, parentBlock.BlockNumber(), parentBlock.Timestamp())
	assert.Nil(t, err)
	assert.True(t, txPool.Contains(tx1))

	tx2 := transactions.CreateTransferTransaction(
		bytesAddrsTo,
		[]uint64{10},
		20,
		misc.UCharVectorToBytes(bobXMSS.PK()),
		nil,
	)
	tx2.Sign(bobXMSS, misc.BytesToUCharVector(tx2.GetHashableBytes()))
	err = txPool.Add(tx2, parentBlock.BlockNumber(), parentBlock.Timestamp())
	assert.Nil(t, err)
	assert.True(t, txPool.Contains(tx1))

	b, err := m.m.CreateBlock(
		minerAddress,
		parentBlock,
		miningNonce,
		txPool,
	)

	assert.NotNil(t, b)
	assert.Nil(t, err)
	assert.Len(t, b.Transactions(), 2)
	assert.Equal(t, b.Transactions()[1], tx2.PBData())
	assert.False(t, txPool.Contains(tx1))
	assert.True(t, txPool.Contains(tx2))
}

func TestMiner_PrepareNextBlockTemplate(t *testing.T) {
	/*
	A transfer transaction(tx1) added into transaction pool that fails validation due to insufficient balance.
	While another transfer transaction(tx2) has been added into the block that successfully passes the validation.
	tx1 should not be included in the block as well as should have been removed from the txPool.
	tx2 should be included in the block and should persist into txPool.
	 */
	m := NewTestMiner()
	defer CleanUp(m.m.config.User.QrlDir)

	bobXMSS := helper.GetBobXMSS(4)
	err := m.s.MockAddressState(misc.UCharVectorToBytes(bobXMSS.Address()), 0, 30)
	assert.Nil(t, err)

	aliceXMSS := helper.GetAliceXMSS(4)
	minerAddress := misc.UCharVectorToBytes(aliceXMSS.Address())
	parentBlock := m.m.chain.GetLastBlock()
	txPool := m.m.chain.GetTransactionPool()

	bytesAddrsTo := misc.StringAddressToBytesArray([]string{aliceXMSS.QAddress()})
	tx1 := transactions.CreateTransferTransaction(
		bytesAddrsTo,
		[]uint64{100},
		20,
		misc.UCharVectorToBytes(bobXMSS.PK()),
		nil,
	)
	tx1.Sign(bobXMSS, misc.BytesToUCharVector(tx1.GetHashableBytes()))
	err = txPool.Add(tx1, parentBlock.BlockNumber(), parentBlock.Timestamp())
	assert.Nil(t, err)
	assert.True(t, txPool.Contains(tx1))

	tx2 := transactions.CreateTransferTransaction(
		bytesAddrsTo,
		[]uint64{10},
		20,
		misc.UCharVectorToBytes(bobXMSS.PK()),
		nil,
	)
	tx2.Sign(bobXMSS, misc.BytesToUCharVector(tx2.GetHashableBytes()))
	err = txPool.Add(tx2, parentBlock.BlockNumber(), parentBlock.Timestamp())
	assert.Nil(t, err)
	assert.True(t, txPool.Contains(tx1))

	err = m.m.PrepareNextBlockTemplate(
		minerAddress,
		parentBlock,
		txPool,
		true,
	)
	assert.Nil(t, err)

	assert.NotNil(t, m.m.miningBlock)
	assert.Nil(t, err)
	assert.Len(t, m.m.miningBlock.Transactions(), 2)
	assert.Equal(t, m.m.miningBlock.Transactions()[1], tx2.PBData())
	assert.False(t, txPool.Contains(tx1))
	assert.True(t, txPool.Contains(tx2))
}

func TestMiner_StartMining(t *testing.T) {
	m := NewTestMiner()
	defer CleanUp(m.m.config.User.QrlDir)

	bobXMSS := helper.GetBobXMSS(4)
	err := m.s.MockAddressState(misc.UCharVectorToBytes(bobXMSS.Address()), 0, 30)
	assert.Nil(t, err)

	aliceXMSS := helper.GetAliceXMSS(4)
	minerAddress := misc.UCharVectorToBytes(aliceXMSS.Address())
	parentBlock := m.m.chain.GetLastBlock()
	txPool := m.m.chain.GetTransactionPool()

	bytesAddrsTo := misc.StringAddressToBytesArray([]string{aliceXMSS.QAddress()})
	tx1 := transactions.CreateTransferTransaction(
		bytesAddrsTo,
		[]uint64{100},
		20,
		misc.UCharVectorToBytes(bobXMSS.PK()),
		nil,
	)
	tx1.Sign(bobXMSS, misc.BytesToUCharVector(tx1.GetHashableBytes()))
	err = txPool.Add(tx1, parentBlock.BlockNumber(), parentBlock.Timestamp())
	assert.Nil(t, err)
	assert.True(t, txPool.Contains(tx1))

	tx2 := transactions.CreateTransferTransaction(
		bytesAddrsTo,
		[]uint64{10},
		20,
		misc.UCharVectorToBytes(bobXMSS.PK()),
		nil,
	)
	tx2.Sign(bobXMSS, misc.BytesToUCharVector(tx2.GetHashableBytes()))
	err = txPool.Add(tx2, parentBlock.BlockNumber(), parentBlock.Timestamp())
	assert.Nil(t, err)
	assert.True(t, txPool.Contains(tx1))

	err = m.m.PrepareNextBlockTemplate(
		minerAddress,
		parentBlock,
		txPool,
		true,
	)
	assert.Nil(t, err)
	notify := make(chan bool)
	m.m.notify = notify
	go m.m.StartMining()
	m.m.mode = 1
	m.m.mineNextBlockChan <- true
	select {
		case <-notify:
			assert.Equal(t, m.m.chain.GetLastBlock().BlockNumber(), uint64(1))
		case <-time.After(15 * time.Second):
			panic("Test Timeout")
	}
}

func TestMiner_HandleEvent(t *testing.T) {
	m := NewTestMiner()
	defer CleanUp(m.m.config.User.QrlDir)

	bobXMSS := helper.GetBobXMSS(4)
	err := m.s.MockAddressState(misc.UCharVectorToBytes(bobXMSS.Address()), 0, 30)
	assert.Nil(t, err)

	aliceXMSS := helper.GetAliceXMSS(4)
	minerAddress := misc.UCharVectorToBytes(aliceXMSS.Address())
	parentBlock := m.m.chain.GetLastBlock()
	txPool := m.m.chain.GetTransactionPool()

	bytesAddrsTo := misc.StringAddressToBytesArray([]string{aliceXMSS.QAddress()})
	tx1 := transactions.CreateTransferTransaction(
		bytesAddrsTo,
		[]uint64{100},
		20,
		misc.UCharVectorToBytes(bobXMSS.PK()),
		nil,
	)
	tx1.Sign(bobXMSS, misc.BytesToUCharVector(tx1.GetHashableBytes()))
	err = txPool.Add(tx1, parentBlock.BlockNumber(), parentBlock.Timestamp())
	assert.Nil(t, err)
	assert.True(t, txPool.Contains(tx1))

	tx2 := transactions.CreateTransferTransaction(
		bytesAddrsTo,
		[]uint64{10},
		20,
		misc.UCharVectorToBytes(bobXMSS.PK()),
		nil,
	)
	tx2.Sign(bobXMSS, misc.BytesToUCharVector(tx2.GetHashableBytes()))
	err = txPool.Add(tx2, parentBlock.BlockNumber(), parentBlock.Timestamp())
	assert.Nil(t, err)
	assert.True(t, txPool.Contains(tx1))

	err = m.m.PrepareNextBlockTemplate(
		minerAddress,
		parentBlock,
		txPool,
		true,
	)
	assert.Nil(t, err)

	event := goqryptonight.NewMinerEvent()
	event.SetXtype(goqryptonight.SOLUTION)
	assert.Equal(t, m.m.HandleEvent(event), uint8(1))

	event.SetXtype(goqryptonight.TIMEOUT)
	assert.Equal(t, m.m.HandleEvent(event), uint8(0))
}

func TestMiner_GetBlockToMine(t *testing.T) {
	m := NewTestMiner()
	defer CleanUp(m.m.config.User.QrlDir)

	n := ntp.GetMockedNTP()
	m.m.ntp = n
	n.SetTimestamp(1615270948)

	bobXMSS := helper.GetBobXMSS(4)
	err := m.s.MockAddressState(misc.UCharVectorToBytes(bobXMSS.Address()), 0, 30)
	assert.Nil(t, err)

	aliceXMSS := helper.GetAliceXMSS(4)
	minerAddress := misc.UCharVectorToBytes(aliceXMSS.Address())
	parentBlock := m.m.chain.GetLastBlock()
	txPool := m.m.chain.GetTransactionPool()

	bytesAddrsTo := misc.StringAddressToBytesArray([]string{aliceXMSS.QAddress()})
	tx1 := transactions.CreateTransferTransaction(
		bytesAddrsTo,
		[]uint64{100},
		20,
		misc.UCharVectorToBytes(bobXMSS.PK()),
		nil,
	)
	tx1.Sign(bobXMSS, misc.BytesToUCharVector(tx1.GetHashableBytes()))
	err = txPool.Add(tx1, parentBlock.BlockNumber(), parentBlock.Timestamp())
	assert.Nil(t, err)
	assert.True(t, txPool.Contains(tx1))

	tx2 := transactions.CreateTransferTransaction(
		bytesAddrsTo,
		[]uint64{10},
		20,
		misc.UCharVectorToBytes(bobXMSS.PK()),
		nil,
	)
	tx2.Sign(bobXMSS, misc.BytesToUCharVector(tx2.GetHashableBytes()))
	err = txPool.Add(tx2, parentBlock.BlockNumber(), parentBlock.Timestamp())
	assert.Nil(t, err)
	assert.True(t, txPool.Contains(tx1))

	parentMetadata, err := m.m.chain.GetBlockMetaData(parentBlock.HeaderHash())
	blob, currentDifficulty, err := m.m.GetBlockToMine(misc.Bin2Qaddress(minerAddress), parentBlock, parentMetadata.BlockDifficulty(), txPool)

	assert.Nil(t, err)
	assert.Equal(t, blob, "002e57e08cff4cb8fe720dd1dfe7078b6196ad181aef9a6cf2b3263ec35c11b5dc9a9fc1595b8d0000000000000000000000000000000000e1d38c798e74c6e882228614848615763b5c158b")
	assert.Equal(t, currentDifficulty, uint64(2))

	assert.NotNil(t, m.m.miningBlock)
	assert.Nil(t, err)
	assert.Len(t, m.m.miningBlock.Transactions(), 2)
	assert.Equal(t, m.m.miningBlock.Transactions()[1], tx2.PBData())
	assert.False(t, txPool.Contains(tx1))
	assert.True(t, txPool.Contains(tx2))
}

func TestMiner_SubmitMinedBlock(t *testing.T) {
	m := NewTestMiner()
	defer CleanUp(m.m.config.User.QrlDir)

	n := ntp.GetMockedNTP()
	m.m.ntp = n
	n.SetTimestamp(1615270948)

	bobXMSS := helper.GetBobXMSS(4)
	err := m.s.MockAddressState(misc.UCharVectorToBytes(bobXMSS.Address()), 0, 30)
	assert.Nil(t, err)

	aliceXMSS := helper.GetAliceXMSS(4)
	minerAddress := misc.UCharVectorToBytes(aliceXMSS.Address())
	parentBlock := m.m.chain.GetLastBlock()
	txPool := m.m.chain.GetTransactionPool()

	bytesAddrsTo := misc.StringAddressToBytesArray([]string{aliceXMSS.QAddress()})
	tx1 := transactions.CreateTransferTransaction(
		bytesAddrsTo,
		[]uint64{100},
		20,
		misc.UCharVectorToBytes(bobXMSS.PK()),
		nil,
	)
	tx1.Sign(bobXMSS, misc.BytesToUCharVector(tx1.GetHashableBytes()))
	err = txPool.Add(tx1, parentBlock.BlockNumber(), parentBlock.Timestamp())
	assert.Nil(t, err)
	assert.True(t, txPool.Contains(tx1))

	tx2 := transactions.CreateTransferTransaction(
		bytesAddrsTo,
		[]uint64{10},
		20,
		misc.UCharVectorToBytes(bobXMSS.PK()),
		nil,
	)
	tx2.Sign(bobXMSS, misc.BytesToUCharVector(tx2.GetHashableBytes()))
	err = txPool.Add(tx2, parentBlock.BlockNumber(), parentBlock.Timestamp())
	assert.Nil(t, err)
	assert.True(t, txPool.Contains(tx1))

	parentMetadata, err := m.m.chain.GetBlockMetaData(parentBlock.HeaderHash())
	blob, currentDifficulty, err := m.m.GetBlockToMine(misc.Bin2Qaddress(minerAddress), parentBlock, parentMetadata.BlockDifficulty(), txPool)

	assert.Nil(t, err)
	assert.Equal(t, blob, "002e57e08cff4cb8fe720dd1dfe7078b6196ad181aef9a6cf2b3263ec35c11b5dc9a9fc1595b8d0000000000000000000000000000000000e1d38c798e74c6e882228614848615763b5c158b")
	assert.Equal(t, currentDifficulty, uint64(2))

	assert.NotNil(t, m.m.miningBlock)
	assert.Nil(t, err)
	assert.Len(t, m.m.miningBlock.Transactions(), 2)
	assert.Equal(t, m.m.miningBlock.Transactions()[1], tx2.PBData())
	assert.False(t, txPool.Contains(tx1))
	assert.True(t, txPool.Contains(tx2))

	time.Sleep(4 * time.Second)

	assert.True(t, m.m.SubmitMinedBlock(misc.HStr2Bin("002e57e08cff4cb8fe720dd1dfe7078b6196ad181aef9a6cf2b3263ec35c11b5dc9a9fc1595b8d0000000400000000000000000000000000e1d38c798e74c6e882228614848615763b5c158b")))
}