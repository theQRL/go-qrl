package transactions

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/theQRL/go-qrl/pkg/core/addressstate"
	"github.com/theQRL/go-qrl/pkg/crypto"
	"github.com/theQRL/go-qrl/pkg/misc"
	"github.com/theQRL/go-qrl/test/helper"
	"github.com/theQRL/qrllib/goqrllib/goqrllib"
)

type TestTransferTokenTransaction struct {
	tx *TransferTokenTransaction
}

func NewTestTransferTokenTransaction(tokenTxHash string, addrsTo []string, amounts []uint64, fee uint64, xmssPK []byte, masterAddr []byte) *TestTransferTokenTransaction {

	bytesAddrsTo := helper.StringAddressToBytesArray(addrsTo)
	tx := CreateTransferTokenTransaction(misc.HStr2Bin(tokenTxHash), bytesAddrsTo, amounts, fee, xmssPK, masterAddr)

	return &TestTransferTokenTransaction{tx: tx}
}

func TestCreateTransferTokenTransaction(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)

	tokenTxHash := "0a0fff"
	addrsTo := []string{bobXMSS.QAddress(), aliceXMSS.QAddress()}
	bytesAddrsTo := helper.StringAddressToBytesArray(addrsTo)
	amounts := []uint64{100, 200}
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(randomXMSS.PK())

	tx := CreateTransferTokenTransaction(misc.HStr2Bin(tokenTxHash), bytesAddrsTo, amounts, fee, xmssPK, nil)

	assert.NotNil(t, tx)
}

func TestTransferTokenTransaction_TokenTxhash(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)

	tokenTxHash := "0a0fff"
	addrsTo := []string{bobXMSS.QAddress(), aliceXMSS.QAddress()}
	amounts := []uint64{100, 200}
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(randomXMSS.PK())

	transferTokenTx := NewTestTransferTokenTransaction(tokenTxHash, addrsTo, amounts, fee, xmssPK, nil)
	assert.NotNil(t, transferTokenTx.tx)

	assert.Equal(t, transferTokenTx.tx.TokenTxhash(), misc.HStr2Bin(tokenTxHash))
}

func TestTransferTokenTransaction_AddrFrom(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)

	tokenTxHash := "0a0fff"
	addrsTo := []string{bobXMSS.QAddress(), aliceXMSS.QAddress()}
	amounts := []uint64{100, 200}
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(randomXMSS.PK())

	transferTokenTx := NewTestTransferTokenTransaction(tokenTxHash, addrsTo, amounts, fee, xmssPK, nil)
	assert.NotNil(t, transferTokenTx.tx)

	assert.Equal(t, transferTokenTx.tx.AddrFrom(), misc.UCharVectorToBytes(randomXMSS.Address()))
}

func TestTransferTokenTransaction_AddrsTo(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)

	tokenTxHash := "0a0fff"
	addrsTo := []string{bobXMSS.QAddress(), aliceXMSS.QAddress()}
	amounts := []uint64{100, 200}
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(randomXMSS.PK())

	transferTokenTx := NewTestTransferTokenTransaction(tokenTxHash, addrsTo, amounts, fee, xmssPK, nil)

	assert.NotNil(t, transferTokenTx.tx)
	assert.Equal(t, transferTokenTx.tx.AddrsTo(), helper.StringAddressToBytesArray(addrsTo))
}

func TestTransferTokenTransaction_Amounts(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)

	tokenTxHash := "0a0fff"
	addrsTo := []string{bobXMSS.QAddress(), aliceXMSS.QAddress()}
	amounts := []uint64{100, 200}
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(randomXMSS.PK())

	transferTokenTx := NewTestTransferTokenTransaction(tokenTxHash, addrsTo, amounts, fee, xmssPK, nil)

	assert.NotNil(t, transferTokenTx.tx)
	assert.Equal(t, transferTokenTx.tx.Amounts(), amounts)
}

func TestTransferTokenTransaction_TotalAmount(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)

	tokenTxHash := "0a0fff"
	addrsTo := []string{bobXMSS.QAddress(), aliceXMSS.QAddress()}
	amounts := []uint64{100, 200}
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(randomXMSS.PK())

	transferTokenTx := NewTestTransferTokenTransaction(tokenTxHash, addrsTo, amounts, fee, xmssPK, nil)

	totalAmount := uint64(0)
	for _, amount := range amounts {
		totalAmount += amount
	}

	assert.NotNil(t, transferTokenTx.tx)
	assert.Equal(t, transferTokenTx.tx.TotalAmount(), totalAmount)
}

func TestTransferTokenTransaction_GetHashableBytes(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)

	tokenTxHash := "0a0fff"
	addrsTo := []string{bobXMSS.QAddress(), aliceXMSS.QAddress()}
	amounts := []uint64{100, 200}
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(randomXMSS.PK())
	hashableBytes := "e48bcc4c43a9300f027815aa6d658414efee8448c103fdbf347159c2dc55e4c9"

	transferTokenTx := NewTestTransferTokenTransaction(tokenTxHash, addrsTo, amounts, fee, xmssPK, nil)

	assert.NotNil(t, transferTokenTx.tx)
	assert.Equal(t, misc.Bin2HStr(transferTokenTx.tx.GetHashableBytes()), hashableBytes)
}

func TestTransferTokenTransaction_Validate(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)

	tokenTxHash := "0a0fff"
	addrsTo := []string{bobXMSS.QAddress(), aliceXMSS.QAddress()}
	amounts := []uint64{100, 200}
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(randomXMSS.PK())

	transferTokenTx := NewTestTransferTokenTransaction(tokenTxHash, addrsTo, amounts, fee, xmssPK, nil)

	assert.NotNil(t, transferTokenTx.tx)
	assert.True(t, transferTokenTx.tx.Validate(false))

	// Signed transaction, signature verification should pass
	transferTokenTx.tx.Sign(randomXMSS, misc.BytesToUCharVector(transferTokenTx.tx.GetHashableBytes()))
	assert.True(t, transferTokenTx.tx.Validate(true))

	// Changed AddressTo And Amounts, validation must fail
	transferTokenTx.tx.PBData().GetTransferToken().AddrsTo = helper.StringAddressToBytesArray([]string{bobXMSS.QAddress()})
	transferTokenTx.tx.PBData().GetTransferToken().Amounts = []uint64{100}
	assert.False(t, transferTokenTx.tx.Validate(true))
}

func TestTransferTokenTransaction_Validate2(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)

	tokenTxHash := "0a0fff"
	addrsTo := []string{bobXMSS.QAddress(), aliceXMSS.QAddress()}
	amounts := []uint64{100, 200}
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(randomXMSS.PK())

	transferTokenTx := NewTestTransferTokenTransaction(tokenTxHash, addrsTo, amounts, fee, xmssPK, nil)

	assert.NotNil(t, transferTokenTx.tx)
	assert.True(t, transferTokenTx.tx.Validate(false))

	// Signed transaction, signature verification should pass
	transferTokenTx.tx.Sign(randomXMSS, misc.BytesToUCharVector(transferTokenTx.tx.GetHashableBytes()))
	assert.True(t, transferTokenTx.tx.Validate(true))

	// Changed Token Transaction Hash to some different address, validation must fail
	transferTokenTx.tx.PBData().GetTransferToken().TokenTxhash = []byte("aa")
	assert.False(t, transferTokenTx.tx.Validate(true))
}

func TestTransferTokenTransaction_Validate3(t *testing.T) {
	/*
		Test for mismatching number of amounts and number of AddressTo
	*/
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)

	tokenTxHash := "0a0fff"
	addrsTo := []string{bobXMSS.QAddress(), aliceXMSS.QAddress()}
	amounts := []uint64{100}
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(randomXMSS.PK())

	transferTokenTx := NewTestTransferTokenTransaction(tokenTxHash, addrsTo, amounts, fee, xmssPK, nil)
	assert.Nil(t, transferTokenTx.tx)

	addrsTo = []string{bobXMSS.QAddress()}
	amounts = []uint64{100, 200}
	fee = uint64(1)
	xmssPK = misc.UCharVectorToBytes(randomXMSS.PK())

	transferTokenTx = NewTestTransferTokenTransaction(tokenTxHash, addrsTo, amounts, fee, xmssPK, nil)
	assert.Nil(t, transferTokenTx.tx)
}

func TestTransferTokenTransaction_Validate4(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)

	tokenTxHash := "0a0fff"
	addrsTo := []string{bobXMSS.QAddress(), aliceXMSS.QAddress()}
	amounts := []uint64{100, 200}
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(randomXMSS.PK())

	transferTokenTx := NewTestTransferTokenTransaction(tokenTxHash, addrsTo, amounts, fee, xmssPK, nil)

	assert.NotNil(t, transferTokenTx.tx)
	transferTokenTx.tx.Sign(randomXMSS, misc.BytesToUCharVector(transferTokenTx.tx.GetHashableBytes()))
	assert.True(t, transferTokenTx.tx.Validate(true))

	// Random Transaction Hash, validate must fail
	transferTokenTx.tx.PBData().TransactionHash = []byte{0, 1, 5}
	assert.False(t, transferTokenTx.tx.Validate(true))
}

func TestTransferTokenTransaction_ValidateCustom(t *testing.T) {
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)

	tokenTxHash := "0a0fff"
	addrsTo := make([]string, 101)
	amounts := make([]uint64, 101)
	for i := 0; i < len(addrsTo); i++ {
		addrsTo[i] = bobXMSS.QAddress()
		amounts[i] = 100
	}
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(randomXMSS.PK())

	transferTokenTx := NewTestTransferTokenTransaction(tokenTxHash, addrsTo, amounts, fee, xmssPK, nil)

	// Transaction must be nil, as addrsTo is more than the limit
	assert.Nil(t, transferTokenTx.tx)
}

func TestTransferTokenTransaction_ValidateCustom2(t *testing.T) {
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)

	tokenTxHash := "0a0fff"
	addrsTo := make([]string, 100)
	amounts := make([]uint64, 100)
	for i := 0; i < len(addrsTo); i++ {
		addrsTo[i] = bobXMSS.QAddress()
		amounts[i] = 100
	}
	amounts[len(amounts)-1] = 0
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(randomXMSS.PK())

	transferTokenTx := NewTestTransferTokenTransaction(tokenTxHash, addrsTo, amounts, fee, xmssPK, nil)

	// Transaction must be nil, as one of amount is 0
	assert.Nil(t, transferTokenTx.tx)
}

func TestTransferTokenTransaction_ValidateCustom3(t *testing.T) {
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)

	tokenTxHash := "0a0fff"
	addrsTo := make([]string, 100)
	amounts := make([]uint64, 100)
	for i := 0; i < len(addrsTo); i++ {
		addrsTo[i] = bobXMSS.QAddress()
		amounts[i] = 100
	}
	addrsTo[len(addrsTo)-1] = "Q01234567"
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(randomXMSS.PK())

	transferTokenTx := NewTestTransferTokenTransaction(tokenTxHash, addrsTo, amounts, fee, xmssPK, nil)

	// Transaction must be nil, as one of the addrsTo is an invalid QRL Address
	assert.Nil(t, transferTokenTx.tx)
}

func TestTransferTokenTransaction_ValidateExtended(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)

	tokenTxHash := "0a0fff"
	addrsTo := []string{bobXMSS.QAddress(), aliceXMSS.QAddress()}
	amounts := []uint64{100, 200}
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(randomXMSS.PK())

	transferTokenTx := NewTestTransferTokenTransaction(tokenTxHash, addrsTo, amounts, fee, xmssPK, nil)
	assert.NotNil(t, transferTokenTx.tx)

	addrFromState := addressstate.GetDefaultAddressState(misc.UCharVectorToBytes(randomXMSS.Address()))
	addrFromState.UpdateTokenBalance(misc.HStr2Bin(tokenTxHash), 300, false)
	transferTokenTx.tx.Sign(randomXMSS, misc.BytesToUCharVector(transferTokenTx.tx.GetHashableBytes()))

	// Since balance is 0, validation should fail as required fee is 1
	assert.False(t, transferTokenTx.tx.ValidateExtended(addrFromState, addrFromState))

	// Added balance
	addrFromState.AddBalance(1)

	assert.True(t, transferTokenTx.tx.ValidateExtended(addrFromState, addrFromState))
}

func TestTransferTokenTransaction_ValidateExtended2(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)
	slaveXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128) // Another random XMSS for Slave

	tokenTxHash := "0a0fff"
	addrsTo := []string{bobXMSS.QAddress(), aliceXMSS.QAddress()}
	amounts := []uint64{100, 200}
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(slaveXMSS.PK())

	transferTokenTx := NewTestTransferTokenTransaction(tokenTxHash, addrsTo, amounts, fee, xmssPK, misc.UCharVectorToBytes(randomXMSS.Address()))
	assert.NotNil(t, transferTokenTx.tx)

	addrFromState := addressstate.GetDefaultAddressState(misc.UCharVectorToBytes(randomXMSS.Address()))
	addrFromState.UpdateTokenBalance(misc.HStr2Bin(tokenTxHash), 300, false)
	addrFromState.AddBalance(1)
	addrFromPKState := addressstate.GetDefaultAddressState(misc.UCharVectorToBytes(slaveXMSS.Address()))
	transferTokenTx.tx.Sign(slaveXMSS, misc.BytesToUCharVector(transferTokenTx.tx.GetHashableBytes()))

	// Slave is not registered, validation must fail
	assert.False(t, transferTokenTx.tx.ValidateExtended(addrFromState, addrFromPKState))

	addrFromState.AddSlavePKSAccessType(misc.UCharVectorToBytes(slaveXMSS.PK()), 0)
	assert.True(t, transferTokenTx.tx.ValidateExtended(addrFromState, addrFromPKState))
}

func TestTransferTokenTransaction_ValidateExtended3(t *testing.T) {
	/*
		Test for signing a transfer token transaction via slave with an used ots key
	*/
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)
	slaveXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128) // Another random XMSS for Slave

	tokenTxHash := "0a0fff"
	addrsTo := []string{bobXMSS.QAddress(), aliceXMSS.QAddress()}
	amounts := []uint64{100, 200}
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(slaveXMSS.PK())

	transferTokenTx := NewTestTransferTokenTransaction(tokenTxHash, addrsTo, amounts, fee, xmssPK, misc.UCharVectorToBytes(randomXMSS.Address()))
	assert.NotNil(t, transferTokenTx.tx)

	addrFromState := addressstate.GetDefaultAddressState(misc.UCharVectorToBytes(randomXMSS.Address()))
	addrFromState.UpdateTokenBalance(misc.HStr2Bin(tokenTxHash), 300, false)
	addrFromState.AddBalance(1)
	addrFromState.AddSlavePKSAccessType(misc.UCharVectorToBytes(slaveXMSS.PK()), 0) // Adding slave

	addrFromPKState := addressstate.GetDefaultAddressState(misc.UCharVectorToBytes(slaveXMSS.Address()))

	transferTokenTx.tx.Sign(slaveXMSS, misc.BytesToUCharVector(transferTokenTx.tx.GetHashableBytes()))
	assert.True(t, transferTokenTx.tx.ValidateExtended(addrFromState, addrFromPKState))
	addrFromPKState.SetOTSKey(0) // Marked ots key 0 as used
	// Signed by an used ots key, validation must fail
	assert.False(t, transferTokenTx.tx.ValidateExtended(addrFromState, addrFromPKState))

	randomXMSS.SetOTSIndex(10)
	transferTokenTx.tx.Sign(randomXMSS, misc.BytesToUCharVector(transferTokenTx.tx.GetHashableBytes()))
	assert.True(t, transferTokenTx.tx.ValidateExtended(addrFromState, addrFromPKState))
	addrFromPKState.SetOTSKey(10) // Marked ots key 10 as used
	// Signed by an used ots key, validation must fail
	assert.False(t, transferTokenTx.tx.ValidateExtended(addrFromState, addrFromPKState))
}

func TestTransferTokenTransaction_ValidateExtended4(t *testing.T) {
	/*
		Test for signing a transfer token transaction without slave with an used ots key
	*/
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)

	tokenTxHash := "0a0fff"
	addrsTo := []string{bobXMSS.QAddress(), aliceXMSS.QAddress()}
	amounts := []uint64{100, 200}
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(randomXMSS.PK())

	transferTokenTx := NewTestTransferTokenTransaction(tokenTxHash, addrsTo, amounts, fee, xmssPK, nil)
	assert.NotNil(t, transferTokenTx.tx)

	addrFromState := addressstate.GetDefaultAddressState(misc.UCharVectorToBytes(randomXMSS.Address()))
	addrFromState.UpdateTokenBalance(misc.HStr2Bin(tokenTxHash), 300, false)
	addrFromState.AddBalance(1)

	transferTokenTx.tx.Sign(randomXMSS, misc.BytesToUCharVector(transferTokenTx.tx.GetHashableBytes()))
	assert.True(t, transferTokenTx.tx.ValidateExtended(addrFromState, addrFromState))
	addrFromState.SetOTSKey(0) // Marked ots key 0 as used
	// Signed by an used ots key, validation must fail
	assert.False(t, transferTokenTx.tx.ValidateExtended(addrFromState, addrFromState))

	randomXMSS.SetOTSIndex(10)
	transferTokenTx.tx.Sign(randomXMSS, misc.BytesToUCharVector(transferTokenTx.tx.GetHashableBytes()))
	assert.True(t, transferTokenTx.tx.ValidateExtended(addrFromState, addrFromState))
	addrFromState.SetOTSKey(10) // Marked ots key 10 as used
	// Signed by an used ots key, validation must fail
	assert.False(t, transferTokenTx.tx.ValidateExtended(addrFromState, addrFromState))
}

func TestTransferTokenTransaction_ApplyStateChanges(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)

	tokenTxHash := "0a0fff"
	bytesTokenTxHash := misc.HStr2Bin(tokenTxHash)
	addrsTo := []string{bobXMSS.QAddress(), aliceXMSS.QAddress()}
	amounts := []uint64{100, 200}
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(randomXMSS.PK())
	initialBalance := uint64(301)
	aliceInitialBalance := uint64(800)
	bobInitialBalance := uint64(500)

	initialTokenBalance := uint64(501)

	transferTokenTx := NewTestTransferTokenTransaction(tokenTxHash, addrsTo, amounts, fee, xmssPK, nil)
	assert.NotNil(t, transferTokenTx.tx)

	transferTokenTx.tx.Sign(randomXMSS, misc.BytesToUCharVector(transferTokenTx.tx.GetHashableBytes()))

	addressesState := make(map[string]*addressstate.AddressState)
	transferTokenTx.tx.SetAffectedAddress(addressesState)

	assert.Len(t, addressesState, 3)

	addressesState[randomXMSS.QAddress()] = addressstate.GetDefaultAddressState(misc.UCharVectorToBytes(randomXMSS.Address()))
	for _, qAddr := range addrsTo {
		addressesState[qAddr] = addressstate.GetDefaultAddressState(misc.Qaddress2Bin(qAddr))
	}

	// Initializing balance
	addressesState[randomXMSS.QAddress()].AddBalance(initialBalance)
	addressesState[aliceXMSS.QAddress()].AddBalance(aliceInitialBalance)
	addressesState[bobXMSS.QAddress()].AddBalance(bobInitialBalance)

	// Initializing Token Balance
	addressesState[randomXMSS.QAddress()].UpdateTokenBalance(bytesTokenTxHash, initialTokenBalance, false)

	transferTokenTx.tx.ApplyStateChanges(addressesState)
	assert.Equal(t, addressesState[randomXMSS.QAddress()].Balance(), initialBalance-fee)
	assert.Equal(t, addressesState[aliceXMSS.QAddress()].Balance(), aliceInitialBalance)
	assert.Equal(t, addressesState[bobXMSS.QAddress()].Balance(), bobInitialBalance)

	assert.Equal(t, addressesState[randomXMSS.QAddress()].GetTokenBalance(bytesTokenTxHash), initialTokenBalance-300)
	assert.Equal(t, addressesState[aliceXMSS.QAddress()].GetTokenBalance(bytesTokenTxHash), uint64(200))
	assert.Equal(t, addressesState[bobXMSS.QAddress()].GetTokenBalance(bytesTokenTxHash), uint64(100))
}

func TestTransferTokenTransaction_ApplyStateChanges2(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)

	tokenTxHash := "0a0fff"
	bytesTokenTxHash := misc.HStr2Bin(tokenTxHash)
	addrsTo := []string{bobXMSS.QAddress(), aliceXMSS.QAddress()}
	amounts := []uint64{100, 200}
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(randomXMSS.PK())
	initialBalance := uint64(301)
	aliceInitialBalance := uint64(800)
	bobInitialBalance := uint64(500)

	initialTokenBalance := uint64(501)
	aliceInitialTokenBalance := uint64(1001)
	bobInitialTokenBalance := uint64(2001)

	transferTokenTx := NewTestTransferTokenTransaction(tokenTxHash, addrsTo, amounts, fee, xmssPK, nil)
	assert.NotNil(t, transferTokenTx.tx)

	transferTokenTx.tx.Sign(randomXMSS, misc.BytesToUCharVector(transferTokenTx.tx.GetHashableBytes()))

	addressesState := make(map[string]*addressstate.AddressState)
	transferTokenTx.tx.SetAffectedAddress(addressesState)

	assert.Len(t, addressesState, 3)

	addressesState[randomXMSS.QAddress()] = addressstate.GetDefaultAddressState(misc.UCharVectorToBytes(randomXMSS.Address()))
	for _, qAddr := range addrsTo {
		addressesState[qAddr] = addressstate.GetDefaultAddressState(misc.Qaddress2Bin(qAddr))
	}

	// Initializing balance
	addressesState[randomXMSS.QAddress()].AddBalance(initialBalance)
	addressesState[aliceXMSS.QAddress()].AddBalance(aliceInitialBalance)
	addressesState[bobXMSS.QAddress()].AddBalance(bobInitialBalance)

	// Initializing Token Balance
	addressesState[randomXMSS.QAddress()].UpdateTokenBalance(bytesTokenTxHash, initialTokenBalance, false)
	addressesState[aliceXMSS.QAddress()].UpdateTokenBalance(bytesTokenTxHash, aliceInitialTokenBalance, false)
	addressesState[bobXMSS.QAddress()].UpdateTokenBalance(bytesTokenTxHash, bobInitialTokenBalance, false)

	transferTokenTx.tx.ApplyStateChanges(addressesState)
	assert.Equal(t, addressesState[randomXMSS.QAddress()].Balance(), initialBalance-fee)
	assert.Equal(t, addressesState[aliceXMSS.QAddress()].Balance(), aliceInitialBalance)
	assert.Equal(t, addressesState[bobXMSS.QAddress()].Balance(), bobInitialBalance)

	assert.Equal(t, addressesState[randomXMSS.QAddress()].GetTokenBalance(bytesTokenTxHash), initialTokenBalance-300)
	assert.Equal(t, addressesState[aliceXMSS.QAddress()].GetTokenBalance(bytesTokenTxHash), aliceInitialTokenBalance+uint64(200))
	assert.Equal(t, addressesState[bobXMSS.QAddress()].GetTokenBalance(bytesTokenTxHash), bobInitialTokenBalance+uint64(100))
}

func TestTransferTokenTransaction_RevertStateChanges(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)

	tokenTxHash := "0a0fff"
	bytesTokenTxHash := misc.HStr2Bin(tokenTxHash)
	addrsTo := []string{bobXMSS.QAddress(), aliceXMSS.QAddress()}
	amounts := []uint64{100, 200}
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(randomXMSS.PK())
	initialBalance := uint64(301)
	aliceInitialBalance := uint64(800)
	bobInitialBalance := uint64(500)

	initialTokenBalance := uint64(501)

	transferTokenTx := NewTestTransferTokenTransaction(tokenTxHash, addrsTo, amounts, fee, xmssPK, nil)
	assert.NotNil(t, transferTokenTx.tx)

	transferTokenTx.tx.Sign(randomXMSS, misc.BytesToUCharVector(transferTokenTx.tx.GetHashableBytes()))

	addressesState := make(map[string]*addressstate.AddressState)
	transferTokenTx.tx.SetAffectedAddress(addressesState)

	assert.Len(t, addressesState, 3)

	addressesState[randomXMSS.QAddress()] = addressstate.GetDefaultAddressState(misc.UCharVectorToBytes(randomXMSS.Address()))
	for _, qAddr := range addrsTo {
		addressesState[qAddr] = addressstate.GetDefaultAddressState(misc.Qaddress2Bin(qAddr))
	}

	// Initializing balance
	addressesState[randomXMSS.QAddress()].AddBalance(initialBalance)
	addressesState[aliceXMSS.QAddress()].AddBalance(aliceInitialBalance)
	addressesState[bobXMSS.QAddress()].AddBalance(bobInitialBalance)

	// Initializing Token Balance
	addressesState[randomXMSS.QAddress()].UpdateTokenBalance(bytesTokenTxHash, initialTokenBalance, false)

	transferTokenTx.tx.ApplyStateChanges(addressesState)
	assert.Equal(t, addressesState[randomXMSS.QAddress()].Balance(), initialBalance-fee)
	assert.Equal(t, addressesState[aliceXMSS.QAddress()].Balance(), aliceInitialBalance)
	assert.Equal(t, addressesState[bobXMSS.QAddress()].Balance(), bobInitialBalance)

	assert.Equal(t, addressesState[randomXMSS.QAddress()].GetTokenBalance(bytesTokenTxHash), initialTokenBalance-300)
	assert.Equal(t, addressesState[aliceXMSS.QAddress()].GetTokenBalance(bytesTokenTxHash), uint64(200))
	assert.Equal(t, addressesState[bobXMSS.QAddress()].GetTokenBalance(bytesTokenTxHash), uint64(100))

	transferTokenTx.tx.RevertStateChanges(addressesState)
	assert.Equal(t, addressesState[randomXMSS.QAddress()].Balance(), initialBalance)
	assert.Equal(t, addressesState[aliceXMSS.QAddress()].Balance(), aliceInitialBalance)
	assert.Equal(t, addressesState[bobXMSS.QAddress()].Balance(), bobInitialBalance)

	assert.Equal(t, addressesState[randomXMSS.QAddress()].GetTokenBalance(bytesTokenTxHash), initialTokenBalance)
	assert.Equal(t, addressesState[aliceXMSS.QAddress()].GetTokenBalance(bytesTokenTxHash), uint64(0))
	assert.Equal(t, addressesState[bobXMSS.QAddress()].GetTokenBalance(bytesTokenTxHash), uint64(0))
}

func TestTransferTokenTransaction_RevertStateChanges2(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)

	tokenTxHash := "0a0fff"
	bytesTokenTxHash := misc.HStr2Bin(tokenTxHash)
	addrsTo := []string{bobXMSS.QAddress(), aliceXMSS.QAddress()}
	amounts := []uint64{100, 200}
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(randomXMSS.PK())
	initialBalance := uint64(301)
	aliceInitialBalance := uint64(800)
	bobInitialBalance := uint64(500)

	initialTokenBalance := uint64(501)
	aliceInitialTokenBalance := uint64(1001)
	bobInitialTokenBalance := uint64(2001)

	transferTokenTx := NewTestTransferTokenTransaction(tokenTxHash, addrsTo, amounts, fee, xmssPK, nil)
	assert.NotNil(t, transferTokenTx.tx)

	transferTokenTx.tx.Sign(randomXMSS, misc.BytesToUCharVector(transferTokenTx.tx.GetHashableBytes()))

	addressesState := make(map[string]*addressstate.AddressState)
	transferTokenTx.tx.SetAffectedAddress(addressesState)

	assert.Len(t, addressesState, 3)

	addressesState[randomXMSS.QAddress()] = addressstate.GetDefaultAddressState(misc.UCharVectorToBytes(randomXMSS.Address()))
	for _, qAddr := range addrsTo {
		addressesState[qAddr] = addressstate.GetDefaultAddressState(misc.Qaddress2Bin(qAddr))
	}

	// Initializing balance
	addressesState[randomXMSS.QAddress()].AddBalance(initialBalance)
	addressesState[aliceXMSS.QAddress()].AddBalance(aliceInitialBalance)
	addressesState[bobXMSS.QAddress()].AddBalance(bobInitialBalance)

	// Initializing Token Balance
	addressesState[randomXMSS.QAddress()].UpdateTokenBalance(bytesTokenTxHash, initialTokenBalance, false)
	addressesState[aliceXMSS.QAddress()].UpdateTokenBalance(bytesTokenTxHash, aliceInitialTokenBalance, false)
	addressesState[bobXMSS.QAddress()].UpdateTokenBalance(bytesTokenTxHash, bobInitialTokenBalance, false)

	transferTokenTx.tx.ApplyStateChanges(addressesState)
	assert.Equal(t, addressesState[randomXMSS.QAddress()].Balance(), initialBalance-fee)
	assert.Equal(t, addressesState[aliceXMSS.QAddress()].Balance(), aliceInitialBalance)
	assert.Equal(t, addressesState[bobXMSS.QAddress()].Balance(), bobInitialBalance)

	assert.Equal(t, addressesState[randomXMSS.QAddress()].GetTokenBalance(bytesTokenTxHash), initialTokenBalance-300)
	assert.Equal(t, addressesState[aliceXMSS.QAddress()].GetTokenBalance(bytesTokenTxHash), aliceInitialTokenBalance+uint64(200))
	assert.Equal(t, addressesState[bobXMSS.QAddress()].GetTokenBalance(bytesTokenTxHash), bobInitialTokenBalance+uint64(100))

	transferTokenTx.tx.RevertStateChanges(addressesState)
	assert.Equal(t, addressesState[randomXMSS.QAddress()].Balance(), initialBalance)
	assert.Equal(t, addressesState[aliceXMSS.QAddress()].Balance(), aliceInitialBalance)
	assert.Equal(t, addressesState[bobXMSS.QAddress()].Balance(), bobInitialBalance)

	assert.Equal(t, addressesState[randomXMSS.QAddress()].GetTokenBalance(bytesTokenTxHash), initialTokenBalance)
	assert.Equal(t, addressesState[aliceXMSS.QAddress()].GetTokenBalance(bytesTokenTxHash), aliceInitialTokenBalance)
	assert.Equal(t, addressesState[bobXMSS.QAddress()].GetTokenBalance(bytesTokenTxHash), bobInitialTokenBalance)
}

func TestTransferTokenTransaction_SetAffectedAddress(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)

	tokenTxHash := "0a0fff"
	addrsTo := []string{bobXMSS.QAddress(), aliceXMSS.QAddress()}
	amounts := []uint64{100, 200}
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(randomXMSS.PK())

	transferTokenTx := NewTestTransferTokenTransaction(tokenTxHash, addrsTo, amounts, fee, xmssPK, nil)
	assert.NotNil(t, transferTokenTx.tx)

	addressesState := make(map[string]*addressstate.AddressState)
	transferTokenTx.tx.SetAffectedAddress(addressesState)

	assert.Len(t, addressesState, 3)

	assert.Contains(t, addressesState, aliceXMSS.QAddress())
	assert.Contains(t, addressesState, bobXMSS.QAddress())
	assert.Contains(t, addressesState, randomXMSS.QAddress())
}

func TestTransferTokenTransaction_FromPBData(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)

	tokenTxHash := "0a0fff"
	addrsTo := []string{bobXMSS.QAddress(), aliceXMSS.QAddress()}
	amounts := []uint64{100, 200}
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(randomXMSS.PK())

	transferTokenTx := NewTestTransferTokenTransaction(tokenTxHash, addrsTo, amounts, fee, xmssPK, nil)
	assert.NotNil(t, transferTokenTx.tx)

	pbdata := transferTokenTx.tx.PBData()
	tx2 := TransferTokenTransaction{}
	tx2.FromPBdata(*pbdata)

	assert.Equal(t, pbdata, tx2.PBData())

	// Test to ensure, FromPBData doesnt use reference object to initialize tx.data
	transferTokenTx.tx.PBData().Fee = 10
	assert.Equal(t, transferTokenTx.tx.Fee(), uint64(10))
	assert.NotEqual(t, tx2.Fee(), transferTokenTx.tx.Fee())

	// A random Transfer Token Txn
	tx3 := TransferTokenTransaction{}
	assert.NotEqual(t, pbdata, tx3.PBData())
}
