// Copyright 2023 The go-ethereum Authors
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

package catalyst

import (
	"math/big"
	"testing"
	"time"

	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/core"
	"github.com/theQRL/go-qrl/core/types"
	"github.com/theQRL/go-qrl/crypto/pqcrypto/wallet"
	"github.com/theQRL/go-qrl/miner"
	"github.com/theQRL/go-qrl/node"
	"github.com/theQRL/go-qrl/p2p"
	"github.com/theQRL/go-qrl/params"
	"github.com/theQRL/go-qrl/qrl"
	"github.com/theQRL/go-qrl/qrl/downloader"
	"github.com/theQRL/go-qrl/qrl/qrlconfig"
)

func startSimulatedBeaconQRLService(t *testing.T, genesis *core.Genesis) (*node.Node, *qrl.QRL, *SimulatedBeacon) {
	t.Helper()

	n, err := node.New(&node.Config{
		P2P: p2p.Config{
			ListenAddr:  "127.0.0.1:8545",
			NoDiscovery: true,
			MaxPeers:    0,
		},
	})
	if err != nil {
		t.Fatal("can't create node:", err)
	}

	qrlcfg := &qrlconfig.Config{Genesis: genesis, SyncMode: downloader.FullSync, TrieTimeout: time.Minute, TrieDirtyCache: 256, TrieCleanCache: 256, Miner: miner.DefaultConfig}
	qrlservice, err := qrl.New(n, qrlcfg)
	if err != nil {
		t.Fatal("can't create qrl service:", err)
	}

	simBeacon, err := NewSimulatedBeacon(1, qrlservice)
	if err != nil {
		t.Fatal("can't create simulated beacon:", err)
	}

	n.RegisterLifecycle(simBeacon)

	if err := n.Start(); err != nil {
		t.Fatal("can't start node:", err)
	}

	qrlservice.SetSynced()
	return n, qrlservice, simBeacon
}

// send 20 transactions, >10 withdrawals and ensure they are included in order
// send enough transactions to fill multiple blocks
func TestSimulatedBeaconSendWithdrawals(t *testing.T) {
	var withdrawals []types.Withdrawal
	txs := make(map[common.Hash]*types.Transaction)

	var (
		// testWallet is a wallet to use for funding a tester account.
		testWallet, _ = wallet.RestoreFromSeedHex("010000b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f29100000000000000000000000000000000")

		// testAddr is the QRL address of the tester account.
		testAddr = testWallet.GetAddress()
	)

	// short period (1 second) for testing purposes
	var gasLimit uint64 = 10_000_000
	genesis := core.DeveloperGenesisBlock(gasLimit, testAddr)
	node, qrlService, mock := startSimulatedBeaconQRLService(t, genesis)
	_ = mock
	defer node.Close()

	chainHeadCh := make(chan core.ChainHeadEvent, 10)
	subscription := qrlService.BlockChain().SubscribeChainHeadEvent(chainHeadCh)
	defer subscription.Unsubscribe()

	// generate some withdrawals
	for i := range 20 {
		withdrawals = append(withdrawals, types.Withdrawal{Index: uint64(i)})
		if err := mock.withdrawals.add(&withdrawals[i]); err != nil {
			t.Fatal("addWithdrawal failed", err)
		}
	}

	// generate a bunch of transactions
	signer := types.NewZondSigner(qrlService.BlockChain().Config().ChainID)
	for i := range 20 {
		tx, err := types.SignTx(types.NewTx(&types.DynamicFeeTx{Nonce: uint64(i), To: &common.Address{}, Value: big.NewInt(1000), Gas: params.TxGas, GasFeeCap: big.NewInt(params.InitialBaseFee), Data: nil}), signer, testWallet)
		if err != nil {
			t.Fatalf("error signing transaction, err=%v", err)
		}
		txs[tx.Hash()] = tx

		if err := qrlService.APIBackend.SendTx(t.Context(), tx); err != nil {
			t.Fatal("SendTx failed", err)
		}
	}

	includedTxs := make(map[common.Hash]struct{})
	var includedWithdrawals []uint64

	timer := time.NewTimer(12 * time.Second)
	for {
		select {
		case evt := <-chainHeadCh:
			for _, includedTx := range evt.Block.Transactions() {
				includedTxs[includedTx.Hash()] = struct{}{}
			}
			for _, includedWithdrawal := range evt.Block.Withdrawals() {
				includedWithdrawals = append(includedWithdrawals, includedWithdrawal.Index)
			}

			// ensure all withdrawals/txs included. this will take two blocks b/c number of withdrawals > 10
			if len(includedTxs) == len(txs) && len(includedWithdrawals) == len(withdrawals) && evt.Block.Number().Cmp(big.NewInt(2)) == 0 {
				return
			}
		case <-timer.C:
			t.Fatal("timed out without including all withdrawals/txs")
		}
	}
}
