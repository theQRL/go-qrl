package pow

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

type QryptonightTest struct {
	q *Qryptonight
}

func CreateQryptonightTest() *QryptonightTest {
	return &QryptonightTest{
		GetQryptonight(),
	}
}

func TestGetQryptonight(t *testing.T) {
	q := GetQryptonight()
	assert.NotNil(t, q.qn)
}

func TestQryptonight_Hash(t *testing.T) {
	q := CreateQryptonightTest()
	input := []byte{0x03, 0x05, 0x07, 0x09, 0x03, 0x05, 0x07, 0x09, 0x03, 0x05, 0x07, 0x09, 0x03, 0x05, 0x07, 0x09,
		0x03, 0x05, 0x07, 0x09, 0x03, 0x05, 0x07, 0x09, 0x03, 0x05, 0x07, 0x09, 0x03, 0x05, 0x07, 0x09,
		0x03, 0x05, 0x07, 0x09, 0x03, 0x05, 0x07, 0x09, 0x03, 0x05, 0x07}
	output := q.q.Hash(input)
	outputExpected := []byte{
		0x96, 0x76, 0x53, 0x26, 0x2F, 0x9B, 0x15, 0x90,
		0xB9, 0x88, 0x0F, 0x64, 0xA3, 0x80, 0x8C, 0x4B,
		0x01, 0xEA, 0x29, 0x2C, 0x48, 0xFC, 0x7C, 0x47,
		0x0D, 0x25, 0x50, 0x00, 0x57, 0xCA, 0x07, 0x70,
		}
	assert.Equal(t, output, outputExpected)
}

func TestQryptonight_Hash2(t *testing.T) {
	q := CreateQryptonightTest()
	input := make([]byte, 0)
	for i := 0; i < 30; i++ {
		input = append(input, []byte{0x03, 0x05, 0x07, 0x09}...)
	}
	output := q.q.Hash(input)
	outputExpected := []byte{
		255, 253, 83, 78, 29, 24, 160, 224, 30, 240, 158,
		39, 233, 125, 90, 170, 78, 59, 157, 146, 97, 86,
		205, 161, 160, 155, 48, 144, 51, 148, 155, 99,
	}
	assert.Equal(t, output, outputExpected)
}

func TestQryptonight_Hash3(t *testing.T) {
	q := CreateQryptonightTest()
	input := make([]byte, 0)
	for i := 0; i < 200; i++ {
		input = append(input, []byte{0x03, 0x05, 0x07, 0x09}...)
	}
	output := q.q.Hash(input)
	outputExpected := []byte{
		216, 31, 227, 138, 9, 118, 5, 200, 136,
		40, 156, 168, 86, 35, 146, 223, 199, 76,
		188, 213, 25, 117, 247, 195, 15, 183, 236,
		219, 104, 212, 75, 211,
	}
	assert.Equal(t, output, outputExpected)
}
