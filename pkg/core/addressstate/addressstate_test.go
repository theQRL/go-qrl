package addressstate

import (
	"github.com/stretchr/testify/assert"
	"github.com/theQRL/go-qrl/pkg/config"
	"github.com/theQRL/go-qrl/pkg/misc"
	"github.com/theQRL/go-qrl/test/helper"
	"math"
	"math/rand"
	"testing"
)

type TestAddressState struct {
	a *AddressState
}

func NewTestAddressState(qAddress string, nonce uint64, balance uint64, otsBitfield [][]byte, tokens map[string]uint64, slavePksAccessType map[string]uint32, otsCounter uint64) *TestAddressState {
	a := CreateAddressState(misc.Qaddress2Bin(qAddress), nonce, balance, otsBitfield, tokens, slavePksAccessType, otsCounter)
	return &TestAddressState{a:a}
}

func TestCreateAddressState(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	nonce := uint64(0)
	balance := uint64(100)
	otsBitfield := make([][]byte, config.GetConfig().Dev.OtsBitFieldSize)

	var tokens map[string]uint64
	var slavePksAccessType map[string]uint32
	addressState := CreateAddressState(misc.UCharVectorToBytes(aliceXMSS.Address()), nonce, balance, otsBitfield, tokens, slavePksAccessType, 0)

	assert.NotNil(t, addressState)
	assert.Equal(t, addressState.Nonce(), nonce)
	assert.Equal(t, addressState.Balance(), balance)
	assert.Equal(t, addressState.OtsBitfield(), otsBitfield)
}

func TestAddressState_Address(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	nonce := uint64(0)
	balance := uint64(100)
	otsBitfield := make([][]byte, config.GetConfig().Dev.OtsBitFieldSize)

	var tokens map[string]uint64
	var slavePksAccessType map[string]uint32
	a := NewTestAddressState(aliceXMSS.QAddress(), nonce, balance, otsBitfield, tokens, slavePksAccessType, 0)

	assert.NotNil(t, a)
	assert.Equal(t, a.a.Address(), misc.UCharVectorToBytes(aliceXMSS.Address()))
}

func TestAddressState_Nonce(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	nonce := uint64(0)
	balance := uint64(100)
	otsBitfield := make([][]byte, config.GetConfig().Dev.OtsBitFieldSize)

	var tokens map[string]uint64
	var slavePksAccessType map[string]uint32
	a := NewTestAddressState(aliceXMSS.QAddress(), nonce, balance, otsBitfield, tokens, slavePksAccessType, 0)

	assert.NotNil(t, a)
	assert.Equal(t, a.a.Nonce(), nonce)
}

func TestAddressState_Balance(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	nonce := uint64(0)
	balance := uint64(100)
	otsBitfield := make([][]byte, config.GetConfig().Dev.OtsBitFieldSize)

	var tokens map[string]uint64
	var slavePksAccessType map[string]uint32
	a := NewTestAddressState(aliceXMSS.QAddress(), nonce, balance, otsBitfield, tokens, slavePksAccessType, 0)

	assert.NotNil(t, a)
	assert.Equal(t, a.a.Balance(), balance)
}

func TestAddressState_SetBalance(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	nonce := uint64(0)
	balance := uint64(100)
	otsBitfield := make([][]byte, config.GetConfig().Dev.OtsBitFieldSize)

	var tokens map[string]uint64
	var slavePksAccessType map[string]uint32
	a := NewTestAddressState(aliceXMSS.QAddress(), nonce, balance, otsBitfield, tokens, slavePksAccessType, 0)

	assert.NotNil(t, a)
	assert.Equal(t, a.a.Balance(), balance)

	newBalance := uint64(10000)
	a.a.SetBalance(newBalance)
	assert.Equal(t, a.a.Balance(), newBalance)
}

func TestAddressState_AddBalance(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	nonce := uint64(0)
	balance := uint64(100)
	otsBitfield := make([][]byte, config.GetConfig().Dev.OtsBitFieldSize)

	var tokens map[string]uint64
	var slavePksAccessType map[string]uint32
	a := NewTestAddressState(aliceXMSS.QAddress(), nonce, balance, otsBitfield, tokens, slavePksAccessType, 0)

	assert.NotNil(t, a)
	assert.Equal(t, a.a.Balance(), balance)

	balance2 := uint64(10000)
	a.a.AddBalance(balance2)
	assert.Equal(t, a.a.Balance(), balance + balance2)
}

func TestAddressState_SubtractBalance(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	nonce := uint64(0)
	balance := uint64(100)
	otsBitfield := make([][]byte, config.GetConfig().Dev.OtsBitFieldSize)

	var tokens map[string]uint64
	var slavePksAccessType map[string]uint32
	a := NewTestAddressState(aliceXMSS.QAddress(), nonce, balance, otsBitfield, tokens, slavePksAccessType, 0)

	assert.NotNil(t, a)
	assert.Equal(t, a.a.Balance(), balance)

	balance2 := uint64(10)
	a.a.SubtractBalance(balance2)
	assert.Equal(t, a.a.Balance(), balance - balance2)
}

func TestAddressState_OtsBitfield(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	nonce := uint64(0)
	balance := uint64(100)
	otsBitfield := make([][]byte, config.GetConfig().Dev.OtsBitFieldSize)
	otsBitfield[2] = []byte{1}

	var tokens map[string]uint64
	var slavePksAccessType map[string]uint32
	a := NewTestAddressState(aliceXMSS.QAddress(), nonce, balance, otsBitfield, tokens, slavePksAccessType, 0)

	assert.NotNil(t, a)
	assert.Equal(t, a.a.OtsBitfield(), otsBitfield)
	assert.Equal(t, a.a.OtsBitfield()[2], otsBitfield[2])
}

func TestAddressState_AppendTransactionHash(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	nonce := uint64(0)
	balance := uint64(100)
	otsBitfield := make([][]byte, config.GetConfig().Dev.OtsBitFieldSize)

	var tokens map[string]uint64
	var slavePksAccessType map[string]uint32
	a := NewTestAddressState(aliceXMSS.QAddress(), nonce, balance, otsBitfield, tokens, slavePksAccessType, 0)

	assert.NotNil(t, a)

	txHash := misc.HStr2Bin("aa")
	a.a.AppendTransactionHash(txHash)
	assert.Contains(t, a.a.TransactionHashes(), txHash)
}

func TestAddressState_TransactionHashes(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	nonce := uint64(0)
	balance := uint64(100)
	otsBitfield := make([][]byte, config.GetConfig().Dev.OtsBitFieldSize)

	var tokens map[string]uint64
	var slavePksAccessType map[string]uint32
	a := NewTestAddressState(aliceXMSS.QAddress(), nonce, balance, otsBitfield, tokens, slavePksAccessType, 0)

	assert.NotNil(t, a)

	txHash := misc.HStr2Bin("aa")
	assert.NotContains(t, a.a.TransactionHashes(), txHash)
	assert.Len(t, a.a.TransactionHashes(), 0)
	a.a.AppendTransactionHash(txHash)
	assert.Contains(t, a.a.TransactionHashes(), txHash)
	assert.Len(t, a.a.TransactionHashes(), 1)
}

func TestAddressState_RemoveTransactionHash(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	nonce := uint64(0)
	balance := uint64(100)
	otsBitfield := make([][]byte, config.GetConfig().Dev.OtsBitFieldSize)

	var tokens map[string]uint64
	var slavePksAccessType map[string]uint32

	a := NewTestAddressState(aliceXMSS.QAddress(), nonce, balance, otsBitfield, tokens, slavePksAccessType, 0)
	assert.NotNil(t, a)

	txHash := misc.HStr2Bin("aa")
	txHash2 := misc.HStr2Bin("bb")
	assert.NotContains(t, a.a.TransactionHashes(), txHash)
	assert.Len(t, a.a.TransactionHashes(), 0)

	a.a.AppendTransactionHash(txHash)
	assert.Contains(t, a.a.TransactionHashes(), txHash)
	assert.Len(t, a.a.TransactionHashes(), 1)

	a.a.AppendTransactionHash(txHash2)
	assert.Contains(t, a.a.TransactionHashes(), txHash)
	assert.Contains(t, a.a.TransactionHashes(), txHash2)
	assert.Len(t, a.a.TransactionHashes(), 2)

	a.a.RemoveTransactionHash(txHash)
	assert.NotContains(t, a.a.TransactionHashes(), txHash)
	assert.Len(t, a.a.TransactionHashes(), 1)
	assert.Contains(t, a.a.TransactionHashes(), txHash2)
}

func TestAddressState_LatticePKList(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	nonce := uint64(0)
	balance := uint64(100)
	otsBitfield := make([][]byte, config.GetConfig().Dev.OtsBitFieldSize)

	var tokens map[string]uint64
	var slavePksAccessType map[string]uint32

	a := NewTestAddressState(aliceXMSS.QAddress(), nonce, balance, otsBitfield, tokens, slavePksAccessType, 0)
	assert.NotNil(t, a)
	assert.Nil(t, a.a.LatticePKList())
}

func TestAddressState_SlavePKSAccessType(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	nonce := uint64(0)
	balance := uint64(100)
	otsBitfield := make([][]byte, config.GetConfig().Dev.OtsBitFieldSize)

	var tokens map[string]uint64
	var slavePksAccessType map[string]uint32

	a := NewTestAddressState(aliceXMSS.QAddress(), nonce, balance, otsBitfield, tokens, slavePksAccessType, 0)
	assert.NotNil(t, a)
	assert.NotNil(t, a.a.SlavePKSAccessType())
	assert.Len(t, a.a.SlavePKSAccessType(), 0)

	bobXMSS := helper.GetBobXMSS(6)
	slavePK := misc.UCharVectorToBytes(bobXMSS.PK())
	a.a.AddSlavePKSAccessType(slavePK, 0)

	assert.NotNil(t, a.a.SlavePKSAccessType())
	assert.Len(t, a.a.SlavePKSAccessType(), 1)
	assert.Contains(t, a.a.SlavePKSAccessType(), string(slavePK))
}

func TestAddressState_GetSlavePermission(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	nonce := uint64(0)
	balance := uint64(100)
	otsBitfield := make([][]byte, config.GetConfig().Dev.OtsBitFieldSize)

	var tokens map[string]uint64
	var slavePksAccessType map[string]uint32

	a := NewTestAddressState(aliceXMSS.QAddress(), nonce, balance, otsBitfield, tokens, slavePksAccessType, 0)
	assert.NotNil(t, a)
	assert.NotNil(t, a.a.SlavePKSAccessType())
	assert.Len(t, a.a.SlavePKSAccessType(), 0)

	bobXMSS := helper.GetBobXMSS(6)
	slavePK := misc.UCharVectorToBytes(bobXMSS.PK())

	permission, ok := a.a.GetSlavePermission(slavePK)
	assert.False(t, ok)
	assert.Zero(t, permission)

	a.a.AddSlavePKSAccessType(slavePK, 0)

	assert.NotNil(t, a.a.SlavePKSAccessType())
	permission, ok = a.a.GetSlavePermission(slavePK)
	assert.True(t, ok)
	assert.Zero(t, permission)
}

func TestAddressState_RemoveSlavePKSAccessType(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	nonce := uint64(0)
	balance := uint64(100)
	otsBitfield := make([][]byte, config.GetConfig().Dev.OtsBitFieldSize)

	var tokens map[string]uint64
	var slavePksAccessType map[string]uint32

	a := NewTestAddressState(aliceXMSS.QAddress(), nonce, balance, otsBitfield, tokens, slavePksAccessType, 0)
	assert.NotNil(t, a)
	assert.NotNil(t, a.a.SlavePKSAccessType())
	assert.Len(t, a.a.SlavePKSAccessType(), 0)

	bobXMSS := helper.GetBobXMSS(6)
	slavePK := misc.UCharVectorToBytes(bobXMSS.PK())
	a.a.AddSlavePKSAccessType(slavePK, 0)

	assert.NotNil(t, a.a.SlavePKSAccessType())
	assert.Len(t, a.a.SlavePKSAccessType(), 1)
	assert.Contains(t, a.a.SlavePKSAccessType(), string(slavePK))

	a.a.RemoveSlavePKSAccessType(slavePK)
	assert.Len(t, a.a.SlavePKSAccessType(), 0)
	assert.NotContains(t, a.a.SlavePKSAccessType(), string(slavePK))
}

func TestAddressState_UpdateTokenBalance(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	nonce := uint64(0)
	balance := uint64(100)
	otsBitfield := make([][]byte, config.GetConfig().Dev.OtsBitFieldSize)
	tokenTxHash := misc.HStr2Bin("a0a0")

	var tokens map[string]uint64
	var slavePksAccessType map[string]uint32

	a := NewTestAddressState(aliceXMSS.QAddress(), nonce, balance, otsBitfield, tokens, slavePksAccessType, 0)
	assert.NotNil(t, a)

	assert.Zero(t, a.a.GetTokenBalance(tokenTxHash))
	a.a.UpdateTokenBalance(tokenTxHash, uint64(100), false)
	assert.Equal(t, a.a.GetTokenBalance(tokenTxHash), uint64(100))
	a.a.UpdateTokenBalance(tokenTxHash, uint64(204), false)
	assert.Equal(t, a.a.GetTokenBalance(tokenTxHash), uint64(304))
	a.a.UpdateTokenBalance(tokenTxHash, uint64(20), true)
	assert.Equal(t, a.a.GetTokenBalance(tokenTxHash), uint64(284))
}

func TestAddressState_GetTokenBalance(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	nonce := uint64(0)
	balance := uint64(100)
	otsBitfield := make([][]byte, config.GetConfig().Dev.OtsBitFieldSize)
	tokenTxHash := misc.HStr2Bin("a0a0")

	var tokens map[string]uint64
	var slavePksAccessType map[string]uint32

	a := NewTestAddressState(aliceXMSS.QAddress(), nonce, balance, otsBitfield, tokens, slavePksAccessType, 0)
	assert.NotNil(t, a)

	assert.Zero(t, a.a.GetTokenBalance(tokenTxHash))
	a.a.UpdateTokenBalance(tokenTxHash, uint64(100), false)
	assert.Equal(t, a.a.GetTokenBalance(tokenTxHash), uint64(100))
}

func TestAddressState_IsTokenExists(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	nonce := uint64(0)
	balance := uint64(100)
	otsBitfield := make([][]byte, config.GetConfig().Dev.OtsBitFieldSize)
	tokenTxHash := misc.HStr2Bin("a0a0")

	var tokens map[string]uint64
	var slavePksAccessType map[string]uint32

	a := NewTestAddressState(aliceXMSS.QAddress(), nonce, balance, otsBitfield, tokens, slavePksAccessType, 0)
	assert.NotNil(t, a)

	assert.False(t, a.a.IsTokenExists(tokenTxHash))
	a.a.UpdateTokenBalance(tokenTxHash, uint64(100), false)
	assert.True(t, a.a.IsTokenExists(tokenTxHash))
	a.a.UpdateTokenBalance(tokenTxHash, uint64(100), true)
	assert.False(t, a.a.IsTokenExists(tokenTxHash))
}

func TestAddressState_IncreaseNonce(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	nonce := uint64(0)
	balance := uint64(100)
	otsBitfield := make([][]byte, config.GetConfig().Dev.OtsBitFieldSize)

	var tokens map[string]uint64
	var slavePksAccessType map[string]uint32

	a := NewTestAddressState(aliceXMSS.QAddress(), nonce, balance, otsBitfield, tokens, slavePksAccessType, 0)
	assert.NotNil(t, a)
	assert.Zero(t, a.a.Nonce())

	a.a.IncreaseNonce()
	assert.Equal(t, a.a.Nonce(), uint64(1))

	a.a.IncreaseNonce()
	assert.Equal(t, a.a.Nonce(), uint64(2))

	a.a.IncreaseNonce()
	assert.Equal(t, a.a.Nonce(), uint64(3))
}

func TestAddressState_DecreaseNonce(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	nonce := uint64(0)
	balance := uint64(100)
	otsBitfield := make([][]byte, config.GetConfig().Dev.OtsBitFieldSize)

	var tokens map[string]uint64
	var slavePksAccessType map[string]uint32

	a := NewTestAddressState(aliceXMSS.QAddress(), nonce, balance, otsBitfield, tokens, slavePksAccessType, 0)
	assert.NotNil(t, a)
	assert.Zero(t, a.a.Nonce())

	a.a.IncreaseNonce()
	assert.Equal(t, a.a.Nonce(), uint64(1))

	a.a.IncreaseNonce()
	assert.Equal(t, a.a.Nonce(), uint64(2))

	a.a.IncreaseNonce()
	assert.Equal(t, a.a.Nonce(), uint64(3))

	a.a.DecreaseNonce()
	assert.Equal(t, a.a.Nonce(), uint64(2))

	a.a.DecreaseNonce()
	assert.Equal(t, a.a.Nonce(), uint64(1))

	a.a.DecreaseNonce()
	assert.Zero(t, a.a.Nonce())
}

func TestAddressState_OTSKeyReuse(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	nonce := uint64(0)
	balance := uint64(100)
	otsBitfield := make([][]byte, config.GetConfig().Dev.OtsBitFieldSize)

	var tokens map[string]uint64
	var slavePksAccessType map[string]uint32

	a := NewTestAddressState(aliceXMSS.QAddress(), nonce, balance, otsBitfield, tokens, slavePksAccessType, 0)
	assert.NotNil(t, a)

	otsIndexes := make([]int, int(math.Pow(2, float64(aliceXMSS.Height()))))
	otsIndexes = rand.Perm(len(otsIndexes))

	for _, value := range otsIndexes {
		otsIndex := uint64(value)
		if otsIndex < config.GetConfig().Dev.MaxOTSTracking {
			assert.False(t, a.a.OTSKeyReuse(otsIndex))
		} else {
			result := a.a.OTSKeyReuse(otsIndex)
			if otsIndex > a.a.OtsCounter() {
				assert.False(t, result)
			} else {
				assert.True(t, result)
			}
		}
		a.a.SetOTSKey(otsIndex)
		assert.True(t, a.a.OTSKeyReuse(otsIndex))
	}
}

func TestAddressState_SetOTSKey(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	nonce := uint64(0)
	balance := uint64(100)
	otsBitfield := make([][]byte, config.GetConfig().Dev.OtsBitFieldSize)

	var tokens map[string]uint64
	var slavePksAccessType map[string]uint32

	a := NewTestAddressState(aliceXMSS.QAddress(), nonce, balance, otsBitfield, tokens, slavePksAccessType, 0)
	assert.NotNil(t, a)

	assert.False(t, a.a.OTSKeyReuse(10))
	a.a.SetOTSKey(10)
	assert.True(t, a.a.OTSKeyReuse(10))

	assert.False(t, a.a.OTSKeyReuse(100))
	a.a.SetOTSKey(100)
	assert.True(t, a.a.OTSKeyReuse(100))
}

func TestAddressState_Serialize(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	nonce := uint64(0)
	balance := uint64(100)
	otsBitfield := make([][]byte, config.GetConfig().Dev.OtsBitFieldSize)

	var tokens map[string]uint64
	var slavePksAccessType map[string]uint32

	a := NewTestAddressState(aliceXMSS.QAddress(), nonce, balance, otsBitfield, tokens, slavePksAccessType, 0)
	assert.NotNil(t, a)
	data, err := a.a.Serialize()
	assert.Nil(t, err)
	assert.NotNil(t, data)
}

func TestDeSerializeAddressState(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	nonce := uint64(0)
	balance := uint64(100)
	otsBitfield := make([][]byte, config.GetConfig().Dev.OtsBitFieldSize)

	var tokens map[string]uint64
	var slavePksAccessType map[string]uint32

	a := NewTestAddressState(aliceXMSS.QAddress(), nonce, balance, otsBitfield, tokens, slavePksAccessType, 0)
	assert.NotNil(t, a)
	data, err := a.a.Serialize()
	assert.Nil(t, err)
	assert.NotNil(t, data)

	b, err := DeSerializeAddressState(data)
	assert.Nil(t, err)
	data2, err := b.Serialize()
	assert.Nil(t, err)
	assert.Equal(t, data, data2)
}

func TestGetDefaultAddressState(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	addressState := GetDefaultAddressState(misc.UCharVectorToBytes(aliceXMSS.Address()))

	assert.NotNil(t, addressState)
	assert.Equal(t, addressState.Balance(), config.GetConfig().Dev.DefaultAccountBalance)
	assert.Equal(t, addressState.Nonce(), uint64(config.GetConfig().Dev.DefaultNonce))
	assert.Equal(t, addressState.Address(), misc.UCharVectorToBytes(aliceXMSS.Address()))
}

func TestIsValidAddress(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)

	assert.True(t, IsValidAddress(misc.UCharVectorToBytes(aliceXMSS.Address())))
	assert.False(t, IsValidAddress(misc.UCharVectorToBytes(aliceXMSS.Address())[0:3]))
	assert.False(t, IsValidAddress(config.GetConfig().Dev.Genesis.CoinbaseAddress))
}

func TestAddressState_PBData(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	nonce := uint64(0)
	balance := uint64(100)
	otsBitfield := make([][]byte, config.GetConfig().Dev.OtsBitFieldSize)

	var tokens map[string]uint64
	var slavePksAccessType map[string]uint32

	a := NewTestAddressState(aliceXMSS.QAddress(), nonce, balance, otsBitfield, tokens, slavePksAccessType, 0)
	assert.NotNil(t, a)
	assert.NotNil(t, a.a.PBData())
}
