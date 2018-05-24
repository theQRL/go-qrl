package core

import (
	"encoding/binary"
	"bytes"
	"github.com/golang/protobuf/jsonpb"
	"github.com/theQRL/qrllib/goqrllib"
	"github.com/cyyber/go-qrl/misc"
	"github.com/cyyber/go-qrl/generated"
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

	UpdateMerkleRoot([]byte)

	SetNonces(uint32, uint64)

	SetMiningNonceFromBlob([]byte)

	Validate(uint64, uint64) bool

	ValidateParentChildRelation(block Block) bool

	VerifyBlob([]byte) bool

	FromJSON(string) BlockHeader

	JSON() string
}

type BlockHeader struct {
	blockHeader *generated.BlockHeader

	config *Config
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

	blob := misc.UcharVector{}
	blob.AddByte(0)
	blob.AddBytes(tmp.Bytes())

	blob.New(goqrllib.Shake128(int64(bh.config.Dev.MiningBlobSize - 13), blob.GetData()))

	if blob.GetData().Size() < int64(bh.config.Dev.MiningNonceOffset) {
		//
	}

	miningNonce := make([]byte, 12)
	binary.BigEndian.PutUint32(miningNonce, bh.MiningNonce())
	binary.BigEndian.PutUint64(miningNonce[4:], bh.ExtraNonce())

	finalBlob := misc.UcharVector{}
	finalBlob.AddBytes(blob.GetBytes()[:bh.NonceOffset()])
	finalBlob.AddBytes(miningNonce)
	finalBlob.AddBytes(blob.GetBytes()[bh.NonceOffset():])

	return finalBlob.GetBytes()
}

func (bh *BlockHeader) UpdateMerkleRoot(hashedtransactions []byte) {
	bh.blockHeader.MerkleRoot = hashedtransactions
}

func (bh *BlockHeader) SetNonces(miningNonce uint32, extraNonce uint64) {
	bh.blockHeader.MiningNonce = miningNonce
	bh.blockHeader.ExtraNonce = extraNonce
}

func (bh *BlockHeader) SetMiningNonceFromBlob(blob []byte) {
	miningNonceBytes := blob[bh.NonceOffset():bh.NonceOffset() + 4]
	miningNonce := binary.BigEndian.Uint32(miningNonceBytes)

	extraNonceBytes := blob[bh.ExtraNonceOffset():bh.ExtraNonceOffset() + 8]
	extraNonce := binary.BigEndian.Uint64(extraNonceBytes)

	bh.SetNonces(miningNonce, extraNonce)
}

func (bh *BlockHeader) Validate(feeReward uint64, coinbaseAmount uint64) bool {

	return true
}

func (bh *BlockHeader) ValidateParentChildRelation(block Block) bool {

	return true
}

func (bh *BlockHeader) VerifyBlob([]byte) bool {

	return true
}

func (bh *BlockHeader) SetPBData(blockHeader *generated.BlockHeader) *BlockHeader {
	return nil
}

func (bh *BlockHeader) FromJSON(jsonData string) *BlockHeader {
	bh.blockHeader = &generated.BlockHeader{}
	jsonpb.UnmarshalString(jsonData, bh.blockHeader)
	return bh
}

func (bh *BlockHeader) JSON() (string, error)  {
	ma := jsonpb.Marshaler{}
	return ma.MarshalToString(bh.blockHeader)
}