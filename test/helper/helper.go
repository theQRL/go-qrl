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

func StringAddressToBytesArray(addrs []string) [][]byte {
	bytesAddrs := make([][]byte, len(addrs))

	for i := 0; i < len(addrs); i++ {
		bytesAddrs[i] = misc.Qaddress2Bin(addrs[i])
	}

	return bytesAddrs
}

func StringArrayToBytesArray(data []string) [][]byte {
	bytesData := make([][]byte, len(data))

	for i := 0; i < len(data); i++ {
		bytesData[i] = misc.HStr2Bin(data[i])
	}

	return bytesData
}
