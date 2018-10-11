package transactions

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/theQRL/go-qrl/pkg/core/addressstate"
	"github.com/theQRL/go-qrl/pkg/misc"
	"github.com/theQRL/go-qrl/test/helper"
)

type TestSlaveTransaction struct {
	tx *SlaveTransaction
}

func NewTestSlaveTransaction(slavePKs [][]byte, accessTypes []uint32, fee uint64, xmssPK []byte, masterAddr []byte) *TestSlaveTransaction {
	tx := CreateSlaveTransaction(slavePKs, accessTypes, fee, xmssPK, masterAddr)

	return &TestSlaveTransaction{tx: tx}
}

func TestCreateSlaveTransaction(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := helper.GetBobXMSS(8)
	slavePKs := [][]byte{misc.UCharVectorToBytes(bobXMSS.PK()), misc.UCharVectorToBytes(randomXMSS.PK())}
	accessTypes := []uint32{0, 0}
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(aliceXMSS.PK())

	tx := CreateSlaveTransaction(slavePKs, accessTypes, fee, xmssPK, nil)

	assert.NotNil(t, tx)
}

func TestSlaveTransaction_AddrFrom(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := helper.GetBobXMSS(8)
	slavePKs := [][]byte{misc.UCharVectorToBytes(bobXMSS.PK()), misc.UCharVectorToBytes(randomXMSS.PK())}
	accessTypes := []uint32{0, 0}
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(aliceXMSS.PK())
	slaveTx := NewTestSlaveTransaction(slavePKs, accessTypes, fee, xmssPK, nil)

	assert.NotNil(t, slaveTx.tx)
	assert.Equal(t, slaveTx.tx.AddrFrom(), misc.UCharVectorToBytes(aliceXMSS.Address()))
}

func TestSlaveTransaction_SlavePKs(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := helper.GetBobXMSS(8)
	slavePKs := [][]byte{misc.UCharVectorToBytes(bobXMSS.PK()), misc.UCharVectorToBytes(randomXMSS.PK())}
	accessTypes := []uint32{0, 0}
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(aliceXMSS.PK())
	slaveTx := NewTestSlaveTransaction(slavePKs, accessTypes, fee, xmssPK, nil)

	assert.NotNil(t, slaveTx.tx)
	assert.Len(t, slaveTx.tx.SlavePKs(), len(slavePKs))
	for _, slavePK := range slavePKs {
		assert.Contains(t, slaveTx.tx.SlavePKs(), slavePK)
	}
}

func TestSlaveTransaction_AccessTypes(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := helper.GetBobXMSS(8)
	slavePKs := [][]byte{misc.UCharVectorToBytes(bobXMSS.PK()), misc.UCharVectorToBytes(randomXMSS.PK())}
	accessTypes := []uint32{0, 0}
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(aliceXMSS.PK())
	slaveTx := NewTestSlaveTransaction(slavePKs, accessTypes, fee, xmssPK, nil)

	assert.NotNil(t, slaveTx.tx)
	assert.Len(t, slaveTx.tx.AccessTypes(), len(slavePKs))
	for _, accessType := range accessTypes {
		assert.Contains(t, slaveTx.tx.AccessTypes(), accessType)
	}
}

func TestSlaveTransaction_GetHashableBytes(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := helper.GetBobXMSS(8)
	slavePKs := [][]byte{misc.UCharVectorToBytes(bobXMSS.PK()), misc.UCharVectorToBytes(randomXMSS.PK())}
	accessTypes := []uint32{0, 0}
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(aliceXMSS.PK())
	slaveTx := NewTestSlaveTransaction(slavePKs, accessTypes, fee, xmssPK, nil)
	hashableBytes := "3458180b6e47adc2f43474a37e16a9ff5800558799255ef9956818f07cf5a579"

	assert.NotNil(t, slaveTx.tx)
	assert.Equal(t, misc.Bin2HStr(slaveTx.tx.GetHashableBytes()), hashableBytes)
}

func TestSlaveTransaction_Validate(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := helper.GetBobXMSS(8)
	slavePKs := [][]byte{misc.UCharVectorToBytes(bobXMSS.PK()), misc.UCharVectorToBytes(randomXMSS.PK())}
	accessTypes := []uint32{0, 0}
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(aliceXMSS.PK())
	slaveTx := NewTestSlaveTransaction(slavePKs, accessTypes, fee, xmssPK, nil)

	assert.NotNil(t, slaveTx.tx)
	assert.True(t, slaveTx.tx.Validate(false))

	// Signed transaction, signature verification should pass
	slaveTx.tx.Sign(aliceXMSS, misc.BytesToUCharVector(slaveTx.tx.GetHashableBytes()))
	assert.True(t, slaveTx.tx.Validate(true))

	// Changed SlavePks, validation must fail
	slaveTx.tx.PBData().GetSlave().SlavePks = [][]byte{slavePKs[0], slavePKs[0]}
	assert.False(t, slaveTx.tx.Validate(true))
}

func TestSlaveTransaction_Validate2(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := helper.GetBobXMSS(8)
	slavePKs := [][]byte{misc.UCharVectorToBytes(bobXMSS.PK()), misc.UCharVectorToBytes(randomXMSS.PK())}
	accessTypes := []uint32{0, 0}
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(aliceXMSS.PK())
	slaveTx := NewTestSlaveTransaction(slavePKs, accessTypes, fee, xmssPK, nil)

	assert.NotNil(t, slaveTx.tx)
	assert.True(t, slaveTx.tx.Validate(false))

	// Signed transaction, signature verification should pass
	slaveTx.tx.Sign(aliceXMSS, misc.BytesToUCharVector(slaveTx.tx.GetHashableBytes()))
	assert.True(t, slaveTx.tx.Validate(true))

	// Changed AccessTypes, validation must fail
	slaveTx.tx.PBData().GetSlave().AccessTypes = []uint32{accessTypes[0], accessTypes[0] + 1}
	assert.False(t, slaveTx.tx.Validate(true))
}

func TestSlaveTransaction_Validate3(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := helper.GetBobXMSS(8)
	slavePKs := [][]byte{misc.UCharVectorToBytes(bobXMSS.PK()), misc.UCharVectorToBytes(randomXMSS.PK())}
	accessTypes := []uint32{0, 0}
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(aliceXMSS.PK())
	slaveTx := NewTestSlaveTransaction(slavePKs, accessTypes, fee, xmssPK, nil)

	assert.NotNil(t, slaveTx.tx)
	slaveTx.tx.Sign(aliceXMSS, misc.BytesToUCharVector(slaveTx.tx.GetHashableBytes()))
	assert.True(t, slaveTx.tx.Validate(true))

	// Random Transaction Hash, validate must fail
	slaveTx.tx.PBData().TransactionHash = []byte{0, 1, 5}
	assert.False(t, slaveTx.tx.Validate(true))
}

func TestSlaveTransaction_ValidateCustom(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := helper.GetBobXMSS(8)
	slavePKs := [][]byte{misc.UCharVectorToBytes(bobXMSS.PK()), misc.UCharVectorToBytes(randomXMSS.PK())}
	accessTypes := []uint32{0, 0, 0}
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(aliceXMSS.PK())
	slaveTx := NewTestSlaveTransaction(slavePKs, accessTypes, fee, xmssPK, nil)

	// Transaction must be nil, mismatch with number of accessTypes and slave PKs
	assert.Nil(t, slaveTx.tx)
}

func TestSlaveTransaction_ValidateCustom2(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)

	bobXMSS := helper.GetBobXMSS(6)
	slavePKs := make([][]byte, 100)
	accessTypes := make([]uint32, 100)
	for i := 0; i < len(slavePKs); i++ {
		slavePKs[i] = misc.UCharVectorToBytes(bobXMSS.PK())
		accessTypes[i] = uint32(0)
	}
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(aliceXMSS.PK())
	slaveTx := NewTestSlaveTransaction(slavePKs, accessTypes, fee, xmssPK, nil)
	// Transaction must not be nil, as the number of slavePKs size is still within limit
	assert.NotNil(t, slaveTx.tx)
}

func TestSlaveTransaction_ValidateCustom3(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)

	bobXMSS := helper.GetBobXMSS(6)
	slavePKs := make([][]byte, 101)
	accessTypes := make([]uint32, 101)
	for i := 0; i < len(slavePKs); i++ {
		slavePKs[i] = misc.UCharVectorToBytes(bobXMSS.PK())
		accessTypes[i] = uint32(0)
	}
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(aliceXMSS.PK())
	slaveTx := NewTestSlaveTransaction(slavePKs, accessTypes, fee, xmssPK, nil)
	// Transaction must be nil, as the number of slavePKs exceeds the limit
	assert.Nil(t, slaveTx.tx)
}

func TestSlaveTransaction_ValidateCustom4(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := helper.GetBobXMSS(8)
	slavePKs := [][]byte{misc.UCharVectorToBytes(bobXMSS.PK()), misc.UCharVectorToBytes(randomXMSS.PK())}
	accessTypes := []uint32{0, 2} // 2 is invalid AccessType
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(aliceXMSS.PK())
	slaveTx := NewTestSlaveTransaction(slavePKs, accessTypes, fee, xmssPK, nil)

	// Transaction must be nil, as one of the accessType is invalid
	assert.Nil(t, slaveTx.tx)
}

func TestSlaveTransaction_ValidateExtended(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := helper.GetBobXMSS(8)
	slavePKs := [][]byte{misc.UCharVectorToBytes(bobXMSS.PK()), misc.UCharVectorToBytes(randomXMSS.PK())}
	accessTypes := []uint32{0, 0}
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(aliceXMSS.PK())

	slaveTx := NewTestSlaveTransaction(slavePKs, accessTypes, fee, xmssPK, nil)
	assert.NotNil(t, slaveTx.tx)

	addrFromState := addressstate.GetDefaultAddressState(misc.UCharVectorToBytes(aliceXMSS.Address()))
	slaveTx.tx.Sign(aliceXMSS, misc.BytesToUCharVector(slaveTx.tx.GetHashableBytes()))

	// Since balance is 0, validation should fail as required fee is 1
	assert.False(t, slaveTx.tx.ValidateExtended(addrFromState, addrFromState))

	// Added balance
	addrFromState.AddBalance(1)

	assert.True(t, slaveTx.tx.ValidateExtended(addrFromState, addrFromState))
}

func TestSlaveTransaction_ValidateExtended2(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := helper.GetBobXMSS(8)
	xmss2 := helper.GetAliceXMSS(8)
	slavePKs := [][]byte{misc.UCharVectorToBytes(bobXMSS.PK()), misc.UCharVectorToBytes(randomXMSS.PK())}
	accessTypes := []uint32{0, 0}
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(xmss2.PK())

	slaveTx := NewTestSlaveTransaction(slavePKs, accessTypes, fee, xmssPK, misc.UCharVectorToBytes(aliceXMSS.Address()))
	assert.NotNil(t, slaveTx.tx)

	addrFromState := addressstate.GetDefaultAddressState(misc.UCharVectorToBytes(aliceXMSS.Address()))
	addrFromState.AddBalance(1)
	addrFromPKState := addressstate.GetDefaultAddressState(misc.UCharVectorToBytes(xmss2.Address()))
	slaveTx.tx.Sign(xmss2, misc.BytesToUCharVector(slaveTx.tx.GetHashableBytes()))

	// Slave is not registered, validation must fail
	assert.False(t, slaveTx.tx.ValidateExtended(addrFromState, addrFromPKState))

	addrFromState.AddSlavePKSAccessType(misc.UCharVectorToBytes(xmss2.PK()), 0)
	assert.True(t, slaveTx.tx.ValidateExtended(addrFromState, addrFromPKState))
}

func TestSlaveTransaction_ValidateExtended3(t *testing.T) {
	/*
		Test for signing a slave transaction via slave with an used ots key
	*/
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := helper.GetBobXMSS(8)
	xmss2 := helper.GetAliceXMSS(8)
	slavePKs := [][]byte{misc.UCharVectorToBytes(bobXMSS.PK()), misc.UCharVectorToBytes(randomXMSS.PK())}
	accessTypes := []uint32{0, 0}
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(xmss2.PK())

	slaveTx := NewTestSlaveTransaction(slavePKs, accessTypes, fee, xmssPK, misc.UCharVectorToBytes(aliceXMSS.Address()))
	assert.NotNil(t, slaveTx.tx)

	addrFromState := addressstate.GetDefaultAddressState(misc.UCharVectorToBytes(aliceXMSS.Address()))
	addrFromState.AddBalance(1)
	addrFromState.AddSlavePKSAccessType(misc.UCharVectorToBytes(xmss2.PK()), 0) // Adding slave

	addrFromPKState := addressstate.GetDefaultAddressState(misc.UCharVectorToBytes(xmss2.Address()))

	slaveTx.tx.Sign(xmss2, misc.BytesToUCharVector(slaveTx.tx.GetHashableBytes()))
	assert.True(t, slaveTx.tx.ValidateExtended(addrFromState, addrFromPKState))
	addrFromPKState.SetOTSKey(0) // Marked ots key 0 as used
	// Signed by an used ots key, validation must fail
	assert.False(t, slaveTx.tx.ValidateExtended(addrFromState, addrFromPKState))

	aliceXMSS.SetOTSIndex(10)
	slaveTx.tx.Sign(aliceXMSS, misc.BytesToUCharVector(slaveTx.tx.GetHashableBytes()))
	assert.True(t, slaveTx.tx.ValidateExtended(addrFromState, addrFromPKState))
	addrFromPKState.SetOTSKey(10) // Marked ots key 10 as used
	// Signed by an used ots key, validation must fail
	assert.False(t, slaveTx.tx.ValidateExtended(addrFromState, addrFromPKState))
}

func TestSlaveTransaction_ValidateExtended4(t *testing.T) {
	/*
		Test for signing a slave transaction without slave with an used ots key
	*/
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := helper.GetBobXMSS(8)
	slavePKs := [][]byte{misc.UCharVectorToBytes(bobXMSS.PK()), misc.UCharVectorToBytes(randomXMSS.PK())}
	accessTypes := []uint32{0, 0}
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(aliceXMSS.PK())

	slaveTx := NewTestSlaveTransaction(slavePKs, accessTypes, fee, xmssPK, nil)
	assert.NotNil(t, slaveTx.tx)

	addrFromState := addressstate.GetDefaultAddressState(misc.UCharVectorToBytes(aliceXMSS.Address()))
	addrFromState.AddBalance(1)

	slaveTx.tx.Sign(aliceXMSS, misc.BytesToUCharVector(slaveTx.tx.GetHashableBytes()))
	assert.True(t, slaveTx.tx.ValidateExtended(addrFromState, addrFromState))
	addrFromState.SetOTSKey(0) // Marked ots key 0 as used
	// Signed by an used ots key, validation must fail
	assert.False(t, slaveTx.tx.ValidateExtended(addrFromState, addrFromState))

	aliceXMSS.SetOTSIndex(10)
	slaveTx.tx.Sign(aliceXMSS, misc.BytesToUCharVector(slaveTx.tx.GetHashableBytes()))
	assert.True(t, slaveTx.tx.ValidateExtended(addrFromState, addrFromState))
	addrFromState.SetOTSKey(10) // Marked ots key 10 as used
	// Signed by an used ots key, validation must fail
	assert.False(t, slaveTx.tx.ValidateExtended(addrFromState, addrFromState))
}

func TestSlaveTransaction_ApplyStateChanges(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := helper.GetBobXMSS(8)
	slavePKs := [][]byte{misc.UCharVectorToBytes(bobXMSS.PK()), misc.UCharVectorToBytes(randomXMSS.PK())}
	accessTypes := []uint32{0, 0}
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(aliceXMSS.PK())

	aliceInitialBalance := uint64(10)

	slaveTx := NewTestSlaveTransaction(slavePKs, accessTypes, fee, xmssPK, nil)
	assert.NotNil(t, slaveTx.tx)
	slaveTx.tx.Sign(aliceXMSS, misc.BytesToUCharVector(slaveTx.tx.GetHashableBytes()))

	addressesState := make(map[string]*addressstate.AddressState)
	slaveTx.tx.SetAffectedAddress(addressesState)

	assert.Len(t, addressesState, 1)

	addressesState[aliceXMSS.QAddress()] = addressstate.GetDefaultAddressState(misc.UCharVectorToBytes(aliceXMSS.Address()))

	// Initializing balance
	addressesState[aliceXMSS.QAddress()].PBData().Balance = aliceInitialBalance

	slaveTx.tx.ApplyStateChanges(addressesState)
	assert.Equal(t, addressesState[aliceXMSS.QAddress()].Balance(), aliceInitialBalance-fee)
}

func TestSlaveTransaction_RevertStateChanges(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := helper.GetBobXMSS(8)
	slavePKs := [][]byte{misc.UCharVectorToBytes(bobXMSS.PK()), misc.UCharVectorToBytes(randomXMSS.PK())}
	accessTypes := []uint32{0, 0}
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(aliceXMSS.PK())

	aliceInitialBalance := uint64(10)

	slaveTx := NewTestSlaveTransaction(slavePKs, accessTypes, fee, xmssPK, nil)
	assert.NotNil(t, slaveTx.tx)
	slaveTx.tx.Sign(aliceXMSS, misc.BytesToUCharVector(slaveTx.tx.GetHashableBytes()))

	addressesState := make(map[string]*addressstate.AddressState)
	slaveTx.tx.SetAffectedAddress(addressesState)

	assert.Len(t, addressesState, 1)

	addressesState[aliceXMSS.QAddress()] = addressstate.GetDefaultAddressState(misc.UCharVectorToBytes(aliceXMSS.Address()))

	// Initializing balance
	addressesState[aliceXMSS.QAddress()].PBData().Balance = aliceInitialBalance

	slaveTx.tx.ApplyStateChanges(addressesState)
	assert.Equal(t, addressesState[aliceXMSS.QAddress()].Balance(), aliceInitialBalance-fee)

	slaveTx.tx.RevertStateChanges(addressesState)
	assert.Equal(t, addressesState[aliceXMSS.QAddress()].Balance(), aliceInitialBalance)
}

func TestSlaveTransaction_SetAffectedAddress(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := helper.GetBobXMSS(8)
	slavePKs := [][]byte{misc.UCharVectorToBytes(bobXMSS.PK()), misc.UCharVectorToBytes(randomXMSS.PK())}
	accessTypes := []uint32{0, 0}
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(aliceXMSS.PK())

	slaveTx := NewTestSlaveTransaction(slavePKs, accessTypes, fee, xmssPK, nil)
	assert.NotNil(t, slaveTx.tx)

	addressesState := make(map[string]*addressstate.AddressState)
	slaveTx.tx.SetAffectedAddress(addressesState)

	assert.Len(t, addressesState, 1)

	assert.Contains(t, addressesState, aliceXMSS.QAddress())
}

func TestSlaveTransaction_FromPBData(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := helper.GetBobXMSS(8)
	slavePKs := [][]byte{misc.UCharVectorToBytes(bobXMSS.PK()), misc.UCharVectorToBytes(randomXMSS.PK())}
	accessTypes := []uint32{0, 0}
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(aliceXMSS.PK())

	slaveTx := NewTestSlaveTransaction(slavePKs, accessTypes, fee, xmssPK, nil)
	assert.NotNil(t, slaveTx.tx)

	pbdata := slaveTx.tx.PBData()
	tx2 := SlaveTransaction{}
	tx2.FromPBdata(*pbdata)

	assert.Equal(t, pbdata, tx2.PBData())

	// Test to ensure, FromPBData doesnt use reference object to initialize tx.data
	slaveTx.tx.PBData().Fee = 10
	assert.Equal(t, slaveTx.tx.Fee(), uint64(10))
	assert.NotEqual(t, tx2.Fee(), slaveTx.tx.Fee())

	// A random Slave Txn
	tx3 := SlaveTransaction{}
	assert.NotEqual(t, pbdata, tx3.PBData())
}
