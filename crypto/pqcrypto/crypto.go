package pqcrypto

import (
	"errors"
	"fmt"

	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/crypto/pqcrypto/wallet"
	cryptomldsa87 "github.com/theQRL/go-qrllib/crypto/ml_dsa_87"
	libwallet "github.com/theQRL/go-qrllib/wallet"
	"github.com/theQRL/go-qrllib/wallet/common/descriptor"
	walletmldsa87 "github.com/theQRL/go-qrllib/wallet/ml_dsa_87"
)

const (
	MLDSA87SignatureLength = cryptomldsa87.CRYPTO_BYTES
	MLDSA87PublicKeyLength = cryptomldsa87.CRYPTO_PUBLIC_KEY_BYTES
	DescriptorSize         = descriptor.DescriptorSize

	// DigestLength sets the signature digest exact length
	DigestLength = 32
)

var ErrBadSignature = errors.New("invalid ML-DSA-87 signature")

func BytesToDescriptor(b []byte) (descriptor.Descriptor, error) {
	return descriptor.FromBytes(b)
}

func PublicKeyAndDescriptorToAddress(pk []byte, d descriptor.Descriptor) (common.Address, error) {
	addrBytes, err := libwallet.GetAddressFromPKAndDescriptor(pk, d)
	if err != nil {
		return common.Address{}, err
	}
	var addr common.Address
	copy(addr[:], addrBytes[:])
	return addr, nil
}

func MLDSA87VerifySignature(sig []byte, msg []byte, pk []byte, desc descriptor.Descriptor) (bool, error) {
	// walletmldsa87.Verify would panic on bad length
	if l := len(sig); l != cryptomldsa87.CRYPTO_BYTES {
		return false, fmt.Errorf("%w: bad length", ErrBadSignature)
	}

	pk87, err := walletmldsa87.BytesToPK(pk)
	if err != nil {
		return false, err
	}

	mlDesc, err := walletmldsa87.NewMLDSA87DescriptorFromDescriptor(desc)
	if err != nil {
		return false, err
	}

	return walletmldsa87.Verify(msg, sig, &pk87, mlDesc), nil
}

func MLDSA87VerifySignatureWithDefaultDescriptor(sig []byte, msg []byte, pk []byte) (bool, error) {
	desc, err := walletmldsa87.NewMLDSA87Descriptor()
	if err != nil {
		return false, err
	}
	return MLDSA87VerifySignature(sig, msg, pk, desc.ToDescriptor())
}

// MLDSA87PublicKeyToAddress derives the account address of an ML-DSA-87 public
// key under the default descriptor, the only descriptor accepted for ML-DSA-87
// signatures.
func MLDSA87PublicKeyToAddress(pk []byte) (common.Address, error) {
	desc, err := walletmldsa87.NewMLDSA87Descriptor()
	if err != nil {
		return common.Address{}, err
	}
	return PublicKeyAndDescriptorToAddress(pk, desc.ToDescriptor())
}

func Sign(digestHash []byte, w wallet.Wallet) ([]byte, error) {
	if len(digestHash) != DigestLength {
		return nil, fmt.Errorf("hash is required to be exactly %d bytes (%d)", DigestLength, len(digestHash))
	}
	signature, err := w.Sign(digestHash)
	if err != nil {
		return nil, err
	}
	return signature[:], nil
}
