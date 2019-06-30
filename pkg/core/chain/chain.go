package chain

import (
	"errors"
	"math/big"
	"reflect"
	"strconv"
	"sync"

	"github.com/syndtr/goleveldb/leveldb"

	c "github.com/theQRL/go-qrl/pkg/config"
	"github.com/theQRL/go-qrl/pkg/core/addressstate"
	"github.com/theQRL/go-qrl/pkg/core/block"
	"github.com/theQRL/go-qrl/pkg/core/metadata"
	"github.com/theQRL/go-qrl/pkg/core/pool"
	"github.com/theQRL/go-qrl/pkg/core/state"
	"github.com/theQRL/go-qrl/pkg/core/transactions"
	"github.com/theQRL/go-qrl/pkg/generated"
	"github.com/theQRL/go-qrl/pkg/log"
	"github.com/theQRL/go-qrl/pkg/misc"
	"github.com/theQRL/qryptonight/goqryptonight"

	"github.com/theQRL/go-qrl/pkg/pow"
)

type Chain struct {
	lock sync.Mutex

	log    log.LoggerInterface
	config *c.Config

	dt           pow.DifficultyTrackerInterface
	triggerMiner bool

	state *state.State

	txPool *pool.TransactionPool

	lastBlock         *block.Block
	currentDifficulty []byte

	newBlockNotificationChannel chan []byte
}

func CreateChain(s *state.State) *Chain {
	txPool := pool.CreateTransactionPool()

	chain := &Chain{
		log:    log.GetLogger(),
		config: c.GetConfig(),
		dt:     &pow.DifficultyTracker{},

		state:  s,
		txPool: txPool,
	}

	return chain
}

func (c *Chain) GetTxPool() *pool.TransactionPool {
	return c.txPool
}

func (c *Chain) SetDifficultyTracker(dt pow.DifficultyTrackerInterface) {
	// To be used by Unit tests only
	c.dt = dt
}

func (c *Chain) GetTransactionPool() *pool.TransactionPool {
	return c.txPool
}

func (c *Chain) SetNewBlockNotificationChannel(newBlockNotificationChannel chan []byte) {
	c.newBlockNotificationChannel = newBlockNotificationChannel
}

func (c *Chain) Height() uint64 {
	return c.lastBlock.BlockNumber()
}

func (c *Chain) GetLastBlock() *block.Block {
	return c.lastBlock
}

func (c *Chain) Load(genesisBlock *block.Block) error {
	// load() has the following tasks:
	// Write Genesis Block into State immediately
	// Register block_number <-> blockhash mapping
	// Calculate difficulty Metadata for Genesis Block
	// Generate AddressStates from Genesis Block balances
	// Apply Genesis Block's transactions to the state
	// Detect if we are forked from genesis block and if so initiate recovery.
	h, err := c.state.GetChainHeight()
	if err != nil {
		c.state.PutBlock(genesisBlock, nil)
		blockNumberMapping := &generated.BlockNumberMapping{Headerhash: genesisBlock.HeaderHash(),
			PrevHeaderhash: genesisBlock.PrevHeaderHash()}

		c.state.PutBlockNumberMapping(genesisBlock.BlockNumber(), blockNumberMapping, nil)

		parentDifficulty := misc.UCharVectorToBytes(goqryptonight.StringToUInt256(strconv.FormatInt(int64(c.config.Dev.Genesis.GenesisDifficulty), 10)))

		currentDifficulty, _ := c.dt.Get(uint64(c.config.Dev.MiningSetpointBlocktime),
			parentDifficulty)

		blockMetaData := metadata.CreateBlockMetadata(currentDifficulty, currentDifficulty, nil)

		c.state.PutBlockMetadata(genesisBlock.HeaderHash(), blockMetaData, nil)

		addressesState := make(map[string]*addressstate.AddressState)

		for _, genesisBalance := range genesisBlock.GenesisBalance() {
			addrState := addressstate.GetDefaultAddressState(genesisBalance.Address)
			addressesState[misc.Bin2Qaddress(addrState.Address())] = addrState
			addrState.SetBalance(genesisBalance.Balance)
		}

		txs := genesisBlock.Transactions()
		for i := 1; i < len(txs); i++ {
			for _, addr := range txs[i].GetTransfer().AddrsTo {
				addressesState[misc.Bin2Qaddress(addr)] = addressstate.GetDefaultAddressState(addr)
			}
		}

		coinBase := &transactions.CoinBase{}
		coinBase.SetPBData(txs[0])
		addressesState[misc.Bin2Qaddress(coinBase.AddrTo())] = addressstate.GetDefaultAddressState(coinBase.AddrTo())

		if !coinBase.ValidateExtendedCoinbase(genesisBlock.BlockNumber()) {
			return errors.New("coinbase validation failed")
		}

		coinBase.ApplyStateChanges(addressesState)

		for i := 1; i < len(txs); i++ {
			tx := transactions.TransferTransaction{}
			tx.SetPBData(txs[i])
			tx.ApplyStateChanges(addressesState)
		}

		err = c.state.PutAddressesState(addressesState, nil)
		if err != nil {
			c.log.Info("Failed PutAddressesState for Genesis Block")
			return err
		}

		c.state.UpdateTxMetadata(genesisBlock, nil)
		c.state.PutChainHeight(0, nil)
		c.lastBlock = genesisBlock
	} else {
		c.lastBlock, err = c.state.GetBlockByNumber(h)
		if err != nil {
			c.log.Info("Error while loading Last Block",
				"Block Number", h,
				"Error", err.Error())
		}
		var blockMetadata *metadata.BlockMetaData
		blockMetadata, err := c.state.GetBlockMetadata(c.lastBlock.HeaderHash())

		if err != nil {
			return err
		}

		c.currentDifficulty = blockMetadata.BlockDifficulty()
		forkState, err := c.state.GetForkState()
		if err == nil {
			b, err := c.state.GetBlock(forkState.InitiatorHeaderhash)
			if err != nil {
				return err
			}
			c.forkRecovery(b, forkState)
		}
	}
	return nil
}

func (c *Chain) addBlock(block *block.Block, batch *leveldb.Batch) (bool, bool) {
	blockSizeLimit, err := c.state.GetBlockSizeLimit(block)
	if err == nil && block.Size() > blockSizeLimit {
		c.log.Warn("Block Size greater than threshold limit %s > %s", block.Size(), blockSizeLimit)
		return false, false
	}
	if reflect.DeepEqual(c.lastBlock.HeaderHash(), block.PrevHeaderHash()) {
		if !c.applyBlock(block, batch) {
			return false, false
		}
	}
	err = c.state.PutBlock(block, batch)

	if err != nil {
		return false, false
	}
	lastBlockMetadata, err := c.state.GetBlockMetadata(c.lastBlock.HeaderHash())
	//newBlockMetadata, err := c.state.GetBlockMetadata(block.HeaderHash())
	newBlockMetadata := c.addBlockMetaData(block.HeaderHash(), block.Timestamp(), block.PrevHeaderHash(), batch)

	lastBlockDifficulty := big.NewInt(0)
	lastBlockDifficulty.SetString(goqryptonight.UInt256ToString(misc.BytesToUCharVector(lastBlockMetadata.TotalDifficulty())), 10)
	newBlockDifficulty := big.NewInt(0)
	newBlockDifficulty.SetString(goqryptonight.UInt256ToString(misc.BytesToUCharVector(newBlockMetadata.TotalDifficulty())), 10)

	if newBlockDifficulty.Cmp(lastBlockDifficulty) == 1 {
		if !reflect.DeepEqual(c.lastBlock.HeaderHash(), block.PrevHeaderHash()) {
			forkState := &generated.ForkState{InitiatorHeaderhash: block.HeaderHash()}
			err = c.state.PutForkState(forkState, batch)
			if err != nil {
				c.log.Info("PutForkState Error %s", err.Error())
				return false, true
			}
			c.state.WriteBatch(batch)
			return c.forkRecovery(block, forkState), true
		}
		if c.newBlockNotificationChannel != nil {
			c.newBlockNotificationChannel <- block.HeaderHash()
		}
		err := c.updateChainState(block, batch)
		if err != nil {
			return false, false
		}
		err = c.txPool.CheckStale(block.BlockNumber(), c.state)
		if err != nil {
			return false, false
		}
		c.triggerMiner = true
	}
	return true, false
}

func (c *Chain) addBlockMetaData(headerhash []byte, timestamp uint64, prevHeaderHash []byte, batch *leveldb.Batch) (*metadata.BlockMetaData) {
	parentMetaData, err := c.state.GetBlockMetadata(prevHeaderHash)
	measurement, err := c.state.GetMeasurement(uint32(timestamp), prevHeaderHash, parentMetaData)
	blockDifficulty, _ := c.dt.Get(measurement, parentMetaData.BlockDifficulty())

	parentBlockTotalDifficulty := big.NewInt(0)
	parentBlockTotalDifficulty.SetString(goqryptonight.UInt256ToString(misc.BytesToUCharVector(parentMetaData.TotalDifficulty())), 10)
	currentBlockTotalDifficulty := big.NewInt(0)
	currentBlockTotalDifficulty.SetString(goqryptonight.UInt256ToString(misc.BytesToUCharVector(blockDifficulty)), 10)
	currentBlockTotalDifficulty.Add(currentBlockTotalDifficulty, parentBlockTotalDifficulty)

	blockMetaData, err := c.state.GetBlockMetadata(headerhash)

	if err != nil {
		blockMetaData = metadata.CreateBlockMetadata(blockDifficulty, currentBlockTotalDifficulty.Bytes(), nil)
	}

	blockMetaData.UpdateLastHeaderHashes(parentMetaData.LastNHeaderHashes(), prevHeaderHash)

	c.state.PutBlockMetadata(prevHeaderHash, parentMetaData, batch)
	c.state.PutBlockMetadata(headerhash, blockMetaData, batch)
	return blockMetaData
}

func (c *Chain) AddBlock(block *block.Block) bool {
	c.lock.Lock()
	defer c.lock.Unlock()

	if c.Height() > c.config.Dev.ReorgLimit {
		if block.BlockNumber() < c.Height()-c.config.Dev.ReorgLimit {
			c.log.Debug("Skipping block, Reason: beyond re-org limit",
				"BlockNumber", block.BlockNumber(),
				"Re-org limit", c.Height()-c.config.Dev.ReorgLimit)
			return false
		}
	}
	_, err := c.state.GetBlock(block.HeaderHash())
	if err == nil {
		c.log.Debug("Skipping block, Reason: duplicate block",
			"BlockNumber", block.BlockNumber())
		return false
	}

	batch := c.state.GetBatch()
	blockFlag, forkFlag := c.addBlock(block, batch)
	if blockFlag {
		if !forkFlag {
			c.state.WriteBatch(batch)
		}
		c.log.Info("Added Block",
			"BlockNumber", block.BlockNumber(),
			"HeaderHash", misc.Bin2HStr(block.HeaderHash()))
		return true
	}
	return false
}

func (c *Chain) applyBlock(block *block.Block, batch *leveldb.Batch) bool {
	addressesState := block.PrepareAddressesList()
	err := c.state.GetAddressesState(addressesState)
	if err != nil {
		c.log.Warn("Failed to GetAddressesState in applyBlock", "error", err)
		return false
	}
	if !block.ApplyStateChanges(addressesState) {
		return false
	}
	err = c.state.PutAddressesState(addressesState, batch)
	if err != nil {
		c.log.Warn("Failed to apply Block", "err", err.Error())
		return false
	}
	return true
}

func (c *Chain) updateChainState(block *block.Block, batch *leveldb.Batch) error {
	c.lastBlock = block
	err := c.updateBlockNumberMapping(block, batch)
	if err != nil {
		c.log.Info("Error while updating blockNumber mapping", "error", err.Error())
		return err
	}
	c.txPool.RemoveTxInBlock(block)
	err = c.state.PutChainHeight(block.BlockNumber(), batch)
	if err != nil {
		c.log.Info("Error while updating chain height", "error", err.Error())
		return err
	}
	err = c.state.UpdateTxMetadata(block, batch)
	if err != nil {
		c.log.Info("Error while updating tx metadata", "error", err.Error())
		return err
	}
	return nil
}

func (c *Chain) updateBlockNumberMapping(block *block.Block, batch *leveldb.Batch) error {
	blockNumberMapping := &generated.BlockNumberMapping{Headerhash: block.HeaderHash(), PrevHeaderhash: block.PrevHeaderHash()}
	err := c.state.PutBlockNumberMapping(block.BlockNumber(), blockNumberMapping, batch)
	if err != nil {
		c.log.Info("Error while updating blockNumber mapping", "error", err.Error())
		return err
	}
	return nil
}

func (c *Chain) RemoveBlockFromMainchain(block *block.Block, blockNumber uint64, batch *leveldb.Batch) {
	addressesState := block.PrepareAddressesList()
	c.state.GetAddressesState(addressesState)
	for i := len(block.Transactions()) - 1; i >= 0; i-- {
		tx := transactions.ProtoToTransaction(block.Transactions()[i])
		tx.RevertStateChanges(addressesState)
		if i != 0 { // Skip OTS reset for Coinbase txn
			addrState, _ := addressesState[misc.PK2Qaddress(tx.PK())]
			c.state.UnsetOTSKey(addrState, uint64(tx.OtsKey()))
		}
	}

	c.txPool.AddTxFromBlock(block, blockNumber)
	c.state.PutChainHeight(block.BlockNumber()-1, batch)
	c.state.RollbackTxMetadata(block, batch)
	c.state.RemoveBlockNumberMapping(block.BlockNumber())
	c.state.PutAddressesState(addressesState, batch)
}

func (c *Chain) Rollback(forkedHeaderHash []byte, forkState *generated.ForkState) [][]byte {
	var hashPath [][]byte
	for !reflect.DeepEqual(c.lastBlock.HeaderHash(), forkedHeaderHash) {
		b, err := c.state.GetBlock(c.lastBlock.HeaderHash())

		if err != nil {
			c.log.Info("No block found by GetBlock")
		}

		mainchainBlock, err := c.state.GetBlockByNumber(b.BlockNumber())

		if err != nil {
			c.log.Info("No block found by GetBlockByNumber")
		}

		if !reflect.DeepEqual(b.HeaderHash(), mainchainBlock.HeaderHash()) {
			break
		}
		hashPath = append(hashPath, c.lastBlock.HeaderHash())

		batch := c.state.GetBatch()
		c.RemoveBlockFromMainchain(c.lastBlock, b.BlockNumber(), batch)

		if forkState != nil {
			forkState.OldMainchainHashPath = append(forkState.OldMainchainHashPath, c.lastBlock.HeaderHash())
			c.state.PutForkState(forkState, batch)
		}

		c.state.WriteBatch(batch)

		c.lastBlock, err = c.state.GetBlock(c.lastBlock.PrevHeaderHash())

		if err != nil {

		}
	}

	return hashPath
}

func (c *Chain) GetForkPoint(block *block.Block) ([]byte, [][]byte, error) {
	tmpBlock := block
	var err error
	var hashPath [][]byte
	for {
		if block == nil {
			return nil, nil, errors.New("No Block Found " + string(block.HeaderHash()) + ", Initiator " +
				string(tmpBlock.HeaderHash()))
		}
		mainchainBlock, err := c.state.GetBlockByNumber(block.BlockNumber())
		if err == nil && reflect.DeepEqual(mainchainBlock.HeaderHash(), block.HeaderHash()) {
			break
		}
		if block.BlockNumber() == 0 {
			return nil, nil, errors.New("Alternate chain genesis is different, Initiator " +
				string(tmpBlock.HeaderHash()))
		}
		hashPath = append(hashPath, block.HeaderHash())
		block, err = c.state.GetBlock(block.PrevHeaderHash())
		if err != nil {
			c.log.Error("[GetForkPoint] Error while getting block",
				"#", block.BlockNumber(),
				"HeaderHash", misc.Bin2HStr(block.HeaderHash()),
				"PrevHeaderHash", misc.Bin2HStr(block.PrevHeaderHash()))
			return nil, nil, err
		}
	}

	return block.HeaderHash(), hashPath, err
}

func (c *Chain) AddChain(hashPath [][]byte, forkState *generated.ForkState) bool {
	var start int

	for i := 0; i < len(hashPath); i++ {
		if reflect.DeepEqual(hashPath[i], c.lastBlock.HeaderHash()) {
			start = i + 1
			break
		}
	}

	for i := start; i < len(hashPath); i++ {
		headerHash := hashPath[i]
		b, err := c.state.GetBlock(headerHash)

		if err != nil {

		}

		batch := c.state.GetBatch()

		if !c.applyBlock(b, batch) {
			return false
		}

		err = c.updateChainState(b, batch)
		if err != nil {
			c.log.Info("Failed to UpdateChainState")
			return false
		}

		c.log.Debug("Apply block",
			"#", b.BlockNumber(),
			"batch", i, "hash", misc.Bin2HStr(hashPath[i]))
		c.state.WriteBatch(batch)
	}

	c.state.DeleteForkState()

	return true
}

func (c *Chain) forkRecovery(block *block.Block, forkState *generated.ForkState) bool {
	c.log.Info("Triggered Fork Recovery")

	var forkHeaderHash []byte
	var err error
	var hashPath, oldHashPath [][]byte

	if len(forkState.ForkPointHeaderhash) > 0 {
		c.log.Info("Recovering from last fork recovery interruption")
		forkHeaderHash = forkState.ForkPointHeaderhash
		hashPath = forkState.NewMainchainHashPath
	} else {
		forkHeaderHash, hashPath, err = c.GetForkPoint(block)
		if err != nil {
			c.log.Info("Failed At GetForkPoint")
		}
		forkState.ForkPointHeaderhash = forkHeaderHash
		forkState.NewMainchainHashPath = hashPath
		c.state.PutForkState(forkState, nil)
	}

	rollbackDone := false
	if len(forkState.OldMainchainHashPath) > 0 {
		b, err := c.state.GetBlock(forkState.OldMainchainHashPath[len(forkState.OldMainchainHashPath)-1])
		if err == nil && reflect.DeepEqual(b.PrevHeaderHash(), forkState.ForkPointHeaderhash) {
			rollbackDone = true
		}
	}

	if !rollbackDone {
		c.log.Info("Rolling back")
		oldHashPath = c.Rollback(forkHeaderHash, forkState)
	} else {
		oldHashPath = forkState.OldMainchainHashPath
	}

	if !c.AddChain(misc.Reverse(hashPath), forkState) {
		c.log.Warn("Fork Recovery Failed... Recovering back to old mainchain")
		// Above condition is true, when the node failed to add_chain
		// Thus old chain state, must be retrieved
		c.Rollback(forkHeaderHash, nil)
		c.AddChain(misc.Reverse(oldHashPath), forkState)
		return false
	}

	c.log.Info("Fork Recovery Finished")

	c.triggerMiner = true
	return true
}

func (c *Chain) GetTransactionByHash(txHash []byte) (*generated.Transaction, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	txMetaData, err := c.state.GetTxMetadata(txHash)
	if err != nil {
		return nil, err
	}
	return txMetaData.Transaction, err
}

func (c *Chain) GetTransactionMetaDataByHash(txHash []byte) (*generated.TransactionMetadata, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	txMetaData, err := c.state.GetTxMetadata(txHash)
	if err != nil {
		return nil, err
	}
	return txMetaData, err
}

func (c *Chain) GetBlock(headerhash []byte) (*block.Block, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	return c.state.GetBlock(headerhash)
}

func (c *Chain) GetBlockByNumber(blockNumber uint64) (*block.Block, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	return c.state.GetBlockByNumber(blockNumber)
}

func (c *Chain) GetBlockMetaData(headerhash []byte) (*metadata.BlockMetaData, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	return c.state.GetBlockMetadata(headerhash)
}

func (c *Chain) GetMeasurement(blockTimestamp uint32, parentHeaderHash []byte, parentMetaData *metadata.BlockMetaData) (uint64, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	return c.state.GetMeasurement(blockTimestamp, parentHeaderHash, parentMetaData)
}

func (c *Chain) GetAddressState(address []byte) (*addressstate.AddressState, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	return c.state.GetAddressState(address)
}

func (c *Chain) GetBlockSizeLimit(b *block.Block) (int, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	return c.state.GetBlockSizeLimit(b)
}

func (c *Chain) GetHeaderHashes(blockNumber uint64, count uint64) (*generated.NodeHeaderHash, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	startBlockNumber := uint64(0)
	if blockNumber > startBlockNumber {
		startBlockNumber = blockNumber
	}

	endBlockNumber := startBlockNumber + 2 * count
	if endBlockNumber > c.lastBlock.BlockNumber() {
		endBlockNumber = c.lastBlock.BlockNumber()
	}

	totalExpectedHeaderHash := int(endBlockNumber - startBlockNumber + 1)

	nodeHeaderHash := &generated.NodeHeaderHash{}
	nodeHeaderHash.BlockNumber = startBlockNumber

	b, err := c.state.GetBlockByNumber(endBlockNumber)
	if err != nil {
		c.log.Warn("GetNodeHeaderHash: Error in GetBlockByNumber",
			"Block Number", endBlockNumber)
		return nil, err
	}
	blockHeaderHash := b.HeaderHash()
	nodeHeaderHash.Headerhashes = append(nodeHeaderHash.Headerhashes, blockHeaderHash)

	for ; endBlockNumber >= startBlockNumber; {
		blockMetaData, err := c.state.GetBlockMetadata(blockHeaderHash)
		if err != nil {
			c.log.Warn("GetNodeHeaderHash: Error in GetBlockMetaData",
				"HeaderHash", misc.Bin2HStr(blockHeaderHash))
			return nil, err
		}
		headerHashes := blockMetaData.LastNHeaderHashes()
		nodeHeaderHash.Headerhashes = append(headerHashes, nodeHeaderHash.Headerhashes...)
		if uint64(len(headerHashes)) >= endBlockNumber {
			endBlockNumber = 0
		} else {
			endBlockNumber -= uint64(len(headerHashes))
		}
		if len(blockMetaData.LastNHeaderHashes()) == 0 {
			break
		}
		blockHeaderHash = headerHashes[0]
	}
	startFrom := 0
	if len(nodeHeaderHash.Headerhashes) > totalExpectedHeaderHash {
		startFrom = len(nodeHeaderHash.Headerhashes) - totalExpectedHeaderHash
	}
	nodeHeaderHash.Headerhashes = nodeHeaderHash.Headerhashes[startFrom:]
	return nodeHeaderHash, nil
}