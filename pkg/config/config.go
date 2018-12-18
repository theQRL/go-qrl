package config

import (
	"github.com/theQRL/go-qrl/pkg/misc"
	"os/user"
	"path"
	"sync"
)

type Config struct {
	Dev  *DevConfig
	User *UserConfig
}

type MinerConfig struct {
	MiningEnabled     bool
	MiningAddress     string
	MiningThreadCount uint
}

type NodeConfig struct {
	EnablePeerDiscovery      bool
	PeerList                 []string
	BindingIP                string
	LocalPort                uint16
	PublicPort               uint16
	PeerRateLimit            uint64
	BanMinutes               uint8
	MaxPeersLimit            uint16
	MaxPeersInPeerList       uint64
	MaxRedundantConnections  int
}

type NotificationServerConfig struct {
	EnableNotificationServer bool
	BindingIP                string
	LocalPort                uint16
}

type EphemeralConfig struct {
	AcceptEphemeral bool
}

type NTPConfig struct {
	Retries int
	Servers []string
	Refresh uint64
}

type TransactionPoolConfig struct {
	TransactionPoolSize          uint64
	PendingTransactionPoolSize   uint64
	PendingTranactionPoolReserve uint64
	StaleTransactionThreshold    uint64
}

type API struct {
	AdminAPI  *APIConfig
	PublicAPI *APIConfig
	MiningAPI *APIConfig
}

type UserConfig struct {
	Node                     *NodeConfig
	NotificationServerConfig *NotificationServerConfig
	Miner                    *MinerConfig
	Ephemeral                *EphemeralConfig

	NTP *NTPConfig

	ChainStateTimeout         uint16
	ChainStateBroadcastPeriod uint16

	TransactionPool *TransactionPoolConfig

	QrlDir             string
	ChainFileDirectory string

	API *API
}

type APIConfig struct {
	Enabled          bool
	Host             string
	Port             uint32
	Threads          uint32
	MaxConcurrentRPC uint16
}

type DevConfig struct {
	Genesis *GenesisConfig

	Version string

	BlocksPerEpoch       uint64
	BlockLeadTimestamp   uint32
	BlockMaxDrift        uint16
	MaxFutureBlockLength uint16
	MaxMarginBlockNumber uint16
	MinMarginBlockNumber uint16

	ReorgLimit uint64

	MessageQSize		  uint32
	MessageReceiptTimeout uint32
	MessageBufferSize     uint32

	MaxOTSTracking  uint64
	OtsBitFieldSize uint64

	MiningNonceOffset int64
	ExtraNonceOffset  uint16
	MiningBlobSize    uint16

	DefaultNonce            uint8
	DefaultAccountBalance   uint64
	MiningSetpointBlocktime uint64

	DBName              string
	PeersFilename       string
	WalletDatFilename   string
	BannedPeersFilename string

	Transaction *TransactionConfig

	Token *TokenConfig

	NMeasurement uint8
	KP           int64

	NumberOfBlockAnalyze uint8
	SizeMultiplier       float64
	BlockMinSizeLimit    int
	TxExtraOverhead		 int

	ShorPerQuanta uint64

	MaxReceivableBytes uint64
	ReservedQuota	   uint64
	MaxBytesOut		   uint64
	SyncDelayMining    uint8

	BlockTimeSeriesSize uint32
}

type TransactionConfig struct {
	MultiOutputLimit uint8
}

type TokenConfig struct {
	MaxSymbolLength uint8
	MaxNameLength   uint8
}

type GenesisConfig struct {
	GenesisPrevHeadehash []byte
	MaxCoinSupply        uint64
	SuppliedCoins        uint64
	GenesisDifficulty    uint64
	CoinbaseAddress      []byte
	GenesisTimestamp     uint32
}

var once sync.Once
var config *Config

func GetConfig() *Config {
	once.Do(func() {
		userConfig := GetUserConfig()
		devConfig := GetDevConfig()
		config = &Config{
			User: userConfig,
			Dev:  devConfig,
		}
	})

	return config
}

func GetUserConfig() (userConf *UserConfig) {
	node := &NodeConfig{
		EnablePeerDiscovery: true,
		PeerList: []string{
			"35.178.79.137:19000",
			"35.177.182.85:19000",
			"18.130.119.29:19000",
			"18.130.25.64:19000",
		},
		BindingIP:               "0.0.0.0",
		LocalPort:               19000,
		PublicPort:              19000,
		PeerRateLimit:           500,
		BanMinutes:              20,
		MaxPeersLimit:           32,
		MaxPeersInPeerList:      100,
		MaxRedundantConnections: 5,
	}

	notificationServerConfig := &NotificationServerConfig{
		EnableNotificationServer: true,
		BindingIP:                "0.0.0.0",
		LocalPort:                18001,
	}

	miner := &MinerConfig{
		MiningEnabled:     false,
		MiningAddress:     "",
		MiningThreadCount: 0,
	}

	ephemeral := &EphemeralConfig{
		AcceptEphemeral: false,
	}

	ntp := &NTPConfig{
		Retries: 6,
		Servers: []string{"pool.ntp.org", "ntp.ubuntu.com"},
		Refresh: 12 * 60 * 60,
	}

	transactionPool := &TransactionPoolConfig{
		TransactionPoolSize:          25000,
		PendingTransactionPoolSize:   75000,
		PendingTranactionPoolReserve: 750,
		StaleTransactionThreshold:    15,
	}

	adminAPI := &APIConfig{
		Enabled:          false,
		Host:             "127.0.0.1",
		Port:             19008,
		Threads:          1,
		MaxConcurrentRPC: 100,
	}

	publicAPI := &APIConfig{
		Enabled:          true,
		Host:             "127.0.0.1",
		Port:             19009,
		Threads:          1,
		MaxConcurrentRPC: 100,
	}

	miningAPI := &APIConfig{
		Enabled:          false,
		Host:             "127.0.0.1",
		Port:             19007,
		Threads:          1,
		MaxConcurrentRPC: 100,
	}

	api := &API{
		AdminAPI:  adminAPI,
		PublicAPI: publicAPI,
		MiningAPI: miningAPI,
	}
	userCurrentDir, _ := user.Current()  // TODO: Handle error
	userConf = &UserConfig{
		Node: node,
		NotificationServerConfig: notificationServerConfig,
		Miner:     miner,
		Ephemeral: ephemeral,

		NTP: ntp,

		ChainStateTimeout:         180,
		ChainStateBroadcastPeriod: 30,

		TransactionPool: transactionPool,

		QrlDir:             path.Join(userCurrentDir.HomeDir, ".qrl"),
		ChainFileDirectory: "data",

		API: api,
	}

	return userConf
}

func (u *UserConfig) DataDir() string {
	return path.Join(u.QrlDir, u.ChainFileDirectory)
}

func (u *UserConfig) SetDataDir(dataDir string) {
	u.QrlDir = dataDir
}

func (u *UserConfig) GetLogFileName() string {
	return path.Join(u.QrlDir, "go-qrl.log")
}

func GetDevConfig() (dev *DevConfig) {
	genesis := &GenesisConfig{
		GenesisPrevHeadehash: []byte("Outside Context Problem"),
		MaxCoinSupply:        105000000000000000,
		SuppliedCoins:        65000000000000000,
		GenesisDifficulty:    10000000,
		CoinbaseAddress:      misc.HStr2Bin("0000000000000000000000000000000000000000000000000000000000000000"),
		GenesisTimestamp:     1530004179,
	}
	transaction := &TransactionConfig{
		MultiOutputLimit: 100,
	}

	token := &TokenConfig{
		MaxSymbolLength: 10,
		MaxNameLength:   30,
	}

	dev = &DevConfig{
		Genesis: genesis,

		Version: "0.0.1 go",

		BlocksPerEpoch:       100,
		BlockLeadTimestamp:   30,
		BlockMaxDrift:        15,
		MaxFutureBlockLength: 256,
		MaxMarginBlockNumber: 32,
		MinMarginBlockNumber: 7,

		ReorgLimit: 22000,

		MessageQSize:		   300,
		MessageReceiptTimeout: 10,
		MessageBufferSize:     64 * 1024 * 1024,

		MaxOTSTracking:  8192,
		OtsBitFieldSize: 8192 / 8,

		MiningNonceOffset: 39,
		ExtraNonceOffset:  43,
		MiningBlobSize:    76,

		DefaultNonce:            0,
		DefaultAccountBalance:   0,
		MiningSetpointBlocktime: 60,

		DBName:              "go-state",
		PeersFilename:       "known_peers.json",
		WalletDatFilename:   "wallet.json",
		BannedPeersFilename: "banned_peers.qrl",

		Transaction: transaction,

		Token: token,

		NMeasurement: 30,
		KP:           5,

		NumberOfBlockAnalyze: 10,
		SizeMultiplier:       1.1,
		BlockMinSizeLimit:    1024 * 1024,
		TxExtraOverhead:      15,

		ShorPerQuanta: 1000000000,

		MaxReceivableBytes: 10 * 1024 * 1024,
		ReservedQuota: 1024,
		SyncDelayMining:    60,

		BlockTimeSeriesSize: 1440,
	}
	dev.MaxBytesOut = dev.MaxReceivableBytes - dev.ReservedQuota
	return dev
}
