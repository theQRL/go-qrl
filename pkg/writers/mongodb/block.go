package mongodb

import (
	"github.com/theQRL/go-qrl/pkg/generated"
)

type Block struct {
	HeaderHash []byte  `json:"header_hash" bson:"header_hash"`
	BlockNumber int64  `json:"block_number" bson:"block_number"`
	Timestamp int64  `json:"timestamp" bson:"timestamp"`
	PrevHeaderHash []byte  `json:"prev_header_hash" bson:"prev_header_hash"`
	BlockReward int64  `json:"block_reward" bson:"block_reward"`
	FeeReward int64  `json:"fee_reward" bson:"fee_reward"`
	MerkleRoot []byte  `json:"merkle_root" bson:"merkle_root"`

	MiningNonce int32  `json:"mining_nonce" bson:"mining_nonce"`
	ExtraNonce int64  `json:"extra_nonce" bson:"extra_nonce"`
}

func (b *Block) BlockFromPBData(b2 *generated.Block) {
	bh2 := b2.Header
	b.HeaderHash = bh2.HashHeader
	b.BlockNumber = int64(bh2.BlockNumber)
	b.Timestamp = int64(bh2.TimestampSeconds)
	b.PrevHeaderHash = bh2.HashHeaderPrev
	b.BlockReward = int64(bh2.RewardBlock)
	b.FeeReward = int64(bh2.RewardFee)
	b.MerkleRoot = bh2.MerkleRoot
	b.MiningNonce = int32(bh2.MiningNonce)
	b.ExtraNonce = int64(bh2.ExtraNonce)
}
