package pow

import (
	"sync"

	"github.com/theQRL/go-qrl/pkg/misc"
	"github.com/theQRL/qryptonight/goqryptonight"
)

type QryptonightInterface interface {
	Hash(blob []byte) []byte
}

type Qryptonight struct {
	lock sync.Mutex
	qn   goqryptonight.Qryptonight
}

func (q *Qryptonight) Hash(blob []byte) []byte {
	q.lock.Lock()
	defer q.lock.Unlock()

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
