package main

type Config struct {
	dev  *DevConfig
	user *UserConfig
}

type MinerConfig struct {
	miningEnabled     bool
	miningAddress     string
	miningThreadCount uint16
}

type NodeConfig struct {
	enablePeerDiscovery bool
	peerList            []string
	localPort        uint16
	publicPort       uint16
	peerRateLimit		uint16
	banMinutes    		uint8
	maxPeersLimit       uint16
}

type EphemeralConfig struct {
	acceptEphemeral bool
}

type TransactionPoolConfig struct {
	transactionPoolSize          uint64
	pendingTransactionPoolSize   uint64
	pendingTranactionPoolReserve uint64
	staleTransactionThreshold    uint64
}

type API struct {
	adminAPI  *APIConfig
	publicAPI *APIConfig
	miningAPI *APIConfig
}

type UserConfig struct {
	node *NodeConfig
	miner *MinerConfig
	ephemeral *EphemeralConfig

	ntpServers    []string

	chainStateTimeout         uint16
	chainStateBroadcastPeriod uint16

	transactionPool *TransactionPoolConfig

	qrlDir string

	api *API
}

type APIConfig struct {
	enabled          bool
	host             string
	port             uint32
	threads          uint32
	maxConcurrentRPC uint16
}

type DevConfig struct {
	genesis              *GenesisConfig

	blockLeadTimestamp   uint32
	blockMaxDrift        uint16
	maxFutureBlockLength uint16
	maxMarginBlocKNumber uint16
	minMarginBlockNumber uint16

	reorgLimit uint64

	messageReceiptTimeout uint32
	messageBufferSize     uint32

	maxOTSTracking  uint16
	otsBitFieldSize uint16

	miningNonceOffset uint16
	extraNonceOffset  uint16
	miningBlobSize    uint16

	defaultNonce            uint8
	defaultAccountBalance   uint64
	miningSetpointBlocktime uint32

	dbName              string
	peersFilename       string
	chainFileDirectory  string
	walletDatFilename   string
	bannedPeersFilename string

	transaction *TransactionConfig

	token *TokenConfig

	nMeasurement uint8
	kp           uint8

	numberOfBlockAnalyze uint8
	sizeMultiplier       float64
	blockMinSizeLimit    uint64

	shorPerQuanta uint64

	maxReceivableBytes uint64
	syncDelayMining    uint8

	blockTimeSeriesSize uint32
}

type TransactionConfig struct {
	multiOutputLimit uint8
}

type TokenConfig struct {
	maxSymbolLength uint8
	maxNameLength   uint8
}

type GenesisConfig struct {
	version              string
	genesisPrevHeadehash []byte
	maxCoinSupply        uint64
	suppliedCoins        uint64
	genesisDifficulty    uint64
	coinbaseAddress      []byte
	genesisTimestamp     uint32
}

func GetUserConfig() (user *UserConfig) {
	node := &NodeConfig {
		enablePeerDiscovery: true,
		peerList: []string{
			"35.177.60.137",
			"104.251.219.215",
			"104.251.219.145",
			"104.251.219.40",
			"104.237.3.185",
		},
		localPort: 9000,
		publicPort: 9000,
		peerRateLimit: 500,
		banMinutes: 20,
		maxPeersLimit: 100,
	}

	miner := &MinerConfig {
		miningEnabled: false,
		miningAddress: "",
		miningThreadCount: 0,
	}

	ephemeral := &EphemeralConfig {
		acceptEphemeral: false,
	}

	transactionPool := &TransactionPoolConfig {
		transactionPoolSize: 25000,
		pendingTransactionPoolSize: 75000,
		pendingTranactionPoolReserve: 750,
		staleTransactionThreshold: 15,
	}

	adminAPI := &APIConfig {
		enabled: false,
		host: "127.0.0.1",
		port: 9008,
		threads: 1,
		maxConcurrentRPC: 100,
	}

	publicAPI := &APIConfig {
		enabled: true,
		host: "127.0.0.1",
		port: 9009,
		threads: 1,
		maxConcurrentRPC: 100,
	}

	miningAPI := &APIConfig {
		enabled: false,
		host: "127.0.0.1",
		port: 9007,
		threads: 1,
		maxConcurrentRPC: 100,
	}

	api := &API{
		adminAPI: adminAPI,
		publicAPI: publicAPI,
		miningAPI: miningAPI,
	}

	user = &UserConfig{
		node: node,
		miner: miner,
		ephemeral: ephemeral,

		ntpServers: []string{"pool.ntp.org", "ntp.ubuntu.com"},

		chainStateTimeout: 180,
		chainStateBroadcastPeriod: 30,

		transactionPool: transactionPool,

		qrlDir: "~/.qrl",

		api: api,
	}

	return user
}

func GetDevConfig() (dev *DevConfig) {
	genesis := &GenesisConfig{
		version: "v0.63",
		genesisPrevHeadehash: []byte("Outside Context Problem"),
		maxCoinSupply: 105000000,
		suppliedCoins: 65000000 * (10 ^ 9),
		genesisDifficulty: 5000,
		coinbaseAddress: []byte("010300082382a52f8ba9c2d33ad807c2cdd5bd086c2c2fe63c6ea13b630d1280894c3a39e1c380"),
		genesisTimestamp: 1524928900,
	}
	transaction := &TransactionConfig{
		multiOutputLimit: 100,
	}

	token := &TokenConfig{
		maxSymbolLength: 10,
		maxNameLength: 30,
	}

	dev = &DevConfig{
		genesis: genesis,

		blockLeadTimestamp:   30,
		blockMaxDrift:        15,
		maxFutureBlockLength: 256,
		maxMarginBlocKNumber: 32,
		minMarginBlockNumber: 7,

		reorgLimit: 7 * 24 * 60,

		messageReceiptTimeout: 10,
		messageBufferSize:     3 * 1024 * 1024,

		maxOTSTracking:  4096,
		otsBitFieldSize: 4096 / 8,

		miningNonceOffset: 39,
		extraNonceOffset:  43,
		miningBlobSize:    76,

		defaultNonce:            0,
		defaultAccountBalance:   0,
		miningSetpointBlocktime: 60,

		dbName:              "state",
		peersFilename:       "peers.qrl",
		chainFileDirectory:  "data",
		walletDatFilename:   "wallet.json",
		bannedPeersFilename: "banned_peers.qrl",

		transaction: transaction,

		token: token,

		nMeasurement: 250,
		kp:           5,

		numberOfBlockAnalyze: 10,
		sizeMultiplier:       1.1,
		blockMinSizeLimit:    1024 * 1024,

		shorPerQuanta: 10 ^ 9,

		maxReceivableBytes: 10 * 1024 * 1024,
		syncDelayMining:    60,

		blockTimeSeriesSize: 1440,
	}
	return dev
}