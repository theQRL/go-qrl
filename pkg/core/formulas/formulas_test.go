package formulas

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCalcCoeff(t *testing.T) {
	assert.Equal(t, "1.664087503734056374552843909E-7", CalcCoeff(40000000).String())
	assert.NotEqual(t, "1.664087503734056374552843908E-7", CalcCoeff(40000000).String())
}

func TestRemainingEmission(t *testing.T) {
	assert.Equal(t, int64(40000000000000000), RemainingEmission(40000000, 1000000000, 0))
	assert.Equal(t, int64(39999993343650538), RemainingEmission(40000000, 1000000000, 1))
	assert.Equal(t, int64(39999986687302185), RemainingEmission(40000000, 1000000000, 2))
	assert.Equal(t, int64(39999334370536850), RemainingEmission(40000000, 1000000000, 100))
}

func TestBlockReward(t *testing.T) {
	assert.Equal(t, uint64(6656349462), BlockReward(40000000, 1000000000, 1))
	assert.Equal(t, uint64(6656348353), BlockReward(40000000, 1000000000, 2))
	assert.Equal(t, uint64(6656347246), BlockReward(40000000, 1000000000, 3))
}

func TestMedian(t *testing.T) {
	data := []int{20, 2, 100, 3, 8, 10, 5, 60}
	result := Median(data)
	assert.Equal(t, result, float64(9))

	data = []int{20}
	result = Median(data)
	assert.Equal(t, result, float64(20))

	data = []int{20, 1, 100, 21}
	result = Median(data)
	assert.Equal(t, result, float64(20.5))

	data = []int{}
	assert.Panics(t, func() { Median(data) }, "Expected Panic")
}
