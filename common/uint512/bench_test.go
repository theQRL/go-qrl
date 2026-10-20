package uint512

import (
	"math/big"
	"testing"

	"github.com/holiman/uint256"
)

// Baseline inputs for benchmarks. Full-width (all 512 bits) to exercise all limbs.
var (
	benchA512 = new(Int).SetAllOne()           // 2^512 - 1
	benchB512 = new(Int).SetBytes(benchBytesB) // large random-ish
	benchM512 = new(Int).SetBytes(benchBytesM) // modulus for *Mod ops

	benchA256 = new(uint256.Int).SetAllOne()
	benchB256 = new(uint256.Int).SetBytes(benchBytesB[32:])

	benchBytesB = []byte{
		0xde, 0xad, 0xbe, 0xef, 0x12, 0x34, 0x56, 0x78,
		0x9a, 0xbc, 0xde, 0xf0, 0x11, 0x22, 0x33, 0x44,
		0x55, 0x66, 0x77, 0x88, 0x99, 0xaa, 0xbb, 0xcc,
		0xdd, 0xee, 0xff, 0x00, 0x11, 0x22, 0x33, 0x44,
		0x55, 0x66, 0x77, 0x88, 0x99, 0xaa, 0xbb, 0xcc,
		0xdd, 0xee, 0xff, 0x00, 0x11, 0x22, 0x33, 0x44,
		0x55, 0x66, 0x77, 0x88, 0x99, 0xaa, 0xbb, 0xcc,
		0xdd, 0xee, 0xff, 0x00, 0x11, 0x22, 0x33, 0x45,
	}
	benchBytesM = []byte{
		0x7f, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x01,
	}
)

// --- uint512 benchmarks (our implementation) -------------------------------

func BenchmarkAdd(b *testing.B) {
	var z Int
	b.ReportAllocs()
	for b.Loop() {
		z.Add(benchA512, benchB512)
	}
}

func BenchmarkSub(b *testing.B) {
	var z Int
	b.ReportAllocs()
	for b.Loop() {
		z.Sub(benchA512, benchB512)
	}
}

func BenchmarkMul(b *testing.B) {
	var z Int
	b.ReportAllocs()
	for b.Loop() {
		z.Mul(benchA512, benchB512)
	}
}

func BenchmarkDiv(b *testing.B) {
	var z Int
	b.ReportAllocs()
	for b.Loop() {
		z.Div(benchA512, benchB512)
	}
}

func BenchmarkMod(b *testing.B) {
	var z Int
	b.ReportAllocs()
	for b.Loop() {
		z.Mod(benchA512, benchB512)
	}
}

func BenchmarkExp(b *testing.B) {
	var z Int
	// Use a smaller exponent (still nontrivial) — full 2^512 exp is astronomically slow.
	exp := NewInt(1234567)
	b.ReportAllocs()
	for b.Loop() {
		z.Exp(benchA512, exp)
	}
}

func BenchmarkAddMod(b *testing.B) {
	var z Int
	b.ReportAllocs()
	for b.Loop() {
		z.AddMod(benchA512, benchB512, benchM512)
	}
}

func BenchmarkMulMod(b *testing.B) {
	var z Int
	b.ReportAllocs()
	for b.Loop() {
		z.MulMod(benchA512, benchB512, benchM512)
	}
}

func BenchmarkAnd(b *testing.B) {
	var z Int
	b.ReportAllocs()
	for b.Loop() {
		z.And(benchA512, benchB512)
	}
}

func BenchmarkLsh(b *testing.B) {
	var z Int
	b.ReportAllocs()
	for b.Loop() {
		z.Lsh(benchA512, 123)
	}
}

func BenchmarkCmp(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		benchA512.Cmp(benchB512)
	}
}

func BenchmarkSetBytes(b *testing.B) {
	var z Int
	b.ReportAllocs()
	for b.Loop() {
		z.SetBytes(benchBytesB)
	}
}

func BenchmarkBytes64(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_ = benchA512.Bytes64()
	}
}

// --- uint256 baseline (target: we should be within ~2x of this) -----------

func BenchmarkAdd256(b *testing.B) {
	var z uint256.Int
	b.ReportAllocs()
	for b.Loop() {
		z.Add(benchA256, benchB256)
	}
}

func BenchmarkSub256(b *testing.B) {
	var z uint256.Int
	b.ReportAllocs()
	for b.Loop() {
		z.Sub(benchA256, benchB256)
	}
}

func BenchmarkMul256(b *testing.B) {
	var z uint256.Int
	b.ReportAllocs()
	for b.Loop() {
		z.Mul(benchA256, benchB256)
	}
}

func BenchmarkDiv256(b *testing.B) {
	var z uint256.Int
	b.ReportAllocs()
	for b.Loop() {
		z.Div(benchA256, benchB256)
	}
}

func BenchmarkExp256(b *testing.B) {
	var z uint256.Int
	exp := uint256.NewInt(1234567)
	b.ReportAllocs()
	for b.Loop() {
		z.Exp(benchA256, exp)
	}
}

// --- math/big baseline for calibration ------------------------------------

func BenchmarkAddBig(b *testing.B) {
	z := new(big.Int)
	x := new(big.Int).SetBytes(benchBytesB)
	y := new(big.Int).SetBytes(benchBytesM)
	b.ReportAllocs()
	for b.Loop() {
		z.Add(x, y)
	}
}

func BenchmarkMulBig(b *testing.B) {
	z := new(big.Int)
	x := new(big.Int).SetBytes(benchBytesB)
	y := new(big.Int).SetBytes(benchBytesM)
	b.ReportAllocs()
	for b.Loop() {
		z.Mul(x, y)
	}
}
