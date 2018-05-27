package core

import (
	"github.com/cyyber/go-qrl/db"
	"github.com/cyyber/go-qrl/log"
	"sync"
	"github.com/cyyber/go-qrl/generated"
	"github.com/golang/protobuf/proto"
	"encoding/binary"
	"reflect"
	"github.com/cyyber/go-qrl/core/transactions"
	"github.com/cyyber/go-qrl/core/metadata"
)

type State struct {
	db	*db.LDB

	lock sync.Mutex
	log log.Logger
	config *Config
}

type RollbackStateInfo struct {
	addressesState     map[string]AddressState
	rollbackHeaderHash []byte
	hashPath           [][]byte
}

func CreateState(log *log.Logger) (*State, error) {
	newDB, err := db.NewDB("qrl", 16, 16, log)

	if err != nil {
		return nil, err
	}

	state := State {
		db: newDB,
		log: *log,
	}

	return &state, err
}

func (s *State) PutBlock(b *Block) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	value, err := b.Serialize()
	if err != nil {
		return err
	}

	if err := s.db.Put(b.HeaderHash(), value); err != nil {
		return err
	}
	return nil
}

func (s *State) GetBlock(headerHash []byte) (*Block, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	value, err := s.db.Get(headerHash)

	if err != nil {
		return nil, err
	}

	return DeSerializeBlock(value)
}

func (s *State) RemoveBlock(headerHash []byte) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	return s.db.Delete(headerHash)
}

func (s *State) PutBlockMetaData(headerHash []byte, b *metadata.BlockMetaData) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	value, err := b.Serialize()
	if err != nil {
		return err
	}

	if err := s.db.Put(append([]byte("metadata_"), headerHash...), value); err != nil {
		return err
	}

	return nil
}

func (s *State) GetBlockMetadata(headerHash []byte) (*metadata.BlockMetaData, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	value, err := s.db.Get(append([]byte("metadata_"), headerHash...))

	if err != nil {

	}

	return metadata.DeSerializeBlockMetaData(value)
}

func (s *State) RemoveBlockMetaData(headerHash []byte) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	return s.db.Delete(append([]byte("metadata_"), headerHash...))
}

func (s *State) PutBlockNumberMapping(blockNumber uint64, blockNumberMapping *generated.BlockNumberMapping) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	value, err := proto.Marshal(blockNumberMapping)
	if err != nil {

	}

	key := make([]byte, 8)
	binary.BigEndian.PutUint64(key[0:], blockNumber)
	if err := s.db.Put(key, value); err != nil {
		return err
	}

	return nil
}

func (s *State) GetBlockNumberMapping(blockNumber uint64) (*generated.BlockNumberMapping, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	key := make([]byte, 8)
	binary.BigEndian.PutUint64(key[0:], blockNumber)

	value, err := s.db.Get(key)

	if err != nil {

	}

	b := &generated.BlockNumberMapping{}
	err = proto.Unmarshal(value, b)

	return b, err
}

func (s *State) RemoveBlockNumberMapping(blockNumber uint64) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	key := make([]byte, 8)
	binary.BigEndian.PutUint64(key[0:], blockNumber)

	return s.db.Delete(key)
}

func (s *State) GetBlockByNumber(blockNumber uint64) (*Block, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	key := make([]byte, 8)
	binary.BigEndian.PutUint64(key[0:], blockNumber)

	value, err := s.db.Get(key)

	if err != nil {
		return nil, err
	}

	b := &generated.BlockNumberMapping{}
	err = proto.Unmarshal(value, b)

	if err != nil {
		return nil, err
	}

	block, err := s.GetBlock(b.Headerhash)

	if err != nil {
		return nil, err
	}

	return block, err
}

func (s *State) GetLastBlock() (*Block, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	blockNumber, err := s.GetChainHeight()

	if err != nil {
		return nil, err
	}

	block, err := s.GetBlockByNumber(blockNumber)

	if err != nil {
		return nil, err
	}

	return block, err
}

func (s *State) PutChainHeight(height uint64) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	key := make([]byte, 8)
	binary.BigEndian.PutUint64(key[0:], height)

	s.db.Put([]byte("blockheight"), key)

	return nil
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


func (s *State) UpdateLastTransactions(block *Block) error {
	// Skip if only coinbase transaction
	if len(block.Transactions()) == 1 {
		return nil
	}

	s.lock.Lock()
	defer s.lock.Unlock()

	lastTransactions, err := s.GetLastTransactions()

	if err != nil {
		return err
	}

	for _, protoTX := range block.Transactions()[1:] {
		txMetadata := &generated.TransactionMetadata{}
		txMetadata.BlockNumber = block.BlockNumber()
		txMetadata.Transaction = protoTX
		txMetadata.Timestamp = block.Timestamp()
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

	if err := s.db.Put([]byte("LastTransactions"), value); err != nil {
		return err
	}

	return nil
}

func (s *State) GetLastTransactions() (*generated.LastTransactions, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	lastTransactions := &generated.LastTransactions{}

	value, err := s.db.Get([]byte("LastTransactions"))

	err = proto.Unmarshal(value, lastTransactions)
	if err != nil {
		return nil, err
	}

	return lastTransactions, err
}

func (s *State) RemoveLastTransactions(block *Block) error {
	// Skip if only coinbase transaction
	if len(block.Transactions()) == 1 {
		return nil
	}

	s.lock.Lock()
	defer s.lock.Unlock()

	lastTransactions, err := s.GetLastTransactions()

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

	if err := s.db.Put([]byte("LastTransactions"), value); err != nil {
		return err
	}

	return nil
}

func (s *State) AddTokenMetadata(token *transactions.TokenTransaction) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	tokenMetadata := metadata.CreateTokenMetadata(token.Txhash(), token.Txhash())

	value, err := tokenMetadata.Serialize()

	if err != nil {
		return err
	}

	key := []byte("token_")
	key = append(key[:], token.Txhash()[:]...)

	err = s.db.Put(key, value)

	if err != nil {
		return err
	}

	return nil
}

func (s *State) GetTokenMetadata(tokenTxHash []byte) (*metadata.TokenMetadata, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

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

func (s *State) UpdateTokenMetadata(transferToken *transactions.TransferTokenTransaction) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	tokenMetadata, err := s.GetTokenMetadata(transferToken.TokenTxhash())

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

	err = s.db.Put(key, value)

	if err != nil {
		return err
	}

	return nil
}

func (s *State) RemoveTransferTokenMetadata(transferToken *transactions.TransferTokenTransaction) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	tokenMetadata, err := s.GetTokenMetadata(transferToken.TokenTxhash())

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

	err = s.db.Put(key, value)

	if err != nil {
		return err
	}

	return nil
}

func (s *State) RemoveTokenMetadata(token *transactions.TokenTransaction) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	key := []byte("token_")
	key = append(key[:], token.Txhash()[:]...)

	err := s.db.Delete(key)

	if err != nil {
		return err
	}

	return nil
}

func (s *State) PutTxMetadata(tx transactions.TransactionInterface, blockNumber uint64, timestamp uint64) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	m := &generated.TransactionMetadata{}
	m.Transaction = tx.PBData()
	m.BlockNumber = blockNumber
	m.Timestamp = timestamp

	value, err := proto.Marshal(m)

	if err != nil {
		return err
	}

	err = s.db.Put(tx.Txhash(), value)

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

func (s *State) UpdateTxMetadata(block *Block) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	var feeReward uint64
	var err error

	for _, protoTX := range block.Transactions() {
		tx := transactions.ProtoToTransaction(protoTX)
		feeReward += tx.Fee()

		s.PutTxMetadata(tx, block.BlockNumber(), block.Timestamp())

		switch protoTX.TransactionType.(type) {
		case *generated.Transaction_Token_:
			t := tx.(*transactions.TokenTransaction)
			err = s.AddTokenMetadata(t)
		case *generated.Transaction_TransferToken_:
			t := tx.(*transactions.TransferTokenTransaction)
			err = s.UpdateTokenMetadata(t)
		}

		if err != nil {
			return err
		}
	}

	tx := block.Transactions()[0]
	err = s.AddTotalCoinSupply(tx.GetCoinbase().Amount - feeReward)
	if err != nil {
		return err
	}

	err = s.UpdateLastTransactions(block)
	if err != nil {
		return err
	}

	return nil
}

func (s *State) RollbackTxMetadata(block *Block) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	var feeReward uint64
	var err error

	for _, protoTX := range block.Transactions() {
		tx := transactions.ProtoToTransaction(protoTX)
		feeReward += tx.Fee()

		s.PutTxMetadata(tx, block.BlockNumber(), block.Timestamp())

		switch protoTX.TransactionType.(type) {
		case *generated.Transaction_Token_:
			t := tx.(*transactions.TokenTransaction)
			err = s.RemoveTokenMetadata(t)
		case *generated.Transaction_TransferToken_:
			t := tx.(*transactions.TransferTokenTransaction)
			err = s.RemoveTransferTokenMetadata(t)
		}

		if err != nil {
			return err
		}
	}

	tx := block.Transactions()[0]
	err = s.ReduceTotalCoinSupply(tx.GetCoinbase().Amount - feeReward)
	if err != nil {
		return err
	}

	err = s.RemoveLastTransactions(block)
	if err != nil {
		return err
	}

	return nil
}

func (s *State) PutAddressesState(addressesState map[string]AddressState) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	for _, addrState := range addressesState {
		value, err := addrState.Serialize()
		if err != nil {
			return err
		}
		s.db.Put(addrState.Address(), value)
	}

	return nil
}

func (s *State) GetAddressState(address []byte) (*AddressState, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	value, err := s.db.Get(address)

	if err != nil {
		return nil, err
	}

	return DeSerializeAddressState(value)
}

func (s *State) GetAddressesState(addressesState map[string]AddressState) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	for address := range addressesState {
		addrState, err := s.GetAddressState([]byte(address))

		if err != nil {
			return err
		}

		addressesState[address] = *addrState
	}

	return nil
}

func (s *State) GetState(headerHash []byte, addressesState map[string]AddressState) (*RollbackStateInfo, error) {
	tmpHeaderHash := headerHash

	var hashPath [][]byte

	for {
		block, err := s.GetBlock(headerHash)
		if err != nil {
			return nil, err
		}

		mainchainBlock, err := s.GetBlockByNumber(block.BlockNumber())

		if err == nil {
			if reflect.DeepEqual(mainchainBlock.HeaderHash(), block.HeaderHash()) {
				break
			}
		}
		if block.BlockNumber() == 0 {
			panic("[GetState] Alternate chain genesis is different, Initiator" + string(tmpHeaderHash))
		}
		hashPath = append(hashPath, headerHash)
		headerHash = block.PrevHeaderHash()
	}

	rollbackHeaderHash := headerHash

	for address := range addressesState {
		addrState, err := s.GetAddressState([]byte(address))
		if err != nil {
			return nil, err
		}
		addressesState[address] = *addrState
	}
	block, err := s.GetLastBlock()

	if err != nil {
		return nil, err
	}

	for reflect.DeepEqual(block.HeaderHash(), rollbackHeaderHash) {
		txs := block.Transactions()
		for i := len(txs); i >= 0; i-- {
			tx := transactions.ProtoToTransaction(txs[i])
			tx.RevertStateChanges(addressesState, s)
		}

		newBlock, err := s.GetBlock(block.PrevHeaderHash())
		if err != nil {
			return nil, err
		}

		block = newBlock
	}

	for i := len(hashPath); i >= 0; i-- {
		block, err := s.GetBlock(hashPath[i])

		if err != nil {
			return nil, err
		}

		for _, protoTX := range block.Transactions() {
			tx := transactions.ProtoToTransaction(protoTX)
			tx.ApplyStateChanges(addressesState)
		}
	}

	rollbackStateInfo := &RollbackStateInfo{}
	rollbackStateInfo.addressesState = addressesState
	rollbackStateInfo.rollbackHeaderHash = rollbackHeaderHash
	rollbackStateInfo.hashPath = hashPath

	return rollbackStateInfo, nil
}

func (s *State) GetTotalCoinSupply() (uint64, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	value, err := s.db.Get([]byte("TotalCoinSupply"))

	if err != nil {
		return 0, err
	}

	return binary.BigEndian.Uint64(value), nil
}

func (s *State) AddTotalCoinSupply(value uint64) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	oldValue, err := s.GetTotalCoinSupply()
	if err != nil {
		return err
	}

	newValue := oldValue + value

	byteValue := make([]byte, 8)
	binary.BigEndian.PutUint64(byteValue, newValue)
	err = s.db.Put([]byte("TotalCoinSupply"), byteValue)

	return err
}

func (s *State) ReduceTotalCoinSupply(value uint64) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	oldValue, err := s.GetTotalCoinSupply()
	if err != nil {
		return err
	}

	newValue := oldValue - value

	byteValue := make([]byte, 8)
	binary.BigEndian.PutUint64(byteValue, newValue)
	err = s.db.Put([]byte("TotalCoinSupply"), byteValue)

	return err
}

func (s *State) GetMeasurement(blockTimestamp uint64, parentHeaderHash []byte, parentMetaData *metadata.BlockMetaData) (uint64, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	var nthBlock *Block
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

	return (blockTimestamp - nthBlockTimestamp) / countHeaderHashes, nil
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