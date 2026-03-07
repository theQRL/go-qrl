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
	MLDSA87SignatureLength = cryptomldsa87.CryptoBytes
	MLDSA87PublicKeyLength = cryptomldsa87.CryptoPublicKeyBytes
	DescriptorSize         = descriptor.DescriptorSize

	// DigestLength sets the signature digest exact length
	DigestLength = 32
)

var ErrBadSignature = errors.New("invalid ML-DSA-87 signature")

func BytesToDescriptor(b []byte) (descriptor.Descriptor, error) {
	return descriptor.FromBytes(b)
}

func PublicKeyAndDescriptorToAddress(pk []byte, d descriptor.Descriptor) (common.Address, error) {
	return libwallet.GetAddressFromPKAndDescriptor(pk, d)
}

func MLDSA87VerifySignature(sig []byte, msg []byte, pk []byte) (bool, error) {
	// walletmldsa87.Verify would panic on bad length
	if l := len(sig); l != cryptomldsa87.CryptoBytes {
		return false, fmt.Errorf("%w: bad length", ErrBadSignature)
	}

	pk87, err := walletmldsa87.BytesToPK(pk)
	if err != nil {
		return false, err
	}

	return walletmldsa87.Verify(msg, sig, &pk87, walletmldsa87.NewMLDSA87Descriptor()), nil
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
