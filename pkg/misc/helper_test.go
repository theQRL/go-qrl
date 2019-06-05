package misc

import (
	"container/list"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestMerkleTXHash(t *testing.T) {
	var hashes list.List
	hash1 := []byte{48}
	hash2 := []byte{49}
	hashes.PushBack(hash1)
	hashes.PushBack(hash2)
	hashes.PushBack([]byte{50})
	result := MerkleTXHash(hashes)

	assert.Equal(t, Bin2HStr(result), "22073806c4a9967bed132107933c5ec151d602847274f6b911d0086c2a41adc0")
}

func TestBytesToString(t *testing.T) {
	stringData := "Hello"
	bytesData := []byte(stringData)

	result := BytesToString(bytesData)
	assert.Equal(t, stringData, result)
}

func TestStringAddressToBytesArray(t *testing.T) {
	addrs := []string{
		"Q010300a1da274e68c88b0ccf448e0b1916fa789b01eb2ed4e9ad565ce264c9390782a9c61ac02f",
		"Q0103001d65d7e59aed5efbeae64246e0f3184d7c42411421eb385ba30f2c1c005a85ebc4419cfd"}

	bytesAddrs := StringAddressToBytesArray(addrs)
	assert.Len(t, bytesAddrs, len(addrs))

	for i := 0; i < len(bytesAddrs); i++ {
		assert.Equal(t, Bin2Qaddress(bytesAddrs[i]), addrs[i])
	}
}
