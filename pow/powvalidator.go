package pow

import (
	"sync"

	"github.com/theQRL/go-qrl/misc"
	"github.com/theQRL/qryptonight/goqryptonight"
)

type PoWValidatorInterface interface {
	VerifyInput([]byte, []byte) bool

	// TODO: Add Cache version for VerifyInput
}

type PoWValidator struct {
	lock sync.Mutex

	ph goqryptonight.PoWHelper
}

func (p *PoWValidator) VerifyInput(miningBlob []byte, target []byte) bool {
	return p.ph.VerifyInput(misc.BytesToUCharVector(miningBlob), misc.BytesToUCharVector(target))
}

var once sync.Once
var pv *PoWValidator

func GetPowValidator() *PoWValidator {
	once.Do(func() {
		pv = &PoWValidator{}
		pv.ph = goqryptonight.NewPoWHelper()
	})

	return pv
}
