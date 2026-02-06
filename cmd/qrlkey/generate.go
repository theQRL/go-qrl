// Copyright 2017 The go-ethereum Authors
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
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/theQRL/go-zond/accounts/keystore"
	"github.com/theQRL/go-zond/cmd/utils"
	"github.com/theQRL/go-zond/common"
	"github.com/theQRL/go-zond/crypto/pqcrypto/wallet"
	"github.com/urfave/cli/v2"
)

type outputGenerate struct {
	Address string
}

var (
	seedFlag = &cli.StringFlag{
		Name:  "seed",
		Usage: "file containing a wallet seed to encrypt",
	}
	lightKDFFlag = &cli.BoolFlag{
		Name:  "lightkdf",
		Usage: "use less secure argon2id parameters",
	}
)

var commandGenerate = &cli.Command{
	Name:      "generate",
	Usage:     "generate new keyfile",
	ArgsUsage: "[ <keyfile> ]",
	Description: `
Generate a new keyfile.

If you want to encrypt an existing wallet seed, it can be specified by setting
--seed with the location of the file containing the seed.
`,
	Flags: []cli.Flag{
		passphraseFlag,
		jsonFlag,
		seedFlag,
		lightKDFFlag,
	},
	Action: func(ctx *cli.Context) error {
		// Check if keyfile path given and make sure it doesn't already exist.
		keyfilepath := ctx.Args().First()
		if keyfilepath == "" {
			keyfilepath = defaultKeyfileName
		}
		if _, err := os.Stat(keyfilepath); err == nil {
			utils.Fatalf("Keyfile already exists at %s.", keyfilepath)
		} else if !os.IsNotExist(err) {
			utils.Fatalf("Error checking if keyfile exists: %v", err)
		}

		var w wallet.Wallet
		var err error
		if file := ctx.String(seedFlag.Name); file != "" {
			// Restore wallet from file.
			w, err = wallet.RestoreFromFile(file)
			if err != nil {
				utils.Fatalf("Can't restore wallet from file: %v", err)
			}
		} else {
			// If not loaded, generate random.
			w, err = wallet.Generate(wallet.ML_DSA_87)
			if err != nil {
				utils.Fatalf("Failed to generate random wallet: %v", err)
			}
		}

		// Create the keyfile object with a random UUID.
		UUID, err := uuid.NewRandom()
		if err != nil {
			utils.Fatalf("Failed to generate random uuid: %v", err)
		}
		key := &keystore.Key{
			Id:      UUID,
			Address: common.Address(w.GetAddress()),
			Wallet:  w,
		}

		// Encrypt key with passphrase.
		passphrase := getPassphrase(ctx, true)
		argon2idT, argon2idM, argon2idP := keystore.StandardArgon2idT, keystore.StandardArgon2idM, keystore.StandardArgon2idP
		if ctx.Bool(lightKDFFlag.Name) {
			argon2idT, argon2idM, argon2idP = keystore.LightArgon2idT, keystore.LightArgon2idM, keystore.LightArgon2idP
		}
		keyjson, err := keystore.EncryptKey(key, passphrase, argon2idT, argon2idM, argon2idP)
		if err != nil {
			utils.Fatalf("Error encrypting key: %v", err)
		}

		// Store the file to disk.
		if err := os.MkdirAll(filepath.Dir(keyfilepath), 0700); err != nil {
			utils.Fatalf("Could not create directory %s", filepath.Dir(keyfilepath))
		}
		if err := os.WriteFile(keyfilepath, keyjson, 0600); err != nil {
			utils.Fatalf("Failed to write keyfile to %s: %v", keyfilepath, err)
		}

		// Output some information.
		out := outputGenerate{
			Address: key.Address.Hex(),
		}
		if ctx.Bool(jsonFlag.Name) {
			mustPrintJSON(out)
		} else {
			fmt.Println("Address:", out.Address)
		}
		return nil
	},
}
