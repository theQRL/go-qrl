package block_test

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/theQRL/go-qrl/pkg/config"
	"github.com/theQRL/go-qrl/pkg/core/block"
	"testing"
)

type BlockHeaderConfig struct {
	blockNumber         uint64
	prevBlockHeaderhash []byte
	prevBlockTimestamp  uint64
	merkleRoot          []byte
	feeReward           uint64
	timestamp           uint64
}

type MockNTP struct {
	mock.Mock
}

func (m *MockNTP) UpdateTime() error {
	m.Called()
	return nil
}

func (m *MockNTP) Time() uint64 {
	args := m.Called()
	return uint64(args.Int(0))
}

// MockBlock implements package block's BlockBareInterface
// Use it whenever you're too lazy to create a real Block
type MockBlock struct {
	blockNumber uint64
	headerHash  []byte
	timestamp   uint64
}

func (m MockBlock) BlockNumber() uint64 {
	return m.blockNumber
}
func (m MockBlock) HeaderHash() []byte {
	return m.headerHash
}
func (m MockBlock) Timestamp() uint64 {
	return m.timestamp
}

var defaultBhConfig = BlockHeaderConfig{
	blockNumber:         uint64(2),
	prevBlockHeaderhash: []byte("Unpublished Sci-Fi Book Title"),
	prevBlockTimestamp:  uint64(1539008441),
	merkleRoot:          []byte("0xDEADBEEF"),
	feeReward:           uint64(1000),
	timestamp:           uint64(1539008488),
}

func NewBlockHeader(config ...BlockHeaderConfig) *block.BlockHeader {
	mockNtp := new(MockNTP)
	mockNtp.On("Time").Return(1539008488)

	var bh *block.BlockHeader
	if config == nil {
		bh = block.CreateBlockHeader(defaultBhConfig.blockNumber, defaultBhConfig.prevBlockHeaderhash, defaultBhConfig.prevBlockTimestamp, defaultBhConfig.merkleRoot, defaultBhConfig.feeReward, defaultBhConfig.timestamp)
	} else {
		bh = block.CreateBlockHeader(config[0].blockNumber, config[0].prevBlockHeaderhash, config[0].prevBlockTimestamp, config[0].merkleRoot, config[0].feeReward, config[0].timestamp)
	}

	bh.Option(block.MockNTP(mockNtp))

	return bh
}

func TestBlockHeaderBlockHeaderCreate(t *testing.T) {
	bh := NewBlockHeader()
	msg := "CreateBlockHeader(attributes) should return a header with the same attributes"

	assert.Equal(t, bh.BlockNumber(), defaultBhConfig.blockNumber)
	assert.Equal(t, bh.PrevHeaderHash(), defaultBhConfig.prevBlockHeaderhash, msg)
	assert.Equal(t, bh.TxMerkleRoot(), defaultBhConfig.merkleRoot, msg)
	assert.Equal(t, bh.FeeReward(), defaultBhConfig.feeReward, msg)
	assert.Equal(t, bh.Timestamp(), defaultBhConfig.timestamp, msg)
	assert.Equal(t, bh.Epoch(), uint64(0), msg)
}

func TestBlockHeaderUpdateMerkleRoot(t *testing.T) {
	bh := NewBlockHeader()
	assert.Equal(t, bh.TxMerkleRoot(), defaultBhConfig.merkleRoot, "Haven't modified Merkle Root yet, it should be the default")
	bh.UpdateMerkleRoot([]byte("LIVEBEEF"))
	assert.Equal(t, bh.TxMerkleRoot(), []byte("LIVEBEEF"), "Haven't modified Merkle Root yet, it should be the default")
}

func TestBlockHeaderValidate(t *testing.T) {
	bh := NewBlockHeader()
	assert.Equal(t, bh.Validate(defaultBhConfig.feeReward, uint64(bh.BlockReward()+defaultBhConfig.feeReward), defaultBhConfig.merkleRoot), true)
}

func TestBlockHeaderValidateTimestampTooNew(t *testing.T) {
	var configTimestampTooNew = defaultBhConfig
	configTimestampTooNew.timestamp = defaultBhConfig.timestamp + 10000
	bh := NewBlockHeader(configTimestampTooNew)
	assert.Equal(t, bh.Validate(configTimestampTooNew.feeReward, uint64(bh.BlockReward()+configTimestampTooNew.feeReward), configTimestampTooNew.merkleRoot), false)
}

func TestBlockHeaderValidateTimestampLowerThanGenesis(t *testing.T) {
	var customConfig = defaultBhConfig
	customConfig.prevBlockTimestamp = uint64(999999999)
	customConfig.timestamp = uint64(1000000000)
	bh := NewBlockHeader(customConfig)
	assert.Equal(t, bh.Validate(customConfig.feeReward, uint64(bh.BlockReward()+customConfig.feeReward), customConfig.merkleRoot), false)
}

// This test is different from TestValidateBadParams/BadMerkleRoot in that there. There we test that it does not have the right Merkle Root, specifically.
// In this test, the BlockHeader itself does not have the right HeaderHash anymore. It just so happened that the only way to change
// a BlockHeader externally is by updating its Merkle Root.
func TestBlockHeaderValidateBadHeaderHash(t *testing.T) {
	bh := NewBlockHeader()
	assert.Equal(t, true, bh.Validate(defaultBhConfig.feeReward, uint64(bh.BlockReward()+defaultBhConfig.feeReward), defaultBhConfig.merkleRoot))
	bh.UpdateMerkleRoot([]byte("Something Else"))
	assert.NotEqual(t, true, bh.Validate(defaultBhConfig.feeReward, uint64(bh.BlockReward()+defaultBhConfig.feeReward), defaultBhConfig.merkleRoot))
}

func TestBlockHeaderValidateInconsistencyInBlockRewardCalculation(t *testing.T) {
	bh := NewBlockHeader()
	block.BlockRewardCalc = func(blockNumber uint64, _ *config.Config) uint64 {
		return 777777
	}
	assert.Equal(t, false, bh.Validate(defaultBhConfig.feeReward, uint64(bh.BlockReward()+defaultBhConfig.feeReward), defaultBhConfig.merkleRoot))
}

func TestBlockHeaderValidateBadParams(t *testing.T) {
	bh := NewBlockHeader()
	testCases := []struct {
		testCaseName   string
		feeReward      uint64
		coinbaseAmount uint64
		txMerkleRoot   []byte
	}{
		{"Bad Fee Reward", defaultBhConfig.feeReward + 2, uint64(bh.BlockReward() + defaultBhConfig.feeReward), defaultBhConfig.merkleRoot},
		{"Bad Coinbase Reward", defaultBhConfig.feeReward, bh.BlockReward(), defaultBhConfig.merkleRoot},
		{"Bad Merkle Root", defaultBhConfig.feeReward, uint64(bh.BlockReward() + defaultBhConfig.feeReward), []byte("0xLIVEBEEF")},
	}
	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%s", tc.testCaseName), func(t *testing.T) {
			bh.Validate(tc.feeReward, tc.coinbaseAmount, tc.txMerkleRoot)
		})
	}
}

func TestBlockHeaderJSONSerialization(t *testing.T) {
	bh2Config := defaultBhConfig
	bh2Config.blockNumber = uint64(3)

	bh1 := NewBlockHeader()
	bh2 := NewBlockHeader(bh2Config)
	bh1Json, _ := bh1.JSON()
	bh2Json, _ := bh2.JSON()

	assert.NotEqual(t, bh1Json, bh2Json, "bh2 should be different from bh1 at this point")

	// Given that bh1 and bh2 are distinct: if bh2.FromJSON(bh1Json), now bh2 should be the same as bh1
	bh2.FromJSON(bh1Json)
	bh2CopiedFromBh1Json, _ := bh2.JSON()
	assert.Equal(t, bh1Json, bh2CopiedFromBh1Json, "bh1 and bh2 should now be the same (but not deeply equal)")
}

// Ensure that GenerateHeaderHash() works, nothing more
func TestBlockHeaderGenerateHeaderHash(t *testing.T) {
	bh := NewBlockHeader()
	headerHash := bh.GenerateHeaderHash()

	assert.Equal(t, headerHash, []byte{0x23, 0xc4, 0xe0, 0xf, 0xc9, 0xb0, 0x20, 0xd1, 0xb, 0x7e, 0x85, 0xc1, 0xfd, 0xa2, 0xc9, 0x19, 0x22, 0x7c, 0x5b, 0x9a, 0x2b, 0x1b, 0xec, 0xdd, 0xe4, 0x45, 0x44, 0x66, 0xa2, 0x2c, 0x8e, 0x7f})
}

func TestBlockHeaderMiningWorkflow(t *testing.T) {
	// On a newly created BlockHeader, the nonce+extranonce is 0
	bh := NewBlockHeader()
	// Miner found nonces of 5 and 23
	bh.SetNonces(uint32(5), uint64(23))
	// We can now hash the BlockHeader.
	headerHash := bh.GenerateHeaderHash()
	assert.Equal(t, headerHash, []byte{0x54, 0x9b, 0x2c, 0xae, 0xd4, 0xbe, 0xd9, 0x6e, 0x70, 0x68, 0x41, 0x8a, 0xcc, 0x9b, 0x1e, 0xa2, 0x23, 0x2d, 0x5, 0xe1, 0x4c, 0x55, 0xd5, 0xca, 0x92, 0x99, 0xd5, 0xee, 0xb7, 0x2a, 0x7e, 0xc8})

	// HeaderHash depends on the nonces
	bh.SetNonces(uint32(5), uint64(25))
	headerHashOther := bh.GenerateHeaderHash()
	assert.NotEqual(t, headerHashOther, []byte{0x54, 0x9b, 0x2c, 0xae, 0xd4, 0xbe, 0xd9, 0x6e, 0x70, 0x68, 0x41, 0x8a, 0xcc, 0x9b, 0x1e, 0xa2, 0x23, 0x2d, 0x5, 0xe1, 0x4c, 0x55, 0xd5, 0xca, 0x92, 0x99, 0xd5, 0xee, 0xb7, 0x2a, 0x7e, 0xc8})
}

func TestBlockHeaderSetMiningNonceFromBlob(t *testing.T) {
	bh1 := NewBlockHeader()

	bh2 := NewBlockHeader()
	bh2.SetNonces(uint32(6), uint64(30))

	// Copy nonces from bh2 to bh1
	bh1.SetMiningNonceFromBlob(bh2.MiningBlob())
	assert.Equal(t, bh1.MiningNonce(), bh2.MiningNonce(), "Both Blockheaders should have the same nonces")
	assert.Equal(t, bh1.ExtraNonce(), bh2.ExtraNonce(), "Both Blockheaders should have the same nonces")
}

// VerifyBlob() ensures the miner did not tamper with anything other than the mining nonce.
func TestBlockHeaderVerifyBlob(t *testing.T) {
	bh2Config := defaultBhConfig
	bh2Config.merkleRoot = []byte("Miner secretly changes Coinbase reward TX here")

	bh1 := NewBlockHeader()
	bh2 := NewBlockHeader(bh2Config)
	bh1.SetNonces(uint32(5), uint64(23))
	bh2.SetNonces(uint32(5), uint64(23))
	assert.Equal(t, bh1.VerifyBlob(bh1.MiningBlob()), true, "bh1.VerifyBlob() should accept its own MiningBlob() as valid")
	assert.Equal(t, bh1.VerifyBlob(bh2.MiningBlob()), false, "Although they have the same nonces, bh2.MiningBlob() should not be accepted by bh1")
}

func TestBlockHeaderValidateParentChildRelation(t *testing.T) {
	parentBlockTimestamp := defaultBhConfig.timestamp - 20

	bh := NewBlockHeader()
	mBlock := MockBlock{
		blockNumber: 1,
		headerHash:  []byte("Unpublished Sci-Fi Book Title"),
		timestamp:   parentBlockTimestamp,
	}
	assert.Equal(t, bh.ValidateParentChildRelation(mBlock), true)
}

func TestBlockHeaderValidateParentChildRelationInvalid(t *testing.T) {
	parentBlockTimestamp := defaultBhConfig.timestamp - 20

	bh := NewBlockHeader()

	testCases := []struct {
		testCaseName string
		blockNumber  uint64
		headerHash   []byte
		timestamp    uint64
	}{
		{"Parent's Blocknumber is higher than Child's", 3, []byte("Unpublished Sci-Fi Book Title"), parentBlockTimestamp},
		{"Different Headerhash", 1, []byte("Published Sci-Fi Book Title"), parentBlockTimestamp},
		{"Parent's Timestamp is newer than Child's", 1, []byte("Unpublished Sci-Fi Book Title"), defaultBhConfig.timestamp + 20},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%s", tc.testCaseName), func(t *testing.T) {
			mBlock := MockBlock{tc.blockNumber, tc.headerHash, tc.timestamp}
			assert.Equal(t, bh.ValidateParentChildRelation(mBlock), false)
		})
	}

	// This didn't fit in with the table-driven tests above.
	assert.Equal(t, bh.ValidateParentChildRelation(nil), false)
}
