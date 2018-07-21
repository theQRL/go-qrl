package pow

import (
	"github.com/theQRL/qryptonight/goqryptonight"
	"github.com/cyyber/go-qrl/misc"
	"sync"
)

type QryptonightInterface interface {
	Hash(blob []byte) []byte
}

type Qryptonight struct {
	qn goqryptonight.Qryptonight
}

func (q *Qryptonight) Hash(blob []byte) []byte {
	return misc.UCharVectorToBytes(q.qn.Hash(misc.BytesToUCharVector(blob)))
}

var onceQ sync.Once
var qryptonight *Qryptonight

func GetQryptonight() *Qryptonight {
	onceQ.Do(func() {
		qryptonight = &Qryptonight{}
		qryptonight.qn = goqryptonight.NewQryptonight()
	})

	return qryptonight
}
