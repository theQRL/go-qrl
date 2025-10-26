package pqcrypto

import (
	"bufio"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"

	cryptomldsa87 "github.com/theQRL/go-qrllib/crypto/ml_dsa_87"
	"github.com/theQRL/go-qrllib/wallet"
	walletcommon "github.com/theQRL/go-qrllib/wallet/common"
	"github.com/theQRL/go-qrllib/wallet/common/descriptor"
	walletmldsa87 "github.com/theQRL/go-qrllib/wallet/ml_dsa_87"
	"github.com/theQRL/go-zond/common"
)

const (
	MLDSA87SignatureLength = cryptomldsa87.CryptoBytes
	MLDSA87PublicKeyLength = cryptomldsa87.CryptoPublicKeyBytes
	DescriptorSize         = descriptor.DescriptorSize

	// DigestLength sets the signature digest exact length
	DigestLength = 32
)

var ErrBadSignature = errors.New("invalid ML-DSA-87 signature")

// LoadWallet loads ML-DSA-87 or SPHINCS+-256s Wallet from the given file having hex seed (not extended hex seed).
func LoadWallet(file string) (*walletmldsa87.Wallet, error) {
	fd, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer fd.Close()

	r := bufio.NewReader(fd)
	buf := make([]byte, walletcommon.SeedSize*2)
	n, err := readASCII(buf, r)
	if err != nil {
		return nil, err
	} else if n != len(buf) {
		return nil, fmt.Errorf("key file too short, want %v hex characters", walletcommon.SeedSize*2)
	}
	if err := checkKeyFileEnd(r); err != nil {
		return nil, err
	}

	return HexToWallet(string(buf))
}

func GenerateWalletKey() (*walletmldsa87.Wallet, error) {
	return walletmldsa87.NewWallet()
}

// readASCII reads into 'buf', stopping when the buffer is full or
// when a non-printable control character is encountered.
func readASCII(buf []byte, r *bufio.Reader) (n int, err error) {
	for ; n < len(buf); n++ {
		buf[n], err = r.ReadByte()
		switch {
		case err == io.EOF || buf[n] < '!':
			return n, nil
		case err != nil:
			return n, err
		}
	}
	return n, nil
}

// checkKeyFileEnd skips over additional newlines at the end of a key file.
func checkKeyFileEnd(r *bufio.Reader) error {
	for i := 0; ; i++ {
		b, err := r.ReadByte()
		switch {
		case err == io.EOF:
			return nil
		case err != nil:
			return err
		case b != '\n' && b != '\r':
			return fmt.Errorf("invalid character %q at end of key file", b)
		case i >= 2:
			return errors.New("key file too long, want 64 hex characters")
		}
	}
}

// ToWalletUnsafe blindly converts a binary blob to a private key. It should almost
// never be used unless you are sure the input is valid and want to avoid hitting
// errors due to bad origin encoding (0 prefixes cut off).
func ToWalletUnsafe(seed []byte) *walletmldsa87.Wallet {
	var sizedSeed [walletcommon.SeedSize]uint8
	copy(sizedSeed[:], seed)
	d, err := walletmldsa87.NewWalletFromSeed(sizedSeed)
	if err != nil {
		return nil
	}
	return d
}

// HexToWallet parses a hex seed (not extended hex seed).
func HexToWallet(hexSeedStr string) (*walletmldsa87.Wallet, error) {
	b, err := hex.DecodeString(hexSeedStr)
	if byteErr, ok := err.(hex.InvalidByteError); ok {
		return nil, fmt.Errorf("invalid hex character %q in seed", byte(byteErr))
	} else if err != nil {
		return nil, errors.New("invalid hex data for seed")
	}

	var hexSeed [walletcommon.SeedSize]uint8
	copy(hexSeed[:], b)

	return walletmldsa87.NewWalletFromSeed(hexSeed)
}

func PKToAddress(pk []byte, d descriptor.Descriptor) (common.Address, error) {
	return wallet.GetAddressFromPKAndDescriptor(pk, d)
}

func Verify(msg []byte, sig []byte, pk []byte, desc [3]byte) (bool, error) {
	// walletmldsa87.Verify would panic on bad length
	if l := len(sig); l != cryptomldsa87.CryptoBytes {
		return false, fmt.Errorf("%w: bad length", ErrBadSignature)
	}

	pk87, err := walletmldsa87.BytesToPK(pk)
	if err != nil {
		return false, err
	}

	return walletmldsa87.Verify(msg, sig, &pk87, desc), nil
}
