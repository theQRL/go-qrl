package pow

import (
	c "github.com/theQRL/go-qrl/pkg/config"
	"github.com/theQRL/go-qrl/pkg/misc"
	"github.com/theQRL/qryptonight/goqryptonight"
)

type DifficultyTrackerInterface interface {
	GetTarget([]byte) []byte

	Get(uint64, []byte) ([]byte, []byte)
}

type DifficultyTracker struct {
}

func (d *DifficultyTracker) GetTarget(currentDifficulty goqryptonight.UcharVector) []byte {
	config := c.GetConfig()
	ph := goqryptonight.NewPoWHelper(config.Dev.KP, config.Dev.MiningSetpointBlocktime)

	return misc.UCharVectorToBytes(ph.GetTarget(currentDifficulty))
}

func (d *DifficultyTracker) Get(measurement uint64, parentDifficulty []byte) ([]byte, []byte) {
	config := c.GetConfig()
	ph := goqryptonight.NewPoWHelper(config.Dev.KP, config.Dev.MiningSetpointBlocktime)

	currentDifficulty := ph.GetDifficulty(measurement, misc.BytesToUCharVector(parentDifficulty))

	currentTarget := d.GetTarget(currentDifficulty)

	return misc.UCharVectorToBytes(currentDifficulty), currentTarget
}
