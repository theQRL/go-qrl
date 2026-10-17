// qrl-migrate exports a stopped legacy mainnet state, builds a migration genesis,
// and prepares messages/calldata without handling private keys or broadcasting.
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"

	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/common/hexutil"
	"github.com/theQRL/go-qrl/core"
	"github.com/theQRL/go-qrl/migration"
)

func main() {
	if err := run(os.Args[1:]); err != nil && !errors.Is(err, flag.ErrHelp) {
		fmt.Fprintln(os.Stderr, "qrl-migrate:", err)
		os.Exit(1)
	}
}

func readJSON(path string, target any) error {
	if path == "" {
		return errors.New("input file path is required")
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	d := json.NewDecoder(f)
	d.DisallowUnknownFields()
	if err := d.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return errors.New("extra data after JSON document")
	}
	return nil
}

// Create outputs atomically and refuse to overwrite any existing snapshot,
// genesis, signing request or transaction. Neither the input DB nor input files
// are changed. Stdout is available explicitly as --out -.
func writeJSON(path string, value any) error {
	return writeOutput(path, func(w io.Writer) error {
		e := json.NewEncoder(w)
		e.SetIndent("", "  ")
		return e.Encode(value)
	})
}

func writeOutput(path string, write func(io.Writer) error) error {
	if path == "" {
		return errors.New("--out is required (use - for stdout)")
	}
	if path == "-" {
		w := bufio.NewWriterSize(os.Stdout, 64<<10)
		if err := write(w); err != nil {
			return err
		}
		return w.Flush()
	}
	if _, err := os.Lstat(path); err == nil {
		return fmt.Errorf("output already exists: %s", path)
	} else if !os.IsNotExist(err) {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".qrl-migrate-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	w := bufio.NewWriterSize(f, 64<<10)
	if err = write(w); err != nil {
		return err
	}
	if err = w.Flush(); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Link(f.Name(), path); err != nil {
		return fmt.Errorf("create output without overwriting: %w", err)
	}
	return nil
}

func run(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: qrl-migrate {export|genesis|message|calldata} [options]")
	}
	if args[0] == "--help" || args[0] == "-h" {
		fmt.Fprintln(os.Stdout, "usage: qrl-migrate {export|genesis|message|calldata} [options]\nUse COMMAND --help for options. See migration/README.md for the complete flow.")
		return flag.ErrHelp
	}
	flags := flag.NewFlagSet(args[0], flag.ContinueOnError)
	out := flags.String("out", "", "new output JSON file, or - for stdout")
	snapshotFile := flags.String("snapshot", "", "snapshot JSON from export")
	statePath := flags.String("state", "", "stopped QRL mainnet LevelDB directory (export)")
	workDir := flags.String("work-dir", "", "scratch-disk parent for temporary account index (default: OS temp directory)")
	genesisFile := flags.String("genesis", "", "base QRVM genesis JSON (genesis)")
	contractText := flags.String("contract", "", "reserved 64-byte QRL contract address")
	chainText := flags.String("chain-id", "", "target QRVM chain ID (message/calldata)")
	source := flags.String("source", "", "legacy QRL source address (message/calldata)")
	recipientText := flags.String("recipient", "", "new ML-DSA-87 wallet address (message/calldata)")
	signatureFile := flags.String("signatures", "", "JSON array of public_key/signature hex pairs (calldata)")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected positional arguments")
	}
	if *out == "" {
		return errors.New("--out is required")
	}
	if args[0] == "export" {
		if *statePath == "" {
			return errors.New("--state must point to a stopped mainnet LevelDB")
		}
		return writeOutput(*out, func(w io.Writer) error {
			s, err := migration.Export(*statePath, *workDir)
			if err != nil {
				return err
			}
			defer s.Close()
			return s.WriteJSON(w)
		})
	}
	if args[0] != "genesis" && args[0] != "message" && args[0] != "calldata" {
		return fmt.Errorf("unknown command %q", args[0])
	}
	s, err := migration.OpenSnapshot(*snapshotFile, *workDir)
	if err != nil {
		return err
	}
	defer s.Close()
	contract, err := common.NewAddressFromString(*contractText)
	if err != nil {
		return fmt.Errorf("--contract: %w", err)
	}
	if args[0] == "genesis" {
		var g core.Genesis
		if err := readJSON(*genesisFile, &g); err != nil {
			return err
		}
		return writeOutput(*out, func(w io.Writer) error { return s.WriteGenesis(w, &g, contract) })
	}
	chainID, ok := new(big.Int).SetString(*chainText, 10)
	if !ok {
		return errors.New("--chain-id must be a positive decimal integer")
	}
	recipient, err := common.NewAddressFromString(*recipientText)
	if err != nil {
		return fmt.Errorf("--recipient: %w", err)
	}
	message, err := s.ClaimMessage(chainID, contract, *source, recipient)
	if err != nil {
		return err
	}
	if args[0] == "message" {
		return writeJSON(*out, struct {
			Source    string         `json:"source"`
			Recipient common.Address `json:"recipient"`
			Contract  common.Address `json:"contract"`
			ChainID   string         `json:"chain_id"`
			Snapshot  string         `json:"snapshot_commitment"`
			Message   hexutil.Bytes  `json:"message"`
		}{*source, recipient, contract, chainID.String(), s.Metadata().Commitment, message})
	}
	var signatures []migration.Signature
	if err := readJSON(*signatureFile, &signatures); err != nil {
		return err
	}
	tx, err := s.PrepareClaim(chainID, contract, *source, recipient, signatures)
	if err != nil {
		return err
	}
	return writeJSON(*out, tx)
}
