package transactions

import (
	"bytes"
	"encoding/binary"
	"math"
	"reflect"

	"github.com/theQRL/go-qrl/pkg/core/addressstate"
	"github.com/theQRL/go-qrl/pkg/generated"
	"github.com/theQRL/go-qrl/pkg/misc"
	"github.com/theQRL/qrllib/goqrllib/goqrllib"
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

func (tx *TokenTransaction) GetHashableBytes() []byte {
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

	return tmptxhash.GetBytes()
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

	if len(tx.InitialBalances()) <= 0 {
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

	allowedDecimals, err := CalcAllowedDecimals(uint64(sumOfInitialBalances / 10 ^ tx.Decimals()))

	if err != nil {
		return false
	}

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

func (tx *TokenTransaction) ValidateExtended(addrFromState *addressstate.AddressState, addrFromPkState *addressstate.AddressState) bool {
	if !tx.ValidateSlave(addrFromState, addrFromPkState) {
		return false
	}

	txBalance := addrFromState.Balance()

	if !addressstate.IsValidAddress(tx.AddrFrom()) {
		tx.log.Warn("Invalid address addr_from: %s", tx.AddrFrom())
		return false
	}

	if !addressstate.IsValidAddress(tx.Owner()) {
		tx.log.Warn("Invalid address owner_addr: %s", tx.Owner())
		return false
	}

	for _, addrBalance := range tx.InitialBalances() {
		if !addressstate.IsValidAddress(addrBalance.Address) {
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

func (tx *TokenTransaction) ApplyStateChanges(addressesState map[string]*addressstate.AddressState) {
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
			addrState.UpdateTokenBalance(tx.Txhash(), addrAmount.Amount, false)
			addrState.AppendTransactionHash(tx.Txhash())
		}
	}

	if !ownerProcessed {
		if addrState, ok := addressesState[string(tx.Owner())]; ok {
			addrState.AppendTransactionHash(tx.Txhash())
		}
	}

	if addrState, ok := addressesState[string(tx.AddrFrom())]; ok {
		addrState.SubtractBalance(tx.Fee())
		if !reflect.DeepEqual(tx.AddrFrom(), tx.Owner()) {
			if !addrFromProcessed {
				addrState.AppendTransactionHash(tx.Txhash())
			}
		}
	}

	if addrState, ok := addressesState[string(addrFromPK)]; ok {
		if !reflect.DeepEqual(tx.AddrFrom(), addrFromPK) && !reflect.DeepEqual(tx.AddrFrom(), tx.Owner()){
			if !addrFromPKProcessed {
				addrState.AppendTransactionHash(tx.Txhash())
			}
		}
		addrState.IncreaseNonce()
		addrState.SetOTSKey(uint64(tx.OtsKey()))
	}
}

func (tx *TokenTransaction) RevertStateChanges(addressesState map[string]*addressstate.AddressState) {
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
			addrState.UpdateTokenBalance(tx.Txhash(), addrAmount.Amount, true)
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
		if  !reflect.DeepEqual(tx.AddrFrom(), tx.Owner()) {
			if !addrFromProcessed {
				addrState.RemoveTransactionHash(tx.Txhash())
			}
		}
	}

	if addrState, ok := addressesState[string(addrFromPK)]; ok {
		if !reflect.DeepEqual(tx.AddrFrom(), addrFromPK) && !reflect.DeepEqual(tx.AddrFrom(), tx.Owner()) {
			if !addrFromPKProcessed {
				addrState.RemoveTransactionHash(tx.Txhash())
			}
		}
		addrState.DecreaseNonce()
		// Remember to Unset OTS Key
	}
}

func (tx *TokenTransaction) SetAffectedAddress(addressesState map[string]*addressstate.AddressState) {
	addressesState[string(tx.AddrFrom())] = &addressstate.AddressState{}
	addressesState[string(tx.PK())] = &addressstate.AddressState{}

	for _, addrAmount := range tx.InitialBalances() {
		addressesState[string(addrAmount.Address)] = &addressstate.AddressState{}
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

	if !tx.Validate(misc.BytesToUCharVector(tx.GetHashableBytes()), false) {
		return nil
	}

	return tx
}

func CalcAllowedDecimals(value uint64) (uint64, error) {
	if value == 0 {
		return 19, nil
	}

	return uint64(math.Max(math.Floor(19 - math.Log10(float64(value))), 0)), nil
}
