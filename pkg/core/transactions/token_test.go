package transactions

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/theQRL/go-qrl/pkg/core/addressstate"
	"github.com/theQRL/go-qrl/pkg/crypto"
	"github.com/theQRL/go-qrl/pkg/misc"
	"github.com/theQRL/go-qrl/test/helper"
	"github.com/theQRL/qrllib/goqrllib/goqrllib"
)

type TestTokenTransaction struct {
	tx *TokenTransaction
}


func NewTestTokenTransaction(symbol string, name string, owner string, decimals uint64, initialTokenBalance map[string] uint64, fee uint64, xmssPK []byte, masterAddr []byte) *TestTokenTransaction {

	tx := CreateTokenTransaction(
		[]byte(symbol),
		[]byte(name),
		misc.Qaddress2Bin(owner),
		decimals, initialTokenBalance,
		fee,
		xmssPK,
		masterAddr)

	return &TestTokenTransaction{tx: tx}
}

func TestCalcAllowedDecimals(t *testing.T) {
	decimal, err := CalcAllowedDecimals(uint64(10000000000000000000))
	assert.Nil(t, err)
	assert.Equal(t, decimal, uint64(0))

	decimal, err = CalcAllowedDecimals(uint64(1))
	assert.Nil(t, err)
	assert.Equal(t, decimal, uint64(19))

	decimal, err = CalcAllowedDecimals(uint64(2))
	assert.Nil(t, err)
	assert.Equal(t, decimal, uint64(18))
}

func TestCreateTokenTransaction(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)

	symbol := "QRL"
	name := "Quantum Resistant Ledger"
	owner := aliceXMSS.QAddress()
	decimals := uint64(5)
	initialTokenBalance := make(map[string] uint64)
	initialTokenBalance[bobXMSS.QAddress()] = 10000 * uint64(math.Pow10(int(decimals)))
	initialTokenBalance[aliceXMSS.QAddress()] = 20000 * uint64(math.Pow10(int(decimals)))
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(randomXMSS.PK())

	tx := CreateTokenTransaction(
		[]byte(symbol),
		[]byte(name),
		misc.Qaddress2Bin(owner),
		decimals,
		initialTokenBalance,
		fee,
		xmssPK,
		nil)

	assert.NotNil(t, tx)
}

func TestTokenTransaction_AddrFrom(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)

	symbol := "QRL"
	name := "Quantum Resistant Ledger"
	owner := aliceXMSS.QAddress()
	decimals := uint64(5)
	initialTokenBalance := make(map[string] uint64)
	initialTokenBalance[bobXMSS.QAddress()] = 10000 * uint64(math.Pow10(int(decimals)))
	initialTokenBalance[aliceXMSS.QAddress()] = 20000 * uint64(math.Pow10(int(decimals)))
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(randomXMSS.PK())

	tokenTx := NewTestTokenTransaction(symbol, name, owner, decimals, initialTokenBalance, fee, xmssPK, nil)
	assert.NotNil(t, tokenTx.tx)

	assert.Equal(t, tokenTx.tx.AddrFrom(), misc.UCharVectorToBytes(randomXMSS.Address()))
}

func TestTokenTransaction_InitialBalances(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)

	symbol := "QRL"
	name := "Quantum Resistant Ledger"
	owner := aliceXMSS.QAddress()
	decimals := uint64(5)
	initialTokenBalance := make(map[string] uint64)
	initialTokenBalance[bobXMSS.QAddress()] = 10000 * uint64(math.Pow10(int(decimals)))
	initialTokenBalance[aliceXMSS.QAddress()] = 20000 * uint64(math.Pow10(int(decimals)))
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(randomXMSS.PK())

	tokenTx := NewTestTokenTransaction(symbol, name, owner, decimals, initialTokenBalance, fee, xmssPK, nil)
	assert.NotNil(t, tokenTx.tx)

	assert.Len(t, tokenTx.tx.InitialBalances(), len(initialTokenBalance))

	for i := 0; i < len(tokenTx.tx.InitialBalances()); i++ {
		qAddress := misc.Bin2Qaddress(tokenTx.tx.InitialBalances()[i].Address)
		assert.Contains(t, initialTokenBalance, qAddress)
		assert.Equal(t, initialTokenBalance[qAddress], tokenTx.tx.InitialBalances()[i].Amount)
	}
}

func TestTokenTransaction_Decimals(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)

	symbol := "QRL"
	name := "Quantum Resistant Ledger"
	owner := aliceXMSS.QAddress()
	decimals := uint64(5)
	initialTokenBalance := make(map[string] uint64)
	initialTokenBalance[bobXMSS.QAddress()] = 10000 * uint64(math.Pow10(int(decimals)))
	initialTokenBalance[aliceXMSS.QAddress()] = 20000 * uint64(math.Pow10(int(decimals)))
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(randomXMSS.PK())

	tokenTx := NewTestTokenTransaction(symbol, name, owner, decimals, initialTokenBalance, fee, xmssPK, nil)
	assert.NotNil(t, tokenTx.tx)
	assert.Equal(t, tokenTx.tx.Decimals(), decimals)
}

func TestTokenTransaction_Owner(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)

	symbol := "QRL"
	name := "Quantum Resistant Ledger"
	owner := aliceXMSS.QAddress()
	decimals := uint64(5)
	initialTokenBalance := make(map[string] uint64)
	initialTokenBalance[bobXMSS.QAddress()] = 10000 * uint64(math.Pow10(int(decimals)))
	initialTokenBalance[aliceXMSS.QAddress()] = 20000 * uint64(math.Pow10(int(decimals)))
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(randomXMSS.PK())

	tokenTx := NewTestTokenTransaction(symbol, name, owner, decimals, initialTokenBalance, fee, xmssPK, nil)
	assert.NotNil(t, tokenTx.tx)
	assert.Equal(t, tokenTx.tx.Owner(), misc.UCharVectorToBytes(aliceXMSS.Address()))
}

func TestTokenTransaction_Name(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)

	symbol := "QRL"
	name := "Quantum Resistant Ledger"
	owner := aliceXMSS.QAddress()
	decimals := uint64(5)
	initialTokenBalance := make(map[string] uint64)
	initialTokenBalance[bobXMSS.QAddress()] = 10000 * uint64(math.Pow10(int(decimals)))
	initialTokenBalance[aliceXMSS.QAddress()] = 20000 * uint64(math.Pow10(int(decimals)))
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(randomXMSS.PK())

	tokenTx := NewTestTokenTransaction(symbol, name, owner, decimals, initialTokenBalance, fee, xmssPK, nil)
	assert.NotNil(t, tokenTx.tx)
	assert.Equal(t, tokenTx.tx.Name(), []byte(name))
}

func TestTokenTransaction_Symbol(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)

	symbol := "QRL"
	name := "Quantum Resistant Ledger"
	owner := aliceXMSS.QAddress()
	decimals := uint64(5)
	initialTokenBalance := make(map[string] uint64)
	initialTokenBalance[bobXMSS.QAddress()] = 10000 * uint64(math.Pow10(int(decimals)))
	initialTokenBalance[aliceXMSS.QAddress()] = 20000 * uint64(math.Pow10(int(decimals)))
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(randomXMSS.PK())

	tokenTx := NewTestTokenTransaction(symbol, name, owner, decimals, initialTokenBalance, fee, xmssPK, nil)
	assert.NotNil(t, tokenTx.tx)
	assert.Equal(t, tokenTx.tx.Symbol(), []byte(symbol))
}

func TestTokenTransaction_GetHashableBytes(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)

	symbol := "QRL"
	name := "Quantum Resistant Ledger"
	owner := aliceXMSS.QAddress()
	decimals := uint64(5)
	initialTokenBalance := make(map[string] uint64)
	initialTokenBalance[bobXMSS.QAddress()] = 10000 * uint64(math.Pow10(int(decimals)))
	initialTokenBalance[aliceXMSS.QAddress()] = 20000 * uint64(math.Pow10(int(decimals)))
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(randomXMSS.PK())
	hashableBytes := "6cdab7800747d44eba0656de701c9bb0251538deb20447305251296f9b2feca5"

	tokenTx := NewTestTokenTransaction(symbol, name, owner, decimals, initialTokenBalance, fee, xmssPK, nil)
	assert.NotNil(t, tokenTx.tx)
	assert.Equal(t, misc.Bin2HStr(tokenTx.tx.GetHashableBytes()), hashableBytes)
}

func TestTokenTransaction_Validate(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)

	symbol := "QRL"
	name := "Quantum Resistant Ledger"
	owner := aliceXMSS.QAddress()
	decimals := uint64(5)
	initialTokenBalance := make(map[string] uint64)
	initialTokenBalance[bobXMSS.QAddress()] = 10000 * uint64(math.Pow10(int(decimals)))
	initialTokenBalance[aliceXMSS.QAddress()] = 20000 * uint64(math.Pow10(int(decimals)))
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(randomXMSS.PK())

	tokenTx := NewTestTokenTransaction(symbol, name, owner, decimals, initialTokenBalance, fee, xmssPK, nil)
	assert.NotNil(t, tokenTx.tx)
	assert.True(t, tokenTx.tx.Validate(false))

	// Signed transaction, signature verification should pass
	tokenTx.tx.Sign(randomXMSS, misc.BytesToUCharVector(tokenTx.tx.GetHashableBytes()))
	assert.True(t, tokenTx.tx.Validate(true))

	// Changed Symbol, validation must fail
	tokenTx.tx.PBData().GetToken().Symbol = []byte("TEST")
	assert.False(t, tokenTx.tx.Validate(true))

	// Revert back to original, validation must pass
	tokenTx.tx.PBData().GetToken().Symbol = []byte(symbol)
	assert.True(t, tokenTx.tx.Validate(true))

	// Changed Token Name, validation must fail
	tokenTx.tx.PBData().GetToken().Name = []byte("TEST")
	assert.False(t, tokenTx.tx.Validate(true))

	// Revert back to original, validation must pass
	tokenTx.tx.PBData().GetToken().Name = []byte(name)
	assert.True(t, tokenTx.tx.Validate(true))

	// Changed Token Decimals, validation must fail
	tokenTx.tx.PBData().GetToken().Decimals = uint64(7)
	assert.False(t, tokenTx.tx.Validate(true))

	// Revert back to original, validation must pass
	tokenTx.tx.PBData().GetToken().Decimals = decimals
	assert.True(t, tokenTx.tx.Validate(true))

	// Changed an address in Initial Balance, validation must fail
	tokenTx.tx.PBData().GetToken().InitialBalances[0].Address = misc.UCharVectorToBytes(crypto.FromHeight(6, goqrllib.SHAKE_128).Address())
	assert.False(t, tokenTx.tx.Validate(true))

	// Revert back to original, validation must pass
	tokenTx.tx.PBData().GetToken().InitialBalances[0].Address = misc.UCharVectorToBytes(bobXMSS.Address())
	assert.True(t, tokenTx.tx.Validate(true))

	// Changed an amount in Initial Balance, validation must fail
	tokenTx.tx.PBData().GetToken().InitialBalances[0].Amount = uint64(2090)
	assert.False(t, tokenTx.tx.Validate(true))

	// Revert back to original, validation must pass
	tokenTx.tx.PBData().GetToken().InitialBalances[0].Amount = initialTokenBalance[bobXMSS.QAddress()]
	assert.True(t, tokenTx.tx.Validate(true))
}

func TestTokenTransaction_ValidateCustom(t *testing.T) {
	/*
	Test for missing Token Symbol
	 */
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)

	symbol := ""
	name := "Quantum Resistant Ledger"
	owner := aliceXMSS.QAddress()
	decimals := uint64(5)
	initialTokenBalance := make(map[string] uint64)
	initialTokenBalance[bobXMSS.QAddress()] = 10000 * uint64(math.Pow10(int(decimals)))
	initialTokenBalance[aliceXMSS.QAddress()] = 20000 * uint64(math.Pow10(int(decimals)))
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(randomXMSS.PK())

	tokenTx := NewTestTokenTransaction(symbol, name, owner, decimals, initialTokenBalance, fee, xmssPK, nil)
	assert.Nil(t, tokenTx.tx)
}


func TestTokenTransaction_ValidateCustom2(t *testing.T) {
	/*
	Test for missing Token Name
	 */
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)

	symbol := "QRL"
	name := ""
	owner := aliceXMSS.QAddress()
	decimals := uint64(5)
	initialTokenBalance := make(map[string] uint64)
	initialTokenBalance[bobXMSS.QAddress()] = 10000 * uint64(math.Pow10(int(decimals)))
	initialTokenBalance[aliceXMSS.QAddress()] = 20000 * uint64(math.Pow10(int(decimals)))
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(randomXMSS.PK())

	tokenTx := NewTestTokenTransaction(symbol, name, owner, decimals, initialTokenBalance, fee, xmssPK, nil)
	assert.Nil(t, tokenTx.tx)
}

func TestTokenTransaction_ValidateCustom3(t *testing.T) {
	/*
	Test for missing Decimals more than limit
	 */
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)

	symbol := "QRL"
	name := "Quantum Resistant Ledger"
	owner := aliceXMSS.QAddress()
	decimals := uint64(15)
	initialTokenBalance := make(map[string] uint64)
	initialTokenBalance[bobXMSS.QAddress()] = 10000 * uint64(math.Pow10(int(decimals)))
	initialTokenBalance[aliceXMSS.QAddress()] = 20000 * uint64(math.Pow10(int(decimals)))
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(randomXMSS.PK())

	tokenTx := NewTestTokenTransaction(symbol, name, owner, decimals, initialTokenBalance, fee, xmssPK, nil)
	assert.Nil(t, tokenTx.tx)
}

func TestTokenTransaction_ValidateCustom4(t *testing.T) {
	/*
	Test for Symbol more than limit
	 */
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)

	symbol := "QRLQRLQRLQR"
	name := "Quantum Resistant Ledger"
	owner := aliceXMSS.QAddress()
	decimals := uint64(5)
	initialTokenBalance := make(map[string] uint64)
	initialTokenBalance[bobXMSS.QAddress()] = 10000 * uint64(math.Pow10(int(decimals)))
	initialTokenBalance[aliceXMSS.QAddress()] = 20000 * uint64(math.Pow10(int(decimals)))
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(randomXMSS.PK())

	tokenTx := NewTestTokenTransaction(symbol, name, owner, decimals, initialTokenBalance, fee, xmssPK, nil)
	assert.Nil(t, tokenTx.tx)
}

func TestTokenTransaction_ValidateCustom5(t *testing.T) {
	/*
	Test for missing Decimals more than limit
	 */
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)

	symbol := "QRL"
	name := "Quantum Resistant LedgerQuantum"
	owner := aliceXMSS.QAddress()
	decimals := uint64(5)
	initialTokenBalance := make(map[string] uint64)
	initialTokenBalance[bobXMSS.QAddress()] = 10000 * uint64(math.Pow10(int(decimals)))
	initialTokenBalance[aliceXMSS.QAddress()] = 20000 * uint64(math.Pow10(int(decimals)))
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(randomXMSS.PK())

	tokenTx := NewTestTokenTransaction(symbol, name, owner, decimals, initialTokenBalance, fee, xmssPK, nil)
	assert.Nil(t, tokenTx.tx)
}

func TestTokenTransaction_ValidateCustom6(t *testing.T) {
	/*
	Test for one of the initial balance as 0
	 */
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)

	symbol := "QRL"
	name := "Quantum Resistant Ledger"
	owner := aliceXMSS.QAddress()
	decimals := uint64(5)
	initialTokenBalance := make(map[string] uint64)
	initialTokenBalance[bobXMSS.QAddress()] = 10000 * uint64(math.Pow10(int(decimals)))
	initialTokenBalance[aliceXMSS.QAddress()] = 0 * uint64(math.Pow10(int(decimals)))
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(randomXMSS.PK())

	tokenTx := NewTestTokenTransaction(symbol, name, owner, decimals, initialTokenBalance, fee, xmssPK, nil)
	assert.Nil(t, tokenTx.tx)
}

func TestTokenTransaction_ValidateCustom7(t *testing.T) {
	/*
	Test for an invalid QRL Address in initialTokenBalance
	 */
	aliceXMSS := helper.GetAliceXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)

	symbol := "QRL"
	name := "Quantum Resistant Ledger"
	owner := aliceXMSS.QAddress()
	decimals := uint64(5)
	initialTokenBalance := make(map[string] uint64)
	initialTokenBalance["Q01020304"] = 10000 * uint64(math.Pow10(int(decimals)))
	initialTokenBalance[aliceXMSS.QAddress()] = 0 * uint64(math.Pow10(int(decimals)))
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(randomXMSS.PK())

	tokenTx := NewTestTokenTransaction(symbol, name, owner, decimals, initialTokenBalance, fee, xmssPK, nil)
	assert.Nil(t, tokenTx.tx)
}

func TestTokenTransaction_ValidateCustom8(t *testing.T) {
	/*
	Test for an invalid Owner Address
	 */
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)

	symbol := "QRL"
	name := "Quantum Resistant Ledger"
	owner := "Q0101010203"
	decimals := uint64(5)
	initialTokenBalance := make(map[string] uint64)
	initialTokenBalance[bobXMSS.QAddress()] = 10000 * uint64(math.Pow10(int(decimals)))
	initialTokenBalance[aliceXMSS.QAddress()] = 0 * uint64(math.Pow10(int(decimals)))
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(randomXMSS.PK())

	tokenTx := NewTestTokenTransaction(symbol, name, owner, decimals, initialTokenBalance, fee, xmssPK, nil)
	assert.Nil(t, tokenTx.tx)
}

func TestTokenTransaction_ValidateExtended(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)

	symbol := "QRL"
	name := "Quantum Resistant Ledger"
	owner := aliceXMSS.QAddress()
	decimals := uint64(5)
	initialTokenBalance := make(map[string] uint64)
	initialTokenBalance[bobXMSS.QAddress()] = 10000 * uint64(math.Pow10(int(decimals)))
	initialTokenBalance[aliceXMSS.QAddress()] = 20000 * uint64(math.Pow10(int(decimals)))
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(randomXMSS.PK())

	tokenTx := NewTestTokenTransaction(symbol, name, owner, decimals, initialTokenBalance, fee, xmssPK, nil)
	assert.NotNil(t, tokenTx.tx)

	addrFromState := addressstate.GetDefaultAddressState(misc.UCharVectorToBytes(randomXMSS.Address()))
	tokenTx.tx.Sign(randomXMSS, misc.BytesToUCharVector(tokenTx.tx.GetHashableBytes()))

	// Balance is 0, validation should fail as required fee is 1
	assert.False(t, tokenTx.tx.ValidateExtended(addrFromState, addrFromState))

	// Added balance
	addrFromState.AddBalance(1)

	assert.True(t, tokenTx.tx.ValidateExtended(addrFromState, addrFromState))
}

func TestTokenTransaction_ValidateExtended2(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)
	slaveXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)

	symbol := "QRL"
	name := "Quantum Resistant Ledger"
	owner := aliceXMSS.QAddress()
	decimals := uint64(5)
	initialTokenBalance := make(map[string] uint64)
	initialTokenBalance[bobXMSS.QAddress()] = 10000 * uint64(math.Pow10(int(decimals)))
	initialTokenBalance[aliceXMSS.QAddress()] = 20000 * uint64(math.Pow10(int(decimals)))
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(slaveXMSS.PK())

	tokenTx := NewTestTokenTransaction(symbol, name, owner, decimals, initialTokenBalance, fee, xmssPK, misc.UCharVectorToBytes(randomXMSS.Address()))
	assert.NotNil(t, tokenTx.tx)

	addrFromState := addressstate.GetDefaultAddressState(misc.UCharVectorToBytes(randomXMSS.Address()))
	addrFromState.AddBalance(1)
	addrFromPKState := addressstate.GetDefaultAddressState(misc.UCharVectorToBytes(slaveXMSS.Address()))
	tokenTx.tx.Sign(slaveXMSS, misc.BytesToUCharVector(tokenTx.tx.GetHashableBytes()))

	// Slave is not registered, validation must fail
	assert.False(t, tokenTx.tx.ValidateExtended(addrFromState, addrFromPKState))

	addrFromState.AddSlavePKSAccessType(misc.UCharVectorToBytes(slaveXMSS.PK()), 0)
	assert.True(t, tokenTx.tx.ValidateExtended(addrFromState, addrFromPKState))
}

func TestTokenTransaction_ValidateExtended3(t *testing.T) {
	/*
	Test for signing a token transaction via slave with an used ots key
	 */
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)
	slaveXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)

	symbol := "QRL"
	name := "Quantum Resistant Ledger"
	owner := aliceXMSS.QAddress()
	decimals := uint64(5)
	initialTokenBalance := make(map[string] uint64)
	initialTokenBalance[bobXMSS.QAddress()] = 10000 * uint64(math.Pow10(int(decimals)))
	initialTokenBalance[aliceXMSS.QAddress()] = 20000 * uint64(math.Pow10(int(decimals)))
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(slaveXMSS.PK())

	tokenTx := NewTestTokenTransaction(symbol, name, owner, decimals, initialTokenBalance, fee, xmssPK, misc.UCharVectorToBytes(randomXMSS.Address()))
	assert.NotNil(t, tokenTx.tx)

	addrFromState := addressstate.GetDefaultAddressState(misc.UCharVectorToBytes(randomXMSS.Address()))
	addrFromState.AddBalance(1)
	addrFromState.AddSlavePKSAccessType(misc.UCharVectorToBytes(slaveXMSS.PK()), 0)  // Adding slave

	addrFromPKState := addressstate.GetDefaultAddressState(misc.UCharVectorToBytes(slaveXMSS.Address()))

	tokenTx.tx.Sign(slaveXMSS, misc.BytesToUCharVector(tokenTx.tx.GetHashableBytes()))
	assert.True(t, tokenTx.tx.ValidateExtended(addrFromState, addrFromPKState))
	addrFromPKState.SetOTSKey(0)  // Marked ots key 0 as used
	// Signed by an used ots key, validation must fail
	assert.False(t, tokenTx.tx.ValidateExtended(addrFromState, addrFromPKState))

	randomXMSS.SetOTSIndex(10)
	tokenTx.tx.Sign(randomXMSS, misc.BytesToUCharVector(tokenTx.tx.GetHashableBytes()))
	assert.True(t, tokenTx.tx.ValidateExtended(addrFromState, addrFromPKState))
	addrFromPKState.SetOTSKey(10)  // Marked ots key 10 as used
	// Signed by an used ots key, validation must fail
	assert.False(t, tokenTx.tx.ValidateExtended(addrFromState, addrFromPKState))
}

func TestTokenTransaction_ValidateExtended4(t *testing.T) {
	/*
	Test for signing a token transaction without slave with an used ots key
	 */
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)

	symbol := "QRL"
	name := "Quantum Resistant Ledger"
	owner := aliceXMSS.QAddress()
	decimals := uint64(5)
	initialTokenBalance := make(map[string] uint64)
	initialTokenBalance[bobXMSS.QAddress()] = 10000 * uint64(math.Pow10(int(decimals)))
	initialTokenBalance[aliceXMSS.QAddress()] = 20000 * uint64(math.Pow10(int(decimals)))
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(randomXMSS.PK())

	tokenTx := NewTestTokenTransaction(symbol, name, owner, decimals, initialTokenBalance, fee, xmssPK, nil)
	assert.NotNil(t, tokenTx.tx)

	addrFromState := addressstate.GetDefaultAddressState(misc.UCharVectorToBytes(randomXMSS.Address()))
	addrFromState.AddBalance(1)

	tokenTx.tx.Sign(randomXMSS, misc.BytesToUCharVector(tokenTx.tx.GetHashableBytes()))
	assert.True(t, tokenTx.tx.ValidateExtended(addrFromState, addrFromState))
	addrFromState.SetOTSKey(0)  // Marked ots key 0 as used
	// Signed by an used ots key, validation must fail
	assert.False(t, tokenTx.tx.ValidateExtended(addrFromState, addrFromState))

	randomXMSS.SetOTSIndex(10)
	tokenTx.tx.Sign(randomXMSS, misc.BytesToUCharVector(tokenTx.tx.GetHashableBytes()))
	assert.True(t, tokenTx.tx.ValidateExtended(addrFromState, addrFromState))
	addrFromState.SetOTSKey(10)  // Marked ots key 10 as used
	// Signed by an used ots key, validation must fail
	assert.False(t, tokenTx.tx.ValidateExtended(addrFromState, addrFromState))
}

func TestTokenTransaction_ApplyStateChanges(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)

	symbol := "QRL"
	name := "Quantum Resistant Ledger"
	owner := aliceXMSS.QAddress()
	decimals := uint64(5)
	initialBalance := uint64(500)
	aliceInitialBalance := uint64(100)
	bobInitialBalance := uint64(200)
	initialTokenBalance := make(map[string] uint64)
	initialTokenBalance[bobXMSS.QAddress()] = 10000 * uint64(math.Pow10(int(decimals)))
	initialTokenBalance[aliceXMSS.QAddress()] = 20000 * uint64(math.Pow10(int(decimals)))
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(randomXMSS.PK())

	tokenTx := NewTestTokenTransaction(symbol, name, owner, decimals, initialTokenBalance, fee, xmssPK, nil)
	assert.NotNil(t, tokenTx.tx)

	tokenTx.tx.Sign(randomXMSS, misc.BytesToUCharVector(tokenTx.tx.GetHashableBytes()))
	tokenTxHash := tokenTx.tx.Txhash()

	addressesState := make(map[string]*addressstate.AddressState)
	tokenTx.tx.SetAffectedAddress(addressesState)

	assert.Len(t, addressesState, 3)

	addressesState[randomXMSS.QAddress()] = addressstate.GetDefaultAddressState(misc.UCharVectorToBytes(randomXMSS.Address()))
	for qAddr := range initialTokenBalance {
		addressesState[qAddr] = addressstate.GetDefaultAddressState(misc.Qaddress2Bin(qAddr))
	}

	// Initializing balance
	addressesState[randomXMSS.QAddress()].AddBalance(initialBalance)
	addressesState[aliceXMSS.QAddress()].AddBalance(aliceInitialBalance)
	addressesState[bobXMSS.QAddress()].AddBalance(bobInitialBalance)

	tokenTx.tx.ApplyStateChanges(addressesState)
	assert.Equal(t, addressesState[randomXMSS.QAddress()].Balance(), initialBalance - fee)
	assert.Equal(t, addressesState[aliceXMSS.QAddress()].Balance(), aliceInitialBalance)
	assert.Equal(t, addressesState[bobXMSS.QAddress()].Balance(), bobInitialBalance)
	assert.Equal(t, addressesState[randomXMSS.QAddress()].GetTokenBalance(tokenTxHash), uint64(0))
	assert.Equal(t, addressesState[aliceXMSS.QAddress()].GetTokenBalance(tokenTxHash), initialTokenBalance[aliceXMSS.QAddress()])
	assert.Equal(t, addressesState[bobXMSS.QAddress()].GetTokenBalance(tokenTxHash), initialTokenBalance[bobXMSS.QAddress()])
}

func TestTokenTransaction_RevertStateChanges(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)

	symbol := "QRL"
	name := "Quantum Resistant Ledger"
	owner := aliceXMSS.QAddress()
	decimals := uint64(5)
	initialBalance := uint64(500)
	aliceInitialBalance := uint64(100)
	bobInitialBalance := uint64(200)
	initialTokenBalance := make(map[string]uint64)
	initialTokenBalance[bobXMSS.QAddress()] = 10000 * uint64(math.Pow10(int(decimals)))
	initialTokenBalance[aliceXMSS.QAddress()] = 20000 * uint64(math.Pow10(int(decimals)))
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(randomXMSS.PK())

	tokenTx := NewTestTokenTransaction(symbol, name, owner, decimals, initialTokenBalance, fee, xmssPK, nil)
	assert.NotNil(t, tokenTx.tx)

	tokenTx.tx.Sign(randomXMSS, misc.BytesToUCharVector(tokenTx.tx.GetHashableBytes()))
	tokenTxHash := tokenTx.tx.Txhash()

	addressesState := make(map[string]*addressstate.AddressState)
	tokenTx.tx.SetAffectedAddress(addressesState)

	assert.Len(t, addressesState, 3)

	addressesState[randomXMSS.QAddress()] = addressstate.GetDefaultAddressState(misc.UCharVectorToBytes(randomXMSS.Address()))
	for qAddr := range initialTokenBalance {
		addressesState[qAddr] = addressstate.GetDefaultAddressState(misc.Qaddress2Bin(qAddr))
	}

	// Initializing balance
	addressesState[randomXMSS.QAddress()].AddBalance(initialBalance)
	addressesState[aliceXMSS.QAddress()].AddBalance(aliceInitialBalance)
	addressesState[bobXMSS.QAddress()].AddBalance(bobInitialBalance)

	tokenTx.tx.ApplyStateChanges(addressesState)
	assert.Equal(t, addressesState[randomXMSS.QAddress()].Balance(), initialBalance-fee)
	assert.Equal(t, addressesState[aliceXMSS.QAddress()].Balance(), aliceInitialBalance)
	assert.Equal(t, addressesState[bobXMSS.QAddress()].Balance(), bobInitialBalance)
	assert.Equal(t, addressesState[randomXMSS.QAddress()].GetTokenBalance(tokenTxHash), uint64(0))
	assert.Equal(t, addressesState[aliceXMSS.QAddress()].GetTokenBalance(tokenTxHash), initialTokenBalance[aliceXMSS.QAddress()])
	assert.Equal(t, addressesState[bobXMSS.QAddress()].GetTokenBalance(tokenTxHash), initialTokenBalance[bobXMSS.QAddress()])

	tokenTx.tx.RevertStateChanges(addressesState)
	assert.Equal(t, addressesState[randomXMSS.QAddress()].Balance(), initialBalance)
	assert.Equal(t, addressesState[aliceXMSS.QAddress()].Balance(), aliceInitialBalance)
	assert.Equal(t, addressesState[bobXMSS.QAddress()].Balance(), bobInitialBalance)
	assert.Equal(t, addressesState[randomXMSS.QAddress()].GetTokenBalance(tokenTxHash), uint64(0))
	assert.Equal(t, addressesState[aliceXMSS.QAddress()].GetTokenBalance(tokenTxHash), uint64(0))
	assert.Equal(t, addressesState[bobXMSS.QAddress()].GetTokenBalance(tokenTxHash), uint64(0))
}

func TestTokenTransaction_SetAffectedAddress(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)

	symbol := "QRL"
	name := "Quantum Resistant Ledger"
	owner := aliceXMSS.QAddress()
	decimals := uint64(5)
	initialTokenBalance := make(map[string] uint64)
	initialTokenBalance[bobXMSS.QAddress()] = 10000 * uint64(math.Pow10(int(decimals)))
	initialTokenBalance[aliceXMSS.QAddress()] = 20000 * uint64(math.Pow10(int(decimals)))
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(randomXMSS.PK())

	tokenTx := NewTestTokenTransaction(symbol, name, owner, decimals, initialTokenBalance, fee, xmssPK, nil)
	assert.NotNil(t, tokenTx.tx)

	addressesState := make(map[string]*addressstate.AddressState)
	tokenTx.tx.SetAffectedAddress(addressesState)

	assert.Len(t, addressesState, 3)

	assert.Contains(t, addressesState, aliceXMSS.QAddress())
	assert.Contains(t, addressesState, bobXMSS.QAddress())
	assert.Contains(t, addressesState, randomXMSS.QAddress())
}

func TestTokenTransaction_FromPBData(t *testing.T) {
	aliceXMSS := helper.GetAliceXMSS(6)
	bobXMSS := helper.GetBobXMSS(6)
	randomXMSS := crypto.FromHeight(6, goqrllib.SHAKE_128)

	symbol := "QRL"
	name := "Quantum Resistant Ledger"
	owner := aliceXMSS.QAddress()
	decimals := uint64(5)
	initialTokenBalance := make(map[string] uint64)
	initialTokenBalance[bobXMSS.QAddress()] = 10000 * uint64(math.Pow10(int(decimals)))
	initialTokenBalance[aliceXMSS.QAddress()] = 20000 * uint64(math.Pow10(int(decimals)))
	fee := uint64(1)
	xmssPK := misc.UCharVectorToBytes(randomXMSS.PK())

	tokenTx := NewTestTokenTransaction(symbol, name, owner, decimals, initialTokenBalance, fee, xmssPK, nil)
	assert.NotNil(t, tokenTx.tx)

	pbdata := tokenTx.tx.PBData()
	tx2 := TokenTransaction{}
	tx2.FromPBdata(*pbdata)

	assert.Equal(t, pbdata, tx2.PBData())

	// Test to ensure, FromPBData doesnt use reference object to initialize tx.data
	tokenTx.tx.PBData().Fee = 10
	assert.Equal(t, tokenTx.tx.Fee(), uint64(10))
	assert.NotEqual(t, tx2.Fee(), tokenTx.tx.Fee())

	// A random Token Txn
	tx3 := TokenTransaction{}
	assert.NotEqual(t, pbdata, tx3.PBData())
}
