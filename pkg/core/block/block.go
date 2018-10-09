package block

import (
	"container/list"

	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"

	"github.com/theQRL/go-qrl/pkg/config"
	"github.com/theQRL/go-qrl/pkg/core/addressstate"
	"github.com/theQRL/go-qrl/pkg/core/metadata"
	"github.com/theQRL/go-qrl/pkg/core/transactions"
	"github.com/theQRL/go-qrl/pkg/generated"
	"github.com/theQRL/go-qrl/pkg/log"
	"github.com/theQRL/go-qrl/pkg/misc"
	"github.com/theQRL/go-qrl/pkg/pow"
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

	FromJSON(string) Block

	JSON() (string, error)

	Serialize() ([]byte, error)

	Create(blockNumber uint64,
		prevHeaderHash []byte,
		prevBlockTimestamp uint64,
		transactions generated.Transaction,
		minerAddress []byte)

	UpdateMiningAddress(miningAddress []byte)

	Validate(futureBlocks map[string]*generated.Block)

	ValidateMiningNonce(bh *BlockHeader, parentBlock *Block, parentMetadata metadata.BlockMetaData, measurement uint64, enableLogging bool) bool

	IsFutureBlock() bool

	ValidateParentChildRelation(block generated.Block) bool

	ApplyStateChanges(addressesState map[string]*addressstate.AddressState)
}

type Block struct {
	block *generated.Block
	blockheader *BlockHeader

	config *config.Config
	log log.LoggerInterface
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

func (b *Block) Timestamp() uint32 {
	return b.blockheader.Timestamp()
}

func (b *Block) MiningBlob() []byte {
	return b.blockheader.MiningBlob()
}

func CreateBlock(minerAddress []byte, blockNumber uint64, prevBlockHeaderhash []byte, prevBlockTimestamp uint64, txs []transactions.Transaction, timestamp uint64) *Block {
	b := &Block{}
	b.block = &generated.Block{}
	b.config = config.GetConfig() // TODO: Make Config Singleton
	b.log = log.GetLogger()

	feeReward := uint64(0)
	for _, tx := range txs {
		feeReward += tx.Fee()
	}

	totalRewardAmount := BlockRewardCalc(blockNumber, b.config) + feeReward
	coinbaseTX := transactions.CreateCoinBase(minerAddress, blockNumber, totalRewardAmount)
	var hashes list.List
	hashes.PushBack(coinbaseTX.Txhash())
	b.block.Transactions = append(b.block.Transactions, coinbaseTX.PBData())

	for _, tx := range txs {
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

func (b *Block) PrepareAddressesList() map[string]*addressstate.AddressState {
	var addressesState map[string]*addressstate.AddressState
	for _, protoTX := range b.Transactions() {
		tx := transactions.ProtoToTransaction(protoTX)
		tx.SetAffectedAddress(addressesState)
	}
	return addressesState
}

func (b *Block) ApplyStateChanges(addressesState map[string]*addressstate.AddressState) bool {
	coinbase := transactions.CoinBase{}
	coinbase.SetPBData(b.block.Transactions[0])

	if !coinbase.ValidateExtendedCoinbase(b.BlockNumber()) {
		b.log.Warn("coinbase transaction failed")
		return false
	}

	coinbase.ApplyStateChanges(addressesState)

	for i := 1; i <= len(b.Transactions()); i++ {
		tx := transactions.ProtoToTransaction(b.Transactions()[i])


		if !tx.Validate(true) {
			b.log.Warn("failed transaction validation")
			return false
		}

		addrFromPKState := addressesState[string(tx.AddrFrom())]
		addrFromPK := tx.GetSlave()
		if addrFromPK != nil {
			addrFromPKState = addressesState[string(addrFromPK)]
		}

		if !tx.ValidateExtended(addressesState[string(tx.AddrFrom())], addrFromPKState) {
			b.log.Warn("tx validateExtend failed")
			return false
		}

		expectedNonce := addrFromPKState.Nonce() + 1

		if tx.Nonce() != expectedNonce {
			b.log.Warn("nonce incorrect, invalid tx")
			//b.log.Warn("subtype %s", tx.Type())
			b.log.Warn("%s actual: %s expected: %s", tx.AddrFrom(), tx.Nonce(), expectedNonce)
			return false
		}

		if addrFromPKState.OTSKeyReuse(tx.OtsKey()) {
			b.log.Warn("pubkey reuse detected: invalid tx %s", string(tx.Txhash()))
			//b.log.Warn("subtype: %s", tx.Type())
			return false
		}

		tx.ApplyStateChanges(addressesState)
	}
	return true
}

func (b *Block) ValidateMiningNonce(bh *BlockHeader, parentBlock *Block, parentMetadata metadata.BlockMetaData, measurement uint64, enableLogging bool) bool {
	// parentMetadata, err := c.state.GetBlockMetadata(bh.HeaderHash())

	// measurement, err := c.state.GetMeasurement(bh.Timestamp(), bh.PrevHeaderHash(), parentMetadata)
	dt := pow.DifficultyTracker{}
	diff, target := dt.Get(measurement, parentMetadata.BlockDifficulty())

	if enableLogging {
		// parentBlock, err := c.state.GetBlock(bh.PrevHeaderHash())

		b.log.Debug("-----------------START--------------------")
		b.log.Debug("Validate                #%s", bh.BlockNumber())
		b.log.Debug("block.timestamp         %s", bh.Timestamp())
		b.log.Debug("parent_block.timestamp  %s", parentBlock.Timestamp())
		b.log.Debug("parent_block.difficulty %s", string(parentMetadata.BlockDifficulty()))
		b.log.Debug("diff                    %s", string(diff))
		b.log.Debug("target                  %s", string(target))
		b.log.Debug("-------------------END--------------------")
	}

	if !pow.GetPowValidator().VerifyInput(bh.MiningBlob(), target) {
		if enableLogging {
			b.log.Warn("PoW verification failed")
		}
		return false
	}

	return true

}

func (b *Block) Validate(blockFromState *Block, parentBlock *Block, parentMetadata metadata.BlockMetaData, measurement uint64, futureBlocks map[string]*Block) bool {
	var ok bool

	if blockFromState != nil {
		b.log.Warn("Duplicate Block #%s %s", b.BlockNumber(), string(b.HeaderHash()))
		return false
	}

	if parentBlock == nil {
		parentBlock, ok = futureBlocks[string(b.PrevHeaderHash())]
		if !ok {
			b.log.Warn("Parent block not found")
			b.log.Warn("Parent block headerhash %s", string(b.PrevHeaderHash()))
			return false
		}
	}

	if !b.blockheader.ValidateParentChildRelation(parentBlock) {
		b.log.Warn("Failed to validate blocks parent child relation")
		return false
	}

	if !b.ValidateMiningNonce(b.blockheader, parentBlock, parentMetadata, measurement,false) {
		b.log.Warn("Failed PoW Validation")
		return false
	}

	feeReward := uint64(0)
	for i := 1; i < len(b.Transactions()); i++ {
		feeReward += b.Transactions()[i].Fee
	}

	if len(b.Transactions()) == 0 {
		return false
	}

	coinbaseTX := transactions.CoinBase{}
	coinbaseTX.FromPBdata(*b.Transactions()[0])
	coinbaseAmount := coinbaseTX.Amount()

	if !coinbaseTX.ValidateExtendedCoinbase(b.BlockNumber()) {
		return false
	}

	var hashes list.List
	hashes.PushBack(coinbaseTX.Txhash())

	for i := 1; i < len(b.Transactions()); i++ {
		hashes.PushBack(b.Transactions()[i].TransactionHash)
	}

	merkleRoot := misc.MerkleTXHash(hashes)

	if !b.blockheader.Validate(feeReward, coinbaseAmount, merkleRoot) {
		return false
	}

	return true
}
