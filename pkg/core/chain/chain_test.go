package chain

import (
	"github.com/stretchr/testify/assert"
	"github.com/theQRL/go-qrl/pkg/config"
	"github.com/theQRL/go-qrl/pkg/core/block"
	"github.com/theQRL/go-qrl/pkg/core/state"
	"github.com/theQRL/go-qrl/pkg/core/transactions"
	"github.com/theQRL/go-qrl/pkg/misc"
	"github.com/theQRL/go-qrl/pkg/ntp"
	"github.com/theQRL/go-qrl/pkg/pow"
	"github.com/theQRL/go-qrl/test/genesis"
	"github.com/theQRL/go-qrl/test/helper"
	"io/ioutil"
	"os"
	"testing"
)

type TestChain struct {
	c *Chain
}

func NewTestChain() (*TestChain, pow.DifficultyTrackerInterface) {
	tempDir, err := ioutil.TempDir("", "")

	conf := config.GetConfig()
	conf.Dev.Genesis.GenesisTimestamp = 1528402558
	conf.Dev.Genesis.GenesisPrevHeadehash = []byte("Thirst of Quantas")
	conf.Dev.Genesis.GenesisDifficulty = 5000
	conf.User.SetDataDir(tempDir)

	// TODO: Replace state with some temporary directory
	s, err := state.CreateState()
	c := CreateChain(s)
	if err != nil {
		panic("Error while Creating Chain for Test")
	}
	dt := &pow.MockDifficultyTracker{}
	dt.SetGetReturnValue(2)
	c.dt = dt
	genesisBlock, err := test.CreateGenesisBlock()
	if err != nil {
		panic("Error while Creating Genesis Block")
	}
	err = c.Load(genesisBlock)
	if err != nil {
		panic("Error loading Chain")
	}
	ntp.GetNTP() // Initialize NTP
	testChain := &TestChain{c: c}

	return testChain, dt
}

func CleanUp(dir string) {
	os.RemoveAll(dir)
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

func TestCreateChain(t *testing.T) {
	c, dt := NewTestChain()
	assert.NotNil(t, c)
	assert.NotNil(t, dt)
}

func TestChain_AddBlock(t *testing.T) {
	/*
		Add block1 on genesis block (that registers Bob as Alice's slave)
    	Bob should be slave of Alice's XMSS.
		Add a competing forkBlock on genesis block (without the SlaveTransaction)
        Add forkBlock2 on forkBlock (without the SlaveTransaction), triggers fork recovery
        Bob should not be slave of Alice's XMSS, as slave transaction is no more the part of main chain.
	*/
	c, dt := NewTestChain()
	defer CleanUp(c.c.config.User.QrlDir)

	genesisBlock := c.c.lastBlock

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
	block1.SetDifficultyTracker(dt)
	block1.SetNonces(2, 0)
	parentMetadata, err := c.c.GetBlockMetaData(genesisBlock.HeaderHash())
	assert.Nil(t, err)
	measurement, err := c.c.GetMeasurement(uint32(block1.Timestamp()), block1.PrevHeaderHash(), parentMetadata)
	assert.Nil(t, err)
	assert.True(t, block1.Validate(nil, genesisBlock, parentMetadata, measurement, nil))
	assert.True(t, c.c.AddBlock(block1))
	assert.True(t, block1.IsEqual(c.c.lastBlock))

	// Check Bob is Alice's slave.
	aliceState, err := c.c.state.GetAddressState(misc.UCharVectorToBytes(aliceXMSS.Address()))
	assert.Equal(t, len(aliceState.SlavePKSAccessType()), 1)
	assert.Contains(t, aliceState.SlavePKSAccessType(), misc.UCharVectorToString(bobXMSS.PK()))

	// Fork Block
	n.SetTimestamp(1715270948)
	forkBlock := block.CreateBlock(
		misc.UCharVectorToBytes(bobXMSS.Address()),
		1,
		genesisBlock.HeaderHash(),
		genesisBlock.Timestamp(),
		[]transactions.TransactionInterface {},
		1715270948)
	forkBlock.SetDifficultyTracker(dt)
	forkBlock.SetNonces(4, 0)
	parentMetadata, err = c.c.GetBlockMetaData(genesisBlock.HeaderHash())
	assert.Nil(t, err)
	measurement, err = c.c.GetMeasurement(uint32(forkBlock.Timestamp()), forkBlock.PrevHeaderHash(), parentMetadata)
	assert.Nil(t, err)
	assert.True(t, forkBlock.Validate(nil, genesisBlock, parentMetadata, measurement, nil))
	assert.True(t, c.c.AddBlock(forkBlock))
	assert.True(t, block1.IsEqual(c.c.lastBlock))

	forkBlockFromState, err := c.c.GetBlock(forkBlock.HeaderHash())
	assert.Nil(t, err)
	assert.True(t, forkBlock.IsEqual(forkBlockFromState))

	// Fork Block 2
	n.SetTimestamp(1815270948)
	forkBlock2 := block.CreateBlock(
		misc.UCharVectorToBytes(bobXMSS.Address()),
		2,
		forkBlock.HeaderHash(),
		forkBlock.Timestamp(),
		[]transactions.TransactionInterface {},
		1815270948)
	forkBlock2.SetDifficultyTracker(dt)
	forkBlock2.SetNonces(1, 0)
	parentMetadata, err = c.c.GetBlockMetaData(forkBlock.HeaderHash())
	assert.Nil(t, err)
	measurement, err = c.c.GetMeasurement(uint32(forkBlock2.Timestamp()), forkBlock2.PrevHeaderHash(), parentMetadata)
	assert.Nil(t, err)
	assert.True(t, forkBlock2.Validate(nil, forkBlock, parentMetadata, measurement, nil))
	assert.True(t, c.c.AddBlock(forkBlock2))
	assert.True(t, forkBlock2.IsEqual(c.c.lastBlock))

	forkBlock2FromState, err := c.c.GetBlock(forkBlock2.HeaderHash())
	assert.Nil(t, err)
	assert.True(t, forkBlock2FromState.IsEqual(forkBlock2))

	// Bob should not be Alice's slave after fork recovery as the slave txn no more belong to mainchain
	aliceState, err = c.c.state.GetAddressState(misc.UCharVectorToBytes(aliceXMSS.Address()))
	assert.Equal(t, len(aliceState.SlavePKSAccessType()), 0)
	assert.NotContains(t, aliceState.SlavePKSAccessType(), misc.UCharVectorToString(bobXMSS.PK()))
}

func TestChain_AddChain2(t *testing.T) {
	c, _ := NewTestChain()
	defer CleanUp(c.c.config.User.QrlDir)

	genesisBlock := c.c.lastBlock

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

	c.c.AddBlock(block1)
	c.c.AddBlock(block2)
	c.c.AddBlock(block3)

	c.c.AddBlock(forkBlock2)
	assert.True(t, block3.IsEqual(c.c.lastBlock))

	c.c.AddBlock(forkBlock3)
	assert.True(t, block3.IsEqual(c.c.lastBlock))

	c.c.AddBlock(forkBlock4)
	assert.True(t, forkBlock4.IsEqual(c.c.lastBlock))
}
