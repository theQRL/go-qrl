package miner

import (
	"github.com/theQRL/go-qrl/pkg/config"
	"github.com/theQRL/go-qrl/pkg/core/addressstate"
	"github.com/theQRL/go-qrl/pkg/core/block"
	"github.com/theQRL/go-qrl/pkg/core/chain"
	"github.com/theQRL/go-qrl/pkg/core/pool"
	"github.com/theQRL/go-qrl/pkg/core/transactions"
	"github.com/theQRL/go-qrl/pkg/log"
	"github.com/theQRL/go-qrl/pkg/misc"
	"github.com/theQRL/go-qrl/pkg/ntp"
	"github.com/theQRL/go-qrl/pkg/pow"
	"github.com/theQRL/qryptonight/goqryptonight"
	"reflect"
	"runtime"
	"strconv"
	"sync"
)

type Miner struct {
	goqryptonight.Qryptominer

	lock sync.Mutex

	miningBlock *block.Block
	chain       *chain.Chain
	ntp         ntp.NTPInterface
	config      *config.Config
	log         log.LoggerInterface
	dt          pow.DifficultyTrackerInterface

	measurement       uint64
	currentDifficulty []byte
	currentTarget     []byte
	miningThreadCount uint
}

func (m *Miner) PrepareNextBlockTemplate(minerAddress []byte, parentBlock *block.Block, txPool *pool.TransactionPool, enableLocking bool) error {
	if enableLocking {
		m.lock.Lock()
		defer m.lock.Unlock()
	}

	var err error

	m.Cancel()
	m.miningBlock, err = m.CreateBlock(minerAddress, parentBlock, 0, txPool)
	if err != nil {
		m.log.Error("Error While Creating Block",
			"Error", err.Error())
		return err
	}

	parentMetaData, err := m.chain.GetBlockMetaData(parentBlock.HeaderHash())
	if err != nil {
		m.log.Error("Error GetBlockMetaData",
			"Error", err.Error())
		return err
	}

	m.measurement, err = m.chain.GetMeasurement(uint32(m.miningBlock.Timestamp()), m.miningBlock.PrevHeaderHash(), parentMetaData)
	if err != nil {
		m.log.Error("Error GetMeasurement",
			"Error", err.Error())
		return err
	}

	parentDifficulty := parentMetaData.BlockDifficulty()
	m.currentDifficulty, m.currentTarget = m.dt.Get(m.measurement, parentDifficulty)
	return nil
}

func (m *Miner) CreateBlock(minerAddress []byte, parentBlock *block.Block, miningNonce uint32, txPool *pool.TransactionPool) (*block.Block, error) {
	dummyBlock := block.CreateBlock(minerAddress, parentBlock.BlockNumber() + 1, parentBlock.HeaderHash(), parentBlock.Timestamp(), nil, m.ntp.Time())
	dummyBlock.SetNonces(miningNonce, 0)

	var txs []transactions.TransactionInterface
	var txsInfo []*pool.TransactionInfo

	blockSize := dummyBlock.Size()
	blockSizeLimit, err := m.chain.GetBlockSizeLimit(parentBlock)
	if err != nil {
		m.log.Error("Error while getting Block Size Limit",
			"Error", err.Error())
		return nil, err
	}

	addressesState := make(map[string]*addressstate.AddressState)
	for {
		txInfo := txPool.Pop()
		if txInfo == nil {
			break
		}
		tx := txInfo.Transaction()
		if blockSize + tx.Size() + m.config.Dev.TxExtraOverhead > blockSizeLimit {
			break
		}
		newAddressesState := make(map[string]*addressstate.AddressState)
		tx.SetAffectedAddress(newAddressesState)
		for qAddress := range newAddressesState {
			if _, ok := addressesState[qAddress]; !ok {
				addressState, err := m.chain.GetAddressState(misc.Qaddress2Bin(qAddress))
				if err != nil {
					m.log.Error("Error getting AddressState miner.CreateBlock",
						"Error", err.Error())
					return nil, err
				}
				addressesState[qAddress] = addressState
			}
		}
		addrFromPKState := addressesState[misc.Bin2Qaddress(tx.AddrFrom())]
		addrFromPK := tx.GetSlave()
		if addrFromPK != nil {
			addrFromPKState = addressesState[tx.AddrFromPK()]
		}

		if !tx.ValidateExtended(addressesState[misc.Bin2Qaddress(tx.AddrFrom())], addrFromPKState) {
			m.log.Warn("Txn validation failed for tx in tx_pool")
			txPool.Remove(tx)
			continue
		}

		tx.ApplyStateChanges(addressesState)
		tx.PBData().Nonce = addrFromPKState.Nonce()
		blockSize += tx.Size() + m.config.Dev.TxExtraOverhead
		txs = append(txs, tx)
		txsInfo = append(txsInfo, txInfo)
	}

	for _, txInfo := range txsInfo {
		err := txPool.Add(txInfo.Transaction(), txInfo.BlockNumber(), txInfo.Timestamp())
		if err != nil {
			m.log.Error("[Miner.CreateBlock] Error while adding transactions into transaction pool",
				"Error", err.Error())
		}
	}

	b := block.CreateBlock(minerAddress, parentBlock.BlockNumber() + 1, parentBlock.HeaderHash(), parentBlock.Timestamp(), txs, m.ntp.Time())
	return b, nil
}

func (m *Miner) HandleEvent(event goqryptonight.MinerEvent) byte {
	if event.GetXtype() != goqryptonight.SOLUTION {
		m.log.Info("UnExpected Event Type",
			"Type", event.GetXtype())
		return 0
	}

	m.log.Debug("HandleEvent - TRY Lock")
	// TODO: Deadlock, compare with python version think of some way to do it
	m.lock.Lock()
	defer m.lock.Unlock()

	m.log.Debug("HandleEvent - LOCKED")
	m.log.Debug("Solution Found", "Nonce", event.GetNonce())
	m.miningBlock.SetNonces(uint32(event.GetNonce()), 0)
	clonedBlock := m.miningBlock
	clonedBlock.SetNonces(uint32(event.GetNonce()), 0)
	// TODO: Call to PreBlockLogic
	return 1
}

func (m *Miner) StartMining(parentBlock *block.Block, parentDifficulty []byte) {
	m.log.Info("Start Mining - TRY LOCKING")
	m.lock.Lock()
	defer m.lock.Unlock()

	m.log.Debug("Start Mining - LOCKED")
	m.Cancel()
	miningBlob := m.miningBlock.MiningBlob()
	nonceOffset := m.config.Dev.MiningNonceOffset

	m.log.Debug("Mining",
		"Block #", m.miningBlock.BlockNumber())

	workSeqId := m.Start(misc.BytesToUCharVector(miningBlob), nonceOffset, misc.BytesToUCharVector(m.currentTarget), uint(0))
	m.log.Info("MINING STARTED",
		"SEQ ID", workSeqId)
}

func (m *Miner) GetBlockToMine(minerQAddress string, lastBlock *block.Block, lastBlockDifficulty []byte, txPool *pool.TransactionPool) (string, uint64, error) {
	m.lock.Lock()
	defer m.lock.Unlock()

	minerAddress := misc.Qaddress2Bin(minerQAddress)
	currentDifficulty, err := strconv.ParseUint(misc.Bin2HStr(lastBlockDifficulty), 16, 64)
	if err != nil {
		m.log.Error("Error while Parsing current difficulty",
			"Difficulty", misc.Bin2HStr(lastBlockDifficulty),
			"Error", err.Error())
		return "", 0, err
	}

	// TODO: Check if valid minerAddress
	if m.miningBlock != nil {
		if reflect.DeepEqual(lastBlock.HeaderHash(), m.miningBlock.PrevHeaderHash()) {
			return misc.Bin2HStr(m.miningBlock.MiningBlob()), currentDifficulty, nil
		} else {
			m.miningBlock.UpdateMiningAddress(minerAddress)
		}
	}
	m.PrepareNextBlockTemplate(minerAddress, lastBlock, txPool, false)

	return misc.Bin2HStr(m.miningBlock.MiningBlob()), currentDifficulty, nil
}

func (m *Miner) SubmitMinedBlock(blob []byte) bool {
	m.lock.Lock()
	defer m.lock.Unlock()

	if !m.miningBlock.VerifyBlob(blob) {
		m.log.Warn("[SubmitMinedBlock] Failed to verify Blob",
			"blob", blob)
		return false
	}

	blockHeader := m.miningBlock.BlockHeader() // Copying BlockHeader / Not a Reference
	blockHeader.SetMiningNonceFromBlob(blob)
	parentBlock, err := m.chain.GetBlock(m.miningBlock.PrevHeaderHash())
	if err != nil {
		m.log.Error("[SubmitMinedBlock] Error while getting parentBlock",
			"Error", err.Error())
		return false
	}
	parentMetadata, err := m.chain.GetBlockMetaData(parentBlock.HeaderHash())
	if err != nil {
		m.log.Error("[SubmitMinedBlock] Error while getting parentBlockMetadata",
			"Error", err.Error())
		return false
	}
	measurement, err := m.chain.GetMeasurement(uint32(m.miningBlock.Timestamp()), m.miningBlock.PrevHeaderHash(), parentMetadata)
	if err != nil {
		m.log.Error("[SubmitMinedBlock] Error while getting Measurement",
			"Error", err.Error())
		return false
	}
	if !m.miningBlock.ValidateMiningNonce(&blockHeader, parentBlock, parentMetadata, measurement, false) {
		m.log.Warn("[SubmitMinedBlock] Mining Nonce Validation failed",
			"blob", misc.Bin2HStr(blob))
		return false
	}

	m.miningBlock.SetNonces(blockHeader.MiningNonce(), blockHeader.ExtraNonce())
	// clonedBlock := *m.miningBlock
	// TODO: Call PreBlockLogic (clonedBlock)
	return true
}

func CreateMiner(chain *chain.Chain) *Miner {
	m := &Miner{
		chain: chain,
		ntp: ntp.GetNTP(),
		config: config.GetConfig(),
		log: log.GetLogger(),
		dt: &pow.DifficultyTracker{},
	}
	m.miningThreadCount = m.config.User.Miner.MiningThreadCount
	qryptoMiner := goqryptonight.NewDirectorQryptominer(m)
	m.Qryptominer = qryptoMiner
	runtime.SetFinalizer(m.Qryptominer,
		func(q goqryptonight.Qryptominer) {
			goqryptonight.DeleteDirectorQryptominer(q)
		})
	return m
}