// Package uint512 provides a 512-bit unsigned integer type with an API
// compatible with github.com/holiman/uint256.Int.
//
// The value is held as 8 little-endian 64-bit limbs (limb 0 is the
// least-significant word). All arithmetic is performed modulo 2^512.
// Division and exponentiation fall through to math/big as a correctness
// baseline; hot paths for Add/Sub/Mul and bitwise operations are
// implemented natively with math/bits carry primitives.
package uint512

import (
	"fmt"
	"math/big"
	"math/bits"
)

const (
	// WordBits is the width of Int in bits.
	WordBits = 512
	// WordBytes is the width of Int in bytes.
	WordBytes = WordBits / 8
)

// Int is a 512-bit unsigned integer stored as 8 little-endian 64-bit limbs.
//
// z[0] is bits 0..63, z[7] is bits 448..511. The zero value represents 0.
type Int [8]uint64

// --- construction -----------------------------------------------------------

// NewInt returns a new Int with value v.
func NewInt(v uint64) *Int {
	z := &Int{}
	z[0] = v
	return z
}

// FromBig returns an Int with value b reduced modulo 2^512 and a boolean
// reporting whether b was outside [0, 2^512).
func FromBig(b *big.Int) (*Int, bool) {
	z := &Int{}
	overflow := z.SetFromBig(b)
	return z, overflow
}

// MustFromBig is like FromBig but panics on overflow.
func MustFromBig(b *big.Int) *Int {
	z, overflow := FromBig(b)
	if overflow {
		panic("uint512: value overflows 512 bits")
	}
	return z
}

// --- basic accessors --------------------------------------------------------

// Uint64 returns the low 64 bits of z.
func (z *Int) Uint64() uint64 {
	return z[0]
}

// Uint64WithOverflow returns the low 64 bits of z together with a flag that
// reports whether z exceeds a uint64.
func (z *Int) Uint64WithOverflow() (uint64, bool) {
	return z[0], (z[1] | z[2] | z[3] | z[4] | z[5] | z[6] | z[7]) != 0
}

// IsUint64 reports whether z fits in a uint64.
func (z *Int) IsUint64() bool {
	return (z[1] | z[2] | z[3] | z[4] | z[5] | z[6] | z[7]) == 0
}

// IsZero reports whether z is 0.
func (z *Int) IsZero() bool {
	return (z[0] | z[1] | z[2] | z[3] | z[4] | z[5] | z[6] | z[7]) == 0
}

// Sign returns -1, 0 or 1 interpreting z as a signed 512-bit integer.
// The zero value returns 0; values with the MSB set are negative.
func (z *Int) Sign() int {
	if z.IsZero() {
		return 0
	}
	if z[7]>>63 != 0 {
		return -1
	}
	return 1
}

// BitLen returns the number of bits required to represent z.
func (z *Int) BitLen() int {
	for i := 7; i >= 0; i-- {
		if z[i] != 0 {
			return i*64 + bits.Len64(z[i])
		}
	}
	return 0
}

// Cmp compares z and x as unsigned integers and returns -1, 0 or 1.
func (z *Int) Cmp(x *Int) int {
	for i := 7; i >= 0; i-- {
		if z[i] != x[i] {
			if z[i] < x[i] {
				return -1
			}
			return 1
		}
	}
	return 0
}

// Eq reports whether z == x.
func (z *Int) Eq(x *Int) bool {
	return z[0] == x[0] && z[1] == x[1] && z[2] == x[2] && z[3] == x[3] &&
		z[4] == x[4] && z[5] == x[5] && z[6] == x[6] && z[7] == x[7]
}

// Lt reports whether z < x (unsigned).
func (z *Int) Lt(x *Int) bool {
	return z.Cmp(x) < 0
}

// Gt reports whether z > x (unsigned).
func (z *Int) Gt(x *Int) bool {
	return z.Cmp(x) > 0
}

// LtUint64 reports whether z < u (unsigned).
func (z *Int) LtUint64(u uint64) bool {
	return (z[1]|z[2]|z[3]|z[4]|z[5]|z[6]|z[7]) == 0 && z[0] < u
}

// GtUint64 reports whether z > u (unsigned).
func (z *Int) GtUint64(u uint64) bool {
	if (z[1] | z[2] | z[3] | z[4] | z[5] | z[6] | z[7]) != 0 {
		return true
	}
	return z[0] > u
}

// toSigned returns z interpreted as a signed 512-bit integer in a fresh
// big.Int. Used by signed-comparison and signed-division helpers.
func (z *Int) toSigned() *big.Int {
	b := z.ToBig()
	if z.Sign() < 0 {
		// b currently holds unsigned value; subtract 2^512 to get signed value.
		b.Sub(b, modulus())
	}
	return b
}

// Slt reports whether z < x when both are interpreted as signed 512-bit.
func (z *Int) Slt(x *Int) bool {
	zsign := z[7] >> 63
	xsign := x[7] >> 63
	if zsign != xsign {
		return zsign == 1 // z negative, x positive
	}
	return z.Cmp(x) < 0
}

// Sgt reports whether z > x when both are interpreted as signed 512-bit.
func (z *Int) Sgt(x *Int) bool {
	zsign := z[7] >> 63
	xsign := x[7] >> 63
	if zsign != xsign {
		return xsign == 1 // x negative, z positive
	}
	return z.Cmp(x) > 0
}

// --- setters ----------------------------------------------------------------

// Set assigns x to z and returns z.
func (z *Int) Set(x *Int) *Int {
	*z = *x
	return z
}

// SetUint64 sets z to v and returns z.
func (z *Int) SetUint64(v uint64) *Int {
	z[0] = v
	z[1], z[2], z[3], z[4], z[5], z[6], z[7] = 0, 0, 0, 0, 0, 0, 0
	return z
}

// SetBytes interprets buf as the big-endian bytes of an unsigned integer,
// truncates to 512 bits if longer (keeping the least-significant bytes),
// and sets z. Returns z.
func (z *Int) SetBytes(buf []byte) *Int {
	if len(buf) > WordBytes {
		buf = buf[len(buf)-WordBytes:]
	}
	// Zero the destination first.
	*z = Int{}
	// Copy bytes into a temporary 64-byte buffer, right-aligned.
	var tmp [WordBytes]byte
	copy(tmp[WordBytes-len(buf):], buf)
	// tmp is big-endian: tmp[0..7] are the most-significant 8 bytes (limb 7).
	for i := 0; i < 8; i++ {
		// limb i (little-endian) comes from tmp bytes (7-i)*8 .. (7-i)*8+7.
		off := (7 - i) * 8
		z[i] = uint64(tmp[off])<<56 | uint64(tmp[off+1])<<48 |
			uint64(tmp[off+2])<<40 | uint64(tmp[off+3])<<32 |
			uint64(tmp[off+4])<<24 | uint64(tmp[off+5])<<16 |
			uint64(tmp[off+6])<<8 | uint64(tmp[off+7])
	}
	return z
}

// SetFromBig sets z to b mod 2^512 and reports whether b was outside
// [0, 2^512).
func (z *Int) SetFromBig(b *big.Int) bool {
	overflow := b.Sign() < 0 || b.BitLen() > WordBits
	if overflow {
		// Reduce to [0, 2^512).
		tmp := new(big.Int).And(b, mask512())
		z.setFromBigUnsafe(tmp)
	} else {
		z.setFromBigUnsafe(b)
	}
	return overflow
}

// setFromBigUnsafe assumes b is in [0, 2^512) and writes it into z.
func (z *Int) setFromBigUnsafe(b *big.Int) {
	*z = Int{}
	bytes := b.Bytes() // big-endian
	if len(bytes) > WordBytes {
		bytes = bytes[len(bytes)-WordBytes:]
	}
	var tmp [WordBytes]byte
	copy(tmp[WordBytes-len(bytes):], bytes)
	for i := 0; i < 8; i++ {
		off := (7 - i) * 8
		z[i] = uint64(tmp[off])<<56 | uint64(tmp[off+1])<<48 |
			uint64(tmp[off+2])<<40 | uint64(tmp[off+3])<<32 |
			uint64(tmp[off+4])<<24 | uint64(tmp[off+5])<<16 |
			uint64(tmp[off+6])<<8 | uint64(tmp[off+7])
	}
}

// SetAllOne sets z to 2^512 - 1 and returns z.
func (z *Int) SetAllOne() *Int {
	z[0], z[1], z[2], z[3] = ^uint64(0), ^uint64(0), ^uint64(0), ^uint64(0)
	z[4], z[5], z[6], z[7] = ^uint64(0), ^uint64(0), ^uint64(0), ^uint64(0)
	return z
}

// SetOne sets z to 1 and returns z.
func (z *Int) SetOne() *Int {
	z[0] = 1
	z[1], z[2], z[3], z[4], z[5], z[6], z[7] = 0, 0, 0, 0, 0, 0, 0
	return z
}

// Clear sets z to 0 and returns z.
func (z *Int) Clear() *Int {
	*z = Int{}
	return z
}

// Reset is an alias for Clear kept for API compatibility.
func (z *Int) Reset() {
	*z = Int{}
}

// Clone returns a deep copy of z.
func (z *Int) Clone() *Int {
	c := *z
	return &c
}

// ToBig returns z as a fresh *big.Int.
func (z *Int) ToBig() *big.Int {
	return new(big.Int).SetBytes(z.bytes64Slice())
}

// bytes64Slice returns the 64-byte big-endian representation of z as a slice,
// without the leading-zero stripping done by Bytes().
func (z *Int) bytes64Slice() []byte {
	out := make([]byte, WordBytes)
	for i := 0; i < 8; i++ {
		off := (7 - i) * 8
		v := z[i]
		out[off+0] = byte(v >> 56)
		out[off+1] = byte(v >> 48)
		out[off+2] = byte(v >> 40)
		out[off+3] = byte(v >> 32)
		out[off+4] = byte(v >> 24)
		out[off+5] = byte(v >> 16)
		out[off+6] = byte(v >> 8)
		out[off+7] = byte(v)
	}
	return out
}

// --- byte serialization -----------------------------------------------------

// Bytes returns the minimal big-endian byte representation of z (no leading
// zeros). An empty slice is returned for zero.
func (z *Int) Bytes() []byte {
	b := z.bytes64Slice()
	i := 0
	for i < len(b) && b[i] == 0 {
		i++
	}
	return b[i:]
}

// Bytes32 returns the 32-byte big-endian representation of the low 256 bits
// of z.
func (z *Int) Bytes32() [32]byte {
	var out [32]byte
	// low 4 limbs (z[0..3]) map to the last 32 bytes of the big-endian output.
	for i := 0; i < 4; i++ {
		off := (3 - i) * 8
		v := z[i]
		out[off+0] = byte(v >> 56)
		out[off+1] = byte(v >> 48)
		out[off+2] = byte(v >> 40)
		out[off+3] = byte(v >> 32)
		out[off+4] = byte(v >> 24)
		out[off+5] = byte(v >> 16)
		out[off+6] = byte(v >> 8)
		out[off+7] = byte(v)
	}
	return out
}

// Bytes64 returns the 64-byte big-endian representation of z.
func (z *Int) Bytes64() [WordBytes]byte {
	var out [WordBytes]byte
	for i := 0; i < 8; i++ {
		off := (7 - i) * 8
		v := z[i]
		out[off+0] = byte(v >> 56)
		out[off+1] = byte(v >> 48)
		out[off+2] = byte(v >> 40)
		out[off+3] = byte(v >> 32)
		out[off+4] = byte(v >> 24)
		out[off+5] = byte(v >> 16)
		out[off+6] = byte(v >> 8)
		out[off+7] = byte(v)
	}
	return out
}

// Hex returns the hexadecimal representation of z prefixed with "0x".
func (z *Int) Hex() string {
	return "0x" + z.ToBig().Text(16)
}

// Format implements fmt.Formatter. It supports the same verbs as big.Int.
func (z Int) Format(s fmt.State, verb rune) {
	z.ToBig().Format(s, verb)
}

// --- arithmetic -------------------------------------------------------------

// Add sets z = x + y (mod 2^512) and returns z.
func (z *Int) Add(x, y *Int) *Int {
	var c uint64
	z[0], c = bits.Add64(x[0], y[0], 0)
	z[1], c = bits.Add64(x[1], y[1], c)
	z[2], c = bits.Add64(x[2], y[2], c)
	z[3], c = bits.Add64(x[3], y[3], c)
	z[4], c = bits.Add64(x[4], y[4], c)
	z[5], c = bits.Add64(x[5], y[5], c)
	z[6], c = bits.Add64(x[6], y[6], c)
	z[7], _ = bits.Add64(x[7], y[7], c)
	return z
}

// Sub sets z = x - y (mod 2^512) and returns z.
func (z *Int) Sub(x, y *Int) *Int {
	var b uint64
	z[0], b = bits.Sub64(x[0], y[0], 0)
	z[1], b = bits.Sub64(x[1], y[1], b)
	z[2], b = bits.Sub64(x[2], y[2], b)
	z[3], b = bits.Sub64(x[3], y[3], b)
	z[4], b = bits.Sub64(x[4], y[4], b)
	z[5], b = bits.Sub64(x[5], y[5], b)
	z[6], b = bits.Sub64(x[6], y[6], b)
	z[7], _ = bits.Sub64(x[7], y[7], b)
	return z
}

// Mul sets z = x * y (mod 2^512) and returns z.
func (z *Int) Mul(x, y *Int) *Int {
	// Schoolbook multiplication, 8x8 limbs, truncated to low 8 limbs.
	//
	// For each (i,j), compute 128-bit partial product x[i]*y[j] and add to
	// res[i+j] with carry to res[i+j+1]. Because max product (2^64-1)^2 =
	// 2^128 - 2^65 + 1 has hi <= 2^64 - 2, the inner accumulation
	//   new_hi = hi + c1 + c2
	// never overflows 64 bits (c1,c2 ∈ {0,1} and cannot both be 1 at hi=2^64-2).
	var res Int
	for i := 0; i < 8; i++ {
		var carry uint64
		for j := 0; i+j < 8; j++ {
			hi, lo := bits.Mul64(x[i], y[j])
			var c1, c2 uint64
			lo, c1 = bits.Add64(lo, res[i+j], 0)
			lo, c2 = bits.Add64(lo, carry, 0)
			res[i+j] = lo
			carry = hi + c1 + c2
		}
		// Final carry is discarded (would land in res[i+8] which doesn't exist).
	}
	*z = res
	return z
}

// Div sets z = x / y (unsigned, truncated). If y == 0 the result is 0.
func (z *Int) Div(x, y *Int) *Int {
	if y.IsZero() {
		z.Clear()
		return z
	}
	bx, by := x.ToBig(), y.ToBig()
	bx.Quo(bx, by)
	z.setFromBigUnsafe(bx)
	return z
}

// Mod sets z = x mod y (unsigned). If y == 0 the result is 0.
func (z *Int) Mod(x, y *Int) *Int {
	if y.IsZero() {
		z.Clear()
		return z
	}
	bx, by := x.ToBig(), y.ToBig()
	bx.Rem(bx, by)
	z.setFromBigUnsafe(bx)
	return z
}

// SDiv sets z = x / y with QRVM signed semantics (truncate toward zero).
// Result is 0 when y == 0.
func (z *Int) SDiv(x, y *Int) *Int {
	if y.IsZero() {
		z.Clear()
		return z
	}
	bx, by := x.toSigned(), y.toSigned()
	bx.Quo(bx, by)
	if bx.Sign() < 0 {
		bx.Add(bx, modulus())
	}
	z.setFromBigUnsafe(bx)
	return z
}

// SMod sets z = x mod y with QRVM signed semantics (result takes the sign of x).
// Result is 0 when y == 0.
func (z *Int) SMod(x, y *Int) *Int {
	if y.IsZero() {
		z.Clear()
		return z
	}
	bx, by := x.toSigned(), y.toSigned()
	bx.Rem(bx, by)
	if bx.Sign() < 0 {
		bx.Add(bx, modulus())
	}
	z.setFromBigUnsafe(bx)
	return z
}

// Exp sets z = base ** exp (mod 2^512) and returns z.
func (z *Int) Exp(base, exp *Int) *Int {
	bb := base.ToBig()
	be := exp.ToBig()
	bb.Exp(bb, be, modulus())
	z.setFromBigUnsafe(bb)
	return z
}

// AddMod sets z = (x + y) mod m. If m == 0 the result is 0.
func (z *Int) AddMod(x, y, m *Int) *Int {
	if m.IsZero() {
		z.Clear()
		return z
	}
	bx, by, bm := x.ToBig(), y.ToBig(), m.ToBig()
	bx.Add(bx, by)
	bx.Rem(bx, bm)
	z.setFromBigUnsafe(bx)
	return z
}

// MulMod sets z = (x * y) mod m. If m == 0 the result is 0.
func (z *Int) MulMod(x, y, m *Int) *Int {
	if m.IsZero() {
		z.Clear()
		return z
	}
	bx, by, bm := x.ToBig(), y.ToBig(), m.ToBig()
	bx.Mul(bx, by)
	bx.Rem(bx, bm)
	z.setFromBigUnsafe(bx)
	return z
}

// ExtendSign sets z to x sign-extended from byte position (byteNum+1) to
// 512 bits. Matches the semantics of EVM SIGNEXTEND but for 64-byte words.
func (z *Int) ExtendSign(x, byteNum *Int) *Int {
	if !byteNum.IsUint64() || byteNum[0] >= WordBytes-1 {
		if z != x {
			*z = *x
		}
		return z
	}
	bn := byteNum[0]
	bit := uint(bn)*8 + 7 // index of the sign bit

	// Bits below and including the sign bit are preserved; bits above are
	// filled with the sign value.
	signSet := (x[bit/64]>>(bit%64))&1 != 0

	// Build mask = 2^(bit+1) - 1.
	var mask Int
	mask[bit/64] = (uint64(1) << ((bit % 64) + 1)) - 1
	for i := 0; i < int(bit/64); i++ {
		mask[i] = ^uint64(0)
	}

	// z = x & mask
	z[0] = x[0] & mask[0]
	z[1] = x[1] & mask[1]
	z[2] = x[2] & mask[2]
	z[3] = x[3] & mask[3]
	z[4] = x[4] & mask[4]
	z[5] = x[5] & mask[5]
	z[6] = x[6] & mask[6]
	z[7] = x[7] & mask[7]

	if signSet {
		// Set bits above the sign bit: z |= ~mask.
		z[0] |= ^mask[0]
		z[1] |= ^mask[1]
		z[2] |= ^mask[2]
		z[3] |= ^mask[3]
		z[4] |= ^mask[4]
		z[5] |= ^mask[5]
		z[6] |= ^mask[6]
		z[7] |= ^mask[7]
	}
	return z
}

// --- bitwise ----------------------------------------------------------------

// And sets z = x & y and returns z.
func (z *Int) And(x, y *Int) *Int {
	z[0] = x[0] & y[0]
	z[1] = x[1] & y[1]
	z[2] = x[2] & y[2]
	z[3] = x[3] & y[3]
	z[4] = x[4] & y[4]
	z[5] = x[5] & y[5]
	z[6] = x[6] & y[6]
	z[7] = x[7] & y[7]
	return z
}

// Or sets z = x | y and returns z.
func (z *Int) Or(x, y *Int) *Int {
	z[0] = x[0] | y[0]
	z[1] = x[1] | y[1]
	z[2] = x[2] | y[2]
	z[3] = x[3] | y[3]
	z[4] = x[4] | y[4]
	z[5] = x[5] | y[5]
	z[6] = x[6] | y[6]
	z[7] = x[7] | y[7]
	return z
}

// Xor sets z = x ^ y and returns z.
func (z *Int) Xor(x, y *Int) *Int {
	z[0] = x[0] ^ y[0]
	z[1] = x[1] ^ y[1]
	z[2] = x[2] ^ y[2]
	z[3] = x[3] ^ y[3]
	z[4] = x[4] ^ y[4]
	z[5] = x[5] ^ y[5]
	z[6] = x[6] ^ y[6]
	z[7] = x[7] ^ y[7]
	return z
}

// Not sets z = ^x (bitwise complement within 512 bits) and returns z.
func (z *Int) Not(x *Int) *Int {
	z[0] = ^x[0]
	z[1] = ^x[1]
	z[2] = ^x[2]
	z[3] = ^x[3]
	z[4] = ^x[4]
	z[5] = ^x[5]
	z[6] = ^x[6]
	z[7] = ^x[7]
	return z
}

// Lsh sets z = x << n (mod 2^512) and returns z.
func (z *Int) Lsh(x *Int, n uint) *Int {
	if n >= WordBits {
		z.Clear()
		return z
	}
	if n == 0 {
		*z = *x
		return z
	}
	limbShift := n / 64
	bitShift := n % 64
	var tmp Int
	if bitShift == 0 {
		for i := uint(0); i < 8-limbShift; i++ {
			tmp[i+limbShift] = x[i]
		}
	} else {
		invShift := 64 - bitShift
		var prev uint64
		for i := uint(0); i < 8-limbShift; i++ {
			v := x[i]
			tmp[i+limbShift] = (v << bitShift) | prev
			prev = v >> invShift
		}
	}
	*z = tmp
	return z
}

// Rsh sets z = x >> n (logical shift right) and returns z.
func (z *Int) Rsh(x *Int, n uint) *Int {
	if n >= WordBits {
		z.Clear()
		return z
	}
	if n == 0 {
		*z = *x
		return z
	}
	limbShift := n / 64
	bitShift := n % 64
	var tmp Int
	if bitShift == 0 {
		for i := limbShift; i < 8; i++ {
			tmp[i-limbShift] = x[i]
		}
	} else {
		invShift := 64 - bitShift
		var prev uint64
		for i := 7; i >= int(limbShift); i-- {
			v := x[i]
			tmp[uint(i)-limbShift] = (v >> bitShift) | prev
			prev = v << invShift
		}
	}
	*z = tmp
	return z
}

// SRsh sets z = x >> n with sign extension (x interpreted as signed 512-bit).
func (z *Int) SRsh(x *Int, n uint) *Int {
	if x[7]>>63 == 0 {
		return z.Rsh(x, n)
	}
	if n >= WordBits {
		return z.SetAllOne()
	}
	if n == 0 {
		*z = *x
		return z
	}
	// For negative numbers: rshift normally, then fill the top `n` bits with 1s.
	z.Rsh(x, n)
	// OR in the sign-extended bits.
	bit := uint(WordBits) - n // first bit index (from LSB) that should be 1
	for i := int(bit) / 64; i < 8; i++ {
		var mask uint64
		if i == int(bit)/64 {
			mask = ^uint64(0) << (bit % 64)
		} else {
			mask = ^uint64(0)
		}
		z[i] |= mask
	}
	return z
}

// Byte replaces z with the byte at index n (big-endian) of itself, where index
// 0 is the most significant byte of the 64-byte representation. If n >= 64, z
// is set to 0.
func (z *Int) Byte(n *Int) *Int {
	if !n.IsUint64() || n[0] >= WordBytes {
		z.Clear()
		return z
	}
	idx := n[0]
	// Big-endian index `idx` corresponds to bit position 8*(WordBytes-1-idx)
	// in the little-endian limb layout. Limb = (WordBytes-1-idx)/8 from the
	// bottom, byte within limb = (WordBytes-1-idx)%8 from the bottom.
	be := WordBytes - 1 - idx
	limb := be / 8
	byteInLimb := be % 8
	b := (z[limb] >> (byteInLimb * 8)) & 0xff
	z.SetUint64(b)
	return z
}

// --- internal ---------------------------------------------------------------

// modulus returns a fresh 2^512 big.Int. Allocated per call to avoid shared
// mutable state with callers that mutate the returned value.
func modulus() *big.Int {
	return new(big.Int).Lsh(big.NewInt(1), WordBits)
}

// mask512 returns a fresh (2^512 - 1) big.Int.
func mask512() *big.Int {
	m := modulus()
	m.Sub(m, big.NewInt(1))
	return m
}
