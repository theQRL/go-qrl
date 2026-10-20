package migration

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/core/vm"
	"github.com/theQRL/go-qrl/core/vm/runtime"
	"github.com/theQRL/go-qrllib/crypto/xmss"
)

func suppliedSignature(pk, sig []byte) Signature {
	return Signature{hex.EncodeToString(pk), hex.EncodeToString(sig)}
}

func TestPrepareClaimAndRejectAlterations(t *testing.T) {
	path, owner, other, multisig := legacyFixture(t)
	s, err := exportSnapshot(path)
	if err != nil {
		t.Fatal(err)
	}
	v := setupVM(t, s)
	recipient := common.BytesToAddress([]byte{0x99})
	source := "Q" + hex.EncodeToString(owner.address)
	sig := signedClaim(t, v, owner, 1, owner.address, recipient)
	prepare := func(source string, pairs []Signature) (*ClaimTransaction, error) {
		return PrepareClaim(s, v.cfg.ChainConfig.ChainID, v.contract, source, recipient, pairs)
	}
	valid := suppliedSignature(owner.pk, sig)
	corrupt := bytes.Clone(sig)
	corrupt[40] ^= 1
	for name, pairs := range map[string][]Signature{
		"missing":     nil,
		"duplicate":   {valid, valid},
		"corrupt":     {suppliedSignature(owner.pk, corrupt)},
		"short":       {suppliedSignature(owner.pk, []byte{1})},
		"wrong owner": {suppliedSignature(other.pk, sig)},
		"bad hex":     {{"xx", valid.Signature}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := prepare(source, pairs); err == nil {
				t.Fatal("prepared invalid claim")
			}
		})
	}
	for _, invalid := range [][]byte{nil, sig[:4], corrupt, append(bytes.Clone(sig), 0)} {
		if _, err := v.call(t, "claim", owner.pk, invalid, recipient); err == nil {
			t.Fatal("contract accepted invalid signature")
		}
	}
	if v.value(t, "usedOTS", AccountID(owner.address), big.NewInt(0)).Bit(1) != 0 {
		t.Fatal("invalid signature consumed OTS")
	}
	tx, err := prepare(source, []Signature{valid})
	if err != nil {
		t.Fatal(err)
	}
	if tx.To != v.contract || tx.Value != "0x0" || tx.ChainID != "9876" || tx.Recipient != recipient || tx.Amount != "1000000003000000000" {
		t.Fatalf("incorrect transaction: %+v", tx)
	}
	if _, _, err := runtime.Call(tx.To, tx.Data, v.cfg); err != nil {
		t.Fatal("prepared ordinary calldata failed", err)
	}
	first := suppliedSignature(owner.pk, signedClaim(t, v, owner, 2, multisig, recipient))
	second := suppliedSignature(other.pk, signedClaim(t, v, other, 0, multisig, recipient))
	group := "Q" + hex.EncodeToString(multisig)
	if _, err := prepare(group, []Signature{first}); err == nil {
		t.Fatal("prepared multisig below threshold")
	}
	if _, err := prepare(group, []Signature{first, first}); err == nil {
		t.Fatal("counted a duplicate signatory")
	}
	// The public helper sorts callers' unordered input for the contract.
	pairs := []Signature{first, second}
	if AccountID(owner.address).Cmp(AccountID(other.address)) < 0 {
		pairs[0], pairs[1] = pairs[1], pairs[0]
	}
	tx, err = prepare(group, pairs)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := runtime.Call(tx.To, tx.Data, v.cfg); err != nil {
		t.Fatal("prepared multisig calldata failed", err)
	}
	if v.state.GetBalance(recipient).String() != s.TotalBalanceBaseUnits {
		t.Fatal("prepared claims paid incorrect total")
	}
}

func TestClaimSignatureDomain(t *testing.T) {
	path, owner, _, _ := legacyFixture(t)
	s, err := exportSnapshot(path)
	if err != nil {
		t.Fatal(err)
	}
	v := setupVM(t, s)
	recipient := common.BytesToAddress([]byte{0x99})
	sig := signedClaim(t, v, owner, 1, owner.address, recipient)
	for _, changed := range []string{"contract", "block", "commitment", "chain"} {
		t.Run(changed, func(t *testing.T) {
			x := setupVM(t, s)
			switch changed {
			case "contract":
				copyAddress := common.BytesToAddress([]byte{0x10, 1})
				allocation, err := GenesisAccount(s, x.cfg.ChainConfig.ChainID, copyAddress)
				if err != nil {
					t.Fatal(err)
				}
				x.state.SetCode(copyAddress, allocation.Code)
				x.state.SetBalance(copyAddress, allocation.Balance)
				for key, value := range allocation.Storage {
					x.state.SetState(copyAddress, key, value)
				}
				x.contract = copyAddress
			case "block":
				x.state.SetState(x.contract, slot(1), common.BytesToStorageValue64([]byte{1}))
			case "commitment":
				x.state.SetState(x.contract, slot(2), common.BytesToStorageValue64([]byte{1}))
			case "chain":
				x.cfg.ChainConfig.ChainID = big.NewInt(9877)
				x.state.SetState(x.contract, slot(3), common.BytesToStorageValue64(big.NewInt(9877).Bytes()))
			}
			if _, err := x.call(t, "claim", owner.pk, sig, recipient); err == nil {
				t.Fatal("signature replay accepted in another domain")
			}
		})
	}
}

func TestOTSSharedBetweenOrdinaryAndMultisig(t *testing.T) {
	path, owner, other, multisig := legacyFixture(t)
	s, err := exportSnapshot(path)
	if err != nil {
		t.Fatal(err)
	}
	v := setupVM(t, s)
	recipient := common.BytesToAddress([]byte{0x99})
	sig := signedClaim(t, v, owner, 1, owner.address, recipient)
	if _, err := v.call(t, "claim", owner.pk, sig, recipient); err != nil {
		t.Fatal(err)
	}
	// Deliberate misuse of a disposable test seed: produce a cryptographically
	// valid signature of a different claim using the already-consumed leaf.
	clone := makeSigner(t, 1, xmss.SHA2_256)
	reused := suppliedSignature(clone.pk, signedClaim(t, v, clone, 1, multisig, recipient))
	second := suppliedSignature(other.pk, signedClaim(t, v, other, 0, multisig, recipient))
	tx, err := PrepareClaim(s, v.cfg.ChainConfig.ChainID, v.contract, "Q"+hex.EncodeToString(multisig), recipient, []Signature{reused, second})
	if err != nil {
		t.Fatal("snapshot-only preparation should accept cryptographically valid, snapshot-unused OTS", err)
	}
	if _, _, err := runtime.Call(tx.To, tx.Data, v.cfg); err == nil {
		t.Fatal("contract accepted OTS consumed by another allocation")
	}
	if v.value(t, "remaining", AccountID(multisig)).String() != "2000000000000000000" || v.value(t, "usedOTS", AccountID(other.address), big.NewInt(0)).Bit(0) != 0 {
		t.Fatal("rejected shared-OTS claim changed multisig state")
	}
}

func TestReentrantRecipientCannotClaimAgainWithFreshOTS(t *testing.T) {
	path, owner, _, _ := legacyFixture(t)
	s, err := exportSnapshot(path)
	if err != nil {
		t.Fatal(err)
	}
	v := setupVM(t, s)
	recipient := common.BytesToAddress([]byte{0x99})
	first := signedClaim(t, v, owner, 1, owner.address, recipient)
	second := signedClaim(t, v, owner, 2, owner.address, recipient)
	input, err := v.abi.Pack("claim", owner.pk, second, recipient)
	if err != nil {
		t.Fatal(err)
	}
	// Receiver code copies the second claim from its own code into memory, calls
	// the migration contract, and records whether that recursive claim succeeds.
	length := uint16(len(input))
	code := []byte{byte(vm.PUSH2), byte(length >> 8), byte(length), byte(vm.PUSH2), 0, 0,
		byte(vm.PUSH1), 0, byte(vm.CODECOPY), byte(vm.PUSH1), 0, byte(vm.PUSH1), 0,
		byte(vm.PUSH2), byte(length >> 8), byte(length), byte(vm.PUSH1), 0,
		byte(vm.PUSH1), 0, byte(vm.PUSH64)}
	code = append(code, v.contract[:]...)
	code = append(code, byte(vm.GAS), byte(vm.CALL), byte(vm.PUSH1), 0, byte(vm.SSTORE), byte(vm.STOP))
	binary.BigEndian.PutUint16(code[4:6], uint16(len(code)))
	code = append(code, input...)
	v.state.SetCode(recipient, code)
	v.state.SetState(recipient, slot(0), common.BytesToStorageValue64([]byte{1}))
	if _, err := v.call(t, "claim", owner.pk, first, recipient); err != nil {
		t.Fatal("outer claim failed", err)
	}
	if v.state.GetState(recipient, slot(0)).Big().Sign() != 0 {
		t.Fatal("recursive claim succeeded or receiver did not execute")
	}
	if v.state.GetBalance(recipient).String() != "1000000003000000000" || v.value(t, "totalUnclaimed").String() != "2000000000000000000" {
		t.Fatal("reentrant receiver was overpaid")
	}
	if v.value(t, "usedOTS", AccountID(owner.address), big.NewInt(0)).Bit(2) != 0 {
		t.Fatal("rejected recursive claim consumed fresh OTS")
	}
}

func TestLegacyZeroThresholdRequiresOriginalSignatory(t *testing.T) {
	path, owner, other, multisig := legacyFixture(t)
	s, err := exportSnapshot(path)
	if err != nil {
		t.Fatal(err)
	}
	for i := range s.Accounts {
		if s.Accounts[i].Kind == "multisig" {
			s.Accounts[i].Threshold = 0
		}
	}
	s.seal()
	v := setupVM(t, s)
	recipient := common.BytesToAddress([]byte{0x99})
	if _, err := v.call(t, "claimMultisig", multisig, [][]byte{}, [][]byte{}, recipient); err == nil {
		t.Fatal("zero-threshold allocation accepted an unauthenticated claim")
	}
	stranger := makeSigner(t, 55, xmss.SHAKE_256)
	bad := signedClaim(t, v, stranger, 0, multisig, recipient)
	if _, err := v.call(t, "claimMultisig", multisig, [][]byte{stranger.pk}, [][]byte{bad}, recipient); err == nil {
		t.Fatal("zero-threshold allocation accepted an unrelated signer")
	}
	sig := signedClaim(t, v, other, 0, multisig, recipient)
	tx, err := PrepareClaim(s, v.cfg.ChainConfig.ChainID, v.contract, "Q"+hex.EncodeToString(multisig), recipient, []Signature{suppliedSignature(other.pk, sig)})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := runtime.Call(tx.To, tx.Data, v.cfg); err != nil {
		t.Fatal("zero-threshold legacy account could not redeem", err)
	}
	if v.state.GetBalance(recipient).String() != "2000000000000000000" || v.value(t, "remaining", AccountID(owner.address)).String() != "1000000003000000000" {
		t.Fatal("zero-threshold redemption changed unrelated balances")
	}
}
