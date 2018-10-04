package metadata

import (
	"reflect"

	"github.com/golang/protobuf/proto"

	"github.com/theQRL/go-qrl/pkg/generated"
)

type TokenMetadata struct {
	data *generated.TokenMetadata
}

func (t *TokenMetadata) PBData() *generated.TokenMetadata {
	return t.data
}

func (t *TokenMetadata) TokenTxHash() []byte {
	return t.data.TokenTxhash
}

func (t *TokenMetadata) Append(transferTokenTxHash []byte) {
	t.data.TransferTokenTxHashes = append(t.data.TransferTokenTxHashes, transferTokenTxHash)
}

func (t *TokenMetadata) Remove(transferTokenTxHash []byte) {
	for index, hash := range t.data.TransferTokenTxHashes {
		if reflect.DeepEqual(hash, transferTokenTxHash) {
			t.data.TransferTokenTxHashes = append(t.data.TransferTokenTxHashes[:index], t.data.TransferTokenTxHashes[index + 1:]...)
		}
	}
}

func (t *TokenMetadata) Serialize() ([]byte, error) {
	return proto.Marshal(t.data)
}

func DeSerializeTokenMetadata(data []byte) (*TokenMetadata, error) {
	t := &TokenMetadata{}

	if err := proto.Unmarshal(data, t.data); err != nil {
		return t, err
	}

	return t, nil
}

func CreateTokenMetadata(tokenTxHash []byte, transferTokenTxHash []byte) *TokenMetadata {
	t := &TokenMetadata{}

	t.data.TokenTxhash = tokenTxHash

	t.Append(transferTokenTxHash)

	return t
}
