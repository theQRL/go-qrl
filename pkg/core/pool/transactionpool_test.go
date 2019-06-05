package pool

import (
	"github.com/stretchr/testify/assert"
	"github.com/theQRL/go-qrl/pkg/config"
	"github.com/theQRL/go-qrl/pkg/core/block"
	"github.com/theQRL/go-qrl/pkg/core/state"
	"github.com/theQRL/go-qrl/pkg/core/transactions"
	"github.com/theQRL/go-qrl/pkg/crypto"
	"github.com/theQRL/go-qrl/pkg/misc"
	"github.com/theQRL/go-qrl/pkg/ntp"
	"github.com/theQRL/go-qrl/test/helper"
	"github.com/theQRL/qrllib/goqrllib/goqrllib"
	"io/ioutil"
	"os"
	"testing"
)

type TestTransactionPool struct {
	t *TransactionPool
	ntp ntp.NTPInterface
}

func CreateTestTransactionPool() *TestTransactionPool {
	return &TestTransactionPool{
		t: CreateTransactionPool(),
		ntp: ntp.GetNTP(),
	}
}

func TestTransactionPool_Add(t *testing.T) {
	testTxPool := CreateTestTransactionPool()
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)

	addrsTo := []string{bobXMSS.QAddress(), aliceXMSS.QAddress()}
	bytesAddrsTo := misc.StringAddressToBytesArray(addrsTo)
	amounts := []uint64{100, 200}
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(randomXMSS.PK())

	tx := transactions.CreateTransferTransaction(bytesAddrsTo, amounts, fee, xmssPK, nil)

	err := testTxPool.t.Add(tx, 0, testTxPool.ntp.Time())
	assert.Nil(t, err)
}

func TestTransactionPool_AddTxFromBlock(t *testing.T) {
	testTxPool := CreateTestTransactionPool()
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)

	addrsTo := []string{bobXMSS.QAddress(), aliceXMSS.QAddress()}
	bytesAddrsTo := misc.StringAddressToBytesArray(addrsTo)
	amounts := []uint64{100, 200}
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(randomXMSS.PK())

	tx := transactions.CreateTransferTransaction(bytesAddrsTo, amounts, fee, xmssPK, nil)
	tx.Sign(randomXMSS, misc.BytesToUCharVector(tx.GetHashableBytes()))
	b := block.CreateBlock(
		misc.Qaddress2Bin(aliceXMSS.QAddress()),
		1,
		nil,
		testTxPool.ntp.Time()-1,
		[]transactions.TransactionInterface{tx},
		testTxPool.ntp.Time())

	err := testTxPool.t.AddTxFromBlock(b, b.BlockNumber())
	assert.Nil(t, err)
	assert.True(t, testTxPool.t.Contains(tx))
	testTxPool.t.RemoveTxInBlock(b)
	assert.False(t, testTxPool.t.Contains(tx))
}

func TestTransactionPool_CheckStale(t *testing.T) {
	testTxPool := CreateTestTransactionPool()
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)

	addrsTo := []string{bobXMSS.QAddress(), aliceXMSS.QAddress()}
	bytesAddrsTo := misc.StringAddressToBytesArray(addrsTo)
	amounts := []uint64{100, 200}
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(randomXMSS.PK())

	tx := transactions.CreateTransferTransaction(bytesAddrsTo, amounts, fee, xmssPK, nil)
	tx.Sign(randomXMSS, misc.BytesToUCharVector(tx.GetHashableBytes()))
	err := testTxPool.t.Add(tx, 0, testTxPool.ntp.Time())
	assert.Nil(t, err)

	tempDir, err := ioutil.TempDir("", "")
	defer os.RemoveAll(tempDir)

	conf := config.GetConfig()
	conf.Dev.Genesis.GenesisTimestamp = 1528402558
	conf.Dev.Genesis.GenesisPrevHeadehash = []byte("Thirst of Quantas")
	conf.Dev.Genesis.GenesisDifficulty = 5000
	conf.User.SetDataDir(tempDir)

	// TODO: Replace state with some temporary directory
	s, err := state.CreateState()
	assert.Nil(t, err)
	err = testTxPool.t.CheckStale(conf.User.TransactionPool.StaleTransactionThreshold + 1, s)
	assert.Nil(t, err)
	assert.False(t, testTxPool.t.Contains(tx))

}

func TestTransactionPool_Contains(t *testing.T) {
	testTxPool := CreateTestTransactionPool()
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)

	addrsTo := []string{bobXMSS.QAddress(), aliceXMSS.QAddress()}
	bytesAddrsTo := misc.StringAddressToBytesArray(addrsTo)
	amounts := []uint64{100, 200}
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(randomXMSS.PK())

	tx := transactions.CreateTransferTransaction(bytesAddrsTo, amounts, fee, xmssPK, nil)

	assert.False(t, testTxPool.t.Contains(tx))

	err := testTxPool.t.Add(tx, 0, testTxPool.ntp.Time())
	assert.Nil(t, err)
	assert.True(t, testTxPool.t.Contains(tx))

}

func TestTransactionPool_Pop(t *testing.T) {
	testTxPool := CreateTestTransactionPool()
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)

	addrsTo := []string{bobXMSS.QAddress(), aliceXMSS.QAddress()}
	bytesAddrsTo := misc.StringAddressToBytesArray(addrsTo)
	amounts := []uint64{100, 200}
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(randomXMSS.PK())

	tx := transactions.CreateTransferTransaction(bytesAddrsTo, amounts, fee, xmssPK, nil)

	err := testTxPool.t.Add(tx, 0, testTxPool.ntp.Time())
	assert.Nil(t, err)
	assert.True(t, testTxPool.t.Contains(tx))

	txInfo := testTxPool.t.Pop()
	assert.Equal(t, tx.PBData(), txInfo.tx.PBData())
	assert.False(t, testTxPool.t.Contains(tx))
	txInfo = testTxPool.t.Pop()
	assert.Nil(t, txInfo)
}

func TestTransactionPool_Remove(t *testing.T) {
	testTxPool := CreateTestTransactionPool()
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)

	addrsTo := []string{bobXMSS.QAddress(), aliceXMSS.QAddress()}
	bytesAddrsTo := misc.StringAddressToBytesArray(addrsTo)
	amounts := []uint64{100, 200}
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(randomXMSS.PK())

	tx := transactions.CreateTransferTransaction(bytesAddrsTo, amounts, fee, xmssPK, nil)

	err := testTxPool.t.Add(tx, 0, testTxPool.ntp.Time())
	assert.Nil(t, err)
	assert.True(t, testTxPool.t.Contains(tx))

	assert.True(t, testTxPool.t.Remove(tx))
	assert.False(t, testTxPool.t.Contains(tx))
	assert.False(t, testTxPool.t.Remove(tx))
}

func TestTransactionPool_RemoveTxInBlock(t *testing.T) {
	testTxPool := CreateTestTransactionPool()
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)

	addrsTo := []string{bobXMSS.QAddress(), aliceXMSS.QAddress()}
	bytesAddrsTo := misc.StringAddressToBytesArray(addrsTo)
	amounts := []uint64{100, 200}
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(randomXMSS.PK())

	tx := transactions.CreateTransferTransaction(bytesAddrsTo, amounts, fee, xmssPK, nil)
	tx.Sign(randomXMSS, misc.BytesToUCharVector(tx.GetHashableBytes()))
	b := block.CreateBlock(
		misc.Qaddress2Bin(aliceXMSS.QAddress()),
		1,
		nil,
		testTxPool.ntp.Time()-1,
		[]transactions.TransactionInterface{tx},
		testTxPool.ntp.Time())

	err := testTxPool.t.Add(tx, 0, testTxPool.ntp.Time())
	assert.Nil(t, err)
	assert.True(t, testTxPool.t.Contains(tx))
	testTxPool.t.RemoveTxInBlock(b)
	assert.False(t, testTxPool.t.Contains(tx))
}
