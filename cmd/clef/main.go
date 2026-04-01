// Copyright 2018 The go-ethereum Authors
// This file is part of go-ethereum.
//
// go-ethereum is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// go-ethereum is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with go-ethereum. If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/mattn/go-colorable"
	"github.com/mattn/go-isatty"
	"github.com/theQRL/go-qrl/accounts"
	"github.com/theQRL/go-qrl/accounts/keystore"
	"github.com/theQRL/go-qrl/cmd/utils"
	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/common/hexutil"
	"github.com/theQRL/go-qrl/core/types"
	"github.com/theQRL/go-qrl/crypto"
	"github.com/theQRL/go-qrl/crypto/pqcrypto/wallet"
	"github.com/theQRL/go-qrl/internal/flags"
	"github.com/theQRL/go-qrl/internal/qrlapi"
	"github.com/theQRL/go-qrl/log"
	"github.com/theQRL/go-qrl/node"
	"github.com/theQRL/go-qrl/params"
	"github.com/theQRL/go-qrl/rpc"
	"github.com/theQRL/go-qrl/signer/core"
	"github.com/theQRL/go-qrl/signer/core/apitypes"
	"github.com/theQRL/go-qrl/signer/fourbyte"
	"github.com/theQRL/go-qrl/signer/rules"
	"github.com/theQRL/go-qrl/signer/storage"
	"github.com/urfave/cli/v2"
)

const legalWarning = `
WARNING!

Clef is an account management tool. It may, like any software, contain bugs.

Please take care to
- backup your keystore files,
- verify that the keystore(s) can be opened with your password.

Clef is distributed in the hope that it will be useful, but WITHOUT ANY WARRANTY;
without even the implied warranty of MERCHANTABILITY or FITNESS FOR A PARTICULAR
PURPOSE. See the GNU General Public License for more details.
`

var (
	logLevelFlag = &cli.IntFlag{
		Name:  "loglevel",
		Value: 3,
		Usage: "log level to emit to the screen",
	}
	advancedMode = &cli.BoolFlag{
		Name:  "advanced",
		Usage: "If enabled, issues warnings instead of rejections for suspicious requests. Default off",
	}
	acceptFlag = &cli.BoolFlag{
		Name:  "suppress-bootwarn",
		Usage: "If set, does not show the warning during boot",
	}
	keystoreFlag = &cli.StringFlag{
		Name:  "keystore",
		Value: filepath.Join(node.DefaultDataDir(), "keystore"),
		Usage: "Directory for the keystore",
	}
	configdirFlag = &cli.StringFlag{
		Name:  "configdir",
		Value: DefaultConfigDir(),
		Usage: "Directory for Clef configuration",
	}
	chainIdFlag = &cli.Int64Flag{
		Name:  "chainid",
		Value: params.MainnetChainConfig.ChainID.Int64(),
		Usage: "Chain id to use for signing (1=mainnet)",
	}
	rpcPortFlag = &cli.IntFlag{
		Name:     "http.port",
		Usage:    "HTTP-RPC server listening port",
		Value:    node.DefaultHTTPPort + 5,
		Category: flags.APICategory,
	}
	signerSecretFlag = &cli.StringFlag{
		Name:  "signersecret",
		Usage: "A file containing the (encrypted) master seed to encrypt Clef data, e.g. keystore credentials and ruleset hash",
	}
	customDBFlag = &cli.StringFlag{
		Name:  "4bytedb-custom",
		Usage: "File used for writing new 4byte-identifiers submitted via API",
		Value: "./4byte-custom.json",
	}
	auditLogFlag = &cli.StringFlag{
		Name:  "auditlog",
		Usage: "File used to emit audit logs. Set to \"\" to disable",
		Value: "audit.log",
	}
	ruleFlag = &cli.StringFlag{
		Name:  "rules",
		Usage: "Path to the rule file to auto-authorize requests with",
	}
	stdiouiFlag = &cli.BoolFlag{
		Name: "stdio-ui",
		Usage: "Use STDIN/STDOUT as a channel for an external UI. " +
			"This means that an STDIN/STDOUT is used for RPC-communication with a e.g. a graphical user " +
			"interface, and can be used when Clef is started by an external process.",
	}
	testFlag = &cli.BoolFlag{
		Name:  "stdio-ui-test",
		Usage: "Mechanism to test interface between Clef and UI. Requires 'stdio-ui'.",
	}
	initCommand = &cli.Command{
		Action:    initializeSecrets,
		Name:      "init",
		Usage:     "Initialize the signer, generate secret storage",
		ArgsUsage: "",
		Flags: []cli.Flag{
			logLevelFlag,
			configdirFlag,
		},
		Description: `
The init command generates a master seed which Clef can use to store credentials and data needed for
the rule-engine to work.`,
	}
	attestCommand = &cli.Command{
		Action:    attestFile,
		Name:      "attest",
		Usage:     "Attest that a js-file is to be used",
		ArgsUsage: "<sha256sum>",
		Flags: []cli.Flag{
			logLevelFlag,
			configdirFlag,
			signerSecretFlag,
		},
		Description: `
The attest command stores the sha256 of the rule.js-file that you want to use for automatic processing of
incoming requests.

Whenever you make an edit to the rule file, you need to use attestation to tell
Clef that the file is 'safe' to execute.`,
	}
	setCredentialCommand = &cli.Command{
		Action:    setCredential,
		Name:      "setpw",
		Usage:     "Store a credential for a keystore file",
		ArgsUsage: "<address>",
		Flags: []cli.Flag{
			logLevelFlag,
			configdirFlag,
			signerSecretFlag,
		},
		Description: `
The setpw command stores a password for a given address (keyfile).
`}
	delCredentialCommand = &cli.Command{
		Action:    removeCredential,
		Name:      "delpw",
		Usage:     "Remove a credential for a keystore file",
		ArgsUsage: "<address>",
		Flags: []cli.Flag{
			logLevelFlag,
			configdirFlag,
			signerSecretFlag,
		},
		Description: `
The delpw command removes a password for a given address (keyfile).
`}
	newAccountCommand = &cli.Command{
		Action:    newAccount,
		Name:      "newaccount",
		Usage:     "Create a new account",
		ArgsUsage: "",
		Flags: []cli.Flag{
			logLevelFlag,
			keystoreFlag,
			utils.LightKDFFlag,
			acceptFlag,
		},
		Description: `
The newaccount command creates a new keystore-backed account. It is a convenience-method
which can be used in lieu of an external UI.
`}
	gendocCommand = &cli.Command{
		Action: GenDoc,
		Name:   "gendoc",
		Usage:  "Generate documentation about json-rpc format",
		Description: `
The gendoc generates example structures of the json-rpc communication types.
`}
	listAccountsCommand = &cli.Command{
		Action: listAccounts,
		Name:   "list-accounts",
		Usage:  "List accounts in the keystore",
		Flags: []cli.Flag{
			logLevelFlag,
			keystoreFlag,
			utils.LightKDFFlag,
			acceptFlag,
		},
		Description: `
	Lists the accounts in the keystore.
	`}
	listWalletsCommand = &cli.Command{
		Action: listWallets,
		Name:   "list-wallets",
		Usage:  "List wallets known to Clef",
		Flags: []cli.Flag{
			logLevelFlag,
			keystoreFlag,
			utils.LightKDFFlag,
			acceptFlag,
		},
		Description: `
	Lists the wallets known to Clef.
	`}
	importRawCommand = &cli.Command{
		Action:    accountImport,
		Name:      "importraw",
		Usage:     "Import a hex-encoded seed.",
		ArgsUsage: "<file>",
		Flags: []cli.Flag{
			logLevelFlag,
			keystoreFlag,
			utils.LightKDFFlag,
			utils.PasswordFileFlag,
			acceptFlag,
		},
		Description: `
Imports a seed from <file> and creates a new account.
Prints the address.
The file is assumed to contain a seed in hexadecimal format.
The account is saved in encrypted format, you are prompted for a password.
`}
)

var app = flags.NewApp("Manage QRL account operations")

func init() {
	app.Name = "Clef"
	app.Flags = []cli.Flag{
		logLevelFlag,
		keystoreFlag,
		configdirFlag,
		chainIdFlag,
		utils.LightKDFFlag,
		utils.HTTPListenAddrFlag,
		utils.HTTPVirtualHostsFlag,
		utils.IPCDisabledFlag,
		utils.IPCPathFlag,
		utils.HTTPEnabledFlag,
		rpcPortFlag,
		signerSecretFlag,
		customDBFlag,
		auditLogFlag,
		ruleFlag,
		stdiouiFlag,
		testFlag,
		advancedMode,
		acceptFlag,
	}
	app.Action = signer
	app.Commands = []*cli.Command{initCommand,
		attestCommand,
		setCredentialCommand,
		delCredentialCommand,
		newAccountCommand,
		importRawCommand,
		gendocCommand,
		listAccountsCommand,
		listWalletsCommand,
	}
}

func main() {
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func initializeSecrets(c *cli.Context) error {
	// Get past the legal message
	if err := initialize(c); err != nil {
		return err
	}
	// Ensure the master key does not yet exist, we're not willing to overwrite
	configDir := c.String(configdirFlag.Name)
	if err := os.Mkdir(configDir, 0700); err != nil && !os.IsExist(err) {
		return err
	}
	location := filepath.Join(configDir, "masterseed.json")
	if _, err := os.Stat(location); err == nil {
		return fmt.Errorf("master key %v already exists, will not overwrite", location)
	}
	// Key file does not exist yet, generate a new one and encrypt it
	masterSeed := make([]byte, 256)
	num, err := io.ReadFull(rand.Reader, masterSeed)
	if err != nil {
		return err
	}
	if num != len(masterSeed) {
		return errors.New("failed to read enough random")
	}
	t, m, p := keystore.StandardArgon2idT, keystore.StandardArgon2idM, keystore.StandardArgon2idP
	if c.Bool(utils.LightKDFFlag.Name) {
		t, m, p = keystore.LightArgon2idT, keystore.LightArgon2idM, keystore.LightArgon2idP
	}
	text := "The master seed of clef will be locked with a password.\nPlease specify a password. Do not forget this password!"
	var password string
	for {
		password = utils.GetPassPhrase(text, true)
		if err := core.ValidatePasswordFormat(password); err != nil {
			fmt.Printf("invalid password: %v\n", err)
		} else {
			fmt.Println()
			break
		}
	}
	cipherSeed, err := encryptSeed(masterSeed, []byte(password), t, m, p)
	if err != nil {
		return fmt.Errorf("failed to encrypt master seed: %v", err)
	}
	// Double check the master key path to ensure nothing wrote there in between
	if err = os.Mkdir(configDir, 0700); err != nil && !os.IsExist(err) {
		return err
	}
	if _, err := os.Stat(location); err == nil {
		return fmt.Errorf("master key %v already exists, will not overwrite", location)
	}
	// Write the file and print the usual warning message
	if err = os.WriteFile(location, cipherSeed, 0400); err != nil {
		return err
	}
	fmt.Printf("A master seed has been generated into %s\n", location)
	fmt.Printf(`
This is required to be able to store credentials, such as:
* Passwords for keystores (used by rule engine)
* Storage for JavaScript auto-signing rules
* Hash of JavaScript rule-file

You should treat 'masterseed.json' with utmost secrecy and make a backup of it!
* The password is necessary but not enough, you need to back up the master seed too!
* The master seed does not contain your accounts, those need to be backed up separately!

`)
	return nil
}

func attestFile(ctx *cli.Context) error {
	if ctx.NArg() < 1 {
		utils.Fatalf("This command requires an argument.")
	}
	if err := initialize(ctx); err != nil {
		return err
	}

	stretchedKey, err := readMasterKey(ctx, nil)
	if err != nil {
		utils.Fatalf(err.Error())
	}
	configDir := ctx.String(configdirFlag.Name)
	vaultLocation := filepath.Join(configDir, common.Bytes2Hex(crypto.Keccak256([]byte("vault"), stretchedKey)[:10]))
	confKey := crypto.Keccak256([]byte("config"), stretchedKey)

	// Initialize the encrypted storages
	configStorage := storage.NewAESEncryptedStorage(filepath.Join(vaultLocation, "config.json"), confKey)
	val := ctx.Args().First()
	configStorage.Put("ruleset_sha256", val)
	log.Info("Ruleset attestation updated", "sha256", val)
	return nil
}

func initInternalApi(c *cli.Context) (*core.UIServerAPI, core.UIClientAPI, error) {
	if err := initialize(c); err != nil {
		return nil, nil, err
	}
	var (
		ui                        = core.NewCommandlineUI()
		pwStorage storage.Storage = &storage.NoStorage{}
		ksLoc                     = c.String(keystoreFlag.Name)
		lightKdf                  = c.Bool(utils.LightKDFFlag.Name)
	)
	am := core.StartClefAccountManager(ksLoc /*false,*/, lightKdf /*""*/)
	api := core.NewSignerAPI(am, 0 /*false,*/, ui, nil, false, pwStorage)
	internalApi := core.NewUIServerAPI(api)
	return internalApi, ui, nil
}

func setCredential(ctx *cli.Context) error {
	if ctx.NArg() < 1 {
		utils.Fatalf("This command requires an address to be passed as an argument")
	}
	if err := initialize(ctx); err != nil {
		return err
	}
	addressStr := ctx.Args().First()
	address, err := common.NewAddressFromString(addressStr)
	if err != nil {
		utils.Fatalf("Invalid address specified: %s", addressStr)
	}

	password := utils.GetPassPhrase("Please enter a password to store for this address:", true)
	fmt.Println()

	stretchedKey, err := readMasterKey(ctx, nil)
	if err != nil {
		utils.Fatalf(err.Error())
	}
	configDir := ctx.String(configdirFlag.Name)
	vaultLocation := filepath.Join(configDir, common.Bytes2Hex(crypto.Keccak256([]byte("vault"), stretchedKey)[:10]))
	pwkey := crypto.Keccak256([]byte("credentials"), stretchedKey)

	pwStorage := storage.NewAESEncryptedStorage(filepath.Join(vaultLocation, "credentials.json"), pwkey)
	pwStorage.Put(address.Hex(), password)

	log.Info("Credential store updated", "set", address)
	return nil
}

func removeCredential(ctx *cli.Context) error {
	if ctx.NArg() < 1 {
		utils.Fatalf("This command requires an address to be passed as an argument")
	}
	if err := initialize(ctx); err != nil {
		return err
	}
	addressStr := ctx.Args().First()
	address, err := common.NewAddressFromString(addressStr)
	if err != nil {
		utils.Fatalf("Invalid address specified: %s", addressStr)
	}

	stretchedKey, err := readMasterKey(ctx, nil)
	if err != nil {
		utils.Fatalf(err.Error())
	}
	configDir := ctx.String(configdirFlag.Name)
	vaultLocation := filepath.Join(configDir, common.Bytes2Hex(crypto.Keccak256([]byte("vault"), stretchedKey)[:10]))
	pwkey := crypto.Keccak256([]byte("credentials"), stretchedKey)

	pwStorage := storage.NewAESEncryptedStorage(filepath.Join(vaultLocation, "credentials.json"), pwkey)
	pwStorage.Del(address.Hex())

	log.Info("Credential store updated", "unset", address)
	return nil
}

func initialize(c *cli.Context) error {
	// Set up the logger to print everything
	logOutput := os.Stdout
	if c.Bool(stdiouiFlag.Name) {
		logOutput = os.Stderr
		// If using the stdioui, we can't do the 'confirm'-flow
		if !c.Bool(acceptFlag.Name) {
			fmt.Fprint(logOutput, legalWarning)
		}
	} else if !c.Bool(acceptFlag.Name) {
		if !confirm(legalWarning) {
			return errors.New("aborted by user")
		}
		fmt.Println()
	}
	usecolor := (isatty.IsTerminal(os.Stderr.Fd()) || isatty.IsCygwinTerminal(os.Stderr.Fd())) && os.Getenv("TERM") != "dumb"
	output := io.Writer(logOutput)
	if usecolor {
		output = colorable.NewColorable(logOutput)
	}
	verbosity := log.FromLegacyLevel(c.Int(logLevelFlag.Name))
	log.SetDefault(log.NewLogger(log.NewTerminalHandlerWithLevel(output, verbosity, usecolor)))

	return nil
}

func newAccount(c *cli.Context) error {
	internalApi, _, err := initInternalApi(c)
	if err != nil {
		return err
	}
	addr, err := internalApi.New(context.Background())
	if err == nil {
		fmt.Printf("Generated account %v\n", addr.String())
	}
	return err
}

func listAccounts(c *cli.Context) error {
	internalApi, _, err := initInternalApi(c)
	if err != nil {
		return err
	}
	accs, err := internalApi.ListAccounts(context.Background())
	if err != nil {
		return err
	}
	if len(accs) == 0 {
		fmt.Println("\nThe keystore is empty.")
	}
	fmt.Println()
	for _, account := range accs {
		fmt.Printf("%v (%v)\n", account.Address, account.URL)
	}
	return err
}

func listWallets(c *cli.Context) error {
	internalApi, _, err := initInternalApi(c)
	if err != nil {
		return err
	}
	wallets := internalApi.ListWallets()
	if len(wallets) == 0 {
		fmt.Println("\nThere are no wallets.")
	}
	fmt.Println()
	for i, wallet := range wallets {
		fmt.Printf("- Wallet %d at %v (%v %v)\n", i, wallet.URL, wallet.Status, wallet.Failure)
		for j, acc := range wallet.Accounts {
			fmt.Printf("  -Account %d: %v (%v)\n", j, acc.Address, acc.URL)
		}
		fmt.Println()
	}
	return nil
}

// accountImport imports a raw hexadecimal seed via CLI.
func accountImport(c *cli.Context) error {
	if c.Args().Len() != 1 {
		return errors.New("<file> must be given as first argument")
	}
	internalApi, ui, err := initInternalApi(c)
	if err != nil {
		return err
	}

	hexSeed, err := wallet.ReadSeedFromFile(c.Args().First())
	if err != nil {
		return err
	}

	var first string
	pwdList := utils.MakePasswordList(c)
	if len(pwdList) > 0 {
		first = pwdList[0]
	} else {
		var err error
		readPw := func(prompt string) (string, error) {
			resp, err := ui.OnInputRequired(core.UserInputRequest{
				Title:      "Password",
				Prompt:     prompt,
				IsPassword: true,
			})
			if err != nil {
				return "", err
			}
			return resp.Text, nil
		}
		first, err = readPw("Please enter a password for the imported account")
		if err != nil {
			return err
		}
		second, err := readPw("Please repeat the password you just entered")
		if err != nil {
			return err
		}
		if first != second {
			//lint:ignore ST1005 This is a message for the user
			return errors.New("Passwords do not match")
		}
	}

	acc, err := internalApi.ImportRawWallet(hexSeed, first)
	if err != nil {
		return err
	}
	ui.ShowInfo(fmt.Sprintf(`Key imported:
  Address %s
  Keystore file: %v

The key is now encrypted; losing the password will result in permanently losing
access to the key and all associated funds!

Make sure to backup keystore and passwords in a safe location.`,
		acc.Address, acc.URL.Path))
	return nil
}

// ipcEndpoint resolves an IPC endpoint based on a configured value, taking into
// account the set data folders as well as the designated platform we're currently
// running on.
func ipcEndpoint(ipcPath, datadir string) string {
	// On windows we can only use plain top-level pipes
	if runtime.GOOS == "windows" {
		if strings.HasPrefix(ipcPath, `\\.\pipe\`) {
			return ipcPath
		}
		return `\\.\pipe\` + ipcPath
	}
	// Resolve names into the data directory full paths otherwise
	if filepath.Base(ipcPath) == ipcPath {
		if datadir == "" {
			return filepath.Join(os.TempDir(), ipcPath)
		}
		return filepath.Join(datadir, ipcPath)
	}
	return ipcPath
}

func signer(c *cli.Context) error {
	// If we have some unrecognized command, bail out
	if c.NArg() > 0 {
		return fmt.Errorf("invalid command: %q", c.Args().First())
	}
	if err := initialize(c); err != nil {
		return err
	}
	var (
		ui core.UIClientAPI
	)
	if c.Bool(stdiouiFlag.Name) {
		log.Info("Using stdin/stdout as UI-channel")
		ui = core.NewStdIOUI()
	} else {
		log.Info("Using CLI as UI-channel")
		ui = core.NewCommandlineUI()
	}
	// 4bytedb data
	fourByteLocal := c.String(customDBFlag.Name)
	db, err := fourbyte.NewWithFile(fourByteLocal)
	if err != nil {
		utils.Fatalf(err.Error())
	}
	embeds, locals := db.Size()
	log.Info("Loaded 4byte database", "embeds", embeds, "locals", locals, "local", fourByteLocal)

	var (
		api       core.ExternalAPI
		pwStorage storage.Storage = &storage.NoStorage{}
	)
	configDir := c.String(configdirFlag.Name)
	if stretchedKey, err := readMasterKey(c, ui); err != nil {
		log.Warn("Failed to open master, rules disabled", "err", err)
	} else {
		vaultLocation := filepath.Join(configDir, common.Bytes2Hex(crypto.Keccak256([]byte("vault"), stretchedKey)[:10]))

		// Generate domain specific keys
		pwkey := crypto.Keccak256([]byte("credentials"), stretchedKey)
		jskey := crypto.Keccak256([]byte("jsstorage"), stretchedKey)
		confkey := crypto.Keccak256([]byte("config"), stretchedKey)

		// Initialize the encrypted storages
		pwStorage = storage.NewAESEncryptedStorage(filepath.Join(vaultLocation, "credentials.json"), pwkey)
		jsStorage := storage.NewAESEncryptedStorage(filepath.Join(vaultLocation, "jsstorage.json"), jskey)
		configStorage := storage.NewAESEncryptedStorage(filepath.Join(vaultLocation, "config.json"), confkey)

		// Do we have a rule-file?
		if ruleFile := c.String(ruleFlag.Name); ruleFile != "" {
			ruleJS, err := os.ReadFile(ruleFile)
			if err != nil {
				log.Warn("Could not load rules, disabling", "file", ruleFile, "err", err)
			} else {
				shasum := sha256.Sum256(ruleJS)
				foundShaSum := hex.EncodeToString(shasum[:])
				storedShasum, _ := configStorage.Get("ruleset_sha256")
				if storedShasum != foundShaSum {
					log.Warn("Rule hash not attested, disabling", "hash", foundShaSum, "attested", storedShasum)
				} else {
					// Initialize rules
					ruleEngine, err := rules.NewRuleEvaluator(ui, jsStorage)
					if err != nil {
						utils.Fatalf(err.Error())
					}
					ruleEngine.Init(string(ruleJS))
					ui = ruleEngine
					log.Info("Rule engine configured", "file", c.String(ruleFlag.Name))
				}
			}
		}
	}
	var (
		chainId  = c.Int64(chainIdFlag.Name)
		ksLoc    = c.String(keystoreFlag.Name)
		lightKdf = c.Bool(utils.LightKDFFlag.Name)
		advanced = c.Bool(advancedMode.Name)
	)
	log.Info("Starting signer", "chainid", chainId, "keystore", ksLoc,
		"light-kdf", lightKdf, "advanced", advanced)
	am := core.StartClefAccountManager(ksLoc, lightKdf)
	defer am.Close()
	apiImpl := core.NewSignerAPI(am, chainId, ui, db, advanced, pwStorage)

	// Establish the bidirectional communication, by creating a new UI backend and registering
	// it with the UI.
	ui.RegisterUIServer(core.NewUIServerAPI(apiImpl))
	api = apiImpl

	// Audit logging
	if logfile := c.String(auditLogFlag.Name); logfile != "" {
		api, err = core.NewAuditLogger(logfile, api)
		if err != nil {
			utils.Fatalf(err.Error())
		}
		log.Info("Audit logs configured", "file", logfile)
	}
	// register signer API with server
	var (
		extapiURL = "n/a"
		ipcapiURL = "n/a"
	)
	rpcAPI := []rpc.API{
		{
			Namespace: "account",
			Service:   api,
		},
	}
	if c.Bool(utils.HTTPEnabledFlag.Name) {
		vhosts := utils.SplitAndTrim(c.String(utils.HTTPVirtualHostsFlag.Name))
		cors := utils.SplitAndTrim(c.String(utils.HTTPCORSDomainFlag.Name))

		srv := rpc.NewServer()
		srv.SetBatchLimits(node.DefaultConfig.BatchRequestLimit, node.DefaultConfig.BatchResponseMaxSize)
		err := node.RegisterApis(rpcAPI, []string{"account"}, srv)
		if err != nil {
			utils.Fatalf("Could not register API: %w", err)
		}
		handler := node.NewHTTPHandlerStack(srv, cors, vhosts, nil)

		// set port
		port := c.Int(rpcPortFlag.Name)

		// start http server
		httpEndpoint := net.JoinHostPort(c.String(utils.HTTPListenAddrFlag.Name), fmt.Sprintf("%d", port))
		httpServer, addr, err := node.StartHTTPEndpoint(httpEndpoint, rpc.DefaultHTTPTimeouts, handler)
		if err != nil {
			utils.Fatalf("Could not start RPC api: %v", err)
		}
		extapiURL = fmt.Sprintf("http://%v/", addr)
		log.Info("HTTP endpoint opened", "url", extapiURL)

		defer func() {
			// Don't bother imposing a timeout here.
			httpServer.Shutdown(context.Background())
			log.Info("HTTP endpoint closed", "url", extapiURL)
		}()
	}
	if !c.Bool(utils.IPCDisabledFlag.Name) {
		givenPath := c.String(utils.IPCPathFlag.Name)
		ipcapiURL = ipcEndpoint(filepath.Join(givenPath, "clef.ipc"), configDir)
		listener, _, err := rpc.StartIPCEndpoint(ipcapiURL, rpcAPI)
		if err != nil {
			utils.Fatalf("Could not start IPC api: %v", err)
		}
		log.Info("IPC endpoint opened", "url", ipcapiURL)
		defer func() {
			listener.Close()
			log.Info("IPC endpoint closed", "url", ipcapiURL)
		}()
	}
	if c.Bool(testFlag.Name) {
		log.Info("Performing UI test")
		go testExternalUI(apiImpl)
	}
	ui.OnSignerStartup(core.StartupInfo{
		Info: map[string]any{
			"intapi_version": core.InternalAPIVersion,
			"extapi_version": core.ExternalAPIVersion,
			"extapi_http":    extapiURL,
			"extapi_ipc":     ipcapiURL,
		}})

	abortChan := make(chan os.Signal, 1)
	signal.Notify(abortChan, os.Interrupt)

	sig := <-abortChan
	log.Info("Exiting...", "signal", sig)

	return nil
}

// DefaultConfigDir is the default config directory to use for the vaults and other
// persistence requirements.
func DefaultConfigDir() string {
	// Try to place the data folder in the user's home dir
	home := flags.HomeDir()
	if home != "" {
		if runtime.GOOS == "darwin" {
			return filepath.Join(home, "Library", "Signer")
		} else if runtime.GOOS == "windows" {
			appdata := os.Getenv("APPDATA")
			if appdata != "" {
				return filepath.Join(appdata, "Signer")
			}
			return filepath.Join(home, "AppData", "Roaming", "Signer")
		}
		return filepath.Join(home, ".clef")
	}
	// As we cannot guess a stable location, return empty and handle later
	return ""
}

func readMasterKey(ctx *cli.Context, ui core.UIClientAPI) ([]byte, error) {
	var (
		file      string
		configDir = ctx.String(configdirFlag.Name)
	)
	if ctx.IsSet(signerSecretFlag.Name) {
		file = ctx.String(signerSecretFlag.Name)
	} else {
		file = filepath.Join(configDir, "masterseed.json")
	}
	if err := checkFile(file); err != nil {
		return nil, err
	}
	cipherKey, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var password string
	// If ui is not nil, get the password from ui.
	if ui != nil {
		resp, err := ui.OnInputRequired(core.UserInputRequest{
			Title:      "Master Password",
			Prompt:     "Please enter the password to decrypt the master seed",
			IsPassword: true})
		if err != nil {
			return nil, err
		}
		password = resp.Text
	} else {
		password = utils.GetPassPhrase("Decrypt master seed of clef", false)
	}
	masterSeed, err := decryptSeed(cipherKey, password)
	if err != nil {
		return nil, errors.New("failed to decrypt the master seed of clef")
	}
	if len(masterSeed) < 256 {
		return nil, fmt.Errorf("master seed of insufficient length, expected >255 bytes, got %d", len(masterSeed))
	}
	// Create vault location
	vaultLocation := filepath.Join(configDir, common.Bytes2Hex(crypto.Keccak256([]byte("vault"), masterSeed)[:10]))
	err = os.Mkdir(vaultLocation, 0700)
	if err != nil && !os.IsExist(err) {
		return nil, err
	}
	return masterSeed, nil
}

// checkFile is a convenience function to check if a file
// * exists
// * is mode 0400 (unix only)
func checkFile(filename string) error {
	info, err := os.Stat(filename)
	if err != nil {
		return fmt.Errorf("failed stat on %s: %v", filename, err)
	}
	// Check the unix permission bits
	// However, on windows, we cannot use the unix perm-bits, see
	// https://github.com/theQRL/go-qrl/issues/20123
	if runtime.GOOS != "windows" && info.Mode().Perm()&0377 != 0 {
		return fmt.Errorf("file (%v) has insecure file permissions (%v)", filename, info.Mode().String())
	}
	return nil
}

// confirm displays a text and asks for user confirmation
func confirm(text string) bool {
	fmt.Print(text)
	fmt.Printf("\nEnter 'ok' to proceed:\n> ")

	text, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		log.Crit("Failed to read user input", "err", err)
	}
	if text := strings.TrimSpace(text); text == "ok" {
		return true
	}
	return false
}

func testExternalUI(api *core.SignerAPI) {
	ctx := context.WithValue(context.Background(), "remote", "clef binary")
	ctx = context.WithValue(ctx, "scheme", "in-proc")
	ctx = context.WithValue(ctx, "local", "main")
	errs := make([]string, 0)

	a, _ := common.NewAddressFromString("Qdeadbeef000000000000000000000000deadbeef")
	addErr := func(errStr string) {
		log.Info("Test error", "err", errStr)
		errs = append(errs, errStr)
	}

	queryUser := func(q string) string {
		resp, err := api.UI.OnInputRequired(core.UserInputRequest{
			Title:  "Testing",
			Prompt: q,
		})
		if err != nil {
			addErr(err.Error())
		}
		return resp.Text
	}
	expectResponse := func(testcase, question, expect string) {
		if got := queryUser(question); got != expect {
			addErr(fmt.Sprintf("%s: got %v, expected %v", testcase, got, expect))
		}
	}
	expectApprove := func(testcase string, err error) {
		if err == nil || err == accounts.ErrUnknownAccount {
			return
		}
		addErr(fmt.Sprintf("%v: expected no error, got %v", testcase, err.Error()))
	}
	expectDeny := func(testcase string, err error) {
		if err == nil || err != core.ErrRequestDenied {
			addErr(fmt.Sprintf("%v: expected ErrRequestDenied, got %v", testcase, err))
		}
	}
	var delay = 1 * time.Second
	// Test display of info and error
	{
		api.UI.ShowInfo("If you see this message, enter 'yes' to next question")
		time.Sleep(delay)
		expectResponse("showinfo", "Did you see the message? [yes/no]", "yes")
		api.UI.ShowError("If you see this message, enter 'yes' to the next question")
		time.Sleep(delay)
		expectResponse("showerror", "Did you see the message? [yes/no]", "yes")
	}
	{ // Sign data test - typed data
		api.UI.ShowInfo("Please approve the next request for signing EIP-712 typed data")
		time.Sleep(delay)
		addr, _ := common.NewMixedcaseAddressFromString("Q0011223344556677889900112233445566778899")
		data := `{"types":{"EIP712Domain":[{"name":"name","type":"string"},{"name":"version","type":"string"},{"name":"chainId","type":"uint256"},{"name":"verifyingContract","type":"address"}],"Person":[{"name":"name","type":"string"},{"name":"test","type":"uint8"},{"name":"wallet","type":"address"}],"Mail":[{"name":"from","type":"Person"},{"name":"to","type":"Person"},{"name":"contents","type":"string"}]},"primaryType":"Mail","domain":{"name":"Ether Mail","version":"1","chainId":"1","verifyingContract":"QCCCcccccCCCCcCCCCCCcCcCccCcCCCcCcccccccC"},"message":{"from":{"name":"Cow","test":"3","wallet":"QcD2a3d9F938E13CD947Ec05AbC7FE734Df8DD826"},"to":{"name":"Bob","wallet":"QbBbBBBBbbBBBbbbBbbBbbbbBBbBbbbbBbBbbBBbB","test":"2"},"contents":"Hello, Bob!"}}`
		// _, err := api.SignData(ctx, accounts.MimetypeTypedData, *addr, hexutil.Encode([]byte(data)))
		var typedData apitypes.TypedData
		json.Unmarshal([]byte(data), &typedData)
		_, err := api.SignTypedData(ctx, *addr, typedData)
		expectApprove("sign 712 typed data", err)
	}
	{ // Sign data test - plain text
		api.UI.ShowInfo("Please approve the next request for signing text")
		time.Sleep(delay)
		addr, _ := common.NewMixedcaseAddressFromString("Q0011223344556677889900112233445566778899")
		_, err := api.SignData(ctx, accounts.MimetypeTextPlain, *addr, hexutil.Encode([]byte("hello world")))
		expectApprove("signdata - text", err)
	}
	{ // Sign data test - plain text reject
		api.UI.ShowInfo("Please deny the next request for signing text")
		time.Sleep(delay)
		addr, _ := common.NewMixedcaseAddressFromString("Q0011223344556677889900112233445566778899")
		_, err := api.SignData(ctx, accounts.MimetypeTextPlain, *addr, hexutil.Encode([]byte("hello world")))
		expectDeny("signdata - text", err)
	}
	{ // Sign transaction
		api.UI.ShowInfo("Please reject next transaction")
		time.Sleep(delay)
		data := hexutil.Bytes([]byte{})
		to := common.NewMixedcaseAddress(a)
		tx := apitypes.SendTxArgs{
			Data:                 &data,
			Nonce:                0x1,
			Value:                hexutil.Big(*big.NewInt(6)),
			From:                 common.NewMixedcaseAddress(a),
			To:                   &to,
			MaxFeePerGas:         (*hexutil.Big)(big.NewInt(5)),
			MaxPriorityFeePerGas: (*hexutil.Big)(big.NewInt(0)),
			Gas:                  1000,
			Input:                nil,
		}
		_, err := api.SignTransaction(ctx, tx, nil)
		expectDeny("signtransaction [1]", err)
		expectResponse("signtransaction [2]", "Did you see any warnings for the last transaction? (yes/no)", "no")
	}
	{ // Listing
		api.UI.ShowInfo("Please reject listing-request")
		time.Sleep(delay)
		_, err := api.List(ctx)
		expectDeny("list", err)
	}
	{ // Import
		api.UI.ShowInfo("Please reject new account-request")
		time.Sleep(delay)
		_, err := api.New(ctx)
		expectDeny("newaccount", err)
	}
	{ // Metadata
		api.UI.ShowInfo("Please check if you see the Origin in next listing (approve or deny)")
		time.Sleep(delay)
		api.List(context.WithValue(ctx, "Origin", "origin.com"))
		expectResponse("metadata - origin", "Did you see origin (origin.com)? [yes/no] ", "yes")
	}

	for _, e := range errs {
		log.Error(e)
	}
	result := fmt.Sprintf("Tests completed. %d errors:\n%s\n", len(errs), strings.Join(errs, "\n"))
	api.UI.ShowInfo(result)
}

type encryptedSeedStorage struct {
	Description string              `json:"description"`
	Version     int                 `json:"version"`
	Params      keystore.CryptoJSON `json:"params"`
}

// encryptSeed uses a similar scheme as the keystore uses, but with a different wrapping,
// to encrypt the master seed
func encryptSeed(seed []byte, auth []byte, argon2idT, argon2idM uint32, argon2idP uint8) ([]byte, error) {
	cryptoStruct, err := keystore.EncryptDataV1(seed, auth, argon2idT, argon2idM, argon2idP)
	if err != nil {
		return nil, err
	}
	return json.Marshal(&encryptedSeedStorage{"Clef seed", 1, cryptoStruct})
}

// decryptSeed decrypts the master seed
func decryptSeed(keyjson []byte, auth string) ([]byte, error) {
	var encSeed encryptedSeedStorage
	if err := json.Unmarshal(keyjson, &encSeed); err != nil {
		return nil, err
	}
	if encSeed.Version != 1 {
		log.Warn(fmt.Sprintf("unsupported encryption format of seed: %d, operation will likely fail", encSeed.Version))
	}
	seed, err := keystore.DecryptDataV1(encSeed.Params, auth)
	if err != nil {
		return nil, err
	}
	return seed, err
}

// GenDoc outputs examples of all structures used in json-rpc communication
func GenDoc(ctx *cli.Context) error {
	var (
		a, _ = common.NewAddressFromString("Qdeadbeef000000000000000000000000deadbeef")
		b, _ = common.NewAddressFromString("Q1111111122222222222233333333334444444444")
		c, _ = common.NewAddressFromString("Qcowbeef000000cowbeef00000000000000000c0w")
		meta = core.Metadata{
			Scheme:    "http",
			Local:     "localhost:8545",
			Origin:    "www.malicious.ru",
			Remote:    "localhost:9999",
			UserAgent: "Firefox 3.2",
		}
		output []string
		add    = func(name, desc string, v any) {
			if data, err := json.MarshalIndent(v, "", "  "); err == nil {
				output = append(output, fmt.Sprintf("### %s\n\n%s\n\nExample:\n```json\n%s\n```", name, desc, data))
			} else {
				log.Error("Error generating output", "err", err)
			}
		}
	)

	{ // Sign plain text request
		desc := "SignDataRequest contains information about a pending request to sign some data. " +
			"The data to be signed can be of various types, defined by content-type. Clef has done most " +
			"of the work in canonicalizing and making sense of the data, and it's up to the UI to present" +
			"the user with the contents of the `message`"
		sighash, msg := accounts.TextAndHash([]byte("hello world"))
		messages := []*apitypes.NameValueType{{Name: "message", Value: msg, Typ: accounts.MimetypeTextPlain}}

		add("SignDataRequest", desc, &core.SignDataRequest{
			Address:     common.NewMixedcaseAddress(a),
			Meta:        meta,
			ContentType: accounts.MimetypeTextPlain,
			Rawdata:     []byte(msg),
			Messages:    messages,
			Hash:        sighash})
	}
	{ // Sign plain text response
		add("SignDataResponse - approve", "Response to SignDataRequest",
			&core.SignDataResponse{Approved: true})
		add("SignDataResponse - deny", "Response to SignDataRequest",
			&core.SignDataResponse{})
	}
	{ // Sign transaction request
		desc := "SignTxRequest contains information about a pending request to sign a transaction. " +
			"Aside from the transaction itself, there is also a `call_info`-struct. That struct contains " +
			"messages of various types, that the user should be informed of." +
			"\n\n" +
			"As in any request, it's important to consider that the `meta` info also contains untrusted data." +
			"\n\n" +
			"The `transaction` (on input into clef) can have either `data` or `input` -- if both are set, " +
			"they must be identical, otherwise an error is generated. " +
			"However, Clef will always use `data` when passing this struct on (if Clef does otherwise, please file a ticket)"

		data := hexutil.Bytes([]byte{0x01, 0x02, 0x03, 0x04})
		add("SignTxRequest", desc, &core.SignTxRequest{
			Meta: meta,
			Callinfo: []apitypes.ValidationInfo{
				{Typ: "Warning", Message: "Something looks odd, show this message as a warning"},
				{Typ: "Info", Message: "User should see this as well"},
			},
			Transaction: apitypes.SendTxArgs{
				Data:                 &data,
				Nonce:                0x1,
				Value:                hexutil.Big(*big.NewInt(6)),
				From:                 common.NewMixedcaseAddress(a),
				To:                   nil,
				MaxFeePerGas:         (*hexutil.Big)(big.NewInt(5)),
				MaxPriorityFeePerGas: (*hexutil.Big)(big.NewInt(0)),
				Gas:                  1000,
				Input:                nil,
			}})
	}
	{ // Sign tx response
		data := hexutil.Bytes([]byte{0x04, 0x03, 0x02, 0x01})
		add("SignTxResponse - approve", "Response to request to sign a transaction. This response needs to contain the `transaction`"+
			", because the UI is free to make modifications to the transaction.",
			&core.SignTxResponse{Approved: true,
				Transaction: apitypes.SendTxArgs{
					Data:                 &data,
					Nonce:                0x4,
					Value:                hexutil.Big(*big.NewInt(6)),
					From:                 common.NewMixedcaseAddress(a),
					To:                   nil,
					MaxFeePerGas:         (*hexutil.Big)(big.NewInt(5)),
					MaxPriorityFeePerGas: (*hexutil.Big)(big.NewInt(0)),
					Gas:                  1000,
					Input:                nil,
				}})
		add("SignTxResponse - deny", "Response to SignTxRequest. When denying a request, there's no need to "+
			"provide the transaction in return",
			&core.SignTxResponse{})
	}
	{ // WHen a signed tx is ready to go out
		desc := "SignTransactionResult is used in the call `clef` -> `OnApprovedTx(result)`" +
			"\n\n" +
			"This occurs _after_ successful completion of the entire signing procedure, but right before the signed " +
			"transaction is passed to the external caller. This method (and data) can be used by the UI to signal " +
			"to the user that the transaction was signed, but it is primarily useful for ruleset implementations." +
			"\n\n" +
			"A ruleset that implements a rate limitation needs to know what transactions are sent out to the external " +
			"interface. By hooking into this methods, the ruleset can maintain track of that count." +
			"\n\n" +
			"**OBS:** Note that if an attacker can restore your `clef` data to a previous point in time" +
			" (e.g through a backup), the attacker can reset such windows, even if he/she is unable to decrypt the content. " +
			"\n\n" +
			"The `OnApproved` method cannot be responded to, it's purely informative"

		rlpdata := common.FromHex("0x02f91c6c8205bc0184773594008506fc23ac008276c094d46e8dd67c5d32be8058bb8eb970870f07244567849184e72a80c08301000080b90a207cf9d7449c8a7a10e48597535f11863efbfe22f62342fd870a35d9f9eff0f2eaeec23775333f5022898cb9d229ec72bc7379aefe5081e114babc8f9bdd65df0391df8e5ba996b83a9336f0727cf0612c733f0192b4595dbbf7189787b7b18a35cc0e903ed5df2131eb3ee3c119a04840d4a1e2f82eccf4d3f58d0a2c6978e70bac1e2e04d20a679a841b90f6baebfe4d5676c93fabe204a92d4d19cf77aba4463d833b6c2c29eddeb2fc5ea51d9bf2c4a1412e78eca346b4227b1e24828629ecb4da52ab3dd5a20fd1c41160c623665cf12b9143d6875345045e4a43e70500f694c651106fd4420f0f3c1147ce4af125abd8eedaaccd931c404503f0d2eb928e33039198a9874d028a9595d3d902e6e6ae8b51e4993ae2730894f22c1b499db9a984906c547d4319e1f8f4f937726d71e48f7044408cd81af2e166a4abd950c2e190dbed738eee164e56e60bf3d0a92d68e9324d5e76f5a32c872df6808c8088c7726ab0b9c6582bd7eb9e6077b068fa143cf24fe3f539ddfb3c6638fa93363380652198f01d34319624988417b19fb6a6b696934848d5ddeb38577a5611c497b87c4d83c541a3c17ce1d0113ba0f95e5a3397240d8727d892affba76ebd6fa2c72c9ee47a8f0134e801e9c15cb873ede7327f514754bd0e73c5f799876cf0a875333f45a9e555cfe7ddc5243df69219ef159142a3409a25469a31a50eee33ab1d86813fada846218711e8a3699b50e8c3c6ce71de622b5db348d1c4ef8e4ac70ea817a3fd7aee65ae23d2ec64f2106dd2c95db2b441213cb5139e206eceb9b0d3cce97f3bc0c529ed97ae97f64363ada73869296e06e3279cc4fcc816c7482813f2e28d6626172e721dfea437dc005dec2830551de4451dbd553943a1721f68de1dcc2b0f41bbc6a1be3938a7ca6448bfdd4e571da06bf03ce783bd09540447ddca82b0068e04d6a9980dfd04f3ee946a7c5bee888f8a07151c27cd88e3e0127b080f75b706e2f6ad3d3a58873f116dd0ae6b12aab10c923c8ed7a9adb99e64e367f3b2ada6c6b1ec575249f9e665527ed8eb0480a56fc7ffcb4c5629c65782000db4339f1718f0de5343083efd9aff15059541cb5c430c33d984adf162e143e4794f69bfeb408716c39349e62178b7d47c0a88921b56d89dea751a8a4b35d76b43ad173b922d9e17b9b325b738e79b06d2296b9b1d8cc9e2f0490481625215c92217e35365d8fd378a4d3f02d373573299c8c52211291de4f2efb3046183556463cad398e30340d41f20f5fd3c886b06cb5331bf6b8a55d28ebcf779e2ecae5e314ac8ab2e48ed0cd86df8fa6efe6ab7ba2d2277f6e80e10d340cf7e8ac0753071500913ee52543043e52507ab1ca0bbc3d10eb16669c0fe012c44da94ae2ca4aa0fa650549075e89632b6a6f29477bb3b53d1d1bd9ed1ff1dff5bf9340e65ab22baf692a30790b75d56dcd21ed0fb146d148c0e6413b467d827f3de15d7cfd474908cb867302aaf7264f5582b282e67d18ca1dcc1491fa5a83b1d6095930434497f28155bb9ab1850a825195fe1ca2634cf3735a97a6c9b99f9f5acfeb647405be04e3ea982a5a49ba0329a8cfb220537cb4480c4b25ce22b9f560ca5ec2e7f3047af9f9a25e41041badc66b560eaa2bb3c7d2edb3a547a40db38b3de99000b34e3bbf120e476a8f05f38d23adeb0f8e6e81275cb9f5935b18c6cde32bb8d0e624437204a6f9c748fec5d925c9549a6422e7ce8877138890c390c323c8a74e679b79ed57dbf8580d0376e571b3b8be18a3dbc6faa43cac4ed466ede4f4aabd3b4d5fd3e52b005ad876692917527e195d070436cc58ebade83ee18bd03d60ee21d6d08bd20effd70c2f8c4bae49c80288cee578d244ba1288c609590112e63713fa047145e4783a0d847765bfa698922bd94360abe5aa183f571d86dcf0bb376f40c177e7c7c0c0d60d6ca11ff5491d36bdad439a134fb785da23cc8e3c7935fa7c5d19e320abb3f5a7ab44221e419990db3da04ba713c06387d8a97e22457c20edfc87d8d4c0091cf30ebcad29897945684e8294c61287bcfdbc4eac567fcf625bb121e4f38aa06f0019993223e199d7ad0b90b42f729fa1493eb12c464fa981615b9f86d53ab97bdcf367d4765b7c563c842d4b1b85821040528d9caac85bff409c9fa8b92b95c9b3a3f666593876d0374f9a07f08882c48abf116cfa813e55585cece4edb85da6131d05581efa255a48b72ce2f422aace86a3f12c14bbde66023d2daff586602d8e4774806ccad442f1453f39005421edeab32eeca8f4fdd56cbe89e1dadfbe958688f40034ede6dc657a662a4169f12cc06fdd214027f0d69cdf6bfc551447f4021dd094d91b205ca32ef7e513a22e0a83889d39b863e6d1d49db97f1241265653b3bbd26d36be0aabd07d6f1973d8b78209ec3790a3d1de894229a0804761b8edb1231c2356f0973c3e55d081e10c3e9775f580a626544a32b982dbf7370a2c0d09ac3ab54b33348bc0e67a4a292b370c8e0d7a9cb063e9cc6fff811f9afe1a80534bef778a5cc529b45578dbb4631dc06de10d65ce2b89a941b33c66fc6498f01c6031de6bd976c12caf1d2d3b00c3a9ef93a770106610808bfc130e289146df0418524ccc1ef59592544e5ff93295070e4feb3fef6f27497f398f162510ed25215d3222b40e82d280b22e31d9adc30487172ffb11e8ff36f5ef374b3f489d2c43a509e79afee2b3141ced4a2408387072edc959fbb27ae889e8ce36be72ee24a10df68b49f061d069c83bcc56cf0173ca392387b2753d967217fc6220d054547842f44a63ef16dece66bc4e5c8d6e3659d5530805a9cdf33247246ec9d8e02727c4139e585edc75a551bd8f539323d2bb92aba62d4f5cd2a6c46b607039532e060bf3ca5aff3bf2f531b801f396bd0e0a88d774cbbf8830278da8438d6c451a821c0cedc70a430ea5f4fa52b2d6b359e2e0a6e22e0a26f818be2afb39a5aaa4b5f5b373f80b224b28e8544a4048cedcd9f75c4c9d3c5065b32a43c0b6923efdb013e17d57574ecb876c2d03b7c70e0c4bd78d401aeb897dd3618381b9b2edc2ff010a32a94bfa7afe6689fc49df52fc9a474e9f8fe1ea4d26ce205e8bbae97b172a467e340afbfd503c0921494df6010121f2c29f471b371ed2523982e5659d875c8c453259200c9cb4c1f684577802d69c4f17bacde2895730b0a5768f53761a1161e6ca1a5ed475cff34d0a85170af1f703152db73209465aa648ce5fece3ed2fe9bd897ac43b12ad8c913225d200a204f41fecf028b4af5c9d06de410adf76f9a190f03be8b6e46536a95dda30735dad9e7ab7f1a4c73f637339fa264fbe9f1a929948ac9136adf92b335ad3902a0fc78c358453dd002828cf600eef95329e078f1d32068bcb61be0e043f4f467fd3a548b17d10b5d8ead2ca277b3467e3bd2a62a254e628cfe6b3d4e8352e2d9a426b345ae5c822d7b801f7f61171bb80f8b6f3a8dde19d5e67207b6cd8dd2bb86745565e3ce17df0160b7ab922d3fb7c2b95f8b6bcfbfe0554c7a529d981db4ad1783a587bc0be69fdb3e4954083fd07df0cc9b4d75596bad0ed08c285efada352d449209bacd6265bda3481dab228c6751cd7b2afa4bcf533ef80b05ff0ea2ccf341bcb9f5b91213029bd037d32b586c766ea0a733eccad88a6266ee0ab04b919b43c15ec7989c0caac5b086f388bc5b909bbb1de63260a8ab2f282f7c576ac3f971aa989845579e1051243f8fc41e6ef5191c21a3c7ffa712e1e92223ccdcd83c535b9f6ca45b3821ef511870a277ba7f408703332f1b3690d55fe5fe48f85c169ee00b11d0ae56de9b69a01b344b55bb18e563bbe5004125a5cd340ad601bc8b6fc0f4360aa138807e6b66de576057db0f79c6482e53e061c863b081403b6ce72854f5b416366bda74ae1712a659e2f171e91a762d86e9e7915bc7b2f001506e728b6fffbe1448efd55fc9ce785f76320cd78df61e840e78e7cf959f37374ff96fdc2c557afe34296b10bfc8a54ec9fd8fce9125fecbb22aa3577781625c34a3336828a1e7cfdd48b2464230dc467eebd9327a3aad23d9caf9cbad1206f102f30008cf79a49f9d49b0e1d5f8f1abad8625a890888b798e59d87fbfae98e869407dfa758927331871d8f191104e21ae9fb8d681a12de539ce31df4deff505205ee910072a5a48f41732fa8e4ae40c40d437a0e6ff78417d821c400a79c2793f12134c0c21163b11ebb3d6e18f7092f36f579fe1195dd11ff1fc4097f78cb07c0a4a2baa95a94860928b1e527ba09894d0d342d00f63fbc3cf8492111c3bfc9ee1142de4590e34625517c6ca9be747b96f5d8f8dc3fda396830eedf05da8bf08c54a206e80f0e02ee334d7ca18537881bfcf5e98356805622b619fa26b86e478f725617ce36260a036fe8520a4628e19ec52892dc0a75fad1f6440783265811d8bdf472a8f84bebc39a25f9b826a4dcc36d6863e982c7b7ca6f47110ca68b8b3e15184ab6c6c4e78bb8e05a08b886ee3b9d3fc9b4da8d945a3480a157ff22a06b2a923ec6928fb49cb88d3905970ffa5096c1d27faf8fd043d09594e2040d86320c37dcb41aaf79c9a6e074ef58b07ae03bd5d4a3e55df70f7fc72287834783a9096eb58ae14bc3e21749eb93d8f82cac8d67adb6318c97f2d39188fb6ed4a2373c1cb177e2cf18d0c4976612f37fb528828debcf6872f66ed0bfa7839ce6ec087bf855272a77fcc4925ac993491edadf7ed3a225f645db3a49cdb735449d959db1facb2ad03c2309d38cdf04a80fc2d7b66bd37701364487673e79c264cdd513e0f7798e94e4d089fb3a60aa1e500a285cdad58b5a75467e0ac2d6195c3d5e650067413d71008cbdeec35a90ec80a13130f2d9fb6291890337f2918da8aa3ea1012ca85d0c19ec7dd88fc5475f273831a0ab03a2b6abb9486ebf50efccae496f8981d01d7d86778de71881a55eeffd7dc0ca67393013466afad8e7d852e7e6cae62b2f7cc6c56a9feaca5d062a2acf207b8e68f6fc087e7e8c539ce83fa1472279eb077f94eb9f2f2c1b9449fad40007709085e8d45039bd704e174b635c88aab5016f633b7fd47686d24f69f8b37dbbc22dbed52aafc1546a2b6dd4eddd395962b03a70eb8b78b293b379d704984ecb960e9dc52dab3c2d80b49e6b99a62679ed4aa88f1e6b99b80086246788c1448d4737edd1fce8fa5b45a598cb67c3225c0f11d63b476b806ed4c446257eb797e2bb29fb43b39d00611b4fefcdb12eb9ed1a784ba317b36d20eb2856cde9e4eb2d159ec08dbf7fa5974dbd6626ad7bcf228ac94f182e3c822a9d797e8f764b80b1f9dc5652a0b90c3405f64a7500155c4e74aa12832575a38546c197cdbd9236ef28f06e79d9d91009213510fb24a78c670a94c766e36d2153d8549325a3c25653dfe5e2eed25d837ddda36c650d0542552d48c32ffa1a7c5bbdab7173eeb83a3a13e328031b53895eea1f367da97a6b0fdb4b54f28c0588340134b56ba00fda16ba3ea7bb9f21b4a620da6e48cfd6b0667f81fa6c6ce615edf8b275e2a54c2482f6f902a64ad1ec84d7af97680a9ec2f9569fd758f1963e9579331739ddc7ff31af3c3d879b450b6fb1c1853945113082c6605e1ff16304ad199062290a0eb9af09101bdd8c2bc4873cce91165cefb71db72bf8d5cef14ab516999609095141d23362b6d8e775db2177646973de0ed540c678ae04d5d5c23949864c5be299b3daf82cda6458481b028754b8a63444f7c0a3a8df91b670ccea0b362e0ded4aab844588746873cf923c2a6283634f32126cdb18d89b35628ddf11ae850ff4f2c55ee99137b6a79d4e6a2148a819dd04be970f1b95f4ab1104bd347e563828088c08b6e91435ed29f1a33283498db3422e7d0fa5f692d3a1a33278ff5351cf52237e53d117e63abd5ab456d48215a1381e72d331e8f8895722bbb07446567da7b5482774b12d608fee815fcac22d4b5bf62b323e8d416617d1b49a301c43475670c7e5ca7182cfc6bc65d6bbccdb3dc9c4bbf9d4e93837aab0199b32d55c10fbcb59b09691456f0d9a3d240f07d224ba1314f752baf1189e078b9700c5be80d07e927cdf494ddad81e504a1f084bb1a5ddfdec3872f77351cc17f64a169b716b8a30e513cf12311f6a35d091bc05a199bb84763bccc6664cc338f0a1dda1c8c007401f57f014219c1fc58dc755b6997404e2b58d1c71786e4ef863d67dbaa31fbc100f1afd0a5fd96019787d68999e1416f738aa0e806049806c1d8a9fad0d660bfa3d2ad5c5c48f062ddd4bb20ffa9e0de28a03239b0c3146c3b919ca13219e7530f2e63a41baf61a4ec8615e48230c82ba1b03644a267c5ac0821187b4eb5ab851f544a423d48f7523c6d97cfdd69a7591736be94bb03ba4eaf1d9c53b8b36d1d11fc956f07cb7d4539a6dcaa78fb2ed6a358515638553b6795f06a18196f66c24335383c4514acbcadcd58f5b14212ee8d47bf2f835bdae3addc8ce79d70ac9501026b1d832a8e105c06209ccd2da50a747fd0d7634b9f795cfcac9456498348bcdcb7ab4fe8aa13da599c33a11b787b45c65cc353adb8e91d5d7967f9d5c4ea5db63b6c2d292984d3dcb923130c038181651672f5d68ac6416fbbebd2fbf5ef02e9b29cb90535a8e79203c7b05e61b55918fdc3546ac5ccd8a2b67b05987680e5b3925950f7e62918882ed050376ae9bbdc3bee8355d99702cca9f711eed45dee7580bc2b84dc3070a0609f6ce36b3cd42af8e7d4dc5cbffc80f942f315a5966191b67ca1079ef708b255be90162eb0c10a498229f364ff466d0ab4746e96ea964a3612d705c10861ef87ceafd69f7174e3f2e6f8a050d2686d76c4cdc271702e60230e4fe9261562d0b04baa7cd781a7d374ccae19ac7885a232dc2dcfb71637d5b0343ce9117e7827d9ebbc332b3d330e9d9865fdf7cbe334ff116eb72c53d07e56443bd2379b3b7d3737e276071f7e3b8eb29f1a17c3bc794334d4b5191ae1583e95b0685aabe920d366b4e206d7ff6327905b699f2102d4872a523d80cc401c9efe4ab1be15bf7c7e886188d71b32f4593016b4f16cbdf9860e59565ede14507338ef1fc88462dff30afac163d3d66a88a3739d0370a037dece78e7a069c9a03e2212c6e77b5b93e4bd174d3176d8da7633cc376ad5dede71d458968859bdcdfa129c7b66c839d3efc6e6c511cb169d5114374abc764fcd71ae6e76b70f0a1750e3be742b88186b137bbd3fb64a0c6d77e230ad143ce4677267ce1dffacb140edb55a5df483e1a78dfaa2ee5c511d8e87f3951a65d6989fe080796ad4f85ef147bcb138cd1f96b6e737e7f0a3ae9b0f4e579090afa959239736a5ff03350a226c87db3b2a27819777509c8651b7ae17950dde8a7158538ae00792be2a3bf76aab5ce7b58b623db90f84f67468daae86f4842e683e3f5ce5e5e9a27e466a4087f5c9a4f455d23fb3a49ad43e14d211ad3e83d4e900b2e12a8e1c6c1270573a0abe5cd927b62552c4c9bd81631ba3f7ce8978f9f95dfbafdc3a8f5052613225440f53a2238c93e214607c6606c28adf8f1438b134d3b50dee47d31ce3c0797d761595fed076a064522efb4d60a0a1038cf10d700c94cc9f4b704ec639c83995f2ab47188bc9d2b50a206a5001c6009468e99a83c19f0e610c584619f6e7157004ee08589ded1c653df935901672833050e422077fc0f605a9d5ec9862513b5c650cfc8839733aaabbcc23d998b3c1d2eb897d7e18fba76f3af9e491544daebb32a2146f41c7843a31808b7f889f3e49f74e87aae608ccf303415bca6f5b4cc0ef48d67b11fabdc0df07b9d62667b269322ce6d062c851c7b87d74e2af956e8331a29fbfbf24ce5e2e7fe969715e93088f4ef451240870470d9db86471601dc2d0bcea798f63884b197ef61e91b075b75e801c315ffea9296822b5b4fa816d0c26adfb017be628944742d4d01d21abf4b17e3663d56e2b8156db7a21e044f44d0476e961effc8fabcaf3da9852382bfb99be0faa642d7373c8c80919ff76cc47bc73743f84f4ce46faa27e1713149bea2012249d999240b90c4441f54e25bbb3799145b7046469344774b410b1f75e2571a5d2cd37b90fd76a7b1b1a16a1aebc010ab37194f5e964ca5cea67bba9c3c152c4207f658c8b106558c60e17c46591fcf0b61e0a3e2a9f9931bd3f0640483be93f35bb29d3db1ab6bd12e09ec56ec494cbf1157cf2b63449b78d42db8d8d7b7875c51166c4e00412d1744711c6e1afaac0d17b13785f9510e3d9d3c3ae8d237820419dfa3d85bb2f57758b5f1585a3a47b8ccf4167f1bda6fc8ad92ec2aebe5e7e94898a14ce7b27e7a0802b05fa72e1192f601ba41af275f3316be5fbf1af0dd8da93ef487faaacb6d824968e49aae10e35599ca1dae81258e32c01e5598bf01d793db1c1572c3a27ecc9fa67419364cf47049d96988a6af1ec39531582cc902eaf38383b56aa318fda03aab99e6d0047b2379676a990f4206cc4276cbcd5ccabb2900bee1143e46edd340ccfc1aec3ebea683bfd9560984b97b4883dbf6ea56b1d72ca90fedeeada7f01bf9bde83262a885310d8c4b01ea854cbd8bace91ff2784dcb32de5e3902b0d0e80651ab861f345ba1a4d2e2c5193635f147b9259b7f3355fb8648cacbb5473d07389d105b006d443036595e4ad40a3491c0db73e3716f45e25ba538cdeff69c0aba0c9694fffa02c56f429a7a95d472bcac1855a78f355832abdd7459d36ac71124ebb92fb9d3afb8a65ec9fca3b9b9b9e3b58dd032617edcc99b9bf82928d6d4af0e56a6aad49f8bd86ac6bc722420a66eec126e1d066d999638e4c9c02dbb5ceaeb7832ef973967bd6fea4cbcde5ac7584e1e7793eea793143429330057276ea1a05640d45d9fb624590b963a157f99dad59a1015a08aedf6e54256641a6ad0818c638f5ad20564730c8cab06a1113fe9c2c503750e279366d02ca2e64abf9e3976ec7d778f1955c3e5ded09315d9c3515a010b97d9abaaffb8c1a3cb6431998b7f5207c4f0d89ef6b338788db82f8cccdd712d021b7bf09a0b7006235953ea946d8744ca3e8a9de79436489eb4486dac83a4e23be131c8ce545987e45f4d25f2f86b9851d2c1b5b5e41920090e88aeffa80a4103c2af33533f1fc8d3b58f743cfbbdc5c11844c4470dde56efa74a1c4a423251413f3528618bd9fcc2b074f57e6f29ab7249c681bd0ce8c93bd058e90d1c3c5a0e64fdfc5edbec24ea7666c02ac6eee8d6636548f9b7d0769d56d6a6b4bd6f99b147e3234517e776b0f74bac92d6b8f8df15d52548208fcddc183154144640d4cd9f29d2a829a569ee89ae725d4671799ece3e804ae1d0cbe06cc490aa19e6a460fd43b18ac5e92582aea782abdf9f78210d5fff28e60cc34b305f0a0ba500684628d718d466d87946e994fc6628aaebc9f1c66abd91f788e39434d59c61d6750263e5eb13b9be2da7fbba5eb1f366a32d0416146877b5d391829534ecc199511955ee642b534223991cd98020dc863e6223d6cf5a17abe1e60321364b614b237ce7ae7d6acd28751e77617f88212bb5495257b3a94f81c50117ec67d1e677638fd87d1f5028ded21b7e1a587b3689f91b893a77bf941f19c7dc1f1a5bb688ef785bdba06c92bad12eb7684cf8e2ffde374eac73c209a70ae61ea1b64e02e1855246de4952de45104229b15b3daf8f547680804d81cce2d4da05a2f5cf16487b59ecd26776353ca1939cc797f471f8bc80f395e5d3370357109646701c3c70e89b3852b152dcb089b310c4ef44adccc92012e1e32d987687e27bd966f1fee974eee0313ee90f15a99dcacc7faa132bb25ab8893058bf6f4efb17b1ff775dab403b2ec611dcdd9ee59a05f0d452191f49b3c8568184bc45fc282ff807852d472b90351c96ae5ea941f1b054715d74987cbafb696cac490e5ec03f73aeed0a543aca27ca329f6589d048e4cc827158ad0c603296436db9b955b5f2d163390f7f3db5ffd5e25406ab88091f96b692f3cb225849c0fde95fa86cc0f0af41b7d3b78a217d122bfa12f96b4bbe1051c58ab103c4f8092abdffcfe09192456647cfd41708db3b5deea01152c3873848c929df207489fef1b40abdb0000000000000000000000000000000000000000000000000000040811181f292d31")
		var tx types.Transaction
		tx.UnmarshalBinary(rlpdata)
		add("OnApproved - SignTransactionResult", desc, &qrlapi.SignTransactionResult{Raw: rlpdata, Tx: &tx})
	}
	{ // User input
		add("UserInputRequest", "Sent when clef needs the user to provide data. If 'password' is true, the input field should be treated accordingly (echo-free)",
			&core.UserInputRequest{IsPassword: true, Title: "The title here", Prompt: "The question to ask the user"})
		add("UserInputResponse", "Response to UserInputRequest",
			&core.UserInputResponse{Text: "The textual response from user"})
	}
	{ // List request
		add("ListRequest", "Sent when a request has been made to list addresses. The UI is provided with the "+
			"full `account`s, including local directory names. Note: this information is not passed back to the external caller, "+
			"who only sees the `address`es. ",
			&core.ListRequest{
				Meta: meta,
				Accounts: []accounts.Account{
					{Address: a, URL: accounts.URL{Scheme: "keystore", Path: "/path/to/keyfile/a"}},
					{Address: b, URL: accounts.URL{Scheme: "keystore", Path: "/path/to/keyfile/b"}}},
			})

		add("ListResponse", "Response to list request. The response contains a list of all addresses to show to the caller. "+
			"Note: the UI is free to respond with any address the caller, regardless of whether it exists or not",
			&core.ListResponse{
				Accounts: []accounts.Account{
					{
						Address: c,
						URL:     accounts.URL{Path: ".. ignored .."},
					},
					{
						Address: common.MaxAddress,
					},
				}})
	}

	fmt.Println(`## UI Client interface

These data types are defined in the channel between clef and the UI`)
	for _, elem := range output {
		fmt.Println(elem)
	}
	return nil
}
