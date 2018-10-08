package helper

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
