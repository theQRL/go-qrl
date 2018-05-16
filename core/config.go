package core

type Config struct {
	Dev  *DevConfig
	User *UserConfig
}

type MinerConfig struct {
	MiningEnabled     bool
	MiningAddress     string
	MiningThreadCount uint16
}

type NodeConfig struct {
	EnablePeerDiscovery bool
	PeerList            []string
	BindingIP			string
	LocalPort           uint16
	PublicPort          uint16
	PeerRateLimit       uint16
	BanMinutes          uint8
	MaxPeersLimit       uint16
}

type EphemeralConfig struct {
	AcceptEphemeral bool
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
	Node *NodeConfig
	Miner *MinerConfig
	Ephemeral *EphemeralConfig

	NtpServers    []string

	ChainStateTimeout         uint16
	ChainStateBroadcastPeriod uint16

	TransactionPool *TransactionPoolConfig

	QrlDir string

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
	Genesis              *GenesisConfig

	BlocksPerEpoch		 uint64
	BlockLeadTimestamp   uint32
	BlockMaxDrift        uint16
	MaxFutureBlockLength uint16
	MaxMarginBlocKNumber uint16
	MinMarginBlockNumber uint16

	ReorgLimit uint64

	MessageReceiptTimeout uint32
	MessageBufferSize     uint32

	MaxOTSTracking  uint16
	OtsBitFieldSize uint16

	MiningNonceOffset uint16
	ExtraNonceOffset  uint16
	MiningBlobSize    uint16

	DefaultNonce            uint8
	DefaultAccountBalance   uint64
	MiningSetpointBlocktime uint32

	DBName              string
	PeersFilename       string
	ChainFileDirectory  string
	WalletDatFilename   string
	BannedPeersFilename string

	Transaction *TransactionConfig

	Token *TokenConfig

	NMeasurement uint8
	KP           uint8

	NumberOfBlockAnalyze uint8
	SizeMultiplier       float64
	BlockMinSizeLimit    uint64

	ShorPerQuanta uint64

	MaxReceivableBytes uint64
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
	Version              string
	GenesisPrevHeadehash []byte
	MaxCoinSupply        uint64
	SuppliedCoins        uint64
	GenesisDifficulty    uint64
	CoinbaseAddress      []byte
	GenesisTimestamp     uint32
}

func GetConfig() (config *Config) {
	userConfig := GetUserConfig()
	devConfig := GetDevConfig()

	return &Config{
		User: userConfig,
		Dev: devConfig,
	}
}

func GetUserConfig() (user *UserConfig) {
	node := &NodeConfig {
		EnablePeerDiscovery: true,
		PeerList: []string{
			"35.177.60.137",
			"104.251.219.215",
			"104.251.219.145",
			"104.251.219.40",
			"104.237.3.185",
		},
		BindingIP: "0.0.0.0",
		LocalPort: 9000,
		PublicPort: 9000,
		PeerRateLimit: 500,
		BanMinutes: 20,
		MaxPeersLimit: 100,
	}

	miner := &MinerConfig {
		MiningEnabled: false,
		MiningAddress: "",
		MiningThreadCount: 0,
	}

	ephemeral := &EphemeralConfig {
		AcceptEphemeral: false,
	}

	transactionPool := &TransactionPoolConfig {
		TransactionPoolSize: 25000,
		PendingTransactionPoolSize: 75000,
		PendingTranactionPoolReserve: 750,
		StaleTransactionThreshold: 15,
	}

	adminAPI := &APIConfig {
		Enabled: false,
		Host: "127.0.0.1",
		Port: 9008,
		Threads: 1,
		MaxConcurrentRPC: 100,
	}

	publicAPI := &APIConfig {
		Enabled: true,
		Host: "127.0.0.1",
		Port: 9009,
		Threads: 1,
		MaxConcurrentRPC: 100,
	}

	miningAPI := &APIConfig {
		Enabled: false,
		Host: "127.0.0.1",
		Port: 9007,
		Threads: 1,
		MaxConcurrentRPC: 100,
	}

	api := &API{
		AdminAPI: adminAPI,
		PublicAPI: publicAPI,
		MiningAPI: miningAPI,
	}

	user = &UserConfig{
		Node: node,
		Miner: miner,
		Ephemeral: ephemeral,

		NtpServers: []string{"pool.ntp.org", "ntp.ubuntu.com"},

		ChainStateTimeout: 180,
		ChainStateBroadcastPeriod: 30,

		TransactionPool: transactionPool,

		QrlDir: "~/.qrl",

		API: api,
	}

	return user
}

func GetDevConfig() (dev *DevConfig) {
	genesis := &GenesisConfig{
		Version: "v0.63",
		GenesisPrevHeadehash: []byte("Outside Context Problem"),
		MaxCoinSupply: 105000000,
		SuppliedCoins: 65000000 * (10 ^ 9),
		GenesisDifficulty: 5000,
		CoinbaseAddress: []byte("010300082382a52f8ba9c2d33ad807c2cdd5bd086c2c2fe63c6ea13b630d1280894c3a39e1c380"),
		GenesisTimestamp: 1524928900,
	}
	transaction := &TransactionConfig{
		MultiOutputLimit: 100,
	}

	token := &TokenConfig{
		MaxSymbolLength: 10,
		MaxNameLength: 30,
	}

	dev = &DevConfig{
		Genesis: genesis,

		BlocksPerEpoch:		 100,
		BlockLeadTimestamp:   30,
		BlockMaxDrift:        15,
		MaxFutureBlockLength: 256,
		MaxMarginBlocKNumber: 32,
		MinMarginBlockNumber: 7,

		ReorgLimit: 7 * 24 * 60,

		MessageReceiptTimeout: 10,
		MessageBufferSize:     3 * 1024 * 1024,

		MaxOTSTracking:  4096,
		OtsBitFieldSize: 4096 / 8,

		MiningNonceOffset: 39,
		ExtraNonceOffset:  43,
		MiningBlobSize:    76,

		DefaultNonce:            0,
		DefaultAccountBalance:   0,
		MiningSetpointBlocktime: 60,

		DBName:              "state",
		PeersFilename:       "peers.qrl",
		ChainFileDirectory:  "data",
		WalletDatFilename:   "wallet.json",
		BannedPeersFilename: "banned_peers.qrl",

		Transaction: transaction,

		Token: token,

		NMeasurement: 250,
		KP:           5,

		NumberOfBlockAnalyze: 10,
		SizeMultiplier:       1.1,
		BlockMinSizeLimit:    1024 * 1024,

		ShorPerQuanta: 10 ^ 9,

		MaxReceivableBytes: 10 * 1024 * 1024,
		SyncDelayMining:    60,

		BlockTimeSeriesSize: 1440,
	}
	return dev
}