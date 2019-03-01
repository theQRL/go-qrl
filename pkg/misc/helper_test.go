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