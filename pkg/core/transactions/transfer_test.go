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

type TestTransferTransaction struct {
	tx *TransferTransaction
}


func NewTestTransferTransaction(addrsTo []string, amounts []uint64, fee uint64, xmssPK []byte, masterAddr []byte) *TestTransferTransaction {
	bytesAddrsTo := helper.StringAddressToBytesArray(addrsTo)
	tx := CreateTransferTransaction(bytesAddrsTo, amounts, fee, xmssPK, masterAddr)

	return &TestTransferTransaction{tx: tx}
}

func TestCreateTransferTransaction(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)

	addrsTo := []string{bobXMSS.QAddress(), aliceXMSS.QAddress()}
	bytesAddrsTo := helper.StringAddressToBytesArray(addrsTo)
	amounts := []uint64{100, 200}
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(randomXMSS.PK())

	tx := CreateTransferTransaction(bytesAddrsTo, amounts, fee, xmssPK, nil)

	assert.NotNil(t, tx)
}

func TestTransferTransaction_AddrFrom(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)

	addrsTo := []string{bobXMSS.QAddress(), aliceXMSS.QAddress()}
	amounts := []uint64{100, 200}
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(randomXMSS.PK())

	transferTx := NewTestTransferTransaction(addrsTo, amounts, fee, xmssPK, nil)
	assert.NotNil(t, transferTx.tx)

	assert.Equal(t, transferTx.tx.AddrFrom(), misc.UCharVectorToBytes(randomXMSS.Address()))
}

func TestTransferTransaction_AddrsTo(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)

	addrsTo := []string{bobXMSS.QAddress(), aliceXMSS.QAddress()}
	amounts := []uint64{100, 200}
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(randomXMSS.PK())

	transferTx := NewTestTransferTransaction(addrsTo, amounts, fee, xmssPK, nil)

	assert.NotNil(t, transferTx.tx)
	assert.Equal(t, transferTx.tx.AddrsTo(), helper.StringAddressToBytesArray(addrsTo))
}

func TestTransferTransaction_Amounts(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)

	addrsTo := []string{bobXMSS.QAddress(), aliceXMSS.QAddress()}
	amounts := []uint64{100, 200}
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(randomXMSS.PK())

	transferTx := NewTestTransferTransaction(addrsTo, amounts, fee, xmssPK, nil)

	assert.NotNil(t, transferTx.tx)
	assert.Equal(t, transferTx.tx.Amounts(), amounts)
}

func TestTransferTransaction_TotalAmounts(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)

	addrsTo := []string{bobXMSS.QAddress(), aliceXMSS.QAddress()}
	amounts := []uint64{100, 200}
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(randomXMSS.PK())

	transferTx := NewTestTransferTransaction(addrsTo, amounts, fee, xmssPK, nil)

	totalAmount := uint64(0)
	for _, amount := range amounts {
		totalAmount += amount
	}

	assert.NotNil(t, transferTx.tx)
	assert.Equal(t, transferTx.tx.TotalAmounts(), totalAmount)
}

func TestTransferTransaction_GetHashableBytes(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)

	addrsTo := []string{bobXMSS.QAddress(), aliceXMSS.QAddress()}
	amounts := []uint64{100, 200}
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(randomXMSS.PK())
	hashableBytes := "ebb71581fe0541190fb9d69cf327536eca669e32780ba2c4f3d3942d4c1f02eb"

	transferTx := NewTestTransferTransaction(addrsTo, amounts, fee, xmssPK, nil)

	assert.NotNil(t, transferTx.tx)
	assert.Equal(t, misc.Bin2HStr(transferTx.tx.GetHashableBytes()), hashableBytes)
}

func TestTransferTransaction_Validate(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)

	addrsTo := []string{bobXMSS.QAddress(), aliceXMSS.QAddress()}
	amounts := []uint64{100, 200}
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(randomXMSS.PK())

	transferTx := NewTestTransferTransaction(addrsTo, amounts, fee, xmssPK, nil)

	assert.NotNil(t, transferTx.tx)
	assert.True(t, transferTx.tx.Validate(false))

	// Signed transaction, signature verification should pass
	transferTx.tx.Sign(randomXMSS, misc.BytesToUCharVector(transferTx.tx.GetHashableBytes()))
	assert.True(t, transferTx.tx.Validate(true))

	// Changed Transaction Hash to some different address, validation must fail
	transferTx.tx.PBData().GetTransfer().AddrsTo = helper.StringAddressToBytesArray([]string{bobXMSS.QAddress()})
	transferTx.tx.PBData().GetTransfer().Amounts = []uint64{100}
	assert.False(t, transferTx.tx.Validate(true))
}

func TestTransferTransaction_Validate2(t *testing.T) {
	/*
	Test for mismatching number of amounts and number of AddressTo
	 */
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)

	addrsTo := []string{bobXMSS.QAddress(), aliceXMSS.QAddress()}
	amounts := []uint64{100}
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(randomXMSS.PK())

	transferTx := NewTestTransferTransaction(addrsTo, amounts, fee, xmssPK, nil)
	assert.Nil(t, transferTx.tx)

	addrsTo = []string{bobXMSS.QAddress()}
	amounts = []uint64{100, 200}
	fee = uint64(1)
	xmssPK = misc.UCharVectorToBytes(randomXMSS.PK())

	transferTx = NewTestTransferTransaction(addrsTo, amounts, fee, xmssPK, nil)
	assert.Nil(t, transferTx.tx)
}

func TestTransferTransaction_Validate3(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)

	addrsTo := []string{bobXMSS.QAddress(), aliceXMSS.QAddress()}
	amounts := []uint64{100, 200}
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(randomXMSS.PK())

	transferTx := NewTestTransferTransaction(addrsTo, amounts, fee, xmssPK, nil)

	assert.NotNil(t, transferTx.tx)
	transferTx.tx.Sign(randomXMSS, misc.BytesToUCharVector(transferTx.tx.GetHashableBytes()))
	assert.True(t, transferTx.tx.Validate(true))

	// Random Transaction Hash, validate must fail
	transferTx.tx.PBData().TransactionHash = []byte{0, 1, 5}
	assert.False(t, transferTx.tx.Validate(true))
}

func TestTransferTransaction_ValidateCustom(t *testing.T) {
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)

	addrsTo := make([]string, 101)
	amounts := make([]uint64, 101)
	for i := 0; i < len(addrsTo); i++ {
		addrsTo[i] = bobXMSS.QAddress()
		amounts[i] = 100
	}
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(randomXMSS.PK())

	transferTx := NewTestTransferTransaction(addrsTo, amounts, fee, xmssPK, nil)

	// Transaction must be nil, as addrsTo is more than the limit
	assert.Nil(t, transferTx.tx)
}

func TestTransferTransaction_ValidateCustom2(t *testing.T) {
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)

	addrsTo := make([]string, 100)
	amounts := make([]uint64, 100)
	for i := 0; i < len(addrsTo); i++ {
		addrsTo[i] = bobXMSS.QAddress()
		amounts[i] = 100
	}
	amounts[len(amounts) - 1] = 0
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(randomXMSS.PK())

	transferTx := NewTestTransferTransaction(addrsTo, amounts, fee, xmssPK, nil)

	// Transaction must be nil, as one of amount is 0
	assert.Nil(t, transferTx.tx)
}

func TestTransferTransaction_ValidateCustom3(t *testing.T) {
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)

	addrsTo := make([]string, 100)
	amounts := make([]uint64, 100)
	for i := 0; i < len(addrsTo); i++ {
		addrsTo[i] = bobXMSS.QAddress()
		amounts[i] = 100
	}
	addrsTo[len(addrsTo) - 1] = "Q01234567"
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(randomXMSS.PK())

	transferTx := NewTestTransferTransaction(addrsTo, amounts, fee, xmssPK, nil)

	// Transaction must be nil, as one of the addrsTo is an invalid QRL Address
	assert.Nil(t, transferTx.tx)
}

func TestTransferTransaction_ValidateExtended(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)

	addrsTo := []string{bobXMSS.QAddress(), aliceXMSS.QAddress()}
	amounts := []uint64{100, 200}
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(randomXMSS.PK())

	transferTx := NewTestTransferTransaction(addrsTo, amounts, fee, xmssPK, nil)
	assert.NotNil(t, transferTx.tx)

	addrFromState := addressstate.GetDefaultAddressState(misc.UCharVectorToBytes(randomXMSS.Address()))
	transferTx.tx.Sign(randomXMSS, misc.BytesToUCharVector(transferTx.tx.GetHashableBytes()))

	// Since balance is 0, validation should fail as required fee is 1
	assert.False(t, transferTx.tx.ValidateExtended(addrFromState, addrFromState))

	// Added balance
	addrFromState.AddBalance(301)

	assert.True(t, transferTx.tx.ValidateExtended(addrFromState, addrFromState))
}

func TestTransferTransaction_ValidateExtended2(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)
	slaveXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)  // Another random XMSS for Slave

	addrsTo := []string{bobXMSS.QAddress(), aliceXMSS.QAddress()}
	amounts := []uint64{100, 200}
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(slaveXMSS.PK())

	transferTx := NewTestTransferTransaction(addrsTo, amounts, fee, xmssPK, misc.UCharVectorToBytes(randomXMSS.Address()))
	assert.NotNil(t, transferTx.tx)

	addrFromState := addressstate.GetDefaultAddressState(misc.UCharVectorToBytes(randomXMSS.Address()))
	addrFromState.AddBalance(301)
	addrFromPKState := addressstate.GetDefaultAddressState(misc.UCharVectorToBytes(slaveXMSS.Address()))
	transferTx.tx.Sign(slaveXMSS, misc.BytesToUCharVector(transferTx.tx.GetHashableBytes()))

	// Slave is not registered, validation must fail
	assert.False(t, transferTx.tx.ValidateExtended(addrFromState, addrFromPKState))

	addrFromState.AddSlavePKSAccessType(misc.UCharVectorToBytes(slaveXMSS.PK()), 0)
	assert.True(t, transferTx.tx.ValidateExtended(addrFromState, addrFromPKState))
}

func TestTransferTransaction_ValidateExtended3(t *testing.T) {
	/*
	Test for signing a transfer transaction via slave with an used ots key
	 */
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)
	slaveXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)  // Another random XMSS for Slave

	addrsTo := []string{bobXMSS.QAddress(), aliceXMSS.QAddress()}
	amounts := []uint64{100, 200}
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(slaveXMSS.PK())

	transferTx := NewTestTransferTransaction(addrsTo, amounts, fee, xmssPK, misc.UCharVectorToBytes(randomXMSS.Address()))
	assert.NotNil(t, transferTx.tx)

	addrFromState := addressstate.GetDefaultAddressState(misc.UCharVectorToBytes(randomXMSS.Address()))
	addrFromState.AddBalance(301)
	addrFromState.AddSlavePKSAccessType(misc.UCharVectorToBytes(slaveXMSS.PK()), 0)  // Adding slave

	addrFromPKState := addressstate.GetDefaultAddressState(misc.UCharVectorToBytes(slaveXMSS.Address()))

	transferTx.tx.Sign(slaveXMSS, misc.BytesToUCharVector(transferTx.tx.GetHashableBytes()))
	assert.True(t, transferTx.tx.ValidateExtended(addrFromState, addrFromPKState))
	addrFromPKState.SetOTSKey(0)  // Marked ots key 0 as used
	// Signed by an used ots key, validation must fail
	assert.False(t, transferTx.tx.ValidateExtended(addrFromState, addrFromPKState))

	randomXMSS.SetOTSIndex(10)
	transferTx.tx.Sign(randomXMSS, misc.BytesToUCharVector(transferTx.tx.GetHashableBytes()))
	assert.True(t, transferTx.tx.ValidateExtended(addrFromState, addrFromPKState))
	addrFromPKState.SetOTSKey(10)  // Marked ots key 10 as used
	// Signed by an used ots key, validation must fail
	assert.False(t, transferTx.tx.ValidateExtended(addrFromState, addrFromPKState))
}

func TestTransferTransaction_ValidateExtended4(t *testing.T) {
	/*
	Test for signing a transfer transaction without slave with an used ots key
	 */
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)

	addrsTo := []string{bobXMSS.QAddress(), aliceXMSS.QAddress()}
	amounts := []uint64{100, 200}
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(randomXMSS.PK())

	transferTx := NewTestTransferTransaction(addrsTo, amounts, fee, xmssPK, nil)
	assert.NotNil(t, transferTx.tx)

	addrFromState := addressstate.GetDefaultAddressState(misc.UCharVectorToBytes(randomXMSS.Address()))
	addrFromState.AddBalance(301)

	transferTx.tx.Sign(randomXMSS, misc.BytesToUCharVector(transferTx.tx.GetHashableBytes()))
	assert.True(t, transferTx.tx.ValidateExtended(addrFromState, addrFromState))
	addrFromState.SetOTSKey(0)  // Marked ots key 0 as used
	// Signed by an used ots key, validation must fail
	assert.False(t, transferTx.tx.ValidateExtended(addrFromState, addrFromState))

	randomXMSS.SetOTSIndex(10)
	transferTx.tx.Sign(randomXMSS, misc.BytesToUCharVector(transferTx.tx.GetHashableBytes()))
	assert.True(t, transferTx.tx.ValidateExtended(addrFromState, addrFromState))
	addrFromState.SetOTSKey(10)  // Marked ots key 10 as used
	// Signed by an used ots key, validation must fail
	assert.False(t, transferTx.tx.ValidateExtended(addrFromState, addrFromState))
}

func TestTransferTransaction_ApplyStateChanges(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)

	addrsTo := []string{bobXMSS.QAddress(), aliceXMSS.QAddress()}
	amounts := []uint64{100, 200}
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(randomXMSS.PK())
	initialBalance := uint64(301)
	aliceInitialBalance := uint64(800)
	bobInitialBalance := uint64(500)

	transferTx := NewTestTransferTransaction(addrsTo, amounts, fee, xmssPK, nil)
	assert.NotNil(t, transferTx.tx)

	transferTx.tx.Sign(randomXMSS, misc.BytesToUCharVector(transferTx.tx.GetHashableBytes()))

	addressesState := make(map[string]*addressstate.AddressState)
	transferTx.tx.SetAffectedAddress(addressesState)

	assert.Len(t, addressesState, 3)

	addressesState[randomXMSS.QAddress()] = addressstate.GetDefaultAddressState(misc.UCharVectorToBytes(randomXMSS.Address()))
	for _, qAddr := range addrsTo {
		addressesState[qAddr] = addressstate.GetDefaultAddressState(misc.Qaddress2Bin(qAddr))
	}

	// Initializing balance
	addressesState[randomXMSS.QAddress()].AddBalance(initialBalance)
	addressesState[aliceXMSS.QAddress()].AddBalance(aliceInitialBalance)
	addressesState[bobXMSS.QAddress()].AddBalance(bobInitialBalance)

	transferTx.tx.ApplyStateChanges(addressesState)
	assert.Equal(t, addressesState[randomXMSS.QAddress()].Balance(), initialBalance - fee - 300)
	assert.Equal(t, addressesState[aliceXMSS.QAddress()].Balance(), aliceInitialBalance + 200)
	assert.Equal(t, addressesState[bobXMSS.QAddress()].Balance(), bobInitialBalance + 100)
}

func TestTransferTransaction_RevertStateChanges(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)

	addrsTo := []string{bobXMSS.QAddress(), aliceXMSS.QAddress()}
	amounts := []uint64{100, 200}
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(randomXMSS.PK())
	initialBalance := uint64(301)
	aliceInitialBalance := uint64(800)
	bobInitialBalance := uint64(500)

	transferTx := NewTestTransferTransaction(addrsTo, amounts, fee, xmssPK, nil)
	assert.NotNil(t, transferTx.tx)

	transferTx.tx.Sign(randomXMSS, misc.BytesToUCharVector(transferTx.tx.GetHashableBytes()))

	addressesState := make(map[string]*addressstate.AddressState)
	transferTx.tx.SetAffectedAddress(addressesState)

	assert.Len(t, addressesState, 3)

	addressesState[randomXMSS.QAddress()] = addressstate.GetDefaultAddressState(misc.UCharVectorToBytes(randomXMSS.Address()))
	for _, qAddr := range addrsTo {
		addressesState[qAddr] = addressstate.GetDefaultAddressState(misc.Qaddress2Bin(qAddr))
	}

	// Initializing balance
	addressesState[randomXMSS.QAddress()].AddBalance(initialBalance)
	addressesState[aliceXMSS.QAddress()].AddBalance(aliceInitialBalance)
	addressesState[bobXMSS.QAddress()].AddBalance(bobInitialBalance)

	transferTx.tx.ApplyStateChanges(addressesState)
	assert.Equal(t, addressesState[randomXMSS.QAddress()].Balance(), initialBalance - fee - 300)
	assert.Equal(t, addressesState[aliceXMSS.QAddress()].Balance(), aliceInitialBalance + 200)
	assert.Equal(t, addressesState[bobXMSS.QAddress()].Balance(), bobInitialBalance + 100)

	transferTx.tx.RevertStateChanges(addressesState)
	assert.Equal(t, addressesState[randomXMSS.QAddress()].Balance(), initialBalance)
	assert.Equal(t, addressesState[aliceXMSS.QAddress()].Balance(), aliceInitialBalance)
	assert.Equal(t, addressesState[bobXMSS.QAddress()].Balance(), bobInitialBalance)
}

func TestTransferTransaction_SetAffectedAddress(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)

	addrsTo := []string{bobXMSS.QAddress(), aliceXMSS.QAddress()}
	amounts := []uint64{100, 200}
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(randomXMSS.PK())

	transferTx := NewTestTransferTransaction(addrsTo, amounts, fee, xmssPK, nil)
	assert.NotNil(t, transferTx.tx)

	addressesState := make(map[string]*addressstate.AddressState)
	transferTx.tx.SetAffectedAddress(addressesState)

	assert.Len(t, addressesState, 3)

	assert.Contains(t, addressesState, aliceXMSS.QAddress())
	assert.Contains(t, addressesState, bobXMSS.QAddress())
	assert.Contains(t, addressesState, randomXMSS.QAddress())
}

func TestTransferTransaction_FromPBData(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)

	addrsTo := []string{bobXMSS.QAddress(), aliceXMSS.QAddress()}
	amounts := []uint64{100, 200}
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(randomXMSS.PK())

	transferTx := NewTestTransferTransaction(addrsTo, amounts, fee, xmssPK, nil)
	assert.NotNil(t, transferTx.tx)

	pbdata := transferTx.tx.PBData()
	tx2 := TransferTransaction{}
	tx2.FromPBdata(*pbdata)

	assert.Equal(t, pbdata, tx2.PBData())

	// Test to ensure, FromPBData doesnt use reference object to initialize tx.data
	transferTx.tx.PBData().Fee = 10
	assert.Equal(t, transferTx.tx.Fee(), uint64(10))
	assert.NotEqual(t, tx2.Fee(), transferTx.tx.Fee())

	// A random Transfer Txn
	tx3 := TransferTransaction{}
	assert.NotEqual(t, pbdata, tx3.PBData())
}
