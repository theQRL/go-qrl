package helper

import (
	"github.com/theQRL/go-qrl/pkg/crypto"
	"github.com/theQRL/go-qrl/pkg/misc"
	"github.com/theQRL/qrllib/goqrllib/goqrllib"
)

func GetAliceXMSS(height uint64) *crypto.XMSS {
	seed := make([]byte, 48)

	for i := 0; i < len(seed); i++ {
		seed[i] = byte(i)
	}

	return crypto.NewXMSS(goqrllib.NewXmssFast__SWIG_2(misc.BytesToUCharVector(seed), byte(height)))
}

func GetBobXMSS(height uint64) *crypto.XMSS {
	seed := make([]byte, 48)

	for i := 0; i < len(seed); i++ {
		seed[i] = byte(i + 5)
	}

	return crypto.NewXMSS(goqrllib.NewXmssFast__SWIG_2(misc.BytesToUCharVector(seed), byte(height)))
}

func StringArrayToBytesArray(data []string) [][]byte {
	bytesData := make([][]byte, len(data))

	for i := 0; i < len(data); i++ {
		bytesData[i] = misc.HStr2Bin(data[i])
	}

	return bytesData
}

func BytesTo32Bytes(data []byte) []byte {
	targetSize := 32
	diff := targetSize - len(data)
	if diff > 0 {
		tmp := make([]byte, diff)
		data = append(tmp, data...)
	}
	return data
}