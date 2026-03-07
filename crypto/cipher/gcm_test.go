package cipher

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/theQRL/go-qrl/common"
)

func TestEncryptDecryptGCM(t *testing.T) {
	// Tests taken from https://github.com/paulmillr/noble-ciphers/blob/main/test/vectors/wycheproof/aes_gcm_test.json
	var testCases = []struct {
		key        []byte
		iv         []byte
		plainText  []byte
		cipherText []byte
	}{
		{
			common.FromHex("80ba3192c803ce965ea371d5ff073cf0f43b6a2ab576b208426e11409c09b9b0"),
			common.FromHex("4da5bf8dfd5852c1ea12379d"),
			common.FromHex(""),
			common.FromHex("4771a7c404a472966cea8f73c8bfe17a"),
		},
		{
			common.FromHex("cc56b680552eb75008f5484b4cb803fa5063ebd6eab91f6ab6aef4916a766273"),
			common.FromHex("99e23ec48985bccdeeab60f1"),
			common.FromHex("2a"),
			common.FromHex("06633c1e9703ef744ffffb40edf9d14355"),
		},
		{
			common.FromHex("51e4bf2bad92b7aff1a4bc05550ba81df4b96fabf41c12c7b00e60e48db7e152"),
			common.FromHex("4f07afedfdc3b6c2361823d3"),
			common.FromHex("be3308f72a2c6aed"),
			common.FromHex("cf332a12fdee800b602e8d7c4799d62c140c9bb834876b09"),
		},
		{
			common.FromHex("67119627bd988eda906219e08c0d0d779a07d208ce8a4fe0709af755eeec6dcb"),
			common.FromHex("68ab7fdbf61901dad461d23c"),
			common.FromHex("51f8c1f731ea14acdb210a6d973e07"),
			common.FromHex("43fc101bff4b32bfadd3daf57a590eec04aacb7148a8b8be44cb7eaf4efa69"),
		},
	}

	for _, tc := range testCases {
		require := require.New(t)
		t.Run("", func(t *testing.T) {
			ciphertext, err := EncryptGCM(nil, tc.key, tc.iv, tc.plainText, nil)
			require.NoError(err)
			require.Equal(tc.cipherText, ciphertext)

			plainText, err := DecryptGCM(tc.key, tc.iv, tc.cipherText, nil)
			require.NoError(err)
			require.Equal(tc.plainText, plainText)
		})
	}
}
