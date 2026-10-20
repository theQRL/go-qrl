package pqcrypto

import (
	"testing"

	cryptomldsa87 "github.com/theQRL/go-qrllib/crypto/ml_dsa_87"
	walletmldsa87 "github.com/theQRL/go-qrllib/wallet/ml_dsa_87"
)

// TestMLDSA87VerifySignatureRejectsZeroT1Key checks that transaction signature
// verification refuses a public key whose t1 component is all zero. Such keys
// are universally forgeable (any party can produce a valid signature for any
// message), so they must never authenticate a sender.
func TestMLDSA87VerifySignatureRejectsZeroT1Key(t *testing.T) {
	wallet, err := walletmldsa87.NewWallet()
	if err != nil {
		t.Fatal(err)
	}
	msg := []byte("zero-t1 key test")
	realSig, err := wallet.Sign(msg)
	if err != nil {
		t.Fatal(err)
	}
	realPK := wallet.GetPK()
	ok, err := MLDSA87VerifySignatureWithDefaultDescriptor(realSig[:], msg, realPK[:])
	if err != nil || !ok {
		t.Fatalf("real key round-trip: ok=%v err=%v", ok, err)
	}

	for _, rho := range []byte{0x00, 0xab} {
		pk := make([]byte, cryptomldsa87.CRYPTO_PUBLIC_KEY_BYTES)
		for i := 0; i < cryptomldsa87.SEED_BYTES; i++ {
			pk[i] = rho
		}
		ok, err := MLDSA87VerifySignatureWithDefaultDescriptor(realSig[:], msg, pk)
		if ok {
			t.Fatalf("rho=%#x: zero-t1 public key was accepted", rho)
		}
		if err == nil {
			t.Fatalf("rho=%#x: expected the key constructor to reject a zero-t1 public key", rho)
		}
	}
}
