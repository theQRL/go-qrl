package transactions

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/theQRL/go-qrl/pkg/core/addressstate"
	"github.com/theQRL/go-qrl/pkg/misc"
	"github.com/theQRL/go-qrl/test/helper"
)

type TestMessageTransaction struct {
	tx *MessageTransaction
}


func NewTestMessageTransaction(message string, fee uint64, xmssPK []byte, masterAddr []byte) *TestMessageTransaction {
	tx := CreateMessageTransaction([]byte(message), fee, xmssPK, masterAddr)

	return &TestMessageTransaction{tx: tx}
}

func TestCreateMessageTransaction(t *testing.T) {
	xmss := helper.GetAliceXMSS(6)
	message := "Hello World!!!"
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(xmss.PK())

	tx := CreateMessageTransaction([]byte(message), fee, xmssPK, nil)

	assert.NotNil(t, tx)
}

func TestMessageTransaction_AddrFrom(t *testing.T) {
	xmss := helper.GetAliceXMSS(6)
	message := "Hello World!!!"
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(xmss.PK())
	messageTx := NewTestMessageTransaction(message, fee, xmssPK, nil)

	assert.NotNil(t, messageTx.tx)
	assert.Equal(t, messageTx.tx.AddrFrom(), misc.UCharVectorToBytes(xmss.Address()))
}

func TestMessageTransaction_MessageHash(t *testing.T) {
	xmss := helper.GetAliceXMSS(6)
	message := "Hello World!!!"
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(xmss.PK())
	messageTx := NewTestMessageTransaction(message, fee, xmssPK, nil)

	assert.NotNil(t, messageTx.tx)
	assert.Equal(t, messageTx.tx.MessageHash(), []byte(message))
}

func TestMessageTransaction_GetHashableBytes(t *testing.T) {
	xmss := helper.GetAliceXMSS(6)
	message := "Hello World!!!"
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(xmss.PK())
	messageTx := NewTestMessageTransaction(message, fee, xmssPK, nil)
	hashableBytes := "cd00dc04142a981d9b1a6cf671c76a5f134c1264cf6f9e18048c66a6ba149b16"

	assert.NotNil(t, messageTx.tx)
	assert.Equal(t, misc.Bin2HStr(messageTx.tx.GetHashableBytes()), hashableBytes)
}

func TestMessageTransaction_Validate(t *testing.T) {
	xmss := helper.GetAliceXMSS(6)
	message := "Hello World!!!"
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(xmss.PK())
	messageTx := NewTestMessageTransaction(message, fee, xmssPK, nil)

	assert.NotNil(t, messageTx.tx)
	assert.True(t, messageTx.tx.Validate(false))

	// Signed transaction, signature verification should pass
	messageTx.tx.Sign(xmss, misc.BytesToUCharVector(messageTx.tx.GetHashableBytes()))
	assert.True(t, messageTx.tx.Validate(true))

	// Changed Message Hash to some different address, validation must fail
	messageTx.tx.PBData().GetMessage().MessageHash = []byte("ok")
	assert.False(t, messageTx.tx.Validate(true))
}

func TestMessageTransaction_Validate2(t *testing.T) {
	xmss := helper.GetAliceXMSS(6)
	message := "Hello World!!!"
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(xmss.PK())
	messageTx := NewTestMessageTransaction(message, fee, xmssPK, nil)

	assert.NotNil(t, messageTx.tx)
	messageTx.tx.Sign(xmss, misc.BytesToUCharVector(messageTx.tx.GetHashableBytes()))
	assert.True(t, messageTx.tx.Validate(true))

	// Random Transaction Hash, validate must fail
	messageTx.tx.PBData().TransactionHash = []byte{0, 1, 5}
	assert.False(t, messageTx.tx.Validate(true))
}

func TestMessageTransaction_ValidateCustom(t *testing.T) {
	xmss := helper.GetAliceXMSS(6)
	message := ""
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(xmss.PK())
	messageTx := NewTestMessageTransaction(message, fee, xmssPK, nil)

	// Transaction must be nil, as message length is 0
	assert.Nil(t, messageTx.tx)
}

func TestMessageTransaction_ValidateCustom2(t *testing.T) {
	xmss := helper.GetAliceXMSS(6)
	message := make([]byte, 80)
	for i := 0; i < len(message); i++ {
		message[i] = 0
	}
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(xmss.PK())
	messageTx := NewTestMessageTransaction(string(message), fee, xmssPK, nil)

	// Transaction must not be nil, as the message size is still within limit
	assert.NotNil(t, messageTx.tx)
}

func TestMessageTransaction_ValidateCustom3(t *testing.T) {
	xmss := helper.GetAliceXMSS(6)
	message := make([]byte, 81)
	for i := 0; i < len(message); i++ {
		message[i] = 0
	}
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(xmss.PK())
	messageTx := NewTestMessageTransaction(string(message), fee, xmssPK, nil)

	// Transaction must be nil, as message length is more than the threshold
	assert.Nil(t, messageTx.tx)
}

func TestMessageTransaction_ValidateExtended(t *testing.T) {
	xmss := helper.GetAliceXMSS(6)
	message := "hello"
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(xmss.PK())

	messageTx := NewTestMessageTransaction(message, fee, xmssPK, nil)
	assert.NotNil(t, messageTx.tx)

	addrFromState := addressstate.GetDefaultAddressState(misc.UCharVectorToBytes(xmss.Address()))
	messageTx.tx.Sign(xmss, misc.BytesToUCharVector(messageTx.tx.GetHashableBytes()))

	// Since balance is 0, validation should fail as required fee is 1
	assert.False(t, messageTx.tx.ValidateExtended(addrFromState, addrFromState))

	// Added balance
	addrFromState.AddBalance(1)

	assert.True(t, messageTx.tx.ValidateExtended(addrFromState, addrFromState))

}

func TestMessageTransaction_ValidateExtended2(t *testing.T) {
	xmss := helper.GetAliceXMSS(6)  // Master XMSS
	masterAddress := misc.UCharVectorToBytes(xmss.Address())
	xmss2 := helper.GetBobXMSS(6)  // Slave XMSS
	message := "hello"
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(xmss2.PK())

	messageTx := NewTestMessageTransaction(message, fee, xmssPK, masterAddress)
	assert.NotNil(t, messageTx.tx)

	addrFromState := addressstate.GetDefaultAddressState(misc.UCharVectorToBytes(xmss.Address()))
	addrFromState.AddBalance(1)
	addrFromPKState := addressstate.GetDefaultAddressState(misc.UCharVectorToBytes(xmss2.Address()))
	messageTx.tx.Sign(xmss2, misc.BytesToUCharVector(messageTx.tx.GetHashableBytes()))

	// Slave is not registered, validation must fail
	assert.False(t, messageTx.tx.ValidateExtended(addrFromState, addrFromPKState))

	addrFromState.AddSlavePKSAccessType(misc.UCharVectorToBytes(xmss2.PK()), 0)
	assert.True(t, messageTx.tx.ValidateExtended(addrFromState, addrFromPKState))
}

func TestMessageTransaction_ValidateExtended3(t *testing.T) {
	/*
	Test for signing a message transaction via slave with an used ots key
	 */
	xmss := helper.GetAliceXMSS(6)  // Master XMSS
	masterAddress := misc.UCharVectorToBytes(xmss.Address())
	xmss2 := helper.GetBobXMSS(6)  // Slave XMSS
	message := "hello"
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(xmss2.PK())

	messageTx := NewTestMessageTransaction(message, fee, xmssPK, masterAddress)
	assert.NotNil(t, messageTx.tx)

	addrFromState := addressstate.GetDefaultAddressState(misc.UCharVectorToBytes(xmss.Address()))
	addrFromState.AddBalance(1)
	addrFromState.AddSlavePKSAccessType(misc.UCharVectorToBytes(xmss2.PK()), 0)  // Adding slave

	addrFromPKState := addressstate.GetDefaultAddressState(misc.UCharVectorToBytes(xmss2.Address()))

	messageTx.tx.Sign(xmss2, misc.BytesToUCharVector(messageTx.tx.GetHashableBytes()))
	assert.True(t, messageTx.tx.ValidateExtended(addrFromState, addrFromPKState))
	addrFromPKState.SetOTSKey(0)  // Marked ots key 0 as used
	// Signed by an used ots key, validation must fail
	assert.False(t, messageTx.tx.ValidateExtended(addrFromState, addrFromPKState))

	xmss.SetOTSIndex(10)
	messageTx.tx.Sign(xmss, misc.BytesToUCharVector(messageTx.tx.GetHashableBytes()))
	assert.True(t, messageTx.tx.ValidateExtended(addrFromState, addrFromPKState))
	addrFromPKState.SetOTSKey(10)  // Marked ots key 10 as used
	// Signed by an used ots key, validation must fail
	assert.False(t, messageTx.tx.ValidateExtended(addrFromState, addrFromPKState))
}

func TestMessageTransaction_ValidateExtended4(t *testing.T) {
	/*
	Test for signing a message transaction without slave with an used ots key
	 */
	xmss := helper.GetAliceXMSS(6)  // Master XMSS
	message := "hello"
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(xmss.PK())

	messageTx := NewTestMessageTransaction(message, fee, xmssPK, nil)
	assert.NotNil(t, messageTx.tx)

	addrFromState := addressstate.GetDefaultAddressState(misc.UCharVectorToBytes(xmss.Address()))
	addrFromState.AddBalance(1)

	messageTx.tx.Sign(xmss, misc.BytesToUCharVector(messageTx.tx.GetHashableBytes()))
	assert.True(t, messageTx.tx.ValidateExtended(addrFromState, addrFromState))
	addrFromState.SetOTSKey(0)  // Marked ots key 0 as used
	// Signed by an used ots key, validation must fail
	assert.False(t, messageTx.tx.ValidateExtended(addrFromState, addrFromState))

	xmss.SetOTSIndex(10)
	messageTx.tx.Sign(xmss, misc.BytesToUCharVector(messageTx.tx.GetHashableBytes()))
	assert.True(t, messageTx.tx.ValidateExtended(addrFromState, addrFromState))
	addrFromState.SetOTSKey(10)  // Marked ots key 10 as used
	// Signed by an used ots key, validation must fail
	assert.False(t, messageTx.tx.ValidateExtended(addrFromState, addrFromState))
}

func TestMessageTransaction_ApplyStateChanges(t *testing.T) {
	xmss := helper.GetAliceXMSS(6)  // Master XMSS
	message := "hello"
	fee := uint64(1)
	initialBalance := uint64(10000000)
	xmssPK := misc.UCharVectorToBytes(xmss.PK())

	messageTx := NewTestMessageTransaction(message, fee, xmssPK, nil)
	assert.NotNil(t, messageTx.tx)

	addressesState := make(map[string]*addressstate.AddressState)
	messageTx.tx.SetAffectedAddress(addressesState)

	assert.Len(t, addressesState, 1)

	addressesState[xmss.QAddress()] = addressstate.GetDefaultAddressState(misc.UCharVectorToBytes(xmss.Address()))

	// Initializing balance
	addressesState[xmss.QAddress()].PBData().Balance = initialBalance

	messageTx.tx.ApplyStateChanges(addressesState)
	assert.Equal(t, addressesState[xmss.QAddress()].Balance(), initialBalance - fee)
}

func TestMessageTransaction_RevertStateChanges(t *testing.T) {
	xmss := helper.GetAliceXMSS(6)  // Master XMSS
	message := "hello"
	fee := uint64(1)
	initialBalance := uint64(10000000)
	xmssPK := misc.UCharVectorToBytes(xmss.PK())

	messageTx := NewTestMessageTransaction(message, fee, xmssPK, nil)
	assert.NotNil(t, messageTx.tx)

	addressesState := make(map[string]*addressstate.AddressState)
	messageTx.tx.SetAffectedAddress(addressesState)

	assert.Len(t, addressesState, 1)

	addressesState[xmss.QAddress()] = addressstate.GetDefaultAddressState(misc.UCharVectorToBytes(xmss.Address()))

	// Initializing balance
	addressesState[xmss.QAddress()].PBData().Balance = initialBalance

	messageTx.tx.ApplyStateChanges(addressesState)
	assert.Equal(t, addressesState[xmss.QAddress()].Balance(), initialBalance - fee)

	messageTx.tx.RevertStateChanges(addressesState)
	assert.Equal(t, addressesState[xmss.QAddress()].Balance(), initialBalance)
}

func TestMessageTransaction_SetAffectedAddress(t *testing.T) {
	xmss := helper.GetAliceXMSS(6)  // Master XMSS
	message := "hello"
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(xmss.PK())

	messageTx := NewTestMessageTransaction(message, fee, xmssPK, nil)
	assert.NotNil(t, messageTx.tx)

	addressesState := make(map[string]*addressstate.AddressState)
	messageTx.tx.SetAffectedAddress(addressesState)

	assert.Len(t, addressesState, 1)

	assert.Contains(t, addressesState, xmss.QAddress())
}

func TestMessageTransaction_FromPBData(t *testing.T) {
	xmss := helper.GetAliceXMSS(6)  // Master XMSS
	message := "hello"
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(xmss.PK())

	messageTx := NewTestMessageTransaction(message, fee, xmssPK, nil)
	assert.NotNil(t, messageTx.tx)

	pbdata := messageTx.tx.PBData()
	tx2 := MessageTransaction{}
	tx2.FromPBdata(*pbdata)

	assert.Equal(t, pbdata, tx2.PBData())

	// Test to ensure, FromPBData doesnt use reference object to initialize tx.data
	messageTx.tx.PBData().Fee = 10
	assert.Equal(t, messageTx.tx.Fee(), uint64(10))
	assert.NotEqual(t, tx2.Fee(), messageTx.tx.Fee())

	// A random Message Txn
	tx3 := MessageTransaction{}
	assert.NotEqual(t, pbdata, tx3.PBData())
}
