package formulas

import (
	"math"
	"sort"
	"time"
)

func CalcCoeff(coinRemainingAtGenesis uint64) float64 {
	//START_DATE = datetime.datetime(2018, 4, 1, 0, 0, 0)
	START_DATE, err := time.Parse("YYYY-MM-DD", "2018-04-01")
	if err != nil {

	}

	END_DATE, err := time.Parse("YYYY-MM-DD", "2218-04-01")
	if err != nil {

	}

	c := END_DATE.Sub(START_DATE)
	c.Nanoseconds()
	TOTAL_MINUTES := c.Nanoseconds()/(60 * 10^9)

	// At 1 block per minute
	TOTAL_BLOCKS := TOTAL_MINUTES

	return math.Log(float64(coinRemainingAtGenesis)) / float64(TOTAL_BLOCKS)
}

func RemainingEmission(coinRemainingAtGenesis uint64, shorPerQuanta uint64, blockNumber uint64) float64 {
	coeff := CalcCoeff(coinRemainingAtGenesis)
	return float64(coinRemainingAtGenesis * shorPerQuanta) * math.Exp(-coeff * float64(blockNumber))
}

func BlockReward(coinRemaininAtGenesis uint64, shorPerQuanta uint64, blockNumber uint64) uint64 {
	return uint64(RemainingEmission(coinRemaininAtGenesis, shorPerQuanta, blockNumber - 1) - RemainingEmission(coinRemaininAtGenesis, shorPerQuanta, blockNumber))
}

func Median(data []int) int {
	sort.SliceStable(data, func(i, j int) bool { return i < j })
	return data[int(math.Floor(float64(len(data)/2)))]
}
