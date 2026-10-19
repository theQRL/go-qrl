package migration

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"

	"github.com/theQRL/go-qrl/accounts/abi"
	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/core"
	"github.com/theQRL/go-qrl/core/vm"
	"github.com/theQRL/go-qrl/crypto"
)

//go:embed LegacyQRLMigration.json
var artifactJSON []byte

//go:embed LegacyQRLMigration.hyp
var contractSource []byte

type Artifact struct {
	Compiler     string          `json:"compiler"`
	SourceSHA256 string          `json:"source_sha256"`
	ABI          json.RawMessage `json:"abi"`
	Runtime      string          `json:"runtime"`
}

func Contract() (*Artifact, abi.ABI, []byte, error) {
	var artifact Artifact
	if err := json.Unmarshal(artifactJSON, &artifact); err != nil {
		return nil, abi.ABI{}, nil, err
	}
	sourceHash := sha256.Sum256(contractSource)
	if artifact.SourceSHA256 != hex.EncodeToString(sourceHash[:]) {
		return nil, abi.ABI{}, nil, errors.New("contract artifact is stale; run migration/compile.py")
	}
	contractABI, err := abi.JSON(bytes.NewReader(artifact.ABI))
	if err != nil {
		return nil, abi.ABI{}, nil, err
	}
	code, err := hex.DecodeString(artifact.Runtime)
	if err != nil || len(code) == 0 {
		return nil, abi.ABI{}, nil, errors.New("invalid contract runtime")
	}
	return &artifact, contractABI, code, nil
}

// QRVM mapping slots hash TWO 64-byte words, unlike Ethereum's 32-byte words.
func mappingSlot(key *big.Int, slot common.Hash) common.Hash {
	input := make([]byte, 128)
	key.FillBytes(input[:64])
	copy(input[96:], slot[:])
	return crypto.Keccak256Hash(input)
}

func slot(number uint64) common.Hash { return common.BigToHash(new(big.Int).SetUint64(number)) }

func validTarget(chainID *big.Int, contract common.Address) error {
	if chainID == nil || chainID.Sign() <= 0 || chainID.BitLen() > 256 {
		return errors.New("target chain ID must be a positive uint256")
	}
	if contract == (common.Address{}) {
		return errors.New("migration contract address must be nonzero")
	}
	if _, exists := vm.PrecompiledContractsZond[contract]; exists {
		return errors.New("migration contract address collides with a precompile")
	}
	return nil
}

// GenesisAccount returns a fully initialized, funded contract account. Genesis
// installs its runtime directly; no constructor or initialization transaction runs.
func GenesisAccount(s *Snapshot, chainID *big.Int, contract common.Address) (core.GenesisAccount, error) {
	var account core.GenesisAccount
	if err := s.Validate(); err != nil {
		return account, err
	}
	if err := validTarget(chainID, contract); err != nil {
		return account, err
	}
	_, _, code, err := Contract()
	if err != nil {
		return account, err
	}
	total, _ := parseAmount(s.TotalBalanceBaseUnits)
	if total.BitLen() > 256 {
		return account, errors.New("target balance exceeds uint256")
	}
	storage := make(map[common.Hash]common.StorageValue64)
	err = walkStorage(s, chainID, func(visit func(Account) error) error {
		for _, entry := range s.Accounts {
			if err := visit(entry); err != nil {
				return err
			}
		}
		return nil
	}, func(key common.Hash, value common.StorageValue64) error {
		storage[key] = value
		return nil
	})
	if err != nil {
		return account, err
	}

	return core.GenesisAccount{Code: code, Storage: storage, Balance: total, Nonce: 1}, nil
}

func AddToGenesis(genesis *core.Genesis, s *Snapshot, contract common.Address) error {
	if err := validateBaseGenesis(genesis, contract); err != nil {
		return err
	}
	account, err := GenesisAccount(s, genesis.Config.ChainID, contract)
	if err != nil {
		return err
	}
	if genesis.Alloc == nil {
		genesis.Alloc = make(core.GenesisAlloc)
	}
	genesis.Alloc[contract] = account
	return nil
}

// ClaimMessage is byte-for-byte identical to the contract's claimMessage.
func ClaimMessage(s *Snapshot, chainID *big.Int, contract common.Address, source string, recipient common.Address) ([]byte, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	allocation, err := memoryAccount(s, source)
	if err != nil {
		return nil, err
	}
	return claimMessage(s, allocation, chainID, contract, source, recipient)
}

func (s *SnapshotStore) ClaimMessage(chainID *big.Int, contract common.Address, source string, recipient common.Address) ([]byte, error) {
	allocation, err := s.Account(source)
	if err != nil {
		return nil, err
	}
	return claimMessage(&s.metadata, allocation, chainID, contract, source, recipient)
}

func claimMessage(s *Snapshot, allocation Account, chainID *big.Int, contract common.Address, source string, recipient common.Address) ([]byte, error) {
	if err := validTarget(chainID, contract); err != nil {
		return nil, err
	}
	if recipient == (common.Address{}) || recipient == contract {
		return nil, errors.New("invalid recipient")
	}
	address, err := LegacyAddress(source)
	if err != nil {
		return nil, err
	}
	if allocation.Address != source || allocation.BalanceShor == "0" {
		return nil, fmt.Errorf("%s has no redeemable snapshot balance", source)
	}
	message := append([]byte("QRL-XMSS-MIGRATION-V1"), chainID.FillBytes(make([]byte, 64))...)
	message = append(message, contract[:]...)
	block, _ := hex.DecodeString(s.BlockHash)
	commitment, _ := hex.DecodeString(s.Commitment)
	message = append(message, block...)
	message = append(message, commitment...)
	message = append(message, address...)
	message = append(message, recipient[:]...)
	return message, nil
}
