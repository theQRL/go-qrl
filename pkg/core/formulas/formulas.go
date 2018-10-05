package formulas

import (
	"github.com/ericlagergren/decimal"
	math2 "github.com/ericlagergren/decimal/math"
	"math"
	"sort"
	"time"
)

var (
	// Precision is 28 as Python Decimal by default supports upto 28 decimal precision
	// Warning: Change in precision will result into fork
	PythonPrecision=28
)

type Generator struct {
	data []math2.Term
	index int
}

func BigDiv(dividend *decimal.Big, divisor *decimal.Big) *decimal.Big {
	g := NewGenerator([]decimal.Big{*dividend, *divisor})
	dividend = math2.Lentz(dividend, g)
	return dividend
}

func NewGenerator(data []decimal.Big) *Generator {
	/*
	data[0] should be dividend
	data[1] should be divisor
	 */
	g := &Generator{}
	g.index = -1
	g.data = make([]math2.Term, len(data))
	zero := decimal.WithPrecision(PythonPrecision)

	g.data[0] = math2.Term{A:zero, B:zero}
	g.data[1] = math2.Term{A:&data[0], B:&data[1]}
	return g
}

func(g *Generator) Next() bool {
	if g.index < len(g.data) - 1 {
		g.index++
		return true
	}
	return false
}

func(g *Generator) Term() math2.Term {
	return g.data[g.index]
}

func CalcCoeff(coinRemainingAtGenesis uint64) *decimal.Big {
	// TODO: Move these calculations to some constant value, to avoid recalculation
	START_DATE, err := time.Parse(time.RFC3339, "2018-04-01T00:00:00+00:00")
	if err != nil {
		panic("Unable to parse START_DATE")
	}

	END_DATE, err := time.Parse(time.RFC3339, "2218-04-01T00:00:00+00:00")
	if err != nil {
		panic("Unable to parse END_DATE")
	}

	c := END_DATE.Sub(START_DATE)
	c.Nanoseconds()
	TOTAL_MINUTES := c.Minutes()

	// At 1 block per minute
	TOTAL_BLOCKS := *decimal.New(int64(TOTAL_MINUTES), 0)

	coinRemaining := decimal.WithPrecision(PythonPrecision)
	coinRemaining.Add(coinRemaining, decimal.New(int64(coinRemainingAtGenesis), 0))
	k := math2.Log(coinRemaining, coinRemaining)
	return BigDiv(k, &TOTAL_BLOCKS)
}

func RemainingEmission(coinRemainingAtGenesis uint64, shorPerQuanta uint64, blockNumber uint64) int64 {
	/*
	The simplified formula of RemainingEmission is

	(coinRemainingAtGenesis * shorPerQuanta) * math.Exp(-coeff * blockNumber)
	 */

	coeff := CalcCoeff(coinRemainingAtGenesis)
	coeff.Neg(coeff)

	bigBlockNumber := decimal.WithPrecision(PythonPrecision)
	bigBlockNumber.Add(bigBlockNumber, decimal.New(int64(blockNumber), 0))
	coeff.Mul(coeff, bigBlockNumber)
	math2.Exp(coeff, coeff)

	coinRemaining := decimal.WithPrecision(PythonPrecision)
	coinRemaining.Add(coinRemaining, decimal.New(int64(coinRemainingAtGenesis), 0))
	coinRemaining.Mul(coinRemaining, decimal.New(int64(shorPerQuanta), 0))

	result := decimal.WithPrecision(PythonPrecision)
	result.Mul(coinRemaining, coeff)
	result.Quantize(1)
	value, ok := result.Int64()

	if !ok {
		panic("Error while converting to Emission result to Int64")
	}
	return value
}

func BlockReward(coinRemainingAtGenesis uint64, shorPerQuanta uint64, blockNumber uint64) uint64 {
	return uint64(RemainingEmission(coinRemainingAtGenesis, shorPerQuanta, blockNumber - 1) - RemainingEmission(coinRemainingAtGenesis, shorPerQuanta, blockNumber))
}

func Median(data []int) float64 {
	if len(data) == 0 {
		panic("Median cannot be calculated for empty data")
	}
	sort.Ints(data)

	if math.Mod(float64(len(data)), 2) == 1 {
		return float64(data[len(data)/2])
	}

	return float64(data[len(data)/2 - 1] + data[len(data)/2]) / 2
}
