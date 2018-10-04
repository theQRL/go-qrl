package metadata

import (
	"reflect"

	"github.com/golang/protobuf/proto"

	"github.com/theQRL/go-qrl/pkg/config"
	"github.com/theQRL/go-qrl/pkg/generated"
	"github.com/theQRL/go-qrl/pkg/log"
)

type BlockMetaData struct {
	data *generated.BlockMetaData
	log log.Logger
	config *config.Config
}

func (b *BlockMetaData) PBData() *generated.BlockMetaData {
	return b.data
}

func (b *BlockMetaData) BlockDifficulty() []byte {
	return b.data.BlockDifficulty
}

func (b *BlockMetaData) TotalDifficulty() []byte {
	return b.data.CumulativeDifficulty
}

func (b *BlockMetaData) ChildHeaderHashes() [][]byte {
	return b.data.ChildHeaderhashes
}

func (b *BlockMetaData) LastNHeaderHashes() [][]byte {
	return b.data.Last_NHeaderhashes
}

func (b *BlockMetaData) SetBlockDifficulty(value []byte) {
	if len(value) != 32 {
		b.log.Warn("Invalid BlockDifficulty")
	}
	b.data.BlockDifficulty = value
}

func (b *BlockMetaData) SetTotalDifficulty(value []byte) {
	if len(value) != 32 {
		b.log.Warn("Invalid TotalDifficulty")
	}
	b.data.CumulativeDifficulty = value
}

func (b *BlockMetaData) AddChildHeaderHash(ChildHeaderHash []byte) {
	for _, headerHash := range b.data.ChildHeaderhashes {
		if reflect.DeepEqual(headerHash, ChildHeaderHash) {
			return
		}
	}
	b.data.ChildHeaderhashes = append(b.data.ChildHeaderhashes, ChildHeaderHash)
}

func (b *BlockMetaData) UpdateLastHeaderHashes(parentLastNHeaderHashes [][]byte, lastHeaderHash []byte) {
	b.data.Last_NHeaderhashes = append(parentLastNHeaderHashes, lastHeaderHash)

	if len(b.data.Last_NHeaderhashes) > int(b.config.Dev.NMeasurement) {
		b.data.Last_NHeaderhashes = b.data.Last_NHeaderhashes[1:]
	}

	if len(b.data.Last_NHeaderhashes) > int(b.config.Dev.NMeasurement) {
		panic("Length of Last N Headerhashes is more than the allowed NMeasurement in config")
	}
}

func CreateBlockMetadata(blockDifficulty []byte, totalDifficulty []byte, childHeaderHashes [][]byte) *BlockMetaData {
	b := &BlockMetaData{}

	b.data.BlockDifficulty = blockDifficulty
	b.data.CumulativeDifficulty = totalDifficulty
	b.data.ChildHeaderhashes = childHeaderHashes

	return b
}

func (b *BlockMetaData) Serialize() ([]byte, error) {
	return proto.Marshal(b.data)
}

func DeSerializeBlockMetaData(data []byte) (*BlockMetaData, error) {
	b := &BlockMetaData{}

	if err := proto.Unmarshal(data, b.data); err != nil {
		return b, err
	}

	return b, nil
}
