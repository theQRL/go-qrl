package main

type UserConfig struct {
	miningEnabled     bool
	miningAddress     string
	miningThreadCount uint16

	acceptEphemeral bool

	enablePeerDiscovery bool
	peerList            []string
	p2pLocalPort        uint16
	p2pPublicPort       uint16

	peerRateLimit uint16
	ntpServers    []string
	banMinutes    uint8

	monitorConnectionInterval uint8
	maxPeersLimit             uint16
	chainStateTimeout         uint16
	chainStateBroadcastPeriod uint16

	transactionPoolSize          uint64
	pendingTransactionPoolSize   uint64
	pendingTranactionPoolReserve uint64
	staleTransactionThreshold    uint64

	qrlDir string

	adminAPI  *APIConfig
	publicAPI *APIConfig
	miningAPI *APIConfig
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
	genesisTimestamp     uint32

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
}