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
	version              string
	genesisPrevHeadehash []byte

	blockLeadTimestamp   uint32
	blockMaxDrift        uint16
	maxFutureBlockLength uint16
	maxMarginBlocKNumber uint16
	minMarginBlockNumber uint16

	reorgLimit uint64

	messageReceiptTimeout uint32
	messageBufferSize     uint32

	maxCoinSupply uint64
	suppliedCoins uint64

	maxOTSTracking uint16

	miningNonceOffset uint16
	extraNonceOffset  uint16
	miningBlobSize    uint16

	otsBitFieldSize uint16

	defaultNonce            uint8
	defaultAccountBalance   uint64
	miningSetpointBlocktime uint32
	genesisDifficulty       uint64
	coinbaseAddress         []byte

	dbName              string
	peersFilename       string
	chainFileDirectory  string
	walletDatFilename   string
	bannedPeersFilename string

	genesisTimestamp uint32

	transactionMultiOutputLimit uint8

	maxTokenSymbolLength uint8
	maxTokenNameLength   uint8

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