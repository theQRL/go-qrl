package transactions

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/theQRL/go-qrl/pkg/core/addressstate"
	"github.com/theQRL/go-qrl/pkg/misc"
)

type TestCoinBase struct {
	tx *CoinBase
}

func NewTestCoinBase(qaddress string, blockNumber uint64, amount uint64) *TestCoinBase {
	minerAddress := misc.Qaddress2Bin(qaddress)
	tx := CreateCoinBase(minerAddress, blockNumber, amount)

	return &TestCoinBase{tx: tx}
}

func TestCreateCoinBase(t *testing.T) {
	qaddress := "Q010400d9f1efe5b272e042dcc8ef690f0e90ca8b0b6edba0d26f81e7aff12a6754b21788169f7f"
	minerAddress := misc.Qaddress2Bin(qaddress)
	blockNumber := uint64(10)
	amount := uint64(1000000)
	tx := CreateCoinBase(minerAddress, blockNumber, amount)

	assert.NotNil(t, tx)
}

func TestCoinBase_AddrTo(t *testing.T) {
	qaddress := "Q010400d9f1efe5b272e042dcc8ef690f0e90ca8b0b6edba0d26f81e7aff12a6754b21788169f7f"
	blockNumber := uint64(5)
	amount := uint64(10)
	coinbase := NewTestCoinBase(qaddress, blockNumber, amount)

	assert.NotNil(t, coinbase.tx)
	assert.Equal(t, coinbase.tx.AddrTo(), misc.Qaddress2Bin(qaddress))
}

func TestCoinBase_Amount(t *testing.T) {
	qaddress := "Q010400d9f1efe5b272e042dcc8ef690f0e90ca8b0b6edba0d26f81e7aff12a6754b21788169f7f"
	blockNumber := uint64(5)
	amount := uint64(10)
	coinbase := NewTestCoinBase(qaddress, blockNumber, amount)

	assert.NotNil(t, coinbase.tx)
	assert.Equal(t, coinbase.tx.Amount(), amount)
}

func TestCoinBase_GetHashableBytes(t *testing.T) {
	qaddress := "Q010400d9f1efe5b272e042dcc8ef690f0e90ca8b0b6edba0d26f81e7aff12a6754b21788169f7f"
	blockNumber := uint64(5)
	amount := uint64(10)
	coinbase := NewTestCoinBase(qaddress, blockNumber, amount)
	hashableBytes := "a8fbfa16370009b63fd58aaf8c0f5eb01e555709a25f81cde5f7d760c212a31a"

	assert.NotNil(t, coinbase.tx)
	assert.Equal(t, misc.Bin2HStr(coinbase.tx.GetHashableBytes()), hashableBytes)
}

func TestCoinBase_UpdateMiningAddress(t *testing.T) {
	qaddress := "Q010400d9f1efe5b272e042dcc8ef690f0e90ca8b0b6edba0d26f81e7aff12a6754b21788169f7f"
	qaddress2 := "Q010400198233fb46d751f798f42630fa5b582a7859ecb70bd918d6df104f63c8ece4924b48335c"
	blockNumber := uint64(5)
	amount := uint64(10)
	coinbase := NewTestCoinBase(qaddress, blockNumber, amount)
	txHash := "a8fbfa16370009b63fd58aaf8c0f5eb01e555709a25f81cde5f7d760c212a31a"
	txHash2 := "8187c8755a9580bedfe7782bdc8f4ce36ee2354b11da5cab26d48c094cadfe48"

	assert.NotNil(t, coinbase.tx)
	assert.Equal(t, coinbase.tx.AddrTo(), misc.Qaddress2Bin(qaddress))
	assert.Equal(t, misc.Bin2HStr(coinbase.tx.Txhash()), txHash)

	coinbase.tx.UpdateMiningAddress(misc.Qaddress2Bin(qaddress2))
	assert.Equal(t, coinbase.tx.AddrTo(), misc.Qaddress2Bin(qaddress2))
	assert.Equal(t, misc.Bin2HStr(coinbase.tx.Txhash()), txHash2)
}

func TestCoinBase_Validate(t *testing.T) {
	qaddress := "Q010400d9f1efe5b272e042dcc8ef690f0e90ca8b0b6edba0d26f81e7aff12a6754b21788169f7f"
	blockNumber := uint64(5)
	amount := uint64(10)
	coinbase := NewTestCoinBase(qaddress, blockNumber, amount)

	assert.NotNil(t, coinbase.tx)
	assert.True(t, coinbase.tx.validateCustom())

	// Changed coinbase address to some different address, validation must fail
	coinbase.tx.PBData().MasterAddr = misc.Qaddress2Bin(qaddress)
	assert.False(t, coinbase.tx.Validate(false))
}

func TestCoinBase_Validate2(t *testing.T) {
	qaddress := "Q010400d9f1efe5b272e042dcc8ef690f0e90ca8b0b6edba0d26f81e7aff12a6754b21788169f7f"
	blockNumber := uint64(5)
	amount := uint64(10)
	coinbase := NewTestCoinBase(qaddress, blockNumber, amount)

	assert.NotNil(t, coinbase.tx)
	assert.True(t, coinbase.tx.validateCustom())

	// Random Transaction Hash, validate must fail
	coinbase.tx.PBData().TransactionHash = []byte{0, 1, 5}
	assert.False(t, coinbase.tx.Validate(false))
}

func TestCoinBase_ValidateCustom(t *testing.T) {
	qaddress := "Q010400d9f1efe5b272e042dcc8ef690f0e90ca8b0b6edba0d26f81e7aff12a6754b21788169f7f"
	blockNumber := uint64(5)
	amount := uint64(10)
	coinbase := NewTestCoinBase(qaddress, blockNumber, amount)

	assert.NotNil(t, coinbase.tx)
	assert.True(t, coinbase.tx.validateCustom())

	// Changed coinbase address to some different address, validation must fail
	coinbase.tx.PBData().MasterAddr = misc.Qaddress2Bin(qaddress)
	assert.False(t, coinbase.tx.validateCustom())
}

func TestCoinBase_ValidateCustom2(t *testing.T) {
	qaddress := "Q010400d9f1efe5b272e042dcc8ef690f0e90ca8b0b6edba0d26f81e7aff12a6754b21788169f7f"
	blockNumber := uint64(5)
	amount := uint64(10)
	coinbase := NewTestCoinBase(qaddress, blockNumber, amount)

	assert.NotNil(t, coinbase.tx)
	assert.True(t, coinbase.tx.validateCustom())

	// Added fee on coinbase address, validation must fail
	coinbase.tx.PBData().Fee = 10
	assert.False(t, coinbase.tx.validateCustom())
}

func TestCoinBase_ValidateExtendedCoinbase(t *testing.T) {
	qaddress := "Q010400d9f1efe5b272e042dcc8ef690f0e90ca8b0b6edba0d26f81e7aff12a6754b21788169f7f"
	blockNumber := uint64(5)
	amount := uint64(10)
	coinbase := NewTestCoinBase(qaddress, blockNumber, amount)

	assert.NotNil(t, coinbase.tx)
	assert.True(t, coinbase.tx.ValidateExtendedCoinbase(blockNumber))

	// Changed coinbase address to some different address, validation must fail
	coinbase.tx.PBData().MasterAddr = misc.Qaddress2Bin(qaddress)
	assert.False(t, coinbase.tx.ValidateExtendedCoinbase(blockNumber))
}

func TestCoinBase_ValidateExtendedCoinbase2(t *testing.T) {
	qaddress := "Q010400d9f1efe5b272e042dcc8ef690f0e90ca8b0b6edba0d26f81e7aff12a6754b21788169f7f"
	blockNumber := uint64(5)
	amount := uint64(10)
	coinbase := NewTestCoinBase(qaddress, blockNumber, amount)

	assert.NotNil(t, coinbase.tx)
	assert.True(t, coinbase.tx.ValidateExtendedCoinbase(blockNumber))

	// Changed addr_to to nil, validation must fail
	coinbase.tx.PBData().GetCoinbase().AddrTo = nil
	assert.False(t, coinbase.tx.ValidateExtendedCoinbase(blockNumber))

	// Changed addr_to to invalid QRL address, validation must fail
	coinbase.tx.PBData().GetCoinbase().AddrTo = misc.Qaddress2Bin(qaddress[:len(qaddress)-2])
	assert.False(t, coinbase.tx.ValidateExtendedCoinbase(blockNumber))
}

func TestCoinBase_ValidateExtendedCoinbase3(t *testing.T) {
	qaddress := "Q010400d9f1efe5b272e042dcc8ef690f0e90ca8b0b6edba0d26f81e7aff12a6754b21788169f7f"
	blockNumber := uint64(5)
	amount := uint64(10)
	coinbase := NewTestCoinBase(qaddress, blockNumber, amount)

	assert.NotNil(t, coinbase.tx)
	assert.True(t, coinbase.tx.ValidateExtendedCoinbase(blockNumber))

	// Changed nonce to invalid value, validation must fail
	coinbase.tx.PBData().Nonce = blockNumber
	assert.False(t, coinbase.tx.ValidateExtendedCoinbase(blockNumber))
}

func TestCoinBase_ApplyStateChanges(t *testing.T) {
	qaddress := "Q010400d9f1efe5b272e042dcc8ef690f0e90ca8b0b6edba0d26f81e7aff12a6754b21788169f7f"
	blockNumber := uint64(5)
	amount := uint64(10)
	coinbase := NewTestCoinBase(qaddress, blockNumber, amount)

	assert.NotNil(t, coinbase.tx)

	addressesState := make(map[string]*addressstate.AddressState)
	coinbase.tx.SetAffectedAddress(addressesState)

	assert.Len(t, addressesState, 2)

	for qaddress := range addressesState {
		addressesState[qaddress] = addressstate.GetDefaultAddressState(misc.Qaddress2Bin(qaddress))
	}

	coinbaseQaddress := misc.Bin2Qaddress(coinbase.tx.MasterAddr())
	coinbaseBalance := uint64(1000000)
	// Initializing balance for coinbase Address
	addressesState[coinbaseQaddress].PBData().Balance = coinbaseBalance

	coinbase.tx.ApplyStateChanges(addressesState)

	assert.Equal(t, addressesState[qaddress].Balance(), amount)
	assert.Equal(t, addressesState[coinbaseQaddress].Balance(), coinbaseBalance-amount)
}

func TestCoinBase_RevertStateChanges(t *testing.T) {
	qaddress := "Q010400d9f1efe5b272e042dcc8ef690f0e90ca8b0b6edba0d26f81e7aff12a6754b21788169f7f"
	blockNumber := uint64(5)
	amount := uint64(10)
	coinbase := NewTestCoinBase(qaddress, blockNumber, amount)

	assert.NotNil(t, coinbase.tx)

	addressesState := make(map[string]*addressstate.AddressState)
	coinbase.tx.SetAffectedAddress(addressesState)

	assert.Len(t, addressesState, 2)

	for qaddress := range addressesState {
		addressesState[qaddress] = addressstate.GetDefaultAddressState(misc.Qaddress2Bin(qaddress))
	}

	coinbaseQaddress := misc.Bin2Qaddress(coinbase.tx.MasterAddr())
	coinbaseBalance := uint64(1000000)
	// Initializing balance for coinbase Address
	addressesState[coinbaseQaddress].PBData().Balance = coinbaseBalance

	coinbase.tx.ApplyStateChanges(addressesState)

	assert.Equal(t, addressesState[qaddress].Balance(), amount)
	assert.Equal(t, addressesState[coinbaseQaddress].Balance(), coinbaseBalance-amount)

	coinbase.tx.RevertStateChanges(addressesState)

	assert.Equal(t, addressesState[qaddress].Balance(), uint64(0))
	assert.Equal(t, addressesState[coinbaseQaddress].Balance(), coinbaseBalance)
}

func TestCoinBase_SetAffectedAddress(t *testing.T) {
	qaddress := "Q010400d9f1efe5b272e042dcc8ef690f0e90ca8b0b6edba0d26f81e7aff12a6754b21788169f7f"
	blockNumber := uint64(5)
	amount := uint64(10)
	coinbase := NewTestCoinBase(qaddress, blockNumber, amount)

	assert.NotNil(t, coinbase.tx)

	addressesState := make(map[string]*addressstate.AddressState)
	coinbase.tx.SetAffectedAddress(addressesState)

	assert.Len(t, addressesState, 2)
	coinbaseQaddress := misc.Bin2Qaddress(coinbase.tx.MasterAddr())

	assert.Contains(t, addressesState, coinbaseQaddress)
	assert.Contains(t, addressesState, qaddress)
}

func TestCoinBase_FromPBData(t *testing.T) {
	qaddress := "Q010400d9f1efe5b272e042dcc8ef690f0e90ca8b0b6edba0d26f81e7aff12a6754b21788169f7f"
	blockNumber := uint64(5)
	amount := uint64(10)
	coinbase := NewTestCoinBase(qaddress, blockNumber, amount)

	assert.NotNil(t, coinbase.tx)

	pbdata := coinbase.tx.PBData()
	tx2 := CoinBase{}
	tx2.FromPBdata(*pbdata)

	assert.Equal(t, pbdata, tx2.PBData())

	// Test to ensure, FromPBData doesnt use reference object to initialize tx.data
	coinbase.tx.PBData().Fee = 10
	assert.Equal(t, coinbase.tx.Fee(), uint64(10))
	assert.NotEqual(t, tx2.Fee(), coinbase.tx.Fee())

	// A random CoinBase Txn
	tx3 := CoinBase{}
	assert.NotEqual(t, pbdata, tx3.PBData())
}
