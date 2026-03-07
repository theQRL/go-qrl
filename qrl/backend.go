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

// Package qrl implements the QRL protocol.
package qrl

import (
	"fmt"
	"math/big"
	"runtime"
	"sync"

	"github.com/theQRL/go-qrl/accounts"
	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/common/hexutil"
	"github.com/theQRL/go-qrl/consensus"
	"github.com/theQRL/go-qrl/core"
	"github.com/theQRL/go-qrl/core/bloombits"
	"github.com/theQRL/go-qrl/core/rawdb"
	"github.com/theQRL/go-qrl/core/state/pruner"
	"github.com/theQRL/go-qrl/core/txpool"
	"github.com/theQRL/go-qrl/core/txpool/legacypool"
	"github.com/theQRL/go-qrl/core/types"
	"github.com/theQRL/go-qrl/core/vm"
	"github.com/theQRL/go-qrl/event"
	"github.com/theQRL/go-qrl/internal/qrlapi"
	"github.com/theQRL/go-qrl/internal/shutdowncheck"
	"github.com/theQRL/go-qrl/internal/version"
	"github.com/theQRL/go-qrl/log"
	"github.com/theQRL/go-qrl/miner"
	"github.com/theQRL/go-qrl/node"
	"github.com/theQRL/go-qrl/p2p"
	"github.com/theQRL/go-qrl/p2p/dnsdisc"
	"github.com/theQRL/go-qrl/p2p/qnode"
	"github.com/theQRL/go-qrl/params"
	"github.com/theQRL/go-qrl/qrl/downloader"
	"github.com/theQRL/go-qrl/qrl/gasprice"
	"github.com/theQRL/go-qrl/qrl/protocols/qrl"
	"github.com/theQRL/go-qrl/qrl/protocols/snap"
	"github.com/theQRL/go-qrl/qrl/qrlconfig"
	"github.com/theQRL/go-qrl/qrldb"
	"github.com/theQRL/go-qrl/rlp"
	"github.com/theQRL/go-qrl/rpc"
	gqrlversion "github.com/theQRL/go-qrl/version"
)

// QRL implements the QRL full node service.
type QRL struct {
	config *qrlconfig.Config

	// Handlers
	txPool *txpool.TxPool

	blockchain         *core.BlockChain
	handler            *handler
	qrlDialCandidates  qnode.Iterator
	snapDialCandidates qnode.Iterator

	// DB interfaces
	chainDb qrldb.Database // Block chain database

	eventMux       *event.TypeMux
	engine         consensus.Engine
	accountManager *accounts.Manager

	bloomRequests     chan chan *bloombits.Retrieval // Channel receiving bloom data retrieval requests
	bloomIndexer      *core.ChainIndexer             // Bloom indexer operating during block imports
	closeBloomHandler chan struct{}

	APIBackend *QRLAPIBackend

	miner    *miner.Miner
	gasPrice *big.Int

	networkID     uint64
	netRPCService *qrlapi.NetAPI

	p2pServer *p2p.Server

	lock sync.RWMutex // Protects the variadic fields (e.g. gas price and etherbase)

	shutdownTracker *shutdowncheck.ShutdownTracker // Tracks if and when the node has shutdown ungracefully
}

// New creates a new QRL object (including the initialisation of the common QRL object),
// whose lifecycle will be managed by the provided node.
func New(stack *node.Node, config *qrlconfig.Config) (*QRL, error) {
	// Ensure configuration values are compatible and sane
	if !config.SyncMode.IsValid() {
		return nil, fmt.Errorf("invalid sync mode %d", config.SyncMode)
	}
	if config.Miner.GasPrice == nil || config.Miner.GasPrice.Sign() <= 0 {
		log.Warn("Sanitizing invalid miner gas price", "provided", config.Miner.GasPrice, "updated", qrlconfig.Defaults.Miner.GasPrice)
		config.Miner.GasPrice = new(big.Int).Set(qrlconfig.Defaults.Miner.GasPrice)
	}
	if config.NoPruning && config.TrieDirtyCache > 0 {
		if config.SnapshotCache > 0 {
			config.TrieCleanCache += config.TrieDirtyCache * 3 / 5
			config.SnapshotCache += config.TrieDirtyCache * 2 / 5
		} else {
			config.TrieCleanCache += config.TrieDirtyCache
		}
		config.TrieDirtyCache = 0
	}
	log.Info("Allocated trie memory caches", "clean", common.StorageSize(config.TrieCleanCache)*1024*1024, "dirty", common.StorageSize(config.TrieDirtyCache)*1024*1024)

	// Assemble the QRL object
	chainDb, err := stack.OpenDatabaseWithFreezer("chaindata", config.DatabaseCache, config.DatabaseHandles, config.DatabaseFreezer, "qrl/db/chaindata/", false)
	if err != nil {
		return nil, err
	}
	// Try to recover offline state pruning only in hash-based.
	if config.StateScheme == rawdb.HashScheme {
		if err := pruner.RecoverPruning(stack.ResolvePath(""), chainDb); err != nil {
			log.Error("Failed to recover state", "error", err)
		}
	}
	chainConfig, err := core.LoadChainConfig(chainDb, config.Genesis)
	if err != nil {
		return nil, err
	}
	engine := qrlconfig.CreateConsensusEngine()
	networkID := config.NetworkId
	if networkID == 0 {
		networkID = chainConfig.ChainID.Uint64()
	}
	qrl := &QRL{
		config:            config,
		chainDb:           chainDb,
		eventMux:          stack.EventMux(),
		accountManager:    stack.AccountManager(),
		engine:            engine,
		closeBloomHandler: make(chan struct{}),
		networkID:         networkID,
		gasPrice:          config.Miner.GasPrice,
		bloomRequests:     make(chan chan *bloombits.Retrieval),
		bloomIndexer:      core.NewBloomIndexer(chainDb, params.BloomBitsBlocks, params.BloomConfirms),
		p2pServer:         stack.Server(),
		shutdownTracker:   shutdowncheck.NewShutdownTracker(chainDb),
	}
	bcVersion := rawdb.ReadDatabaseVersion(chainDb)
	var dbVer = "<nil>"
	if bcVersion != nil {
		dbVer = fmt.Sprintf("%d", *bcVersion)
	}
	log.Info("Initialising QRL protocol", "network", networkID, "dbversion", dbVer)

	if !config.SkipBcVersionCheck {
		if bcVersion != nil && *bcVersion > core.BlockChainVersion {
			return nil, fmt.Errorf("database version is v%d, Gqrl %s only supports v%d", *bcVersion, version.WithMeta, core.BlockChainVersion)
		} else if bcVersion == nil || *bcVersion < core.BlockChainVersion {
			if bcVersion != nil { // only print warning on upgrade, not on init
				log.Warn("Upgrade blockchain database version", "from", dbVer, "to", core.BlockChainVersion)
			}
			rawdb.WriteDatabaseVersion(chainDb, core.BlockChainVersion)
		}
	}
	var (
		vmConfig = vm.Config{
			EnablePreimageRecording: config.EnablePreimageRecording,
		}
		cacheConfig = &core.CacheConfig{
			TrieCleanLimit:      config.TrieCleanCache,
			TrieCleanNoPrefetch: config.NoPrefetch,
			TrieDirtyLimit:      config.TrieDirtyCache,
			TrieDirtyDisabled:   config.NoPruning,
			TrieTimeLimit:       config.TrieTimeout,
			SnapshotLimit:       config.SnapshotCache,
			Preimages:           config.Preimages,
			StateHistory:        config.StateHistory,
			StateScheme:         config.StateScheme,
		}
	)
	qrl.blockchain, err = core.NewBlockChain(chainDb, cacheConfig, config.Genesis, qrl.engine, vmConfig, &config.TransactionHistory)
	if err != nil {
		return nil, err
	}
	qrl.bloomIndexer.Start(qrl.blockchain)

	if config.TxPool.Journal != "" {
		config.TxPool.Journal = stack.ResolvePath(config.TxPool.Journal)
	}
	legacyPool := legacypool.New(config.TxPool, qrl.blockchain)
	qrl.txPool, err = txpool.New(new(big.Int).SetUint64(config.TxPool.PriceLimit), qrl.blockchain, []txpool.SubPool{legacyPool})
	if err != nil {
		return nil, err
	}
	// Permit the downloader to use the trie cache allowance during fast sync
	cacheLimit := cacheConfig.TrieCleanLimit + cacheConfig.TrieDirtyLimit + cacheConfig.SnapshotLimit
	if qrl.handler, err = newHandler(&handlerConfig{
		NodeID:         qrl.p2pServer.Self().ID(),
		Database:       chainDb,
		Chain:          qrl.blockchain,
		TxPool:         qrl.txPool,
		Network:        networkID,
		Sync:           config.SyncMode,
		BloomCache:     uint64(cacheLimit),
		EventMux:       qrl.eventMux,
		RequiredBlocks: config.RequiredBlocks,
	}); err != nil {
		return nil, err
	}

	qrl.miner = miner.New(qrl, config.Miner, qrl.engine)
	qrl.miner.SetExtra(makeExtraData(config.Miner.ExtraData))

	qrl.APIBackend = &QRLAPIBackend{stack.Config().ExtRPCEnabled(), qrl, nil}

	gpoParams := config.GPO
	if gpoParams.Default == nil {
		gpoParams.Default = config.Miner.GasPrice
	}
	qrl.APIBackend.gpo = gasprice.NewOracle(qrl.APIBackend, gpoParams)

	// Setup DNS discovery iterators.
	dnsclient := dnsdisc.NewClient(dnsdisc.Config{})
	qrl.qrlDialCandidates, err = dnsclient.NewIterator(qrl.config.QRLDiscoveryURLs...)
	if err != nil {
		return nil, err
	}
	qrl.snapDialCandidates, err = dnsclient.NewIterator(qrl.config.SnapDiscoveryURLs...)
	if err != nil {
		return nil, err
	}

	// Start the RPC service
	qrl.netRPCService = qrlapi.NewNetAPI(qrl.p2pServer, networkID)

	// Register the backend on the node
	stack.RegisterAPIs(qrl.APIs())
	stack.RegisterProtocols(qrl.Protocols())
	stack.RegisterLifecycle(qrl)

	// Successful startup; push a marker and check previous unclean shutdowns.
	qrl.shutdownTracker.MarkStartup()

	return qrl, nil
}

func makeExtraData(extra []byte) []byte {
	if len(extra) == 0 {
		// create default extradata
		extra, _ = rlp.EncodeToBytes([]any{
			uint(gqrlversion.Major<<16 | gqrlversion.Minor<<8 | gqrlversion.Patch),
			"gqrl",
			runtime.Version(),
			runtime.GOOS,
		})
	}
	if uint64(len(extra)) > params.MaximumExtraDataSize {
		log.Warn("Miner extra data exceed limit", "extra", hexutil.Bytes(extra), "limit", params.MaximumExtraDataSize)
		extra = nil
	}
	return extra
}

// APIs return the collection of RPC services the qrl package offers.
// NOTE, some of these services probably need to be moved to somewhere else.
func (s *QRL) APIs() []rpc.API {
	apis := qrlapi.GetAPIs(s.APIBackend)

	// Append any APIs exposed explicitly by the consensus engine
	apis = append(apis, s.engine.APIs(s.BlockChain())...)

	// Append all the local APIs and return
	return append(apis, []rpc.API{
		{
			Namespace: "miner",
			Service:   NewMinerAPI(s),
		}, {
			Namespace: "qrl",
			Service:   downloader.NewDownloaderAPI(s.handler.downloader, s.eventMux),
		}, {
			Namespace: "admin",
			Service:   NewAdminAPI(s),
		}, {
			Namespace: "debug",
			Service:   NewDebugAPI(s),
		}, {
			Namespace: "net",
			Service:   s.netRPCService,
		},
	}...)
}

func (s *QRL) ResetWithGenesisBlock(gb *types.Block) {
	s.blockchain.ResetWithGenesisBlock(gb)
}

func (s *QRL) Miner() *miner.Miner { return s.miner }

func (s *QRL) AccountManager() *accounts.Manager  { return s.accountManager }
func (s *QRL) BlockChain() *core.BlockChain       { return s.blockchain }
func (s *QRL) TxPool() *txpool.TxPool             { return s.txPool }
func (s *QRL) EventMux() *event.TypeMux           { return s.eventMux }
func (s *QRL) Engine() consensus.Engine           { return s.engine }
func (s *QRL) ChainDb() qrldb.Database            { return s.chainDb }
func (s *QRL) IsListening() bool                  { return true } // Always listening
func (s *QRL) Downloader() *downloader.Downloader { return s.handler.downloader }
func (s *QRL) Synced() bool                       { return s.handler.synced.Load() }
func (s *QRL) SetSynced()                         { s.handler.enableSyncedFeatures() }
func (s *QRL) ArchiveMode() bool                  { return s.config.NoPruning }
func (s *QRL) BloomIndexer() *core.ChainIndexer   { return s.bloomIndexer }

// Protocols returns all the currently configured
// network protocols to start.
func (s *QRL) Protocols() []p2p.Protocol {
	protos := qrl.MakeProtocols((*qrlHandler)(s.handler), s.networkID, s.qrlDialCandidates)
	if s.config.SnapshotCache > 0 {
		protos = append(protos, snap.MakeProtocols((*snapHandler)(s.handler), s.snapDialCandidates)...)
	}
	return protos
}

// Start implements node.Lifecycle, starting all internal goroutines needed by the
// QRL protocol implementation.
func (s *QRL) Start() error {
	qrl.StartQNRUpdater(s.blockchain, s.p2pServer.LocalNode())

	// Start the bloom bits servicing goroutines
	s.startBloomHandlers(params.BloomBitsBlocks)

	// Regularly update shutdown marker
	s.shutdownTracker.Start()

	// Figure out a max peers count based on the server limits
	maxPeers := s.p2pServer.MaxPeers

	// Start the networking layer if requested
	s.handler.Start(maxPeers)
	return nil
}

// Stop implements node.Lifecycle, terminating all internal goroutines used by the
// QRL protocol.
func (s *QRL) Stop() error {
	// Stop all the peer-related stuff first.
	s.qrlDialCandidates.Close()
	s.snapDialCandidates.Close()
	s.handler.Stop()

	// Then stop everything else.
	s.bloomIndexer.Close()
	close(s.closeBloomHandler)
	s.txPool.Close()
	s.blockchain.Stop()
	s.engine.Close()

	// Clean shutdown marker as the last thing before closing db
	s.shutdownTracker.Stop()

	s.chainDb.Close()
	s.eventMux.Stop()

	return nil
}

// SyncMode retrieves the current sync mode, either explicitly set, or derived
// from the chain status.
func (s *QRL) SyncMode() downloader.SyncMode {
	// If we're in snap sync mode, return that directly
	if s.handler.snapSync.Load() {
		return downloader.SnapSync
	}
	// We are probably in full sync, but we might have rewound to before the
	// snap sync pivot, check if we should re-enable snap sync.
	head := s.blockchain.CurrentBlock()
	if pivot := rawdb.ReadLastPivotNumber(s.chainDb); pivot != nil {
		if head.Number.Uint64() < *pivot {
			return downloader.SnapSync
		}
	}
	// We are in a full sync, but the associated head state is missing. To complete
	// the head state, forcefully rerun the snap sync. Note it doesn't mean the
	// persistent state is corrupted, just mismatch with the head block.
	if !s.blockchain.HasState(head.Root) {
		log.Info("Reenabled snap sync as chain is stateless")
		return downloader.SnapSync
	}
	// Nope, we're really full syncing
	return downloader.FullSync
}
