package core

import (
	"github.com/cyyber/go-qrl/generated"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/jsonpb"
)

type BlockInterface interface {

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

func (b *Block) Create() *Block {
	feeReward := uint64(0)
	for _, tx := range b.Transactions() {
		feeReward += tx.Fee
	}

	//totalRewardAmount := b.blockheader.BlockRewardCalc(b.BlockNumber()) + feeReward



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