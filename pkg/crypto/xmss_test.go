package crypto

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/theQRL/go-qrl/pkg/misc"
	"github.com/theQRL/qrllib/goqrllib/goqrllib"
)

type TestXMSS struct {
	xmss *XMSS
}

func TestXMSS_FromExtendedSeed(t *testing.T) {
	x := TestXMSS{}
	hexseed := "010400589c3e0bf438bcdc5575c9cb367e9dbb96c7182c9ed5fd6711f832b94d1d5782b345f77d71c1f2f4fa4aa609f040b69f"
	qaddress := "Q010400d9f1efe5b272e042dcc8ef690f0e90ca8b0b6edba0d26f81e7aff12a6754b21788169f7f"

	binSeed := goqrllib.Hstr2bin(hexseed)
	x.xmss = FromExtendedSeed(binSeed)

	assert.Equal(t, x.xmss.QAddress(), qaddress, "%v != %v", x.xmss.QAddress(), qaddress)
}

func TestXMSS_FromHeight(t *testing.T) {
	x := TestXMSS{}
	x.xmss = FromHeight(6, goqrllib.SHAKE_128)

	assert.Equal(t, x.xmss.Height(), uint64(6), "%v != %v", x.xmss.Height(), uint64(6))
	assert.Equal(t, x.xmss.HashFunction(), hashFunctionsReverse[goqrllib.SHAKE_128], "%v != %v", x.xmss.Height(), hashFunctionsReverse[goqrllib.SHAKE_128])
}

func TestXMSS_HashFunction(t *testing.T) {
	x := TestXMSS{}
	hexseed := "010400589c3e0bf438bcdc5575c9cb367e9dbb96c7182c9ed5fd6711f832b94d1d5782b345f77d71c1f2f4fa4aa609f040b69f"
	qaddress := "Q010400d9f1efe5b272e042dcc8ef690f0e90ca8b0b6edba0d26f81e7aff12a6754b21788169f7f"

	binSeed := goqrllib.Hstr2bin(hexseed)
	x.xmss = FromExtendedSeed(binSeed)

	assert.Equal(t, x.xmss.QAddress(), qaddress, "%v != %v", x.xmss.QAddress(), qaddress)
	assert.Equal(t, x.xmss.HashFunction(), hashFunctionsReverse[goqrllib.SHAKE_128], "%v != %v", x.xmss.HashFunction(), hashFunctionsReverse[goqrllib.SHAKE_128])

	x.xmss = FromHeight(6, goqrllib.SHAKE_256)

	assert.Equal(t, x.xmss.Height(), uint64(6), "%v != %v", x.xmss.Height(), uint64(6))
	assert.Equal(t, x.xmss.HashFunction(), hashFunctionsReverse[goqrllib.SHAKE_256], "%v != %v", x.xmss.Height(), hashFunctionsReverse[goqrllib.SHAKE_256])

	x.xmss = FromHeight(6, goqrllib.SHA2_256)

	assert.Equal(t, x.xmss.Height(), uint64(6), "%v != %v", x.xmss.Height(), uint64(6))
	assert.Equal(t, x.xmss.HashFunction(), hashFunctionsReverse[goqrllib.SHA2_256], "%v != %v", x.xmss.Height(), hashFunctionsReverse[goqrllib.SHA2_256])
}

func TestXMSS_SignatureType(t *testing.T) {
	x := TestXMSS{}
	hexseed := "010400589c3e0bf438bcdc5575c9cb367e9dbb96c7182c9ed5fd6711f832b94d1d5782b345f77d71c1f2f4fa4aa609f040b69f"
	qaddress := "Q010400d9f1efe5b272e042dcc8ef690f0e90ca8b0b6edba0d26f81e7aff12a6754b21788169f7f"

	binSeed := goqrllib.Hstr2bin(hexseed)
	x.xmss = FromExtendedSeed(binSeed)

	assert.Equal(t, x.xmss.QAddress(), qaddress, "%v != %v", x.xmss.QAddress(), qaddress)
	assert.Equal(t, x.xmss.SignatureType(), goqrllib.XMSS, "%v != %v", x.xmss.HashFunction(), goqrllib.XMSS)
}

func TestXMSS_Height(t *testing.T) {
	x := TestXMSS{}
	hexseed := "010400589c3e0bf438bcdc5575c9cb367e9dbb96c7182c9ed5fd6711f832b94d1d5782b345f77d71c1f2f4fa4aa609f040b69f"
	qaddress := "Q010400d9f1efe5b272e042dcc8ef690f0e90ca8b0b6edba0d26f81e7aff12a6754b21788169f7f"

	binSeed := goqrllib.Hstr2bin(hexseed)
	x.xmss = FromExtendedSeed(binSeed)

	assert.Equal(t, x.xmss.QAddress(), qaddress, "%v != %v", x.xmss.QAddress(), qaddress)
	assert.Equal(t, x.xmss.Height(), uint64(8), "%v != %v", x.xmss.Height(), uint64(8))
}

func TestXMSS_sk(t *testing.T) {
	x := TestXMSS{}
	hexseed := "010400589c3e0bf438bcdc5575c9cb367e9dbb96c7182c9ed5fd6711f832b94d1d5782b345f77d71c1f2f4fa4aa609f040b69f"
	qaddress := "Q010400d9f1efe5b272e042dcc8ef690f0e90ca8b0b6edba0d26f81e7aff12a6754b21788169f7f"
	expectedSK := []byte {0, 0, 0, 0, 169, 173, 27, 40, 30, 180, 23, 79, 145, 237, 116, 50, 91, 60, 27, 187, 253, 105, 115, 29, 151, 180, 39, 150, 174, 81, 161, 118, 227, 235, 115, 192, 125, 4, 204, 206, 188, 171, 109, 211, 151, 217, 230, 12, 202, 158, 219, 40, 129, 97, 155, 114, 43, 89, 39, 42, 28, 237, 125, 17, 254, 178, 159, 189, 189, 117, 199, 174, 37, 183, 171, 128, 23, 112, 166, 20, 119, 214, 153, 43, 123, 94, 93, 215, 60, 127, 215, 140, 184, 63, 143, 206, 213, 221, 176, 175, 124, 94, 44, 151, 128, 6, 175, 40, 215, 49, 48, 163, 181, 227, 45, 87, 136, 241, 225, 173, 133, 102, 191, 175, 155, 215, 251, 248, 168, 207, 121, 100}

	binSeed := goqrllib.Hstr2bin(hexseed)
	x.xmss = FromExtendedSeed(binSeed)

	assert.Equal(t, x.xmss.QAddress(), qaddress, "%v != %v", x.xmss.QAddress(), qaddress)
	assert.Equal(t, misc.UCharVectorToBytes(x.xmss.sk()), expectedSK, "%v != %v", misc.UCharVectorToBytes(x.xmss.sk()), expectedSK)
}

func TestXMSS_PK(t *testing.T) {
	x := TestXMSS{}
	hexseed := "010400589c3e0bf438bcdc5575c9cb367e9dbb96c7182c9ed5fd6711f832b94d1d5782b345f77d71c1f2f4fa4aa609f040b69f"
	qaddress := "Q010400d9f1efe5b272e042dcc8ef690f0e90ca8b0b6edba0d26f81e7aff12a6754b21788169f7f"
	expectedPK := []byte {1, 4, 0, 124, 94, 44, 151, 128, 6, 175, 40, 215, 49, 48, 163, 181, 227, 45, 87, 136, 241, 225, 173, 133, 102, 191, 175, 155, 215, 251, 248, 168, 207, 121, 100, 189, 117, 199, 174, 37, 183, 171, 128, 23, 112, 166, 20, 119, 214, 153, 43, 123, 94, 93, 215, 60, 127, 215, 140, 184, 63, 143, 206, 213, 221, 176, 175}

	binSeed := goqrllib.Hstr2bin(hexseed)
	x.xmss = FromExtendedSeed(binSeed)

	assert.Equal(t, x.xmss.QAddress(), qaddress, "%v != %v", x.xmss.QAddress(), qaddress)
	assert.Equal(t, misc.UCharVectorToBytes(x.xmss.PK()), expectedPK, "%v != %v", misc.UCharVectorToBytes(x.xmss.PK()), expectedPK)
}

func TestXMSS_NumberSignatures(t *testing.T) {
	x := TestXMSS{}
	hexseed := "010400589c3e0bf438bcdc5575c9cb367e9dbb96c7182c9ed5fd6711f832b94d1d5782b345f77d71c1f2f4fa4aa609f040b69f"
	qaddress := "Q010400d9f1efe5b272e042dcc8ef690f0e90ca8b0b6edba0d26f81e7aff12a6754b21788169f7f"
	expectedNumberSigs := uint64(math.Pow(2, 8))

	binSeed := goqrllib.Hstr2bin(hexseed)
	x.xmss = FromExtendedSeed(binSeed)

	assert.Equal(t, x.xmss.QAddress(), qaddress, "%v != %v", x.xmss.QAddress(), qaddress)
	assert.Equal(t, x.xmss.NumberSignatures(), expectedNumberSigs, "%v != %v", x.xmss.NumberSignatures(), expectedNumberSigs)
}

func TestXMSS_RemainingSignatures(t *testing.T) {
	x := TestXMSS{}
	hexseed := "010400589c3e0bf438bcdc5575c9cb367e9dbb96c7182c9ed5fd6711f832b94d1d5782b345f77d71c1f2f4fa4aa609f040b69f"
	qaddress := "Q010400d9f1efe5b272e042dcc8ef690f0e90ca8b0b6edba0d26f81e7aff12a6754b21788169f7f"
	expectedRemainingSigs := uint64(math.Pow(2, 8))

	binSeed := goqrllib.Hstr2bin(hexseed)
	x.xmss = FromExtendedSeed(binSeed)

	assert.Equal(t, x.xmss.QAddress(), qaddress, "%v != %v", x.xmss.QAddress(), qaddress)
	assert.Equal(t, x.xmss.RemainingSignatures(), expectedRemainingSigs, "%v != %v", x.xmss.RemainingSignatures(), expectedRemainingSigs)

	x.xmss.Sign(misc.NewUCharVector().GetData())

	assert.Equal(t, x.xmss.RemainingSignatures(), expectedRemainingSigs - 1, "%v != %v", x.xmss.RemainingSignatures(), expectedRemainingSigs - 1)
}

func TestXMSS_Mnemonic(t *testing.T) {
	x := TestXMSS{}
	hexseed := "010400589c3e0bf438bcdc5575c9cb367e9dbb96c7182c9ed5fd6711f832b94d1d5782b345f77d71c1f2f4fa4aa609f040b69f"
	qaddress := "Q010400d9f1efe5b272e042dcc8ef690f0e90ca8b0b6edba0d26f81e7aff12a6754b21788169f7f"
	mnemonic := "absorb drank fruity set aside earth sacred she junior over daisy trend rude huge blew size stem sticky baron lowest robert spicy friar clear elude knack invoke buggy volume pit plead paris drift highly"

	binSeed := goqrllib.Hstr2bin(hexseed)
	x.xmss = FromExtendedSeed(binSeed)

	assert.Equal(t, x.xmss.QAddress(), qaddress, "%v != %v", x.xmss.QAddress(), qaddress)
	assert.Equal(t, x.xmss.Mnemonic(), mnemonic, "%v != %v", x.xmss.Mnemonic(), mnemonic)
}

func TestXMSS_Address(t *testing.T) {
	x := TestXMSS{}
	hexseed := "010400589c3e0bf438bcdc5575c9cb367e9dbb96c7182c9ed5fd6711f832b94d1d5782b345f77d71c1f2f4fa4aa609f040b69f"
	qaddress := "Q010400d9f1efe5b272e042dcc8ef690f0e90ca8b0b6edba0d26f81e7aff12a6754b21788169f7f"
	address := []byte {1, 4, 0, 217, 241, 239, 229, 178, 114, 224, 66, 220, 200, 239, 105, 15, 14, 144, 202, 139, 11, 110, 219, 160, 210, 111, 129, 231, 175, 241, 42, 103, 84, 178, 23, 136, 22, 159, 127}

	binSeed := goqrllib.Hstr2bin(hexseed)
	x.xmss = FromExtendedSeed(binSeed)

	assert.Equal(t, x.xmss.QAddress(), qaddress, "%v != %v", x.xmss.QAddress(), qaddress)
	assert.Equal(t, misc.UCharVectorToBytes(x.xmss.Address()), address, "%v != %v", misc.UCharVectorToBytes(x.xmss.Address()), address)
}

func TestXMSS_QAddress(t *testing.T) {
	x := TestXMSS{}
	hexseed := "010400589c3e0bf438bcdc5575c9cb367e9dbb96c7182c9ed5fd6711f832b94d1d5782b345f77d71c1f2f4fa4aa609f040b69f"
	qaddress := "Q010400d9f1efe5b272e042dcc8ef690f0e90ca8b0b6edba0d26f81e7aff12a6754b21788169f7f"

	binSeed := goqrllib.Hstr2bin(hexseed)
	x.xmss = FromExtendedSeed(binSeed)

	assert.Equal(t, x.xmss.QAddress(), qaddress, "%v != %v", x.xmss.QAddress(), qaddress)
}

func TestXMSS_OTSIndex(t *testing.T) {
	x := TestXMSS{}
	hexseed := "010400589c3e0bf438bcdc5575c9cb367e9dbb96c7182c9ed5fd6711f832b94d1d5782b345f77d71c1f2f4fa4aa609f040b69f"
	qaddress := "Q010400d9f1efe5b272e042dcc8ef690f0e90ca8b0b6edba0d26f81e7aff12a6754b21788169f7f"
	expectedOTSIndex := uint64(0)

	binSeed := goqrllib.Hstr2bin(hexseed)
	x.xmss = FromExtendedSeed(binSeed)

	assert.Equal(t, x.xmss.QAddress(), qaddress, "%v != %v", x.xmss.QAddress(), qaddress)
	assert.Equal(t, x.xmss.OTSIndex(), expectedOTSIndex, "%v != %v", x.xmss.OTSIndex(), expectedOTSIndex)

	x.xmss.Sign(misc.NewUCharVector().GetData())

	assert.Equal(t, x.xmss.OTSIndex(), expectedOTSIndex + 1, "%v != %v", x.xmss.OTSIndex(), expectedOTSIndex + 1)
}

func TestXMSS_SetOTSIndex(t *testing.T) {
	x := TestXMSS{}
	hexseed := "010400589c3e0bf438bcdc5575c9cb367e9dbb96c7182c9ed5fd6711f832b94d1d5782b345f77d71c1f2f4fa4aa609f040b69f"
	qaddress := "Q010400d9f1efe5b272e042dcc8ef690f0e90ca8b0b6edba0d26f81e7aff12a6754b21788169f7f"

	binSeed := goqrllib.Hstr2bin(hexseed)
	x.xmss = FromExtendedSeed(binSeed)

	assert.Equal(t, x.xmss.QAddress(), qaddress, "%v != %v", x.xmss.QAddress(), qaddress)
	assert.Equal(t, x.xmss.OTSIndex(), uint64(0), "%v != %v", x.xmss.OTSIndex(), uint64(0))

	x.xmss.SetOTSIndex(55)
	x.xmss.Sign(misc.NewUCharVector().GetData())

	assert.Equal(t, x.xmss.OTSIndex(), uint64(56), "%v != %v", x.xmss.OTSIndex(), uint64(56))

	x.xmss.SetOTSIndex(60)

	assert.Equal(t, x.xmss.OTSIndex(), uint64(60), "%v != %v", x.xmss.OTSIndex(), uint64(60))
}

func TestXMSS_HexSeed(t *testing.T) {
	x := TestXMSS{}
	hexseed := "010400589c3e0bf438bcdc5575c9cb367e9dbb96c7182c9ed5fd6711f832b94d1d5782b345f77d71c1f2f4fa4aa609f040b69f"
	qaddress := "Q010400d9f1efe5b272e042dcc8ef690f0e90ca8b0b6edba0d26f81e7aff12a6754b21788169f7f"

	binSeed := goqrllib.Hstr2bin(hexseed)
	x.xmss = FromExtendedSeed(binSeed)

	assert.Equal(t, x.xmss.QAddress(), qaddress, "%v != %v", x.xmss.QAddress(), qaddress)
	assert.Equal(t, x.xmss.HexSeed(), hexseed, "%v != %v", x.xmss.HexSeed(), hexseed)
}

func TestXMSS_ExtendedSeed(t *testing.T) {
	x := TestXMSS{}
	hexseed := "010400589c3e0bf438bcdc5575c9cb367e9dbb96c7182c9ed5fd6711f832b94d1d5782b345f77d71c1f2f4fa4aa609f040b69f"
	qaddress := "Q010400d9f1efe5b272e042dcc8ef690f0e90ca8b0b6edba0d26f81e7aff12a6754b21788169f7f"

	binSeed := goqrllib.Hstr2bin(hexseed)
	x.xmss = FromExtendedSeed(binSeed)

	assert.Equal(t, x.xmss.QAddress(), qaddress, "%v != %v", x.xmss.QAddress(), qaddress)
	assert.Equal(t, goqrllib.Bin2hstr(x.xmss.ExtendedSeed()), hexseed, "%v != %v", goqrllib.Bin2hstr(x.xmss.ExtendedSeed()), hexseed)
}

func TestXMSS_Seed(t *testing.T) {
	x := TestXMSS{}
	hexseed := "010400589c3e0bf438bcdc5575c9cb367e9dbb96c7182c9ed5fd6711f832b94d1d5782b345f77d71c1f2f4fa4aa609f040b69f"
	qaddress := "Q010400d9f1efe5b272e042dcc8ef690f0e90ca8b0b6edba0d26f81e7aff12a6754b21788169f7f"
	seed := "589c3e0bf438bcdc5575c9cb367e9dbb96c7182c9ed5fd6711f832b94d1d5782b345f77d71c1f2f4fa4aa609f040b69f"

	binSeed := goqrllib.Hstr2bin(hexseed)
	x.xmss = FromExtendedSeed(binSeed)

	assert.Equal(t, x.xmss.QAddress(), qaddress, "%v != %v", x.xmss.QAddress(), qaddress)
	assert.Equal(t, goqrllib.Bin2hstr(x.xmss.Seed()), seed, "%v != %v", goqrllib.Bin2hstr(x.xmss.Seed()), seed)
}

func TestXMSS_Sign(t *testing.T) {
	x := TestXMSS{}
	hexseed := "010400589c3e0bf438bcdc5575c9cb367e9dbb96c7182c9ed5fd6711f832b94d1d5782b345f77d71c1f2f4fa4aa609f040b69f"
	qaddress := "Q010400d9f1efe5b272e042dcc8ef690f0e90ca8b0b6edba0d26f81e7aff12a6754b21788169f7f"
	expectedRemainingSigs := uint64(math.Pow(2, 8))
	expectedHexSignature := "000000003beecbd115bcd9afe1d57f4f5732cd878dd5df00537f62b5dc2eed73abc8d1c752f198aa09794f697afd6f2f6c78305c3cec0323ff66785099d65191ebdf53974c3324f00c44fb8a0ab70dbe749544e7f3091cfea9dca66360e47bb0218a7714dc7619da033418b89ace44940772071f2e541ff1df03ae957c04e35afed6219157c629b943da7feaa4b9ca6e93e8f1506b2889b495c50c07863a423499d76e84b34f30c2df473b418c4d3ca43357021348b6e790010546dbe421b86a3c9652c6702270ba11e2bc2848dfa33227a2be8eaf99d0d6db087cdcd7211be2cdc5fb126e0f71ff10308cd866c1df5d1bd69a8cfa79501087d82776bc72d4e6f3319fbe40b48c19cfb4c5c59b4c1a8ad0031291a4ab50808993c324e94bc26a35b14793741cc428e1d0c81e1c7cdd1b626107b35fd16a6f56740f5c757f674bc0db2346e37d0e8680e62095ff8ccea92d84a9aa741a2f76747f44d7a1088e0ee0e0ad3392a9d790ac1d475dfdc99972915954a711ba8ab5832bf1de01210e770c4a2e41f3973ba53ba937780453b98277acfbdcf64862a3044539f4be685bc78c0f2c8c9e249322f96ef1c713cfa72a24dc2f416090bfe7ee3eca0b0621362448ec27bc6ba2f5613415a601972590482a3f9cb3e7d47c37a494ae7cb8b90f4d0bd8c579659c2a2a9381f88e2830201f93442c4655f6234a890969fd28e884cf7e4766bde7a09bedadf9fcb20b333093a20d88f9f239d21cc2fa04210a295324ea5ddcbc343dcbf19e6f1e514310c4aa6d9eda36eb9703fc8e974ebd78f83b1d3f87a035790c065571fcc58bfdb3b4a33b07d96c4179b42d73350d55bafb518111a6a06dac5c84b9eeae8ec50b61c8fb0d207c174bb9948fe6e1bfc30507c45323dc6be3ccc7d2dfd9130f47b364d54c997a7dee8f5a0fa6862926480ff15039f8de21559566ed46db4e33c43429b656d39ae2c45e5c102d54366599c897bc1029e9e336bf8ad85b16cedb12da734f28bdbf85e736f7a4176812f7b3c24f800ba1fa23df8fdc6eb59510286767bc507215ecf0563827930dcc5053a84ef4abbef4fe72dd56c2824216d7caab8b4387f3d568f1cf3f0904113c738e6df2d2c15182f27634acbad53dbf3a0389e27a99b6f294eb0a736e09ca39d5a18586142cedb666611719bb859121bd537abd9eaeca0e61bf67f26d2a18b3edd717e3ca6d0873a9da3987482affed2314f69c3e631a91c5965d781f2606d146c6aa1a735f640cf607c454848249039ea39364fab3c5e0c05278fec9fb84641efae5c8110a6bc3e8877e2bcf42b967211634af87e7d831cc5b5ba93abfe9c44f5366cffd1166cbc0c460b92048ec2e675d4021053fc744cf660b06447c200de597321e3941579454cede09c4bcc810bf0edc014212566b4689159eb95e9486b308f78d1b923c4902c620f175ae39290bc687fc0ca547e666db7b617a52748097e352d8a02325040dc1a4e52b59be9c60bab134fcf61690f24ab1155e3b2cc6b740ce9d0367458b99dace7714c96282055f850defb3777d0ee4f2bcc831829cbf0cd818a62596126ec1804343d84d4d30af459d1b6292b01929e3534680afaa9a9f7f49dbbae296000e5dfc3a9867d1dc6c52a6e13b223e32c67c6cd34615b6dccc5885452cbd064f65c73e9bf2d737267b3e7d481c66e3e43c3dcbae032c0b018b757c8a3ef18cb1e6e3e38085a1d6131c2ce82542771b5f2dac4e092d499afe1250a3b3643b8f3810691ff410b1e853adb618da9a30e2cd01781fedfd30659dfb2b41bcba7947c7be1c634d402accd029230bea616cebfe1758f471468d3dc2733a44d514481948b146da2d2f07000a0dbcdc247edd700c5ddc78265d0bfff1f95b54a5472238291e5fce605015fe9a15cb9d9c7f988bf69f4cf1d5e18257fdaec3b4a0c0a93e0429d6efe715939d84e427ba6adb24bcc679563b76d6a46d1f15da067f5ffc513f860cb1e239e366ada5abf2261d5e10ab4ef4477064e8711312ec0fbc5ade05d90fc12b95d5222a8243460fd7c2068507c15f9f3b86a23070283d630ed74d756fbe27fa9195b86a204c69feee51537107d4efad8cb277446eec4e9c153fbb8e9a98c1478e3d835140104af7d8436e4d1cc2de5ced7ea73db71a47216588139b3198aeedfc3369ac480b458eb10984a8354de6f04ad109b851ef08c0f000a8a4a5f2e2845888aa8adac4346565668e175372559308fc62ef2951301272a854b3e5aa785b23d509bbb03c90eeeac5580fbe93ab063328652182cdc5f66dc4a9b383e14d200215da033f4293f8bbbd151425657cc893e0f56d5e5db7dbba23e00c94fe685b7d4d16f2828bbd5582f92aa7f0f86f615d884f7ce05d7bfc2f46f952f7f63ed768a0252297ce394cdbe3a04e82b60bfe5d52949363b7c20f53bf8f9795a6d282c649e27c5999e6a383addb5cd66da69cedd6d46ef1d68d8536717011a16069062240ce58a35317beffe11fcad8cb483781c4c3f4fa5d738fcfa94a16c16c384e7d2594411a01cf3ea5fdced1b8ee319168a90f0ef28908a5674bc4b02740c079755768fa59ddb2fa016119cf258b3cefd8cf887d0a81f531e02bdc9ad5a015f2c12555fec903f46aa25b12f63f99d88102e4cf06bbdecc2bf9a3aa4641a1457b86bc14cc81dc693bc8432407a65799441e49b2fa2717b5fd28f6e56a7c46ff92ddb0a61411865a6f3947cf25b43a7335d3edc282a2308b5efcfc3d5be66589c395b2d31ff652077bb122c2dc5c89f52bff60b41c04fcf9c481f0ec718122a97102fc893ff67fa7fecf9176fd446b4e51d217e989e464b85b8dd032790ff6c20ac530fefc903b9eac9f04d85ac2c82aab1db55b70f509a12c58ec6a76dbf0cf96ffa7cc2aacbce4dc9392e9b180345b9cb492de8bf4f9a3bf61639151ea79fc51c4c95ac06bfaeaf1e0d99e4d2f94f49319449bc9725d42dee8ee71db4ddb0d3d9dba53a765596ee3498c1ab17e06a7b4f70479b93f2b5f4ea0d0d6676e10f4fd5def82361161de30b5e595bcfe8e7868d0c993236c17944c81a6d0e5426a7dc4c17530504ef77fddbd561c837268ee8568c9f136333f0cc186fab02e51a3dc0791675cae294e7442705d22306d42e5ceedfe04ce5259dcd9f3488a9cfab938838644fe134d3b61bbbf05923ddbd7bb4d0864780cfaa3240a2e6d02ab1028eaf0d395e333332c38db2ae41f75f981faf5df76ce85fbffbc352e0251b175ea72609e6f3f792d651df43c06f27fc47026a873383f5a81c9bc914b62be1bc7ab4ce8013123bf09441b7a807efbb1d939c7554b0f8c6af365fbea876af463ba1592aaf9420315c79f2ea65bca2b912a6e6764672cc5f3150dcc0da2ef36f6b5d7b21a2b7e22c393f68b3ea5685789a608aacec31274fa8cbeb38c90cc579031ac05"

	binSeed := goqrllib.Hstr2bin(hexseed)
	x.xmss = FromExtendedSeed(binSeed)

	assert.Equal(t, x.xmss.QAddress(), qaddress, "%v != %v", x.xmss.QAddress(), qaddress)
	assert.Equal(t, x.xmss.RemainingSignatures(), expectedRemainingSigs, "%v != %v", x.xmss.RemainingSignatures(), expectedRemainingSigs)

	signature := x.xmss.Sign(misc.NewUCharVector().GetData())
	hexSignature := goqrllib.Bin2hstr(misc.BytesToUCharVector(signature))
	assert.Equal(t, hexSignature, expectedHexSignature, "%v != %v", hexSignature, expectedHexSignature)

	assert.Equal(t, x.xmss.RemainingSignatures(), expectedRemainingSigs - 1, "%v != %v", x.xmss.RemainingSignatures(), expectedRemainingSigs - 1)

}
