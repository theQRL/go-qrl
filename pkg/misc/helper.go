package misc

import (
	"bytes"
	"container/list"
	"encoding/binary"
	"fmt"
	"math"
	"runtime"

	"github.com/theQRL/qrllib/goqrllib/goqrllib"
	"github.com/theQRL/qrllib/tests/golang/misc"
)

type UcharVector struct {
	data goqrllib.UcharVector
}

func NewUCharVector() *UcharVector {
	u := &UcharVector{}
	u.data = goqrllib.NewUcharVector()

	// Finalizer to clean up memory allocated by C++ when object becomes unreachable
	runtime.SetFinalizer(u,
		func(u *UcharVector) {
			goqrllib.DeleteUcharVector(u.data)
		})
	return u
}

func (v *UcharVector) AddBytes(data []byte) {
	for _, element := range data {
		v.data.Add(element)
	}
}

func (v *UcharVector) AddByte(data byte) {
	v.data.Add(data)
}

func (v *UcharVector) GetBytesBuffer() bytes.Buffer {
	var data bytes.Buffer
	for i := int64(0); i < v.data.Size(); i++ {
		value := v.data.Get(int(i))
		data.WriteByte(value)
	}
	return data
}

func (v *UcharVector) GetBytes() []byte {
	data := v.GetBytesBuffer()
	return data.Bytes()
}

func (v *UcharVector) GetString() string {
	data := v.GetBytesBuffer()
	return data.String()
}

func (v *UcharVector) GetData() goqrllib.UcharVector {
	return v.data
}

func (v *UcharVector) AddAt() goqrllib.UcharVector {
	return v.data
}

func (v *UcharVector) New(data goqrllib.UcharVector) {
	v.data = data
}

func BytesToUCharVector(data []byte) goqrllib.UcharVector {
	vector := goqrllib.NewUcharVector__SWIG_0()
	for _, element := range data {
		vector.Add(element)
	}

	return vector
}

func Int64ToUCharVector(data int64) goqrllib.UcharVector {
	return goqrllib.NewUcharVector__SWIG_1(data)
}

func UCharVectorToBytes(data goqrllib.UcharVector) []byte {
	vector := UcharVector{}
	vector.New(data)

	return vector.GetBytes()
}

func UCharVectorToString(data goqrllib.UcharVector) string {
	return string(UCharVectorToBytes(data))
}

func MerkleTXHash(hashes list.List) []byte {
	j := int(math.Ceil(math.Log2(float64(hashes.Len()))))

	lArray := hashes
	for x := 0; x < j; x++ {
		var nextLayer list.List
		h := lArray
		i := h.Len()%2 + h.Len()/2
		e := h.Front()
		z := 0

		for k := 0; k < i; k++ {
			if h.Len() == z+1 {
				nextLayer.PushBack(e.Value.([]byte))
			} else {
				tmp := NewUCharVector()
				tmp.AddBytes(e.Value.([]byte))
				e = e.Next()
				tmp.AddBytes(e.Value.([]byte))
				e = e.Next()
				nextLayer.PushBack(UCharVectorToBytes(goqrllib.Sha2_256(tmp.GetData())))
			}
			z += 2
		}
		lArray = nextLayer
	}
	return lArray.Back().Value.([]byte)
}

func Reverse(s [][]byte) [][]byte {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}

	return s
}

func Bin2HStr(data []byte) string {
	return goqrllib.Bin2hstr(BytesToUCharVector(data))
}

func HStr2Bin(data string) []byte {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in f")
		}
	}()
	return UCharVectorToBytes(goqrllib.Hstr2bin(data))
}

func Qaddress2Bin(qaddress string) []byte {
	return HStr2Bin(qaddress[1:])
}

func Bin2Qaddress(binAddress []byte) string {
	return "Q" + Bin2HStr(binAddress)
}

func Bin2QAddresses(binAddresses [][]byte) []string {
	QAddresses := make([]string, 0)
	for i := 0 ; i < len(binAddresses); i++ {
		QAddresses = append(QAddresses, Bin2Qaddress(binAddresses[i]))
	}
	return QAddresses
}

func Bin2Pks(binPks [][]byte) []string {
	pks := make([]string, 0)
	for i := 0 ; i < len(binPks); i++ {
		pks = append(pks, Bin2HStr(binPks[i]))
	}
	return pks
}

func PK2BinAddress(pk []byte) []byte {
	return misc.UCharVectorToBytes(goqrllib.QRLHelperGetAddress(misc.BytesToUCharVector(pk)))
}

func PK2Qaddress(pk []byte) string {
	return Bin2Qaddress(PK2BinAddress(pk))
}

func ConvertBytesToLong(b []byte) uint32 {
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
}

func OTSKeyFromSig(signature []byte) uint64 {
	return uint64(binary.BigEndian.Uint32(signature[0:4]))
}

func BytesToString(data []byte) string {
	buff := bytes.NewBuffer(data)
	return buff.String()
}

func Sha256(message string, length int64) []byte {
	return misc.UCharVectorToBytes(goqrllib.Sha2_256_n(misc.BytesToUCharVector([]byte(message)), length))
}
