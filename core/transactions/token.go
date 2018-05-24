package transactions

import (
	"github.com/cyyber/go-qrl/generated"
	"bytes"
	"encoding/binary"
	"github.com/cyyber/go-qrl/misc"
	"github.com/theQRL/qrllib/goqrllib"
	"github.com/cyyber/go-qrl/core"
	"reflect"
)

type TokenTransaction struct {
	Transaction
}

func (tx *TokenTransaction) Symbol() []byte {
	return tx.data.GetToken().Symbol
}

func (tx *TokenTransaction) Name() []byte {
	return tx.data.GetToken().Name
}

func (tx *TokenTransaction) Owner() []byte {
	return tx.data.GetToken().Owner
}

func (tx *TokenTransaction) Decimals() uint64 {
	return tx.data.GetToken().Decimals
}

func (tx *TokenTransaction) InitialBalances() []*generated.AddressAmount {
	return tx.data.GetToken().InitialBalances
}

func (tx *TokenTransaction) GetHashable() goqrllib.UcharVector {
	tmp := new(bytes.Buffer)
	tmp.Write(tx.MasterAddr())
	binary.Write(tmp, binary.BigEndian, tx.Fee())
	tmp.Write(tx.Symbol())
	tmp.Write(tx.Name())
	tmp.Write(tx.Owner())
	binary.Write(tmp, binary.BigEndian, tx.Decimals())

	for _, addrAmount := range tx.InitialBalances() {
		tmp.Write(addrAmount.Address)
		binary.Write(tmp, binary.BigEndian, addrAmount.Amount)
	}

	tmptxhash := misc.UcharVector{}
	tmptxhash.AddBytes(tmp.Bytes())
	tmptxhash.New(goqrllib.Sha2_256(tmptxhash.GetData()))

	return tmptxhash.GetData()
}

func (tx *TokenTransaction) validateCustom() bool {
	if len(tx.Symbol()) > int(tx.config.Dev.Token.MaxSymbolLength) {
		tx.log.Warn("Token Symbol Length exceeds maximum limit")
		tx.log.Warn("Found Symbol Length %s", len(tx.Symbol()))
		tx.log.Warn("Expected Symbol length %s", tx.config.Dev.Token.MaxSymbolLength)
		return false
	}

	if len(tx.Name()) > int(tx.config.Dev.Token.MaxNameLength) {
		tx.log.Warn("Token Name Length exceeds maximum limit")
		tx.log.Warn("Found Name Length %s", len(tx.Symbol()))
		tx.log.Warn("Expected Name length %s", tx.config.Dev.Token.MaxSymbolLength)
		return false
	}

	if len(tx.Symbol()) == 0 {
		tx.log.Warn("Missing Token Symbol")
		return false
	}

	if len(tx.Name()) == 0 {
		tx.log.Warn("Missing Token Name")
		return false
	}

	if len(tx.InitialBalances()) == 0 {
		tx.log.Warn("Invalid Token Transaction, without any initial balance")
		return false
	}

	var sumOfInitialBalances uint64
	for _, addrBalance := range tx.InitialBalances() {
		sumOfInitialBalances += addrBalance.Amount
		if addrBalance.Amount <= 0 {
			tx.log.Warn("Invalid Initial Amount in Token Transaction")
			tx.log.Warn("Address %s | Amount %s", addrBalance.Address, addrBalance.Amount)
			return false
		}
	}

	allowedDecimals := CalcAllowedDecimals(sumOfInitialBalances)

	if tx.Decimals() > allowedDecimals {
		tx.log.Warn("Decimal is greater than maximum allowed decimal")
		tx.log.Warn("Allowed Decimal %s", allowedDecimals)
		tx.log.Warn("Decimals Found %s", tx.Decimals())
		return false
	}

	if tx.Fee() < 0 {
		tx.log.Warn("TokenTransaction [%s] Invalid Fee = %d", string(tx.Txhash()), tx.Fee())
		return false
	}

	return true
}

func (tx *TokenTransaction) validateExtended(addrFromState *core.AddressState, addrFromPkState *core.AddressState) bool {
	if !tx.ValidateSlave(addrFromState, addrFromPkState) {
		return false
	}

	txBalance := addrFromState.Balance()

	if !core.IsValidAddress(tx.AddrFrom()) {
		tx.log.Warn("Invalid address addr_from: %s", tx.AddrFrom())
		return false
	}

	if !core.IsValidAddress(tx.Owner()) {
		tx.log.Warn("Invalid address owner_addr: %s", tx.Owner())
		return false
	}

	for _, addrBalance := range tx.InitialBalances() {
		if !core.IsValidAddress(addrBalance.Address) {
			tx.log.Warn("Invalid address address in initial_balances: %s", addrBalance.Address)
			return false
		}
	}

	if txBalance < tx.Fee() {
		tx.log.Warn("TokenTxn State validation failed for %s because: Insufficient funds", string(tx.Txhash()))
		tx.log.Warn("balance: %s, Fee: %s", txBalance, tx.Fee())
		return false
	}

	if addrFromState.OTSKeyReuse(tx.OtsKey()) {
		tx.log.Warn("TokenTxn State validation failed for %s because: OTS Public key re-use detected",
			string(tx.Txhash()))
		return false
	}

	return true
}

func (tx *TokenTransaction) ApplyStateChanges(addressesState map[string]core.AddressState) {
	addrFromPK := misc.UCharVectorToString(goqrllib.QRLHelperGetAddress(misc.BytesToUCharVector(tx.PK())))
	ownerProcessed := false
	addrFromProcessed := false
	addrFromPKProcessed := false

	for _, addrAmount := range tx.InitialBalances() {
		if reflect.DeepEqual(addrAmount.Address, tx.Owner()) {
			ownerProcessed = true
		}
		if reflect.DeepEqual(addrAmount.Address, tx.AddrFrom()) {
			addrFromProcessed = true
		}
		if reflect.DeepEqual(addrAmount.Address, addrFromPK) {
			addrFromPKProcessed = true
		}
		if addrState, ok := addressesState[string(addrAmount.Address)]; ok {
			addrState.UpdateTokenBalance(tx.Txhash(), addrAmount.Amount)
			addrState.AppendTransactionHash(tx.Txhash())
		}
	}

	if !ownerProcessed {
		if addrState, ok := addressesState[string(tx.Owner())]; ok {
			addrState.AppendTransactionHash(tx.Txhash())
		}
	}

	if addrState, ok := addressesState[string(tx.AddrFrom())]; ok {
		addrState.AddBalance(tx.Fee() * -1)
		if !addrFromProcessed {
			addrState.AppendTransactionHash(tx.Txhash())
		}
	}

	if addrState, ok := addressesState[string(addrFromPK)]; ok {
		if reflect.DeepEqual(tx.AddrFrom(), addrFromPK) {
			if !addrFromPKProcessed {
				addrState.AppendTransactionHash(tx.Txhash())
			}
		}
		addrState.IncreaseNonce()
		addrState.SetOTSKey(uint64(tx.OtsKey()))
	}
}

func (tx *TokenTransaction) RevertStateChanges(addressesState map[string]core.AddressState) {
	addrFromPK := misc.UCharVectorToString(goqrllib.QRLHelperGetAddress(misc.BytesToUCharVector(tx.PK())))
	ownerProcessed := false
	addrFromProcessed := false
	addrFromPKProcessed := false

	for _, addrAmount := range tx.InitialBalances() {
		if reflect.DeepEqual(addrAmount.Address, tx.Owner()) {
			ownerProcessed = true
		}
		if reflect.DeepEqual(addrAmount.Address, tx.AddrFrom()) {
			addrFromProcessed = true
		}
		if reflect.DeepEqual(addrAmount.Address, addrFromPK) {
			addrFromPKProcessed = true
		}
		if addrState, ok := addressesState[string(addrAmount.Address)]; ok {
			addrState.UpdateTokenBalance(tx.Txhash(), addrAmount.Amount * -1)
			addrState.RemoveTransactionHash(tx.Txhash())
		}
	}

	if !ownerProcessed {
		if addrState, ok := addressesState[string(tx.Owner())]; ok {
			addrState.RemoveTransactionHash(tx.Txhash())
		}
	}

	if addrState, ok := addressesState[string(tx.AddrFrom())]; ok {
		addrState.AddBalance(tx.Fee())
		if !addrFromProcessed {
			addrState.RemoveTransactionHash(tx.Txhash())
		}
	}

	if addrState, ok := addressesState[string(addrFromPK)]; ok {
		if reflect.DeepEqual(tx.AddrFrom(), addrFromPK) {
			if !addrFromPKProcessed {
				addrState.RemoveTransactionHash(tx.Txhash())
			}
		}
		addrState.IncreaseNonce()
		addrState.UnsetOTSKey(uint64(tx.OtsKey()))
	}
}

func (tx *TokenTransaction) SetAffectedAddress(addressesState map[string]core.AddressState) {
	addressesState[string(tx.AddrFrom())] = core.AddressState{}
	addressesState[string(tx.PK())] = core.AddressState{}

	for _, addrAmount := range tx.InitialBalances() {
		addressesState[string(addrAmount.Address)] = core.AddressState{}
	}
}

func CreateToken(
	symbol []byte,
	name []byte,
	owner []byte,
	decimals uint64,
	initialBalance []*generated.AddressAmount,
	fee uint64,
	xmssPK []byte,
	masterAddr []byte) *TokenTransaction {
	tx := &TokenTransaction{}

	tx.data.MasterAddr = masterAddr
	tx.data.PublicKey = xmssPK
	tx.data.Fee = fee

	tokenTx := tx.data.GetToken()
	tokenTx.Symbol = symbol
	tokenTx.Name = name
	tokenTx.Owner = owner
	tokenTx.Decimals = decimals
	tokenTx.InitialBalances = initialBalance

	return tx
}
