// Copyright 2017 The go-ethereum Authors
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
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/cespare/cp"
	"github.com/davecgh/go-spew/spew"
	"github.com/theQRL/go-qrl/accounts"
	"github.com/theQRL/go-qrl/common"
)

// makeCachetestAccounts creates three freshly generated keystore accounts in a
// new temporary directory and returns the directory plus the accounts sorted
// the way the cache orders them by filename. Using live keygen keeps the
// testdata tree free of address-format-specific encrypted keystore blobs.
func makeCachetestAccounts(t *testing.T) (string, []accounts.Account) {
	t.Helper()
	dir := t.TempDir()
	ks := NewKeyStore(dir, veryLightArgon2idT, veryLightArgon2idM, veryLightArgon2idP)
	mk := func(basename string) accounts.Account {
		acc, err := ks.NewAccount("")
		if err != nil {
			t.Fatalf("NewAccount: %v", err)
		}
		target := filepath.Join(dir, basename)
		if err := os.Rename(acc.URL.Path, target); err != nil {
			t.Fatalf("rename %q → %q: %v", acc.URL.Path, target, err)
		}
		return accounts.Account{
			Address: acc.Address,
			URL:     accounts.URL{Scheme: KeyStoreScheme, Path: target},
		}
	}
	// Byte-wise filename ordering is 'U' (0x55) < 'a' (0x61) < 'z' (0x7A),
	// so the UTC fixture comes first, then aaa, then zzz.
	utc := mk("UTC--2025-11-06T07-34-54.273240000Z--keystore-test-0")
	a := mk("aaa")
	z := mk("zzz")
	return dir, []accounts.Account{utc, a, z}
}

// waitWatcherStart waits up to 1s for the keystore watcher to start.
func waitWatcherStart(ks *KeyStore) bool {
	// On systems where file watch is not supported, just return "ok".
	if !ks.cache.watcher.enabled() {
		return true
	}
	// The watcher should start, and then exit.
	for t0 := time.Now(); time.Since(t0) < 1*time.Second; time.Sleep(100 * time.Millisecond) {
		if ks.cache.watcherStarted() {
			return true
		}
	}
	return false
}

func waitForAccounts(wantAccounts []accounts.Account, ks *KeyStore) error {
	var list []accounts.Account
	for t0 := time.Now(); time.Since(t0) < 5*time.Second; time.Sleep(200 * time.Millisecond) {
		list = ks.Accounts()
		if reflect.DeepEqual(list, wantAccounts) {
			// ks should have also received change notifications
			select {
			case <-ks.changes:
			default:
				return errors.New("wasn't notified of new accounts")
			}
			return nil
		}
	}
	return fmt.Errorf("\ngot  %v\nwant %v", list, wantAccounts)
}

func TestWatchNewFile(t *testing.T) {
	t.Parallel()

	srcDir, srcAccounts := makeCachetestAccounts(t)
	_ = srcDir

	dir, ks := tmpKeyStore(t)

	// Ensure the watcher is started before adding any files.
	ks.Accounts()
	if !waitWatcherStart(ks) {
		t.Fatal("keystore watcher didn't start in time")
	}
	// Move in the files.
	wantAccounts := make([]accounts.Account, len(srcAccounts))
	for i := range srcAccounts {
		wantAccounts[i] = accounts.Account{
			Address: srcAccounts[i].Address,
			URL:     accounts.URL{Scheme: KeyStoreScheme, Path: filepath.Join(dir, filepath.Base(srcAccounts[i].URL.Path))},
		}
		if err := cp.CopyFile(wantAccounts[i].URL.Path, srcAccounts[i].URL.Path); err != nil {
			t.Fatal(err)
		}
	}

	// ks should see the accounts.
	if err := waitForAccounts(wantAccounts, ks); err != nil {
		t.Error(err)
	}
}

func TestWatchNoDir(t *testing.T) {
	t.Parallel()
	_, srcAccounts := makeCachetestAccounts(t)
	// Create ks but not the directory that it watches.
	dir := filepath.Join(t.TempDir(), fmt.Sprintf("qrl-keystore-watchnodir-test-%d-%d", os.Getpid(), rand.Int()))
	ks := NewKeyStore(dir, LightArgon2idT, LightArgon2idM, LightArgon2idP)
	list := ks.Accounts()
	if len(list) > 0 {
		t.Error("initial account list not empty:", list)
	}
	// The watcher should start, and then exit.
	if !waitWatcherStart(ks) {
		t.Fatal("keystore watcher didn't start in time")
	}
	// Create the directory and copy a key file into it.
	os.MkdirAll(dir, 0700)
	file := filepath.Join(dir, "aaa")
	if err := cp.CopyFile(file, srcAccounts[0].URL.Path); err != nil {
		t.Fatal(err)
	}

	// ks should see the account.
	wantAccounts := []accounts.Account{srcAccounts[0]}
	wantAccounts[0].URL = accounts.URL{Scheme: KeyStoreScheme, Path: file}
	for d := 200 * time.Millisecond; d < 8*time.Second; d *= 2 {
		list = ks.Accounts()
		if reflect.DeepEqual(list, wantAccounts) {
			// ks should have also received change notifications
			select {
			case <-ks.changes:
			default:
				t.Fatalf("wasn't notified of new accounts")
			}
			return
		}
		time.Sleep(d)
	}
	t.Errorf("\ngot  %v\nwant %v", list, wantAccounts)
}

func TestCacheInitialReload(t *testing.T) {
	t.Parallel()
	dir, want := makeCachetestAccounts(t)
	cache, _ := newAccountCache(dir)
	accts := cache.accounts()
	if !reflect.DeepEqual(accts, want) {
		t.Fatalf("got initial accounts: %swant %s", spew.Sdump(accts), spew.Sdump(want))
	}
}

func TestCacheAddDeleteOrder(t *testing.T) {
	t.Parallel()
	cache, _ := newAccountCache("testdata/no-such-dir")
	cache.watcher.running = true // prevent unexpected reloads

	// QRL addresses constructed from distinct byte patterns (left-padded
	// into the current AddressLength); the bytes themselves are only used
	// for uniqueness / hasAddress lookups.
	address1 := common.BytesToAddress(common.FromHex("095e7baea6a6c7c4c2dfeb977efac326af552d87095e7baea6a6c7c4c2dfeb977efac326af552d87a1b2c3d4e5f60102"))
	address2 := common.BytesToAddress(common.FromHex("2cac1adea150210703ba75ed097ddfe24e14f2132cac1adea150210703ba75ed097ddfe24e14f213b2c3d4e5f6010203"))
	address3 := common.BytesToAddress(common.FromHex("8bda78331c916a08481428e4b07c96d3e916d1658bda78331c916a08481428e4b07c96d3e916d165c3d4e5f601020304"))
	address4 := common.BytesToAddress(common.FromHex("d49ff4eeb0b2686ed89c0fc0f2b6ea533ddbbd5ed49ff4eeb0b2686ed89c0fc0f2b6ea533ddbbd5ed4e5f60102030405"))
	address5 := common.BytesToAddress(common.FromHex("7ef5a6135f1fd6a02593eedc869c6d41d934aef87ef5a6135f1fd6a02593eedc869c6d41d934aef8e5f6010203040506"))
	address6 := common.BytesToAddress(common.FromHex("f466859ead1932d743d622cb74fc058882e8648af466859ead1932d743d622cb74fc058882e8648af60102030405060708"))
	address7 := common.BytesToAddress(common.FromHex("289d485d9771714cce91d3393d764e1311907acc289d485d9771714cce91d3393d764e1311907acc0102030405060708ff"))
	accs := []accounts.Account{
		{
			Address: address1,
			URL:     accounts.URL{Scheme: KeyStoreScheme, Path: "-309830980"},
		},
		{
			Address: address2,
			URL:     accounts.URL{Scheme: KeyStoreScheme, Path: "ggg"},
		},
		{
			Address: address3,
			URL:     accounts.URL{Scheme: KeyStoreScheme, Path: "zzzzzz-the-very-last-one.keyXXX"},
		},
		{
			Address: address4,
			URL:     accounts.URL{Scheme: KeyStoreScheme, Path: "SOMETHING.key"},
		},
		{
			Address: address5,
			URL:     accounts.URL{Scheme: KeyStoreScheme, Path: "UTC--2016-03-22T12-57-55.920751759Z--" + address5.Hex()},
		},
		{
			Address: address6,
			URL:     accounts.URL{Scheme: KeyStoreScheme, Path: "aaa"},
		},
		{
			Address: address7,
			URL:     accounts.URL{Scheme: KeyStoreScheme, Path: "zzz"},
		},
	}
	for _, a := range accs {
		cache.add(a)
	}
	// Add some of them twice to check that they don't get reinserted.
	cache.add(accs[0])
	cache.add(accs[2])

	// Check that the account list is sorted by filename.
	wantAccounts := make([]accounts.Account, len(accs))
	copy(wantAccounts, accs)
	slices.SortFunc(wantAccounts, byURL)
	list := cache.accounts()
	if !reflect.DeepEqual(list, wantAccounts) {
		t.Fatalf("got accounts: %s\nwant %s", spew.Sdump(accs), spew.Sdump(wantAccounts))
	}
	for _, a := range accs {
		if !cache.hasAddress(a.Address) {
			t.Errorf("expected hasAccount(%x) to return true", a.Address)
		}
	}
	// A distinct 64-byte address not added to the cache above.
	address := common.BytesToAddress(common.FromHex("bb81a0496aa34a64f96c2bcd28793165e1e6c08abb81a0496aa34a64f96c2bcd28793165e1e6c08aaabbccddee0102030405"))
	if cache.hasAddress(address) {
		t.Errorf("expected hasAccount(%x) to return false", address)
	}

	// Delete a few keys from the cache.
	for i := 0; i < len(accs); i += 2 {
		cache.delete(wantAccounts[i])
	}
	cache.delete(accounts.Account{Address: address, URL: accounts.URL{Scheme: KeyStoreScheme, Path: "something"}})

	// Check content again after deletion.
	wantAccountsAfterDelete := []accounts.Account{
		wantAccounts[1],
		wantAccounts[3],
		wantAccounts[5],
	}
	list = cache.accounts()
	if !reflect.DeepEqual(list, wantAccountsAfterDelete) {
		t.Fatalf("got accounts after delete: %s\nwant %s", spew.Sdump(list), spew.Sdump(wantAccountsAfterDelete))
	}
	for _, a := range wantAccountsAfterDelete {
		if !cache.hasAddress(a.Address) {
			t.Errorf("expected hasAccount(%x) to return true", a.Address)
		}
	}
	if cache.hasAddress(wantAccounts[0].Address) {
		t.Errorf("expected hasAccount(%x) to return false", wantAccounts[0].Address)
	}
}

func TestCacheFind(t *testing.T) {
	t.Parallel()
	dir := filepath.Join("testdata", "dir")
	cache, _ := newAccountCache(dir)
	cache.watcher.running = true // prevent unexpected reloads

	address1 := common.BytesToAddress(common.FromHex("095e7baea6a6c7c4c2dfeb977efac326af552d87095e7baea6a6c7c4c2dfeb977efac326af552d87a1b2c3d4e5f60102"))
	address2 := common.BytesToAddress(common.FromHex("2cac1adea150210703ba75ed097ddfe24e14f2132cac1adea150210703ba75ed097ddfe24e14f213b2c3d4e5f6010203"))
	address3 := common.BytesToAddress(common.FromHex("d49ff4eeb0b2686ed89c0fc0f2b6ea533ddbbd5ed49ff4eeb0b2686ed89c0fc0f2b6ea533ddbbd5ed4e5f60102030405"))
	accs := []accounts.Account{
		{
			Address: address1,
			URL:     accounts.URL{Scheme: KeyStoreScheme, Path: filepath.Join(dir, "a.key")},
		},
		{
			Address: address2,
			URL:     accounts.URL{Scheme: KeyStoreScheme, Path: filepath.Join(dir, "b.key")},
		},
		{
			Address: address3,
			URL:     accounts.URL{Scheme: KeyStoreScheme, Path: filepath.Join(dir, "c.key")},
		},
		{
			Address: address3,
			URL:     accounts.URL{Scheme: KeyStoreScheme, Path: filepath.Join(dir, "c2.key")},
		},
	}
	for _, a := range accs {
		cache.add(a)
	}

	address := common.BytesToAddress(common.FromHex("f466859ead1932d743d622cb74fc058882e8648af466859ead1932d743d622cb74fc058882e8648af60102030405060708"))
	nomatchAccount := accounts.Account{
		Address: address,
		URL:     accounts.URL{Scheme: KeyStoreScheme, Path: filepath.Join(dir, "something")},
	}
	tests := []struct {
		Query      accounts.Account
		WantResult accounts.Account
		WantError  error
	}{
		// by address
		{Query: accounts.Account{Address: accs[0].Address}, WantResult: accs[0]},
		// by file
		{Query: accounts.Account{URL: accs[0].URL}, WantResult: accs[0]},
		// by basename
		{Query: accounts.Account{URL: accounts.URL{Scheme: KeyStoreScheme, Path: filepath.Base(accs[0].URL.Path)}}, WantResult: accs[0]},
		// by file and address
		{Query: accs[0], WantResult: accs[0]},
		// ambiguous address, tie resolved by file
		{Query: accs[2], WantResult: accs[2]},
		// ambiguous address error
		{
			Query: accounts.Account{Address: accs[2].Address},
			WantError: &AmbiguousAddrError{
				Addr:    accs[2].Address,
				Matches: []accounts.Account{accs[2], accs[3]},
			},
		},
		// no match error
		{Query: nomatchAccount, WantError: ErrNoMatch},
		{Query: accounts.Account{URL: nomatchAccount.URL}, WantError: ErrNoMatch},
		{Query: accounts.Account{URL: accounts.URL{Scheme: KeyStoreScheme, Path: filepath.Base(nomatchAccount.URL.Path)}}, WantError: ErrNoMatch},
		{Query: accounts.Account{Address: nomatchAccount.Address}, WantError: ErrNoMatch},
	}
	for i, test := range tests {
		a, err := cache.find(test.Query)
		if !reflect.DeepEqual(err, test.WantError) {
			t.Errorf("test %d: error mismatch for query %v\ngot %q\nwant %q", i, test.Query, err, test.WantError)
			continue
		}
		if a != test.WantResult {
			t.Errorf("test %d: result mismatch for query %v\ngot %v\nwant %v", i, test.Query, a, test.WantResult)
			continue
		}
	}
}

// TestUpdatedKeyfileContents tests that updating the contents of a keystore file
// is noticed by the watcher, and the account cache is updated accordingly
func TestUpdatedKeyfileContents(t *testing.T) {
	t.Parallel()
	_, srcAccounts := makeCachetestAccounts(t)

	// Create a temporary keystore to test with
	dir := t.TempDir()
	ks := NewKeyStore(dir, LightArgon2idT, LightArgon2idM, LightArgon2idP)

	list := ks.Accounts()
	if len(list) > 0 {
		t.Error("initial account list not empty:", list)
	}
	if !waitWatcherStart(ks) {
		t.Fatal("keystore watcher didn't start in time")
	}
	// Copy a key file into it
	file := filepath.Join(dir, "aaa")

	// Place one of our testfiles in there
	if err := cp.CopyFile(file, srcAccounts[0].URL.Path); err != nil {
		t.Fatal(err)
	}

	// ks should see the account.
	wantAccounts := []accounts.Account{srcAccounts[0]}
	wantAccounts[0].URL = accounts.URL{Scheme: KeyStoreScheme, Path: file}
	if err := waitForAccounts(wantAccounts, ks); err != nil {
		t.Error(err)
		return
	}
	// needed so that modTime of `file` is different to its current value after forceCopyFile
	os.Chtimes(file, time.Now().Add(-time.Second), time.Now().Add(-time.Second))

	// Now replace file contents
	if err := forceCopyFile(file, srcAccounts[1].URL.Path); err != nil {
		t.Fatal(err)
		return
	}
	wantAccounts = []accounts.Account{srcAccounts[1]}
	wantAccounts[0].URL = accounts.URL{Scheme: KeyStoreScheme, Path: file}
	if err := waitForAccounts(wantAccounts, ks); err != nil {
		t.Errorf("First replacement failed")
		t.Error(err)
		return
	}

	// needed so that modTime of `file` is different to its current value after forceCopyFile
	os.Chtimes(file, time.Now().Add(-time.Second), time.Now().Add(-time.Second))

	// Now replace file contents again
	if err := forceCopyFile(file, srcAccounts[2].URL.Path); err != nil {
		t.Fatal(err)
		return
	}
	wantAccounts = []accounts.Account{srcAccounts[2]}
	wantAccounts[0].URL = accounts.URL{Scheme: KeyStoreScheme, Path: file}
	if err := waitForAccounts(wantAccounts, ks); err != nil {
		t.Errorf("Second replacement failed")
		t.Error(err)
		return
	}

	// needed so that modTime of `file` is different to its current value after os.WriteFile
	os.Chtimes(file, time.Now().Add(-time.Second), time.Now().Add(-time.Second))

	// Now replace file contents with crap
	if err := os.WriteFile(file, []byte("foo"), 0600); err != nil {
		t.Fatal(err)
		return
	}
	if err := waitForAccounts([]accounts.Account{}, ks); err != nil {
		t.Errorf("Emptying account file failed")
		t.Error(err)
		return
	}
}

// forceCopyFile is like cp.CopyFile, but doesn't complain if the destination exists.
func forceCopyFile(dst, src string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}
