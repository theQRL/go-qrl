// Copyright 2014 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package keystore

import (
	"encoding/hex"
	"fmt"
	"reflect"
	"testing"

	"github.com/theQRL/go-zond/common"
)

func tmpKeyStoreIface(t *testing.T, encrypted bool) (dir string, ks keyStore) {
	d := t.TempDir()
	if encrypted {
		ks = &keyStorePassphrase{d, veryLightArgon2idT, veryLightArgon2idM, veryLightArgon2idP, true}
	} else {
		ks = &keyStorePlain{d}
	}
	return d, ks
}

func TestKeyStorePlain(t *testing.T) {
	t.Parallel()
	_, ks := tmpKeyStoreIface(t, false)

	pass := "" // not used but required by API
	k1, account, err := storeNewKey(ks, pass)
	if err != nil {
		t.Fatal(err)
	}
	k2, err := ks.GetKey(k1.Address, account.URL.Path, pass)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(k1.Address, k2.Address) {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(k1.Wallet.GetSeed(), k2.Wallet.GetSeed()) {
		t.Fatal(err)
	}
}

func TestKeyStorePassphrase(t *testing.T) {
	t.Parallel()
	_, ks := tmpKeyStoreIface(t, true)

	pass := "foo"
	k1, account, err := storeNewKey(ks, pass)
	if err != nil {
		t.Fatal(err)
	}
	k2, err := ks.GetKey(k1.Address, account.URL.Path, pass)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(k1.Address, k2.Address) {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(k1.Wallet.GetSeed(), k2.Wallet.GetSeed()) {
		t.Fatal(err)
	}
}

func TestKeyStorePassphraseDecryptionFail(t *testing.T) {
	t.Parallel()
	_, ks := tmpKeyStoreIface(t, true)

	pass := "foo"
	k1, account, err := storeNewKey(ks, pass)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ks.GetKey(k1.Address, account.URL.Path, "bar"); err != ErrDecrypt {
		t.Fatalf("wrong error for invalid password\ngot %q\nwant %q", err, ErrDecrypt)
	}
}

// Test and utils for the key store tests in the QRL JSON tests;
// testdataKeyStoreTests/basic_tests.json
type KeyStoreTestV1 struct {
	Json     encryptedKeyJSONV1
	Password string
	Seed     string
}

func TestV1_Argon2id_1(t *testing.T) {
	t.Parallel()
	tests := loadKeyStoreTestV1("testdata/v1_test_vector.json", t)
	testDecryptV1(tests["test_vector_argon2id"], t)
}

func TestV1_Argon2id_2(t *testing.T) {
	t.Parallel()
	tests := loadKeyStoreTestV1("testdata/v1_test_vector.json", t)
	testDecryptV1(tests["test_vector_argon2id_2"], t)
}

func testDecryptV1(test KeyStoreTestV1, t *testing.T) {
	seedBytes, _, err := decryptKeyV1(&test.Json, test.Password)
	if err != nil {
		t.Fatal(err)
	}
	seedHex := hex.EncodeToString(seedBytes)
	if test.Seed != seedHex {
		t.Fatal(fmt.Errorf("Decrypted bytes not equal to test, expected %v have %v", test.Seed, seedHex))
	}
}

func loadKeyStoreTestV1(file string, t *testing.T) map[string]KeyStoreTestV1 {
	tests := make(map[string]KeyStoreTestV1)
	err := common.LoadJSON(file, &tests)
	if err != nil {
		t.Fatal(err)
	}
	return tests
}
