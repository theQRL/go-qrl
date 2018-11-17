package helper

import (
	"github.com/stretchr/testify/assert"
	"github.com/theQRL/go-qrl/pkg/misc"
	"testing"
)

func TestGetAliceXMSS(t *testing.T) {
	qaddress := "Q010300a1da274e68c88b0ccf448e0b1916fa789b01eb2ed4e9ad565ce264c9390782a9c61ac02f"
	xmss := GetAliceXMSS(6)

	assert.NotNil(t, xmss)
	assert.Equal(t, xmss.QAddress(), qaddress)
}

func TestGetBobXMSS(t *testing.T) {
	qaddress := "Q0103001d65d7e59aed5efbeae64246e0f3184d7c42411421eb385ba30f2c1c005a85ebc4419cfd"
	xmss := GetBobXMSS(6)

	assert.NotNil(t, xmss)
	assert.Equal(t, xmss.QAddress(), qaddress)
}

func TestStringAddressToBytesArray(t *testing.T) {
	addrs := []string{
		"Q010300a1da274e68c88b0ccf448e0b1916fa789b01eb2ed4e9ad565ce264c9390782a9c61ac02f",
		"Q0103001d65d7e59aed5efbeae64246e0f3184d7c42411421eb385ba30f2c1c005a85ebc4419cfd"}

	bytesAddrs := StringAddressToBytesArray(addrs)
	assert.Len(t, bytesAddrs, len(addrs))

	for i := 0; i < len(bytesAddrs); i++ {
		assert.Equal(t, misc.Bin2Qaddress(bytesAddrs[i]), addrs[i])
	}
}

func TestStringArrayToBytesArray(t *testing.T) {
	aliceXMSS := GetAliceXMSS(6)
	bobXMSS := GetBobXMSS(6)
	data := []string{misc.Bin2HStr(misc.UCharVectorToBytes(aliceXMSS.PK())),
		misc.Bin2HStr(misc.UCharVectorToBytes(bobXMSS.PK()))}

	bytesArray := StringArrayToBytesArray(data)
	assert.Len(t, bytesArray, len(data))

	for i := 0; i < len(bytesArray); i++ {
		assert.Equal(t, bytesArray[i], misc.HStr2Bin(data[i]))
	}
}
