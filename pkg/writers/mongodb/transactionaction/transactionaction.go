package transactionaction

import "github.com/theQRL/go-qrl/pkg/generated"

type TransactionAction struct {
	Transaction *generated.Transaction
	Timestamp   uint64
	IsAdd       bool // if false, then remove the unconfirmed txn from mongodb
}
