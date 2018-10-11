package block

import (
	"bytes"
	"encoding/binary"
	"reflect"

	"github.com/golang/protobuf/jsonpb"

	c "github.com/theQRL/go-qrl/pkg/config"
	"github.com/theQRL/go-qrl/pkg/core/formulas"
	"github.com/theQRL/go-qrl/pkg/generated"
	"github.com/theQRL/go-qrl/pkg/log"
	"github.com/theQRL/go-qrl/pkg/misc"
	"github.com/theQRL/go-qrl/pkg/ntp"
	"github.com/theQRL/go-qrl/pkg/pow"
	"github.com/theQRL/qrllib/goqrllib/goqrllib"
)

type BlockHeaderInterface interface {
	BlockNumber() uint64

	Epoch() uint64

	Timestamp() uint64

	Headerhash() []byte

	PrevHeaderHash() []byte

	BlockReward() uint64

	FeeReward() uint64

	TxMerkleRoot() []byte

	ExtraNonce() uint64

	MiningNonce() uint32

	NonceOffset() uint16

	ExtraNonceOffset() uint16

	MiningBlob() []byte

	GenerateHeaderHash() []byte

	UpdateMerkleRoot([]byte)

	SetNonces(uint32, uint64)

	SetMiningNonceFromBlob([]byte)

	Validate(uint64, uint64) bool

	ValidateParentChildRelation(block Block) bool

	VerifyBlob([]byte) bool

	SetPBData(*generated.BlockHeader)

	FromJSON(string) BlockHeader

	JSON() string
}

type BlockHeader struct {
	blockHeader *generated.BlockHeader

	config *c.Config
	log    log.LoggerInterface
}

func (bh *BlockHeader) BlockNumber() uint64 {
	return bh.blockHeader.BlockNumber
}

func (bh *BlockHeader) Epoch() uint64 {
	return bh.blockHeader.BlockNumber / bh.config.Dev.BlocksPerEpoch
}

func (bh *BlockHeader) Timestamp() uint64 {
	return bh.blockHeader.TimestampSeconds
}

func (bh *BlockHeader) HeaderHash() []byte {
	return bh.blockHeader.HashHeader
}

func (bh *BlockHeader) PrevHeaderHash() []byte {
	return bh.blockHeader.HashHeaderPrev
}

func (bh *BlockHeader) BlockReward() uint64 {
	return bh.blockHeader.RewardBlock
}

func (bh *BlockHeader) FeeReward() uint64 {
	return bh.blockHeader.RewardFee
}

func (bh *BlockHeader) TxMerkleRoot() []byte {
	return bh.blockHeader.MerkleRoot
}

func (bh *BlockHeader) ExtraNonce() uint64 {
	return bh.blockHeader.ExtraNonce
}

func (bh *BlockHeader) MiningNonce() uint32 {
	return bh.blockHeader.MiningNonce
}

func (bh *BlockHeader) NonceOffset() uint16 {
	return bh.config.Dev.MiningNonceOffset
}

func (bh *BlockHeader) ExtraNonceOffset() uint16 {
	return bh.config.Dev.ExtraNonceOffset
}

func (bh *BlockHeader) MiningBlob() []byte {
	tmp := new(bytes.Buffer)
	binary.Write(tmp, binary.BigEndian, uint64(bh.BlockNumber()))
	binary.Write(tmp, binary.BigEndian, uint64(bh.Timestamp()))
	tmp.Write(bh.PrevHeaderHash())
	binary.Write(tmp, binary.BigEndian, uint64(bh.BlockReward()))
	binary.Write(tmp, binary.BigEndian, uint64(bh.FeeReward()))
	tmp.Write(bh.TxMerkleRoot())

	blob := misc.NewUCharVector()
	blob.AddByte(0)
	blob.AddBytes(tmp.Bytes())

	blob.New(goqrllib.Shake128(int64(bh.config.Dev.MiningBlobSize-18), blob.GetData()))

	if blob.GetData().Size() < int64(bh.config.Dev.MiningNonceOffset) {
		panic("Mining blob size below 56 bytes")
	}

	miningNonce := make([]byte, 12)
	binary.BigEndian.PutUint32(miningNonce, bh.MiningNonce())
	binary.BigEndian.PutUint64(miningNonce[4:], bh.ExtraNonce())

	finalBlob := misc.NewUCharVector()
	finalBlob.AddBytes(blob.GetBytes()[:bh.NonceOffset()])
	finalBlob.AddBytes(miningNonce)
	finalBlob.AddBytes(blob.GetBytes()[bh.NonceOffset():])

	return finalBlob.GetBytes()
}

func (bh *BlockHeader) GenerateHeaderHash() []byte {
	qn := pow.GetQryptonight()
	miningBlob := bh.MiningBlob()
	return qn.Hash(miningBlob)
}

func (bh *BlockHeader) UpdateMerkleRoot(hashedtransactions []byte) {
	bh.blockHeader.MerkleRoot = hashedtransactions
}

func (bh *BlockHeader) SetNonces(miningNonce uint32, extraNonce uint64) {
	bh.blockHeader.MiningNonce = miningNonce
	bh.blockHeader.ExtraNonce = extraNonce
	bh.blockHeader.HashHeader = bh.GenerateHeaderHash()
}

func (bh *BlockHeader) SetMiningNonceFromBlob(blob []byte) {
	miningNonceBytes := blob[bh.NonceOffset() : bh.NonceOffset()+4]
	miningNonce := binary.BigEndian.Uint32(miningNonceBytes)

	extraNonceBytes := blob[bh.ExtraNonceOffset() : bh.ExtraNonceOffset()+8]
	extraNonce := binary.BigEndian.Uint64(extraNonceBytes)

	bh.SetNonces(miningNonce, extraNonce)
}

func (bh *BlockHeader) Validate(feeReward uint64, coinbaseAmount uint64, txMerkleRoot []byte) bool {
	n := ntp.GetNTP()
	currentTime := uint64(n.Time())
	allowedTimestamp := currentTime + uint64(bh.config.Dev.BlockLeadTimestamp)
	if bh.Timestamp() > allowedTimestamp {
		bh.log.Warn("BLOCK timestamp is more than the allowed block lead timestamp")
		bh.log.Warn("Block timestamp %s", bh.Timestamp())
		bh.log.Warn("threshold timestamp %s", allowedTimestamp)
		return false
	}

	if bh.Timestamp() < uint64(bh.config.Dev.Genesis.GenesisTimestamp) {
		bh.log.Warn("Timestamp lower than genesis timestamp")
		bh.log.Warn("Genesis Timestamp %s", bh.config.Dev.Genesis.GenesisTimestamp)
		bh.log.Warn("Block Timestamp %s", bh.Timestamp())
		return false
	}

	if !reflect.DeepEqual(bh.GenerateHeaderHash(), bh.HeaderHash()) {
		bh.log.Warn("Headerhash false for block: failed validation")
		return false
	}

	if bh.BlockReward() != BlockRewardCalc(bh.BlockNumber(), bh.config) {
		bh.log.Warn("Block reward incorrect for block: failed validation")
		return false
	}

	if bh.FeeReward() != feeReward {
		bh.log.Warn("Block Fee reward incorrect for block: failed validation")
		return false
	}

	if bh.BlockReward()+bh.FeeReward() != coinbaseAmount {
		bh.log.Warn("Block_reward + fee_reward doesnt sums up to coinbase_amount")
		return false
	}

	if !reflect.DeepEqual(bh.TxMerkleRoot(), txMerkleRoot) {
		bh.log.Warn("Invalid TX Merkle Root")
		return false
	}

	return true
}

func (bh *BlockHeader) ValidateParentChildRelation(parentBlock *Block) bool {
	if parentBlock == nil {
		bh.log.Warn("Parent Block not found")
		return false
	}

	if parentBlock.BlockNumber() != bh.BlockNumber()-1 {
		bh.log.Warn("Block numbers out of sequence: failed validation")
		return false
	}

	if !reflect.DeepEqual(parentBlock.HeaderHash(), bh.PrevHeaderHash()) {
		bh.log.Warn("Headerhash not in sequence: failed validation")
		return false
	}

	if bh.Timestamp() <= parentBlock.Timestamp() {
		bh.log.Warn("BLOCK timestamp must be greater than parent block timestamp")
		bh.log.Warn("block timestamp %s", bh.Timestamp())
		bh.log.Warn("must be greater than %s", parentBlock.Timestamp())
		return false
	}

	return true
}

func (bh *BlockHeader) VerifyBlob(blob []byte) bool {
	miningNonceOffset := bh.config.Dev.MiningNonceOffset
	blob = append(blob[:miningNonceOffset], blob[miningNonceOffset+17:]...)

	actualBlob := bh.MiningBlob()
	actualBlob = append(actualBlob[:miningNonceOffset], actualBlob[miningNonceOffset+17:]...)

	if !reflect.DeepEqual(blob, actualBlob) {
		return false
	}

	return true
}

func (bh *BlockHeader) SetPBData(blockHeader *generated.BlockHeader) {
	bh.blockHeader = blockHeader
}

func (bh *BlockHeader) FromJSON(jsonData string) *BlockHeader {
	bh.blockHeader = &generated.BlockHeader{}
	jsonpb.UnmarshalString(jsonData, bh.blockHeader)
	return bh
}

func (bh *BlockHeader) JSON() (string, error) {
	ma := jsonpb.Marshaler{}
	return ma.MarshalToString(bh.blockHeader)
}

func CreateBlockHeader(blockNumber uint64, prevBlockHeaderHash []byte, prevBlockTimestamp uint64, merkleRoot []byte, feeReward uint64, timestamp uint64) *BlockHeader {
	bh := &BlockHeader{
		blockHeader: &generated.BlockHeader{BlockNumber: blockNumber},
		config:      c.GetConfig(),
		log:         log.GetLogger(),
	}

	if bh.blockHeader.BlockNumber != 0 {
		bh.blockHeader.TimestampSeconds = timestamp
		// If current block timestamp is less than or equals to the previous block timestamp
		// then set current block timestamp 1 sec higher than prev_block_timestamp
		if bh.blockHeader.TimestampSeconds <= prevBlockTimestamp {
			bh.blockHeader.TimestampSeconds = prevBlockTimestamp + 1
		}
		if bh.blockHeader.TimestampSeconds == 0 {
			bh.log.Warn("Failed to get NTP timestamp")
			return nil
		}
	} else {
		bh.blockHeader.TimestampSeconds = prevBlockTimestamp // Set timestamp for genesis block
	}

	bh.blockHeader.HashHeaderPrev = prevBlockHeaderHash
	bh.blockHeader.MerkleRoot = merkleRoot
	bh.blockHeader.RewardFee = feeReward

	bh.blockHeader.RewardBlock = BlockRewardCalc(bh.BlockNumber(), bh.config)

	bh.SetNonces(0, 0)
	return bh
}

func BlockRewardCalc(blockNumber uint64, config *c.Config) uint64 {
	if blockNumber == 0 {
		return config.Dev.Genesis.SuppliedCoins
	}
	return formulas.BlockReward(config.Dev.Genesis.MaxCoinSupply-config.Dev.Genesis.SuppliedCoins, config.Dev.ShorPerQuanta, blockNumber)
}
