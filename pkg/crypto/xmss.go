package crypto

import (
	"github.com/theQRL/go-qrl/pkg/misc"
	"github.com/theQRL/qrllib/goqrllib/goqrllib"
)

var hashFunctions = map[string] goqrllib.EHashFunction {
	"shake128": goqrllib.SHAKE_128,
	"shake256": goqrllib.SHAKE_256,
	"sha2_256": goqrllib.SHA2_256,
}

var hashFunctionsReverse = map[goqrllib.EHashFunction] string {
	goqrllib.SHAKE_128: "shake128",
	goqrllib.SHAKE_256: "shake256" ,
	goqrllib.SHA2_256: "sha2_256",
}

type XMSSInterface interface {

	FromExtendedSeed([]byte) *XMSSInterface

	FromHeight(treeHeight uint64, hashFunctions string) *XMSSInterface

	HashFunction() string

	SignatureType() goqrllib.ESignatureType

	Height() uint64

	sk() []byte

	pk() []byte

	NumberSignatures() uint

	RemainingSignatures() uint

	Mnemonic() string

	Address() []byte

	QAddress() string

	OTSIndex() uint64

	SetOTSIndex(newIndex uint)

	HexSeed() string

	ExtendedSeed() string

	Seed() string

	Sign(message goqrllib.UcharVector) []byte
}

type XMSS struct {

	xmss goqrllib.XmssFast

}

func (x *XMSS) FromExtendedSeed(extendedSeed goqrllib.UcharVector) *XMSS {
	moddedExtendedSeed := misc.UcharVector{}
	moddedExtendedSeed.New(extendedSeed)
	if extendedSeed.Size() != 51 {
		//RAISE EXCEPTION
	}

	tmp := misc.UcharVector{}
	tmp.AddBytes(moddedExtendedSeed.GetBytes()[0:3])
	descr := goqrllib.QRLDescriptorFromBytes(tmp.GetData())
	if descr.GetSignatureType() != goqrllib.XMSS {
		//RAISE EXCEPTION
	}

	height := descr.GetHeight()
	hashFunction := descr.GetHashFunction()
	tmp = misc.UcharVector{}
	tmp.AddBytes(moddedExtendedSeed.GetBytes()[3:])
	goqrllib.NewXmssFast__SWIG_1(tmp.GetData(), height, hashFunction)

	return x
}

func (x *XMSS) FromHeight(treeHeight uint, hashFunction string) *XMSS {

	if _, ok := hashFunctions[hashFunction]; !ok {
		//RAISE EXCEPTION
	}

	seed := goqrllib.GetRandomSeed(48, "")
	x.xmss = goqrllib.NewXmssFast__SWIG_1(seed, byte(treeHeight), hashFunctions[hashFunction])

	return x
}

func (x *XMSS) HashFunction() string {
	descr := x.xmss.GetDescriptor()
	functionNum := descr.GetHashFunction()
	functionName, ok := hashFunctionsReverse[functionNum]
	if !ok {
		//RAISE EXCEPTION + LOG
	}
	return functionName

}

func (x *XMSS) SignatureType() goqrllib.ESignatureType {
	descr := x.xmss.GetDescriptor()
	answer := descr.GetSignatureType()
	return answer
}

func (x *XMSS) Height() uint64 {
	return x.Height()
}

func (x *XMSS) sk() goqrllib.UcharVector {
	return x.xmss.GetSK()
}

func (x *XMSS) pk() goqrllib.UcharVector {
	return x.xmss.GetPK()
}

func (x *XMSS) NumberSignatures() uint {
	return x.xmss.GetNumberSignatures()
}

func (x *XMSS) RemainingSignatures() uint {
	return x.xmss.GetRemainingSignatures()
}

func (x *XMSS) Mnemonic() string {
	return goqrllib.Bin2mnemonic(x.xmss.GetExtendedSeed())
}

func (x *XMSS) Address() goqrllib.UcharVector {
	return x.xmss.GetAddress()
}

func (x *XMSS) QAddress() string {
	return "Q" + goqrllib.Bin2hstr(x.Address())
}

func (x *XMSS) OTSIndex() uint {
	return x.xmss.GetIndex()
}

func (x *XMSS) SetOTSIndex(newIndex uint) {
	x.xmss.SetIndex(newIndex)
}

func (x *XMSS) HexSeed() string {
	return goqrllib.Bin2hstr(x.xmss.GetExtendedSeed())
}

func (x *XMSS) ExtendedSeed() goqrllib.UcharVector {
	return x.xmss.GetExtendedSeed()
}

func (x *XMSS) Seed() goqrllib.UcharVector {
	return x.xmss.GetSeed()
}

func (x *XMSS) Sign(message goqrllib.UcharVector) []byte {
	msg := misc.UcharVector{}
	msg.New(x.xmss.Sign(message))
	return msg.GetBytes()
}
