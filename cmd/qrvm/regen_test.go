// Copyright 2026 The go-qrl Authors
// This file is part of the go-qrl library.

package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/crypto/pqcrypto/wallet"
)

// TestRegenerateT8nFixtures is a one-shot helper that regenerates the minimal
// testdata/simple scenario exercised by TestT8n. Run it with
//
//	WRITE_FIXTURES=1 go test -run TestRegenerateT8nFixtures ./cmd/qrvm/
//
// Requires a `qrvm` binary to exist at /tmp/qrvm (build it via
// `go build -o /tmp/qrvm ./cmd/qrvm/`). The helper restores a deterministic
// ML-DSA-87 wallet, seeds an alloc/env/txs triple at ./testdata/simple/, and
// captures the resulting t8n/t9n/b11r outputs.
func TestRegenerateT8nFixtures(t *testing.T) {
	if os.Getenv("WRITE_FIXTURES") == "" {
		t.Skip("set WRITE_FIXTURES=1 to regenerate t8n fixtures")
	}
	qrvm := os.Getenv("QRVM_BIN")
	if qrvm == "" {
		qrvm = "/tmp/qrvm"
	}
	if _, err := os.Stat(qrvm); err != nil {
		t.Fatalf("qrvm binary not found at %s (set QRVM_BIN or build to /tmp/qrvm first)", qrvm)
	}
	const (
		fixtureSeedHex = "010000d00b21539420cb9ff91e44b0b9d25ac67642ebba7a459f8fa2bbc477c3a216a20110ca8811758940cfb6dd984316dd74"
		invalidRLP     = "0xf852328001825208870b9331677e6ebf0a801ca098ff921201554726367d2be8c804a7ff89ccf285ebc57dff8ae4c44b9c19ac4aa03887321be575c8095f789dd4c743dfe42c1820f9231f98a962b210e3ac2452a3"
	)
	w, err := wallet.RestoreFromSeedHex(fixtureSeedHex)
	if err != nil {
		t.Fatalf("wallet.RestoreFromSeedHex: %v", err)
	}
	sender := w.GetAddress()
	recipient := common.BytesToAddress(common.FromHex(
		"c0decafec0decafec0decafec0decafec0decafec0decafec0decafec0decafec0decafec0decafec0decafec0decafe"))
	seedHex := "0x" + fixtureSeedHex

	dir := filepath.Join(t.TempDir(), "simple")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	alloc := map[string]map[string]any{
		qPrefixed(sender): {
			"balance": "0x1000000000000000",
			"nonce":   "0x0",
		},
		qPrefixed(recipient): {
			"balance": "0x0",
			"nonce":   "0x0",
		},
	}
	env := map[string]any{
		"currentCoinbase":  fmt.Sprintf("%#x", recipient),
		"currentRandom":    "0xdeadc0de",
		"currentGasLimit":  "0x750a163df65e8a",
		"currentNumber":    "1",
		"currentTimestamp": "1000",
		"currentBaseFee":   "0x1",
		"withdrawals":      []any{},
	}
	envMissingRandom := map[string]any{
		"currentCoinbase":  fmt.Sprintf("%#x", recipient),
		"currentGasLimit":  "0x750a163df65e8a",
		"currentNumber":    "1",
		"currentTimestamp": "1000",
		"currentBaseFee":   "0x1",
		"withdrawals":      []any{},
	}
	txs := []map[string]any{
		{
			"type":                 "0x2",
			"chainID":              "0x1",
			"gas":                  "0x5208",
			"maxFeePerGas":         "0x1",
			"maxPriorityFeePerGas": "0x0",
			"input":                "0x",
			"nonce":                "0x0",
			"descriptor":           "0x",
			"extraParams":          "0x",
			"signature":            "0x",
			"publicKey":            "0x",
			"seed":                 seedHex,
			"to":                   fmt.Sprintf("%#x", recipient),
			"value":                "0x1",
		},
	}
	writeJSON := func(name string, v any) {
		b, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			t.Fatalf("marshal %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), append(b, '\n'), 0644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	writeJSON("alloc.json", alloc)
	writeJSON("env.json", env)
	writeJSON("env-missingrandom.json", envMissingRandom)
	writeJSON("txs.json", txs)
	invalidRLPJSON, err := json.Marshal(invalidRLP)
	if err != nil {
		t.Fatalf("marshal invalid.rlp: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "invalid.rlp"), append(invalidRLPJSON, '\n'), 0644); err != nil {
		t.Fatalf("write invalid.rlp: %v", err)
	}
	withdrawals := []map[string]any{
		{
			"index":          "0x42",
			"validatorIndex": "0x43",
			"address":        qPrefixed(recipient),
			"amount":         "0x2a",
		},
	}
	writeJSON("withdrawals.json", withdrawals)

	// Invoke t8n CLI to capture the expected output (alloc+result combined).
	cmd := exec.Command(qrvm, "t8n",
		"--input.alloc", filepath.Join(dir, "alloc.json"),
		"--input.env", filepath.Join(dir, "env.json"),
		"--input.txs", filepath.Join(dir, "txs.json"),
		"--state.fork", "Zond",
		"--output.basedir", dir,
		"--output.alloc", "stdout",
		"--output.result", "stdout",
		"--output.body", "",
	)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("t8n invocation failed: %v\nstderr: %s", err, asStderr(err))
	}
	// Persist the combined alloc+result JSON as exp.json so cmpJson can load it.
	var wrapped map[string]json.RawMessage
	if err := json.Unmarshal(out, &wrapped); err != nil {
		t.Fatalf("parse t8n stdout: %v\noutput: %s", err, out)
	}
	pretty, err := json.MarshalIndent(wrapped, "", "  ")
	if err != nil {
		t.Fatalf("marshal exp.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "exp.json"), append(pretty, '\n'), 0644); err != nil {
		t.Fatalf("write exp.json: %v", err)
	}
	// Compute transaction-sender checksummed form for reference in txs.json
	senderAddr := common.Address(sender)
	t.Logf("regenerated %s/{alloc,env,txs,exp}.json (sender=%s)", dir, senderAddr.Hex())

	// --- TestT9n fixture: signed_txs.rlp + exp.json ---
	// Reuse the same signed tx we just assembled via t8n. The signed RLP is
	// emitted into dir/txs.rlp when we pass --output.body; emit it now.
	bodyCmd := exec.Command(qrvm, "t8n",
		"--input.alloc", filepath.Join(dir, "alloc.json"),
		"--input.env", filepath.Join(dir, "env.json"),
		"--input.txs", filepath.Join(dir, "txs.json"),
		"--state.fork", "Zond",
		"--output.basedir", dir,
		"--output.alloc", "",
		"--output.result", "",
		"--output.body", "signed_txs.rlp",
	)
	if out, err := bodyCmd.CombinedOutput(); err != nil {
		t.Fatalf("t8n --output.body: %v\n%s", err, out)
	}
	// Now run t9n against the RLP to capture the expected validation output.
	t9nCmd := exec.Command(qrvm, "t9n",
		"--input.txs", filepath.Join(dir, "signed_txs.rlp"),
		"--state.fork", "Zond",
	)
	t9nOut, err := t9nCmd.Output()
	if err != nil {
		t.Fatalf("t9n invocation failed: %v\nstderr: %s", err, asStderr(err))
	}
	var t9nWrapped any
	if err := json.Unmarshal(t9nOut, &t9nWrapped); err != nil {
		t.Fatalf("parse t9n stdout: %v\noutput: %s", err, t9nOut)
	}
	t9nPretty, _ := json.MarshalIndent(t9nWrapped, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "t9n_exp.json"), append(t9nPretty, '\n'), 0644); err != nil {
		t.Fatalf("write t9n_exp.json: %v", err)
	}
	t.Logf("regenerated %s/t9n_exp.json + signed_txs.rlp", dir)

	// --- TestB11r fixture: header.json + txs.rlp + exp.json ---
	header := map[string]any{
		"parentHash":       "0x0000000000000000000000000000000000000000000000000000000000000000",
		"miner":            qPrefixed(recipient),
		"stateRoot":        "0x4e8c240fc34843c9b49d936ef9e03f2422ca4cfcfc8502a38cdf3ebb92eec6b2",
		"transactionsRoot": "0x7fcad3606f4d5779972c5f4b2840da02006fb2a1cbb1a76517ec745e0a7f6123",
		"receiptsRoot":     "0xf78dfb743fbd92ade140711c8bbc542b5e307f0ab7984eff35d751969fe57efa",
		"logsBloom":        "0x" + hex.EncodeToString(make([]byte, 256)),
		"number":           "0x1",
		"gasLimit":         "0x750a163df65e8a",
		"gasUsed":          "0x5208",
		"timestamp":        "0x3e8",
		"extraData":        "0x",
		"prevRandao":       "0x0000000000000000000000000000000000000000000000000000000000000000",
		"baseFeePerGas":    "0x1",
	}
	writeJSON("header.json", header)
	b11rCmd := exec.Command(qrvm, "b11r",
		"--input.header", filepath.Join(dir, "header.json"),
		"--input.txs", filepath.Join(dir, "signed_txs.rlp"),
		"--output.basedir", dir,
		"--output.block", "stdout",
	)
	b11rOut, err := b11rCmd.Output()
	if err != nil {
		t.Fatalf("b11r invocation failed: %v\nstderr: %s", err, asStderr(err))
	}
	var b11rWrapped any
	if err := json.Unmarshal(b11rOut, &b11rWrapped); err != nil {
		t.Fatalf("parse b11r stdout: %v\noutput: %s", err, b11rOut)
	}
	b11rPretty, _ := json.MarshalIndent(b11rWrapped, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "b11r_exp.json"), append(b11rPretty, '\n'), 0644); err != nil {
		t.Fatalf("write b11r_exp.json: %v", err)
	}
	b11rWithdrawalsCmd := exec.Command(qrvm, "b11r",
		"--input.header", filepath.Join(dir, "header.json"),
		"--input.withdrawals", filepath.Join(dir, "withdrawals.json"),
		"--input.txs", filepath.Join(dir, "signed_txs.rlp"),
		"--output.basedir", dir,
		"--output.block", "stdout",
	)
	b11rWithdrawalsOut, err := b11rWithdrawalsCmd.Output()
	if err != nil {
		t.Fatalf("b11r withdrawals invocation failed: %v\nstderr: %s", err, asStderr(err))
	}
	var b11rWithdrawalsWrapped any
	if err := json.Unmarshal(b11rWithdrawalsOut, &b11rWithdrawalsWrapped); err != nil {
		t.Fatalf("parse b11r withdrawals stdout: %v\noutput: %s", err, b11rWithdrawalsOut)
	}
	b11rWithdrawalsPretty, _ := json.MarshalIndent(b11rWithdrawalsWrapped, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "b11r_withdrawals_exp.json"), append(b11rWithdrawalsPretty, '\n'), 0644); err != nil {
		t.Fatalf("write b11r_withdrawals_exp.json: %v", err)
	}
	t.Logf("regenerated %s fixtures", dir)
	regenerateLegacyNamedFixtures(t, qrvm, dir)
}

// qPrefixed returns the canonical QIP-55 Q-prefixed address expected by the
// t8ntool JSON loader.
func qPrefixed(a [64]byte) string {
	return common.Address(a).Hex()
}

func asStderr(err error) string {
	if ee, ok := err.(*exec.ExitError); ok {
		return string(ee.Stderr)
	}
	return err.Error()
}

func regenerateLegacyNamedFixtures(t *testing.T, qrvm string, simpleDir string) {
	t.Helper()

	copyFile := func(src, dst string) {
		t.Helper()
		blob, err := os.ReadFile(filepath.Join(simpleDir, src))
		if err != nil {
			t.Fatalf("read %s: %v", src, err)
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(dst), err)
		}
		if err := os.WriteFile(dst, blob, 0644); err != nil {
			t.Fatalf("write %s: %v", dst, err)
		}
	}
	writeReadme := func(dir string) {
		t.Helper()
		readmes := map[string]string{
			filepath.Join("testdata", "3"): `These files exemplify a transition where a transaction (executed on block 5) requests
the blockhash for block ` + "`1`" + `.

Fixture data is regenerated for 64-byte QRL addresses. Run ` + "`WRITE_FIXTURES=1 go test -run TestRegenerateT8nFixtures ./cmd/qrvm`" + ` to refresh.
`,
			filepath.Join("testdata", "4"): `These files exemplify a transition where a transaction (executed on block 5) requests
the blockhash for block ` + "`4`" + `, but where the hash for that block is missing.
It's expected that executing these should cause ` + "`exit`" + ` with error code ` + "`4`" + `.

Fixture data is regenerated for 64-byte QRL addresses. Run ` + "`WRITE_FIXTURES=1 go test -run TestRegenerateT8nFixtures ./cmd/qrvm`" + ` to refresh.
`,
			filepath.Join("testdata", "8"): `## EIP-2930 testing

This test contains testcases for access-list transactions. The alloc contains
a small contract that performs two SLOAD operations, plus a funded sender.

Fixture data is regenerated for 64-byte QRL addresses. Run ` + "`WRITE_FIXTURES=1 go test -run TestRegenerateT8nFixtures ./cmd/qrvm`" + ` to refresh.
`,
			filepath.Join("testdata", "9"): `## EIP-1559 testing

This test contains an EIP-1559 access-list transaction and exercises warm/cold
storage access accounting with a regenerated 64-byte QRL address fixture.

Run ` + "`WRITE_FIXTURES=1 go test -run TestRegenerateT8nFixtures ./cmd/qrvm`" + ` to refresh.
`,
			filepath.Join("testdata", "10"): `## EIP-1559 gas-limit testing

This test contains EIP-1559 transactions that exercise gas-limit rejection
behavior.

Fixture data is regenerated for 64-byte QRL addresses. Run ` + "`WRITE_FIXTURES=1 go test -run TestRegenerateT8nFixtures ./cmd/qrvm`" + ` to refresh.
`,
			filepath.Join("testdata", "11"): `## Test missing base fee

In this test, ` + "`currentBaseFee`" + ` is missing from the env portion. On a live
blockchain, the base fee is present in the header and verified as part of
header validation. In ` + "`qrvm t8n`" + `, there are no full blocks, so it must be
provided in ` + "`env`" + ` instead. When it is missing, an error is expected.

Fixture data is regenerated for 64-byte QRL addresses. Run ` + "`WRITE_FIXTURES=1 go test -run TestRegenerateT8nFixtures ./cmd/qrvm`" + ` to refresh.
`,
			filepath.Join("testdata", "12"): `## Test EIP-1559 balance plus gas cap

This test covers an EIP-1559 consensus case where the sender balance must cover
both the value transfer and ` + "`max_fee_per_gas * gas_limit`" + `.

Fixture data is regenerated for 64-byte QRL addresses. Run ` + "`WRITE_FIXTURES=1 go test -run TestRegenerateT8nFixtures ./cmd/qrvm`" + ` to refresh.
`,
			filepath.Join("testdata", "13"): `## Input transactions in RLP form

This testdata folder demonstrates how transaction input can be provided in RLP
form. See the README in the ` + "`qrvm`" + ` folder for how this is performed.

Fixture data is regenerated for 64-byte QRL addresses. Run ` + "`WRITE_FIXTURES=1 go test -run TestRegenerateT8nFixtures ./cmd/qrvm`" + ` to refresh.
`,
			filepath.Join("testdata", "18"): `# Invalid RLP

This folder contains a sample of invalid RLP, and it's expected that ` + "`t9n`" + `
handles this properly:

` + "```console" + `
$ go run . t9n --input.txs=./testdata/18/invalid.rlp --state.fork=Zond
ERROR(11): rlp: value size exceeds available input length
` + "```" + `

Run ` + "`WRITE_FIXTURES=1 go test -run TestRegenerateT8nFixtures ./cmd/qrvm`" + ` to refresh.
`,
			filepath.Join("testdata", "20"): `# Block building

This test shows how ` + "`b11r`" + ` can be used to assemble an unsealed block.

Fixture data is regenerated for 64-byte QRL addresses. Run ` + "`WRITE_FIXTURES=1 go test -run TestRegenerateT8nFixtures ./cmd/qrvm`" + ` to refresh.
`,
		}
		readme := []byte("Regenerated for 64-byte QRL addresses. Run WRITE_FIXTURES=1 go test -run TestRegenerateT8nFixtures ./cmd/qrvm to refresh.\n")
		if custom, ok := readmes[dir]; ok {
			readme = []byte(custom)
		}
		for _, name := range []string{"readme.md", "README.md"} {
			path := filepath.Join(dir, name)
			if _, err := os.Stat(path); err == nil {
				if err := os.WriteFile(path, readme, 0644); err != nil {
					t.Fatalf("write %s: %v", path, err)
				}
			}
		}
	}

	for _, name := range []string{"1"} {
		dir := filepath.Join("testdata", name)
		copyFile("alloc.json", filepath.Join(dir, "alloc.json"))
		copyFile("env.json", filepath.Join(dir, "env.json"))
		copyFile("txs.json", filepath.Join(dir, "txs.json"))
		writeReadme(dir)
	}
	for _, name := range []string{"1", "3", "13", "24", "25", "26"} {
		copyFile("exp.json", filepath.Join("testdata", name, "exp.json"))
	}
	copyFile("env-missingrandom.json", filepath.Join("testdata", "24", "env-missingrandom.json"))
	copyFile("signed_txs.rlp", filepath.Join("testdata", "13", "signed_txs.rlp"))
	signedResultCmd := exec.Command(qrvm, "t8n",
		"--input.alloc", filepath.Join("testdata", "13", "alloc.json"),
		"--input.env", filepath.Join("testdata", "13", "env.json"),
		"--input.txs", filepath.Join("testdata", "13", "signed_txs.rlp"),
		"--state.fork", "Zond",
		"--output.alloc", "",
		"--output.result", "stdout",
		"--output.body", "",
	)
	signedResultOut, err := signedResultCmd.Output()
	if err != nil {
		t.Fatalf("t8n signed result invocation failed: %v\nstderr: %s", err, asStderr(err))
	}
	if err := os.WriteFile(filepath.Join("testdata", "13", "exp2.json"), signedResultOut, 0644); err != nil {
		t.Fatalf("write testdata/13/exp2.json: %v", err)
	}

	for _, name := range []string{"15", "16", "17"} {
		dir := filepath.Join("testdata", name)
		copyFile("signed_txs.rlp", filepath.Join(dir, "signed_txs.rlp"))
	}
	for _, name := range []string{"16", "17"} {
		copyFile("t9n_exp.json", filepath.Join("testdata", name, "exp.json"))
	}
	copyFile("txs.json", filepath.Join("testdata", "16", "unsigned_txs.json"))
	copyFile("t9n_exp.json", filepath.Join("testdata", "15", "exp2.json"))
	copyFile("invalid.rlp", filepath.Join("testdata", "18", "invalid.rlp"))
	writeReadme(filepath.Join("testdata", "18"))

	for _, name := range []string{"20", "27"} {
		dir := filepath.Join("testdata", name)
		copyFile("header.json", filepath.Join(dir, "header.json"))
		copyFile("signed_txs.rlp", filepath.Join(dir, "txs.rlp"))
		copyFile("b11r_exp.json", filepath.Join(dir, "exp.json"))
		writeReadme(dir)
	}
	copyFile("withdrawals.json", filepath.Join("testdata", "20", "withdrawals.json"))
	copyFile("withdrawals.json", filepath.Join("testdata", "27", "withdrawals.json"))
	copyFile("b11r_withdrawals_exp.json", filepath.Join("testdata", "27", "exp.json"))
}
