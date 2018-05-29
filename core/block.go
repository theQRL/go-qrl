package core

import (
	"github.com/cyyber/go-qrl/generated"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/jsonpb"
	"github.com/cyyber/go-qrl/core/transactions"
	"container/list"
	"github.com/cyyber/go-qrl/misc"
)

type BlockInterface interface {

	PBData() *generated.Block

	Size() int

	BlockNumber() uint64

	Epoch() uint64

	HeaderHash() []byte

	PrevHeaderHash() []byte

	Transactions() []*generated.Transaction

	MiningNonce() uint32

	BlockReward() []uint64

	FeeReward() []uint64

	Timestamp() []uint64

	MiningBlob() []byte

	MiningNonceOffset() []byte

	VerifyBlob([]byte) bool

	SetNonces(uint32, uint64)

	FromJSON() Block

	Json() string

	Serialize() string

	Create(blockNumber uint64,
		prevHeaderHash []byte,
		prevBlockTimestamp uint64,
		transactions generated.Transaction,
		miner_address []byte)

	UpdateMiningAddress(miningAddress []byte)

	Validate(futureBlocks map[string]generated.Block)

	ApplyStateChanges(addressTxn map[string]generated.Transaction) bool

	IsDuplicate() bool

	IsFutureBlock() bool

	ValidateParentChildRelation(block generated.Block) bool
}

type Block struct {
	block *generated.Block
	blockheader *BlockHeader

	config *Config
}

func (b *Block) PBData() *generated.Block {
	return b.block
}

func (b *Block) Size() int {
	return proto.Size(b.block)
}

func (b *Block) BlockNumber() uint64 {
	return b.blockheader.BlockNumber()
}

func (b *Block) Epoch() uint64 {
	return b.blockheader.BlockNumber() / b.config.Dev.BlocksPerEpoch
}

func (b *Block) HeaderHash() []byte {
	return b.blockheader.HeaderHash()
}

func (b *Block) PrevHeaderHash() []byte {
	return b.blockheader.PrevHeaderHash()
}

func (b *Block) Transactions() []*generated.Transaction {
	return b.block.GetTransactions()
}

func (b *Block) MiningNonce() uint32 {
	return b.blockheader.MiningNonce()
}

func (b *Block) BlockReward() uint64 {
	return b.blockheader.BlockReward()
}

func (b *Block) Timestamp() uint64 {
	return b.blockheader.Timestamp()
}

func (b *Block) MiningBlob() []byte {
	return b.blockheader.MiningBlob()
}

func (b *Block) CreateBlock(minerAddress []byte, blockNumber uint64, prevBlockHeaderhash []byte, prevBlockTimestamp uint64, txs list.List, timestamp uint64) *Block {
	feeReward := uint64(0)
	for _, tx := range b.Transactions() {
		feeReward += tx.Fee
	}

	totalRewardAmount := BlockRewardCalc(blockNumber, b.config) + feeReward
	coinbaseTX := transactions.CreateCoinBase(minerAddress, blockNumber, totalRewardAmount)
	var hashes list.List
	hashes.PushBack(coinbaseTX.Txhash())
	b.block.Transactions = append(b.block.Transactions, coinbaseTX.PBData())

	for e := txs.Front(); e != nil; e = e.Next() {
		tx := e.Value.(transactions.TransactionInterface)
		hashes.PushBack(tx.Txhash())
		b.block.Transactions = append(b.block.Transactions, tx.PBData())
	}

	merkleRoot := misc.MerkleTXHash(hashes)

	b.blockheader = CreateBlockHeader(blockNumber, prevBlockHeaderhash, prevBlockTimestamp, merkleRoot, feeReward, timestamp)
	b.block.Header = b.blockheader.blockHeader
	b.blockheader.SetNonces(0 ,0)

	return b
}

func (b *Block) FromJSON(jsonData string) *Block {
	b.block = &generated.Block{}
	jsonpb.UnmarshalString(jsonData, b.block)
	b.blockheader = new(BlockHeader)
	b.blockheader.SetPBData(b.block.Header)
	return b
}

func (b *Block) JSON() (string, error) {
	ma := jsonpb.Marshaler{}
	return ma.MarshalToString(b.block)
}

func (b *Block) Serialize() ([]byte, error) {
	return proto.Marshal(b.block)
}

func DeSerializeBlock(data []byte) (*Block, error) {
	b := &Block{}

	if err := proto.Unmarshal(data, b.block); err != nil {
		return b, err
	}

	b.blockheader.blockHeader = b.block.Header

	return b, nil
}

func (b *Block) PrepareAddressesList() map[string]AddressState {
	var addressesState map[string]AddressState
	for _, protoTX := range b.Transactions() {
		tx := transactions.ProtoToTransaction(protoTX)
		tx.SetAffectedAddress(addressesState)
	}
	return addressesState
}