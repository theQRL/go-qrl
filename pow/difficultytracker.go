package pow

import (
	"github.com/theQRL/qryptonight/goqryptonight"
	"github.com/cyyber/go-qrl/misc"
	"github.com/cyyber/go-qrl/core"
)

type DifficultyTrackerInterface interface {

	GetTarget([]byte) []byte

	Get(uint64, []byte) ([]byte, []byte)
}

type DifficultyTracker struct {

}

func (d *DifficultyTracker) GetTarget(currentDifficulty goqryptonight.UcharVector) []byte {
	c := core.GetConfig()
	ph := goqryptonight.NewPoWHelper(c.Dev.KP, c.Dev.MiningSetpointBlocktime)

	return misc.UCharVectorToBytes(ph.GetTarget(currentDifficulty))
}

func (d *DifficultyTracker) Get(measurement uint64, parentDifficulty []byte) ([]byte, []byte) {
	c := core.GetConfig()
	ph := goqryptonight.NewPoWHelper(c.Dev.KP, c.Dev.MiningSetpointBlocktime)

	currentDifficulty := ph.GetDifficulty(measurement, misc.BytesToUCharVector(parentDifficulty))

	currentTarget := d.GetTarget(currentDifficulty)

	return misc.UCharVectorToBytes(currentDifficulty), currentTarget
}
