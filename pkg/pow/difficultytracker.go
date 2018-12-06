package pow

import (
	c "github.com/theQRL/go-qrl/pkg/config"
	"github.com/theQRL/go-qrl/pkg/misc"
	"github.com/theQRL/qryptonight/goqryptonight"
	"strconv"
)

type DifficultyTrackerInterface interface {
	GetTarget(currentDifficulty goqryptonight.UcharVector) []byte

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


type MockDifficultyTracker struct {
	DifficultyTracker
	mockGet       [][]byte
}

func (d *MockDifficultyTracker) SetGetReturnValue(difficulty int64) {
	d.mockGet = make([][]byte, 2)

	tmpDifficulty := goqryptonight.StringToUInt256(strconv.FormatInt(difficulty, 10))
	d.mockGet[0] = misc.UCharVectorToBytes(tmpDifficulty)
	d.mockGet[1] = d.GetTarget(tmpDifficulty)
}

func (d *MockDifficultyTracker) Get(measurement uint64, parentDifficulty []byte) ([]byte, []byte) {
	return d.mockGet[0], d.mockGet[1]
}