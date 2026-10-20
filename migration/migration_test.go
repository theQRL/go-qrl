package migration

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"path/filepath"
	"sort"
	"testing"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/theQRL/go-qrl/accounts/abi"
	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/core"
	"github.com/theQRL/go-qrl/core/rawdb"
	"github.com/theQRL/go-qrl/core/state"
	"github.com/theQRL/go-qrl/core/vm/runtime"
	"github.com/theQRL/go-qrl/crypto/pqcrypto/wallet"
	"github.com/theQRL/go-qrl/params"
	"github.com/theQRL/go-qrl/trie"
	"github.com/theQRL/go-qrllib/crypto/xmss"
	"google.golang.org/protobuf/encoding/protowire"
)

type testSigner struct {
	tree        *xmss.XMSS
	pk, address []byte
}

func makeSigner(t *testing.T, seed byte, hash xmss.HashFunction) testSigner {
	t.Helper()
	tree, err := xmss.InitializeTree(4, hash, bytes.Repeat([]byte{seed}, 48))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(tree.Zeroize)
	pk := append([]byte{byte(hash), 2, 0}, tree.GetRoot()...)
	pk = append(pk, tree.GetPKSeed()...)
	address, err := AddressFromPublicKey(pk)
	if err != nil {
		t.Fatal(err)
	}
	return testSigner{tree, pk, address}
}

func pbUint(b []byte, n protowire.Number, v uint64) []byte {
	return protowire.AppendVarint(protowire.AppendTag(b, n, protowire.VarintType), v)
}
func pbBytes(b []byte, n protowire.Number, v []byte) []byte {
	return protowire.AppendBytes(protowire.AppendTag(b, n, protowire.BytesType), v)
}

func legacyFixture(t *testing.T) (string, testSigner, testSigner, []byte) {
	t.Helper()
	a := makeSigner(t, 1, xmss.SHA2_256)
	b := makeSigner(t, 2, xmss.SHAKE_128)
	path := filepath.Join(t.TempDir(), "state")
	db, err := leveldb.OpenFile(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	put := func(k, v []byte) {
		t.Helper()
		if err := db.Put(k, v, nil); err != nil {
			t.Fatal(err)
		}
	}
	put([]byte("state_version"), []byte("3"))
	put([]byte("blockheight"), binary.BigEndian.AppendUint64(nil, 123))
	put([]byte("total_coin_supply"), binary.BigEndian.AppendUint64(nil, 3_000_000_003))
	put(make([]byte, 32), pbUint(pbBytes(nil, 1, make([]byte, 32)), 2, 42))
	for _, h := range []uint64{0, 123} {
		hash := sha256.Sum256([]byte{byte(h)})
		if h == 0 {
			decoded, _ := hex.DecodeString(MainnetGenesisHash)
			copy(hash[:], decoded)
		}
		mapping, _ := json.Marshal(map[string][]byte{"headerhash": hash[:]})
		put([]byte(new(big.Int).SetUint64(h).String()), mapping)
		put(hash[:], pbBytes(nil, 1, pbUint(pbBytes(nil, 1, hash[:]), 2, h)))
	}
	put(a.address, pbUint(pbUint(pbBytes(nil, 1, a.address), 2, 1_000_000_003), 5, 2))
	put(b.address, pbUint(pbBytes(nil, 1, b.address), 5, 1))
	for _, signer := range []testSigner{a, b} {
		bits := make([]byte, 1024)
		if bytes.Equal(signer.address, a.address) {
			bits[0] = 1
			bits[1] = 1
		} else {
			bits[0] = 1 << 4
		}
		var data []byte
		for _, v := range bits {
			data = pbBytes(data, 1, []byte{v})
		}
		key := append(append([]byte("bitfield_"), signer.address...), '_')
		key = binary.BigEndian.AppendUint64(key, 1)
		put(key, data)
	}
	creation := sha256.Sum256([]byte("multisig creation"))
	keyHash := sha256.Sum256(append([]byte{0x11, 0, 0}, creation[:]...))
	multisig := append([]byte{0x11, 0, 0}, keyHash[:]...)
	checksum := sha256.Sum256(multisig)
	multisig = append(multisig, checksum[28:]...)
	m := pbBytes(pbBytes(nil, 1, multisig), 2, creation[:])
	m = pbUint(m, 4, 2_000_000_000)
	m = pbBytes(pbBytes(m, 5, a.address), 5, b.address)
	m = pbBytes(m, 6, []byte{2, 1})
	m = pbUint(m, 7, 3)
	put(multisig, m)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	return path, a, b, multisig
}

type testVM struct {
	s        *Snapshot
	state    *state.StateDB
	abi      abi.ABI
	cfg      *runtime.Config
	contract common.Address
}

func setupVM(t *testing.T, s *Snapshot) *testVM {
	t.Helper()
	chain := &params.ChainConfig{ChainID: big.NewInt(9876)}
	contract := common.BytesToAddress([]byte{0x10, 0})
	g := &core.Genesis{Config: chain, GasLimit: 30_000_000, BaseFee: big.NewInt(1), Alloc: make(core.GenesisAlloc)}
	if err := AddToGenesis(g, s, contract); err != nil {
		t.Fatal(err)
	}
	// Exercise the actual JSON consumed by gqrl init, not just an in-memory map.
	encoded, err := json.Marshal(g)
	if err != nil {
		t.Fatal(err)
	}
	var decoded core.Genesis
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	db := rawdb.NewMemoryDatabase()
	t.Cleanup(func() { db.Close() })
	nodes := trie.NewDatabase(db, nil)
	t.Cleanup(func() { nodes.Close() })
	block, err := decoded.Commit(db, nodes)
	if err != nil {
		t.Fatal(err)
	}
	st, err := state.New(block.Root(), state.NewDatabaseWithNodeDB(db, nodes), nil)
	if err != nil {
		t.Fatal(err)
	}
	_, contractABI, _, err := Contract()
	if err != nil {
		t.Fatal(err)
	}
	return &testVM{s, st, contractABI, &runtime.Config{State: st, ChainConfig: chain, Origin: common.BytesToAddress([]byte{0xaa}), GasLimit: 30_000_000, BaseFee: big.NewInt(1)}, contract}
}

func (v *testVM) call(t *testing.T, name string, args ...any) ([]byte, error) {
	t.Helper()
	input, err := v.abi.Pack(name, args...)
	if err != nil {
		t.Fatal(err)
	}
	ret, _, err := runtime.Call(v.contract, input, v.cfg)
	return ret, err
}

func (v *testVM) value(t *testing.T, name string, args ...any) *big.Int {
	t.Helper()
	ret, err := v.call(t, name, args...)
	if err != nil {
		t.Fatalf("%s: %v (return %x)", name, err, ret)
	}
	values, err := v.abi.Unpack(name, ret)
	if err != nil {
		t.Fatal(err)
	}
	return values[0].(*big.Int)
}

func signedClaim(t *testing.T, v *testVM, signer testSigner, index uint32, source []byte, recipient common.Address) []byte {
	t.Helper()
	message, err := ClaimMessage(v.s, v.cfg.ChainConfig.ChainID, v.contract, "Q"+hex.EncodeToString(source), recipient)
	if err != nil {
		t.Fatal(err)
	}
	if err := signer.tree.SetIndex(index); err != nil {
		t.Fatal(err)
	}
	sig, err := signer.tree.Sign(message)
	if err != nil {
		t.Fatal(err)
	}
	return sig
}

func TestExportGenesisAndRedemption(t *testing.T) {
	path, owner, other, multisig := legacyFixture(t)
	s, err := exportSnapshot(path)
	if err != nil {
		t.Fatal(err)
	}
	if s.Height != 123 || len(s.Accounts) != 3 || s.TotalBalanceBaseUnits != "3000000003000000000" || s.UnminedReserveShor != "42" {
		t.Fatalf("unexpected snapshot: %+v", s)
	}
	v := setupVM(t, s)
	destination, err := wallet.Generate(wallet.ML_DSA_87)
	if err != nil {
		t.Fatal(err)
	}
	recipient := common.Address(destination.GetAddress())
	if got := v.value(t, "remaining", AccountID(owner.address)); got.String() != "1000000003000000000" {
		t.Fatal("wrong genesis mapping", got)
	}
	if got := v.value(t, "usedOTS", AccountID(owner.address), big.NewInt(0)); got.Uint64() != 257 {
		t.Fatal("wrong OTS bitmap", got)
	}
	message, err := ClaimMessage(s, v.cfg.ChainConfig.ChainID, v.contract, "Q"+hex.EncodeToString(owner.address), recipient)
	if err != nil {
		t.Fatal(err)
	}
	ret, err := v.call(t, "claimMessage", owner.address, recipient)
	if err != nil {
		t.Fatal(err)
	}
	values, err := v.abi.Unpack("claimMessage", ret)
	if err != nil || !bytes.Equal(values[0].([]byte), message) {
		t.Fatal("off-chain/on-chain message mismatch", err)
	}
	ret, err = v.call(t, "legacyAddress", owner.pk)
	if err != nil {
		t.Fatal(err)
	}
	values, err = v.abi.Unpack("legacyAddress", ret)
	if err != nil || !bytes.Equal(values[0].([]byte), owner.address) {
		t.Fatal("legacy address derivation mismatch", err)
	}
	usedSig := signedClaim(t, v, owner, 0, owner.address, recipient)
	if _, err = v.call(t, "claim", owner.pk, usedSig, recipient); err == nil {
		t.Fatal("accepted previously used OTS")
	}
	sig := signedClaim(t, v, owner, 1, owner.address, recipient)
	if _, err = v.call(t, "claim", owner.pk, sig, common.BytesToAddress([]byte{0x43})); err == nil {
		t.Fatal("accepted recipient substitution")
	}
	v.cfg.ChainConfig.ChainID = big.NewInt(9877)
	if _, err = v.call(t, "claim", owner.pk, sig, recipient); err == nil {
		t.Fatal("accepted wrong target chain")
	}
	v.cfg.ChainConfig.ChainID = big.NewInt(9876)
	if _, err = v.call(t, "claim", owner.pk, sig, recipient); err != nil {
		t.Fatal("valid claim failed", err)
	}
	if got := v.state.GetBalance(recipient).String(); got != "1000000003000000000" {
		t.Fatal("wrong payout", got)
	}
	if _, err = v.call(t, "claim", owner.pk, sig, recipient); err == nil {
		t.Fatal("accepted double claim")
	}
	if v.value(t, "usedOTS", AccountID(owner.address), big.NewInt(0)).Bit(1) != 1 {
		t.Fatal("claim OTS not consumed")
	}

	first := signedClaim(t, v, owner, 2, multisig, recipient)
	second := signedClaim(t, v, other, 0, multisig, recipient)
	if _, err = v.call(t, "claimMultisig", multisig, [][]byte{owner.pk}, [][]byte{first}, recipient); err == nil {
		t.Fatal("accepted insufficient multisig weight")
	}
	if v.value(t, "usedOTS", AccountID(owner.address), big.NewInt(0)).Bit(2) != 0 {
		t.Fatal("failed multisig consumed OTS")
	}
	if _, err = v.call(t, "claimMultisig", multisig, [][]byte{owner.pk, owner.pk}, [][]byte{first, first}, recipient); err == nil {
		t.Fatal("accepted duplicate signer")
	}
	keys, sigs := [][]byte{owner.pk, other.pk}, [][]byte{first, second}
	if AccountID(owner.address).Cmp(AccountID(other.address)) > 0 {
		keys[0], keys[1] = keys[1], keys[0]
		sigs[0], sigs[1] = sigs[1], sigs[0]
	}
	if _, err = v.call(t, "claimMultisig", multisig, keys, sigs, recipient); err != nil {
		t.Fatal("valid multisig failed", err)
	}
	if v.state.GetBalance(recipient).String() != s.TotalBalanceBaseUnits || v.value(t, "totalUnclaimed").Sign() != 0 || v.state.GetBalance(v.contract).Sign() != 0 {
		t.Fatal("redemption supply mismatch")
	}
}

func TestFailedPayoutDoesNotConsumeClaim(t *testing.T) {
	path, owner, _, _ := legacyFixture(t)
	s, err := exportSnapshot(path)
	if err != nil {
		t.Fatal(err)
	}
	v := setupVM(t, s)
	recipient := common.BytesToAddress([]byte{0x99})
	v.state.SetCode(recipient, common.FromHex("60006000fd"))
	sig := signedClaim(t, v, owner, 1, owner.address, recipient)
	if _, err = v.call(t, "claim", owner.pk, sig, recipient); err == nil {
		t.Fatal("expected payout failure")
	}
	if v.value(t, "remaining", AccountID(owner.address)).Sign() == 0 || v.value(t, "usedOTS", AccountID(owner.address), big.NewInt(0)).Bit(1) != 0 {
		t.Fatal("failed payout consumed claim")
	}
	v.state.SetCode(recipient, nil)
	if _, err = v.call(t, "claim", owner.pk, sig, recipient); err != nil {
		t.Fatal("retry failed", err)
	}
}

func TestSnapshotTamperAndAllocationCollision(t *testing.T) {
	path, _, _, _ := legacyFixture(t)
	s, err := exportSnapshot(path)
	if err != nil {
		t.Fatal(err)
	}
	s.Accounts[0].BalanceShor = "999"
	if err = s.Validate(); err == nil {
		t.Fatal("accepted altered snapshot")
	}
	s, err = exportSnapshot(path)
	if err != nil {
		t.Fatal(err)
	}
	g := &core.Genesis{Config: &params.ChainConfig{ChainID: big.NewInt(99)}, Alloc: core.GenesisAlloc{common.BytesToAddress([]byte{8}): {Balance: big.NewInt(1)}}}
	if err = AddToGenesis(g, s, common.BytesToAddress([]byte{8})); err == nil {
		t.Fatal("overwrote existing allocation")
	}
	if err = AddToGenesis(g, s, common.BytesToAddress([]byte{7})); err == nil {
		t.Fatal("allowed precompile collision")
	}
}

func TestExporterRejectsIncompleteState(t *testing.T) {
	for _, test := range []string{"missing OTS page", "wrong supply", "unfinished reorg", "wrong network", "wrong state version", "incomplete full page"} {
		t.Run(test, func(t *testing.T) {
			path, owner, _, _ := legacyFixture(t)
			db, err := leveldb.OpenFile(path, nil)
			if err != nil {
				t.Fatal(err)
			}
			switch test {
			case "missing OTS page":
				key := append(append([]byte("bitfield_"), owner.address...), '_')
				key = binary.BigEndian.AppendUint64(key, 1)
				err = db.Delete(key, nil)
			case "wrong supply":
				err = db.Put([]byte("total_coin_supply"), binary.BigEndian.AppendUint64(nil, 1), nil)
			case "unfinished reorg":
				err = db.Put([]byte("fork_state"), []byte{1}, nil)
			case "wrong network":
				data, _ := json.Marshal(map[string][]byte{"headerhash": bytes.Repeat([]byte{1}, 32)})
				err = db.Put([]byte("0"), data, nil)
			case "wrong state version":
				err = db.Put([]byte("state_version"), []byte("2"), nil)
			case "incomplete full page":
				data, readErr := db.Get(owner.address, nil)
				if readErr != nil {
					t.Fatal(readErr)
				}
				err = db.Put(owner.address, pbUint(data, 4, 1), nil)
			}
			if err != nil {
				t.Fatal(err)
			}
			db.Close()
			if _, err = exportSnapshot(path); err == nil {
				t.Fatal("accepted incomplete state")
			}
		})
	}
}

func TestOTSRangeStorageAcrossWords(t *testing.T) {
	path, _, _, _ := legacyFixture(t)
	s, err := exportSnapshot(path)
	if err != nil {
		t.Fatal(err)
	}
	pk := bytes.Repeat([]byte{7}, 67)
	pk[0] = 0
	pk[1] = 7
	pk[2] = 0
	addr, err := AddressFromPublicKey(pk)
	if err != nil {
		t.Fatal(err)
	}
	a := newAccount(addr, 0)
	a.Kind = "xmss"
	a.Height = 14
	a.UsedOTS = []OTSRange{{510, 514}, {8191, 8193}}
	a.UsedOTSCount = 8
	s.Accounts = append(s.Accounts, a)
	sort.Slice(s.Accounts, func(i, j int) bool { return s.Accounts[i].Address < s.Accounts[j].Address })
	s.seal()
	v := setupVM(t, s)
	for _, index := range []uint64{510, 511, 512, 513, 514, 8191, 8192, 8193} {
		word := v.value(t, "usedOTS", AccountID(addr), new(big.Int).SetUint64(index/512))
		if word.Bit(int(index%512)) != 1 {
			t.Fatalf("OTS %d missing", index)
		}
	}
	if v.value(t, "usedOTS", AccountID(addr), big.NewInt(1)).Bit(3) != 0 {
		t.Fatal("unused OTS marked used")
	}
}
