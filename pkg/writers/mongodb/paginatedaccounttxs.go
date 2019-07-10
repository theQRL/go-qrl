package mongodb

import "fmt"

type PaginatedAccountTxs struct {
	Key               []byte   `json:"key" bson:"key"`
	TransactionHashes [][]byte `json:"transaction_hashes" bson:"transaction_hashes"`
	TransactionTypes  []int8   `json:"transaction_types" bson:"transaction_types"`
}

func GetPaginatedAccountTxsKey(address []byte, currentPage int64) []byte {
	return append(address, []byte(fmt.Sprintf("_%d", currentPage))...)
}