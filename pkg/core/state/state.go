package state

import (
	"encoding/binary"
	"encoding/hex"
	"github.com/theQRL/go-qrl/pkg/misc"
	"math"
	"reflect"
	"sync"

	"github.com/golang/protobuf/proto"
	"github.com/syndtr/goleveldb/leveldb"

	c "github.com/theQRL/go-qrl/pkg/config"
	"github.com/theQRL/go-qrl/pkg/core/addressstate"
	"github.com/theQRL/go-qrl/pkg/core/block"
	"github.com/theQRL/go-qrl/pkg/core/formulas"
	"github.com/theQRL/go-qrl/pkg/core/metadata"
	"github.com/theQRL/go-qrl/pkg/core/transactions"
	"github.com/theQRL/go-qrl/pkg/db"
	"github.com/theQRL/go-qrl/pkg/generated"
	"github.com/theQRL/go-qrl/pkg/log"
)

type State struct {
	db *db.LDB

	lock   sync.Mutex
	log    *log.Logger
	config *c.Config
}

type RollbackStateInfo struct {
	addressesState     map[string]*addressstate.AddressState
	rollbackHeaderHash []byte
	hashPath           [][]byte
}

func CreateState() (*State, error) {
	c := c.GetConfig()
	newDB, err := db.NewDB(c.User.DataDir(), c.Dev.DBName, 51, 500)

	if err != nil {
		return nil, err
	}

	state := State{
		db:     newDB,
		log:    log.GetLogger(),
		config: c,
	}

	return &state, err
}

func (s *State) GetBatch() *leveldb.Batch {
	return s.db.GetBatch()
}

func (s *State) WriteBatch(batch *leveldb.Batch) {
	s.db.WriteBatch(batch, false)
}

func (s *State) GetBlockSizeLimit(b *block.Block) (int, error) {
	blockSizeList := make([]int, 10)
	for i := 0; i < 10; i++ {
		b, err := s.GetBlock(b.HeaderHash())
		if err != nil {
			return 0, err
		}
		blockSizeList[i] = b.Size()
		if b.BlockNumber() == 0 {
			break
		}
	}

	return int(math.Max(float64(s.config.Dev.BlockMinSizeLimit), float64(s.config.Dev.SizeMultiplier*formulas.Median(blockSizeList)))), nil
}

func (s *State) PutBlock(b *block.Block, batch *leveldb.Batch) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	value, err := b.Serialize()
	if err != nil {
		return err
	}

	if err := s.db.Put(b.HeaderHash(), value, batch); err != nil {
		return err
	}

	return nil
}

func (s *State) GetBlock(headerHash []byte) (*block.Block, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	value, err := s.db.Get(headerHash)

	if err != nil {
		return nil, err
	}

	return block.DeSerializeBlock(value)
}

func (s *State) RemoveBlock(headerHash []byte) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	return s.db.Delete(headerHash)
}

func (s *State) PutBlockMetadata(headerHash []byte, b *metadata.BlockMetaData, batch *leveldb.Batch) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	value, err := b.Serialize()
	if err != nil {
		return err
	}

	if err := s.db.Put(append([]byte("metadata_"), headerHash...), value, batch); err != nil {
		return err
	}

	return nil
}

func (s *State) GetBlockMetadata(headerHash []byte) (*metadata.BlockMetaData, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	value, err := s.db.Get(append([]byte("metadata_"), headerHash...))

	if err != nil {
		return nil, err
	}

	return metadata.DeSerializeBlockMetaData(value)
}

func (s *State) RemoveBlockMetadata(headerHash []byte) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	return s.db.Delete(append([]byte("metadata_"), headerHash...))
}

func (s *State) PutBlockNumberMapping(blockNumber uint64, blockNumberMapping *generated.BlockNumberMapping, batch *leveldb.Batch) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	value, err := proto.Marshal(blockNumberMapping)
	if err != nil {
		s.log.Error("Error while Encoding BlockNumberMapping",
			"#", blockNumber,
			"Error", err.Error())
		return err
	}

	key := make([]byte, 8)
	binary.BigEndian.PutUint64(key[0:], blockNumber)
	return s.db.Put(key, value, batch)
}

func (s *State) GetBlockNumberMapping(blockNumber uint64) (*generated.BlockNumberMapping, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	key := make([]byte, 8)
	binary.BigEndian.PutUint64(key, blockNumber)

	value, err := s.db.Get(key)

	if err != nil {
		if err != leveldb.ErrNotFound {
			s.log.Error("[GetBlockNumberMapping] Error while loading key",
				"key", key,
				"blockNumber", blockNumber)
		}
		return nil, err
	}

	b := &generated.BlockNumberMapping{}
	err = proto.Unmarshal(value, b)
	if err != nil {
		s.log.Error("[GetBlockNumberMapping] Error Decoding")
	}
	return b, err
}

func (s *State) RemoveBlockNumberMapping(blockNumber uint64) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	key := make([]byte, 8)
	binary.BigEndian.PutUint64(key[0:], blockNumber)

	return s.db.Delete(key)
}

func (s *State) GetBlockByNumber(blockNumber uint64) (*block.Block, error) {

	bm, err := s.GetBlockNumberMapping(blockNumber)

	if err != nil {
		if err != leveldb.ErrNotFound {
			s.log.Info("Failed to Unmarshal")
		}
		return nil, err
	}

	b, err := s.GetBlock(bm.Headerhash)

	if err != nil {
		s.log.Info("Failed to get for Headerhash",
			"header hash", hex.EncodeToString(bm.Headerhash))
		return nil, err
	}

	return b, err
}

func (s *State) GetLastBlock() (*block.Block, error) {
	blockNumber, err := s.GetChainHeight()

	if err != nil {
		return nil, err
	}

	b, err := s.GetBlockByNumber(blockNumber)

	if err != nil {
		return nil, err
	}

	return b, err
}

func (s *State) PutChainHeight(height uint64, batch *leveldb.Batch) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	key := make([]byte, 8)
	binary.BigEndian.PutUint64(key[0:], height)

	return s.db.Put([]byte("blockheight"), key, batch)
}

func (s *State) GetChainHeight() (uint64, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	value, err := s.db.Get([]byte("blockheight"))

	if err != nil {
		return 0, err
	}

	return binary.BigEndian.Uint64(value), nil
}

func (s *State) updateLastTransactions(block *block.Block, batch *leveldb.Batch) error {
	// Skip if only coinbase transaction
	if len(block.Transactions()) == 1 {
		return nil
	}

	lastTransactions, err := s.getLastTransactions()

	if err != nil {
		return err
	}

	for _, protoTX := range block.Transactions()[1:] {
		txMetadata := &generated.TransactionMetadata{}
		txMetadata.BlockNumber = block.BlockNumber()
		txMetadata.Transaction = protoTX
		txMetadata.Timestamp = uint64(block.Timestamp())
		start := 1
		if len(lastTransactions.TxMetadata) < 20 {
			start = 0
		}
		lastTransactions.TxMetadata = append(lastTransactions.TxMetadata[start:], txMetadata)
	}

	value, err := proto.Marshal(lastTransactions)

	if err != nil {
		return err
	}

	if err := s.db.Put([]byte("LastTransactions"), value, batch); err != nil {
		return err
	}

	return nil
}

func (s *State) getLastTransactions() (*generated.LastTransactions, error) {
	lastTransactions := &generated.LastTransactions{}

	value, err := s.db.Get([]byte("LastTransactions"))

	err = proto.Unmarshal(value, lastTransactions)
	if err != nil {
		return nil, err
	}

	return lastTransactions, err
}

func (s *State) removeLastTransactions(block *block.Block, batch *leveldb.Batch) error {
	// Skip if only coinbase transaction
	if len(block.Transactions()) == 1 {
		return nil
	}

	lastTransactions, err := s.getLastTransactions()

	if err != nil {
		return err
	}

	for _, protoTX := range block.Transactions()[1:] {
		for index, txMetadata := range lastTransactions.TxMetadata {
			if reflect.DeepEqual(txMetadata.Transaction.TransactionHash, protoTX.TransactionHash) {
				lastTransactions.TxMetadata = append(lastTransactions.TxMetadata[:index], lastTransactions.TxMetadata[index+1:]...)
				break
			}
		}
	}

	value, err := proto.Marshal(lastTransactions)

	if err != nil {
		return err
	}

	if err := s.db.Put([]byte("LastTransactions"), value, batch); err != nil {
		return err
	}

	return nil
}

func (s *State) addTokenMetadata(token *transactions.TokenTransaction, batch *leveldb.Batch) error {
	tokenMetadata := metadata.CreateTokenMetadata(token.Txhash(), token.Txhash())

	value, err := tokenMetadata.Serialize()

	if err != nil {
		return err
	}

	key := []byte("token_")
	key = append(key[:], token.Txhash()[:]...)

	err = s.db.Put(key, value, batch)

	if err != nil {
		return err
	}

	return nil
}

func (s *State) getTokenMetadata(tokenTxHash []byte) (*metadata.TokenMetadata, error) {
	key := []byte("token_")
	key = append(key[:], tokenTxHash[:]...)
	value, err := s.db.Get(key)

	if err != nil {
		return nil, err
	}

	m, err := metadata.DeSerializeTokenMetadata(value)

	if err != nil {
		return nil, err
	}

	return m, nil
}

func (s *State) updateTokenMetadata(transferToken *transactions.TransferTokenTransaction, batch *leveldb.Batch) error {
	tokenMetadata, err := s.getTokenMetadata(transferToken.TokenTxhash())

	if err != nil {
		return err
	}

	tokenMetadata.Append(transferToken.Txhash())

	value, err := tokenMetadata.Serialize()

	if err != nil {
		return err
	}

	key := []byte("token_")
	key = append(key[:], transferToken.Txhash()[:]...)

	err = s.db.Put(key, value, batch)

	if err != nil {
		return err
	}

	return nil
}

func (s *State) removeTransferTokenMetadata(transferToken *transactions.TransferTokenTransaction, batch *leveldb.Batch) error {
	tokenMetadata, err := s.getTokenMetadata(transferToken.TokenTxhash())

	if err != nil {
		return err
	}

	tokenMetadata.Remove(transferToken.Txhash())

	value, err := tokenMetadata.Serialize()

	if err != nil {
		return err
	}

	key := []byte("token_")
	key = append(key[:], transferToken.Txhash()[:]...)

	err = s.db.Put(key, value, batch)

	if err != nil {
		return err
	}

	return nil
}

func (s *State) removeTokenMetadata(token *transactions.TokenTransaction) error {
	key := []byte("token_")
	key = append(key[:], token.Txhash()[:]...)

	err := s.db.Delete(key)

	if err != nil {
		return err
	}

	return nil
}

func (s *State) putTxMetadata(tx transactions.TransactionInterface, blockNumber uint64, timestamp uint64, batch *leveldb.Batch) error {
	m := &generated.TransactionMetadata{}
	m.Transaction = tx.PBData()
	m.BlockNumber = blockNumber
	m.Timestamp = timestamp

	value, err := proto.Marshal(m)

	if err != nil {
		return err
	}

	err = s.db.Put(tx.Txhash(), value, batch)

	if err != nil {
		return err
	}

	return nil
}

func (s *State) GetTxMetadata(txHash []byte) (*generated.TransactionMetadata, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	value, err := s.db.Get(txHash)

	if err != nil {
		return nil, err
	}

	m := &generated.TransactionMetadata{}
	err = proto.Unmarshal(value, m)

	if err != nil {
		return nil, err
	}

	return m, nil

}

func (s *State) RemoveTxMetadata(tx *transactions.Transaction) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	err := s.db.Delete(tx.Txhash())

	if err != nil {
		return err
	}

	return nil
}

func (s *State) UpdateTxMetadata(block *block.Block, batch *leveldb.Batch) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	var feeReward uint64
	var err error

	for _, protoTX := range block.Transactions() {
		tx := transactions.ProtoToTransaction(protoTX)
		feeReward += tx.Fee()

		s.putTxMetadata(tx, block.BlockNumber(), uint64(block.Timestamp()), batch)

		switch protoTX.TransactionType.(type) {
		case *generated.Transaction_Token_:
			t := tx.(*transactions.TokenTransaction)
			err = s.addTokenMetadata(t, batch)
		case *generated.Transaction_TransferToken_:
			t := tx.(*transactions.TransferTokenTransaction)
			err = s.updateTokenMetadata(t, batch)
		}

		if err != nil {
			return err
		}
	}

	tx := block.Transactions()[0]
	err = s.addTotalCoinSupply(tx.GetCoinbase().Amount-feeReward, batch)
	if err != nil {
		s.log.Warn("Error while adding total coin supply")
		s.log.Info(err.Error())
		return err
	}

	err = s.updateLastTransactions(block, batch)
	if err != nil {
		return err
	}

	return nil
}

func (s *State) RollbackTxMetadata(block *block.Block, batch *leveldb.Batch) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	var feeReward uint64
	var err error

	for _, protoTX := range block.Transactions() {
		tx := transactions.ProtoToTransaction(protoTX)
		feeReward += tx.Fee()

		s.putTxMetadata(tx, block.BlockNumber(), uint64(block.Timestamp()), batch)

		switch protoTX.TransactionType.(type) {
		case *generated.Transaction_Token_:
			t := tx.(*transactions.TokenTransaction)
			err = s.removeTokenMetadata(t)
		case *generated.Transaction_TransferToken_:
			t := tx.(*transactions.TransferTokenTransaction)
			err = s.removeTransferTokenMetadata(t, batch)
		}

		if err != nil {
			return err
		}
	}

	tx := block.Transactions()[0]
	err = s.ReduceTotalCoinSupply(tx.GetCoinbase().Amount-feeReward, batch)
	if err != nil {
		return err
	}

	err = s.removeLastTransactions(block, batch)
	if err != nil {
		return err
	}

	return nil
}

func (s *State) PutForkState(forkState *generated.ForkState, batch *leveldb.Batch) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	value, err := proto.Marshal(forkState)

	if err != nil {
		return err
	}

	key := []byte("fork_state")
	err = s.db.Put(key, value, batch)

	if err != nil {
		return err
	}

	return nil
}

func (s *State) GetForkState() (*generated.ForkState, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	value, err := s.db.Get([]byte("fork_state"))

	if err != nil {
		return nil, err
	}

	m := &generated.ForkState{}
	err = proto.Unmarshal(value, m)

	if err != nil {
		return nil, err
	}

	return m, nil

}

func (s *State) DeleteForkState() error {
	s.lock.Lock()
	defer s.lock.Unlock()

	err := s.db.Delete([]byte("fork_state"))

	if err != nil {
		return err
	}

	return nil
}

func (s *State) PutAddressesState(addressesState map[string]*addressstate.AddressState, batch *leveldb.Batch) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	for _, addrState := range addressesState {
		value, err := addrState.Serialize()
		if err != nil {
			return err
		}
		s.db.Put(addrState.Address(), value, batch)
	}

	return nil
}

func (s *State) GetAddressState(address []byte) (*addressstate.AddressState, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	return s.getAddressState(address)
}

func (s *State) getAddressState(address []byte) (*addressstate.AddressState, error) {
	value, err := s.db.Get(address)

	if err != nil {
		if err == leveldb.ErrNotFound {
			return addressstate.GetDefaultAddressState(address), nil
		} else {
			s.log.Info("Error in getAddressState",
				"address", address,
				"error", err.Error())
			return nil, err
		}
	}

	return addressstate.DeSerializeAddressState(value)
}

func (s *State) GetAddressesState(addressesState map[string]*addressstate.AddressState) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	for address := range addressesState {
		addrState, err := s.getAddressState(misc.Qaddress2Bin(address))
		if err != nil {
			s.log.Warn("Error in GetAddressesState",
				"error", err.Error())
			return err
		}
		addressesState[address] = addrState
	}

	return nil
}

func (s *State) GetTotalCoinSupply() (uint64, error) {
	value, err := s.db.Get([]byte("TotalCoinSupply"))

	if err != nil {
		if err == leveldb.ErrNotFound {
			return 0, nil
		} else {
			return 0, err
		}
	}

	return binary.BigEndian.Uint64(value), nil
}

func (s *State) addTotalCoinSupply(value uint64, batch *leveldb.Batch) error {
	oldValue, err := s.GetTotalCoinSupply()
	if err != nil {
		return err
	}

	newValue := oldValue + value

	byteValue := make([]byte, 8)
	binary.BigEndian.PutUint64(byteValue, newValue)
	err = s.db.Put([]byte("TotalCoinSupply"), byteValue, batch)

	return err
}

func (s *State) ReduceTotalCoinSupply(value uint64, batch *leveldb.Batch) error {
	oldValue, err := s.GetTotalCoinSupply()
	if err != nil {
		return err
	}

	newValue := oldValue - value

	byteValue := make([]byte, 8)
	binary.BigEndian.PutUint64(byteValue, newValue)
	err = s.db.Put([]byte("TotalCoinSupply"), byteValue, batch)

	return err
}

func (s *State) GetMeasurement(blockTimestamp uint32, parentHeaderHash []byte, parentMetaData *metadata.BlockMetaData) (uint64, error) {
	//return 1000000000, nil
	var nthBlock *block.Block
	var err error

	countHeaderHashes := uint64(len(parentMetaData.LastNHeaderHashes()))

	if countHeaderHashes == 0 {
		return uint64(s.config.Dev.MiningSetpointBlocktime), nil
	} else if countHeaderHashes == 1 {
		nthBlock, err = s.GetBlock(parentHeaderHash)

		if err != nil {
			return 0, err
		}
		countHeaderHashes += 1
	} else {
		nthBlock, err = s.GetBlock(parentMetaData.LastNHeaderHashes()[1])

		if err != nil {
			return 0, err
		}
	}

	nthBlockTimestamp := nthBlock.Timestamp()
	if countHeaderHashes < uint64(s.config.Dev.NMeasurement) {
		nthBlockTimestamp -= uint64(s.config.Dev.MiningSetpointBlocktime)
	}

	return (uint64(blockTimestamp) - nthBlockTimestamp)/countHeaderHashes, nil
}

func (s *State) UnsetOTSKey(a *addressstate.AddressState, otsKeyIndex uint64) error {
	if otsKeyIndex < uint64(s.config.Dev.MaxOTSTracking) {
		offset := otsKeyIndex >> 3
		relative := otsKeyIndex % 8
		bitfield := a.PBData().OtsBitfield[offset]
		a.PBData().OtsBitfield[offset][0] = bitfield[0] & ^(1 << relative)
		return nil
	}

	a.PBData().OtsCounter = 0
	hashes := a.TransactionHashes()
	for i := len(hashes) - 1; i >= 0; i-- {
		tm, err := s.GetTxMetadata(hashes[i])
		if err != nil {
			return err
		}
		tx := transactions.ProtoToTransaction(tm.Transaction)
		if tx.OtsKey() >= s.config.Dev.MaxOTSTracking {
			a.PBData().OtsCounter = uint64(tx.OtsKey())
			return nil
		}
	}
	return nil
}

// TODO: Needed for API
//func (s *State) GetBlockDataPoint(headerHash []byte) (*generated.BlockDataPoint, error) {
//	s.lock.Lock()
//	defer s.lock.Unlock()
//
//	block, err := s.GetBlock(headerHash)
//	if err != nil {
//		return nil, err
//	}
//
//	blockMetaData, err := s.GetBlockMetadata(headerHash)
//	if err != nil {
//		return nil, err
//	}
//
//	prevBlockMetaData, err := s.GetBlockMetadata(block.PrevHeaderHash())
//	if err != nil {
//		return nil, err
//	}
//
//	dataPoint := &generated.BlockDataPoint{}
//	dataPoint.Number = block.BlockNumber()
//	dataPoint.HeaderHash = block.HeaderHash()
//	dataPoint.Timestamp = block.Timestamp()
//	dataPoint.TimeLast = 0
//	dataPoint.TimeMovavg = 0
//	blockDifficulty := big.NewInt(0)
//	blockDifficulty.SetBytes(blockMetaData.BlockDifficulty())
//	dataPoint.Difficulty = blockDifficulty.String()
//
//	prevBlock, err := s.GetBlock(block.PrevHeaderHash())
//	if err == nil {
//		dataPoint.HeaderHashPrev = prevBlock.HeaderHash()
//		dataPoint.TimeLast = block.Timestamp() - prevBlock.Timestamp()
//		if prevBlock.BlockNumber() == 0 {
//			dataPoint.TimeLast = uint64(s.config.Dev.MiningSetpointBlocktime)
//		}
//
//		movAvg, err := s.GetMeasurement(block.Timestamp(), block.PrevHeaderHash(), prevBlockMetaData)
//		if err != nil {
//			return nil, err
//		}
//
//		dataPoint.TimeMovavg = movAvg
//
//
//		//dataPoint.HashPower = number.Uint256(dataPoint.Difficulty).Mul(s.config.Dev.MiningSetpointBlocktime).Div(movAvg)
//		bigNum := blockDifficulty.Mul(blockDifficulty, big.NewInt(int64(s.config.Dev.MiningSetpointBlocktime)))
//		bigFloatNum := big.NewFloat(0).SetInt(bigNum)
//		bigFloatNum.
//		dataPoint.HashPower = bigNum.String()
//
//		}
//	}
//}

func (s *State) MockAddressState(address []byte, nonce uint64, balance uint64) error {
	otsBitfield := make([][]byte, s.config.Dev.OtsBitFieldSize)
	var tokens map[string]uint64
	var slavePksAccessType map[string]uint32
	addressState := addressstate.CreateAddressState(
		address,
		nonce,
		balance,
		otsBitfield,
		tokens,
		slavePksAccessType,
		0,
		)
	addressesState := make(map[string]*addressstate.AddressState)
	addressesState[misc.Bin2Qaddress(address)] = addressState
	return s.PutAddressesState(addressesState, nil)
}