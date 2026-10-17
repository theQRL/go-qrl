package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/common/hexutil"
	"github.com/theQRL/go-qrl/core"
	"github.com/theQRL/go-qrl/migration"
	"github.com/theQRL/go-qrl/params"
	"github.com/theQRL/go-qrllib/crypto/xmss"
)

func TestGenesisMessageAndCalldataCommands(t *testing.T) {
	tree, err := xmss.InitializeTree(4, xmss.SHAKE_256, bytes.Repeat([]byte{11}, 48))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(tree.Zeroize)
	pk := append([]byte{2, 2, 0}, tree.GetRoot()...)
	pk = append(pk, tree.GetPKSeed()...)
	address, err := migration.AddressFromPublicKey(pk)
	if err != nil {
		t.Fatal(err)
	}
	source := "Q" + hex.EncodeToString(address)
	snapshot := migration.Snapshot{
		Format: migration.Format, Network: "qrl-mainnet", StateVersion: 3,
		Height: 42, BlockHash: strings.Repeat("ab", 32), GenesisHash: migration.MainnetGenesisHash,
		TotalBalanceShor: "7", TotalBalanceBaseUnits: "7000000000",
		RecordedCoinSupplyShor: "7", SupplyGapShor: "0", UnminedReserveShor: "100",
		Accounts: []migration.Account{{Address: source, Kind: "xmss", Height: 4,
			BalanceShor: "7", BalanceBaseUnits: "7000000000", Quanta: "0.000000007"}},
	}
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	commitment := sha256.Sum256(encoded)
	snapshot.Commitment = hex.EncodeToString(commitment[:])
	dir := t.TempDir()
	snapshotPath := filepath.Join(dir, "snapshot.json")
	if err := writeJSON(snapshotPath, snapshot); err != nil {
		t.Fatal(err)
	}
	contract := common.BytesToAddress([]byte{0x10, 0})
	recipient := common.BytesToAddress([]byte{0x22})
	genesisPath := filepath.Join(dir, "base.json")
	genesis := core.Genesis{Config: &params.ChainConfig{ChainID: big.NewInt(1234)}, GasLimit: 30_000_000,
		Alloc: core.GenesisAlloc{recipient: {Balance: big.NewInt(99)}}}
	if err := writeJSON(genesisPath, genesis); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(dir, "genesis.json")
	if err := run([]string{"genesis", "--snapshot", snapshotPath, "--genesis", genesisPath, "--contract", contract.Hex(), "--out", output}); err != nil {
		t.Fatal(err)
	}
	var generated core.Genesis
	if err := readJSON(output, &generated); err != nil {
		t.Fatal(err)
	}
	if len(generated.Alloc[contract].Code) == 0 || generated.Alloc[contract].Balance.String() != "7000000000" || generated.Alloc[recipient].Balance.Int64() != 99 {
		t.Fatal("CLI did not fund contract and preserve existing allocation")
	}
	messagePath := filepath.Join(dir, "message.json")
	args := []string{"message", "--snapshot", snapshotPath, "--chain-id", "1234", "--contract", contract.Hex(), "--source", source, "--recipient", recipient.Hex(), "--out", messagePath}
	if err := run(args); err != nil {
		t.Fatal(err)
	}
	var signingRequest struct {
		Message hexutil.Bytes `json:"message"`
	}
	// Extract the message without imposing a second schema on the CLI response.
	data, err := os.ReadFile(messagePath)
	if err != nil || json.Unmarshal(data, &signingRequest) != nil {
		t.Fatal("invalid signing request", err)
	}
	expected, err := migration.ClaimMessage(&snapshot, big.NewInt(1234), contract, source, recipient)
	if err != nil || !bytes.Equal(expected, signingRequest.Message) {
		t.Fatal("CLI changed signing bytes", err)
	}
	sig, err := tree.Sign(signingRequest.Message)
	if err != nil {
		t.Fatal(err)
	}
	signaturePath := filepath.Join(dir, "signatures.json")
	if err := writeJSON(signaturePath, []migration.Signature{{PublicKey: hex.EncodeToString(pk), Signature: hex.EncodeToString(sig)}}); err != nil {
		t.Fatal(err)
	}
	claimPath := filepath.Join(dir, "claim.json")
	args[0], args[len(args)-1] = "calldata", claimPath
	args = append(args, "--signatures", signaturePath)
	if err := run(args); err != nil {
		t.Fatal(err)
	}
	var tx migration.ClaimTransaction
	if err := readJSON(claimPath, &tx); err != nil {
		t.Fatal(err)
	}
	if len(tx.Data) == 0 || tx.To != contract || tx.Recipient != recipient || tx.Amount != "7000000000" || tx.Value != "0x0" {
		t.Fatalf("wrong transaction draft: %+v", tx)
	}
	if err := run(args); err == nil {
		t.Fatal("overwrote existing claim")
	}
}

func TestOutputDoesNotOverwriteAndInputIsRequired(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "existing.json")
	original := []byte("preserve this file")
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(path, map[string]string{"new": "data"}); err == nil {
		t.Fatal("overwrote existing output")
	}
	actual, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(original, actual) {
		t.Fatal("existing file changed", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatal("temporary output leaked", err)
	}
	for _, args := range [][]string{nil, {"export", "--out", path}, {"export", "--state", dir}, {"genesis", "--out", path}, {"unknown", "--out", path}} {
		if err := run(args); err == nil {
			t.Fatalf("accepted missing required inputs: %v", args)
		}
	}
}

func TestStreamingOutputFailureNeverPublishesPartialFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "snapshot.json")
	expected := errors.New("interrupted export")
	err := writeOutput(path, func(w io.Writer) error {
		if _, err := io.WriteString(w, strings.Repeat("x", 128<<10)); err != nil {
			return err
		}
		return expected
	})
	if !errors.Is(err, expected) {
		t.Fatal("lost export error", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatal("published or leaked partial output", err)
	}
}
