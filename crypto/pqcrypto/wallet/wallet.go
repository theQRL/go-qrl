package wallet

import (
	"bufio"
	"errors"
	"fmt"
	"os"

	walletcommon "github.com/theQRL/go-qrllib/wallet/common"
	"github.com/theQRL/go-qrllib/wallet/common/descriptor"
	"github.com/theQRL/go-qrllib/wallet/common/wallettype"
	walletmldsa87 "github.com/theQRL/go-qrllib/wallet/ml_dsa_87"
	"github.com/theQRL/go-zond/common"
)

const (
	SeedSizeBytes = walletcommon.ExtendedSeedSize

	ML_DSA_87 wallettype.WalletType = wallettype.ML_DSA_87
)

var ErrBadWalletType = errors.New("unsupported wallet type")

type Wallet interface {
	GetSeed() walletcommon.ExtendedSeed
	GetAddress() [common.AddressLength]uint8
	GetDescriptor() descriptor.Descriptor
	GetPK() []byte
	Sign([]uint8) ([]byte, error)
}

type MLDSA87Wallet struct {
	*walletmldsa87.Wallet
}

func (w *MLDSA87Wallet) Sign(message []uint8) ([]byte, error) {
	sig, err := w.Wallet.Sign(message)
	if err != nil {
		return nil, err
	}
	return sig[:], nil
}

func (w *MLDSA87Wallet) GetPK() []byte {
	pk := w.Wallet.GetPK()
	return pk[:]
}

func (w *MLDSA87Wallet) GetDescriptor() descriptor.Descriptor {
	return w.Wallet.GetDescriptor().ToDescriptor()
}

func (w *MLDSA87Wallet) GetSeed() walletcommon.ExtendedSeed {
	return w.Wallet.GetExtendedSeed()
}

func Generate(t wallettype.WalletType) (Wallet, error) {
	var w Wallet
	switch t {
	case ML_DSA_87:
		mldsa87, err := walletmldsa87.NewWallet()
		if err != nil {
			return nil, err
		}
		w = &MLDSA87Wallet{mldsa87}
	default:
		return nil, ErrBadWalletType
	}

	return w, nil
}

func RestoreFromSeedBytes(seed []byte) (Wallet, error) {
	ext, err := walletcommon.NewExtendedSeedFromBytes(seed)
	if err != nil {
		return nil, err
	}
	return restoreWalletFromExtendedSeed(ext)
}

func RestoreFromSeedHex(seed string) (Wallet, error) {
	// NOTE(rgeraldes24): NewExtendedSeedFromHexString does not support 0x prefix
	return RestoreFromSeedBytes(common.FromHex(seed))
}

func restoreWalletFromExtendedSeed(ext walletcommon.ExtendedSeed) (Wallet, error) {
	var w Wallet
	desc := descriptor.New(ext.GetDescriptorBytes())
	switch desc.Type() {
	case byte(wallettype.ML_DSA_87):
		mldsa87, err := walletmldsa87.NewWalletFromSeed(ext.GetSeed())
		if err != nil {
			return nil, err
		}
		w = &MLDSA87Wallet{mldsa87}
	default:
		return nil, fmt.Errorf("unsupported wallet type in descriptor: %v", desc.Type())
	}

	return w, nil
}

func RestoreFromFile(file string) (Wallet, error) {
	seed, err := ReadSeedFromFile(file)
	if err != nil {
		return nil, err
	}

	w, err := RestoreFromSeedHex(seed)
	if err != nil {
		return nil, err
	}

	return w, nil
}

func ReadSeedFromFile(file string) (string, error) {
	fd, err := os.Open(file)
	if err != nil {
		return "", err
	}
	defer fd.Close()

	r := bufio.NewReader(fd)
	buf := make([]byte, SeedSizeBytes*2)
	n, err := common.ReadASCII(buf, r)
	if err != nil {
		return "", err
	} else if n != len(buf) {
		return "", fmt.Errorf("seed too short, want %d hex characters", SeedSizeBytes*2)
	}
	if err := common.CheckKeyFileEnd(r); err != nil {
		return "", err
	}

	return string(buf), nil
}
