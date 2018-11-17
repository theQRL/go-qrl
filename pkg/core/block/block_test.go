package block_test

import (
	"encoding/binary"
	"github.com/stretchr/testify/assert"
	"github.com/theQRL/go-qrl/pkg/config"
	"github.com/theQRL/go-qrl/pkg/core/addressstate"
	"github.com/theQRL/go-qrl/pkg/core/block"
	"github.com/theQRL/go-qrl/pkg/core/metadata"
	"github.com/theQRL/go-qrl/pkg/core/transactions"
	"github.com/theQRL/go-qrl/pkg/misc"
	"testing"
)

type BlockConfig struct {
	minerAddress        string
	blockNumber         uint64
	prevBlockHeaderhash []byte
	prevBlockTimestamp  uint64
	txs                 []transactions.Transaction
	timestamp           uint64
}

var c = config.GetDevConfig()
var emptyBlock = BlockConfig{
	minerAddress:        "Q010400d9f1efe5b272e042dcc8ef690f0e90ca8b0b6edba0d26f81e7aff12a6754b21788169f7f",
	blockNumber:         uint64(2),
	prevBlockHeaderhash: []byte{97, 32, 152, 0, 196, 227, 205, 0, 224, 111, 250, 160, 236, 121, 75, 145, 108, 177, 251, 190, 131, 157, 169, 157, 7, 185, 119, 25, 177, 69, 132, 109},
	prevBlockTimestamp:  uint64(c.Genesis.GenesisTimestamp),
	txs:                 []transactions.Transaction{},
	timestamp:           uint64(c.Genesis.GenesisTimestamp + 20),
}

// txJson taken from transfer_test.go, but signed and hashed first
// Q010300a1da274e68c88b0ccf448e0b1916fa789b01eb2ed4e9ad565ce264c9390782a9c61ac02f sends to Q0103001d65d7e59aed5efbeae64246e0f3184d7c42411421eb385ba30f2c1c005a85ebc4419cfd
var transferTx = `{"fee":"1","publicKey":"AQMAOOpjdQafgnLMGmYBs8dsIVGUVWA9NwA2uXx3mto1ZYVOOYO9VkKYxJri5/puKNS5VNjNWTmPEiWwjWFEhUruDg==","signature":"AAAAAEwFsBo5wdqtPG19lhOmQwIuOYAHZtRS350x3Q2jXds1u4fvFhEJ9KdfFayJSRLLzP4HUATwoq0di6netLesoP5hY3znf7UwAK6IdIOrA9FhmQoKZT8CI0RxtkM2r28mgiTj5C8VrYGhsLkUi1TpOdXyFKQDQV8wn3B9C4wlzkX/FmiTQqvCisBB7TD6AMzApSOtAHfs/QoZ8D90Jln99qJidynOzmSNHaTv7caKzSsqdd0LeUbeQWjMxPWzOMDkAzkyJByvPkq8ZyJEDQbiDW1mfCj3HeQFVLXYb2jIYdKBC4bjztjQK6fIUKHl68Nh1MFhiLo95OqIbzp037HzZBW494q0YB/PnWq/V2rUctjvoZDQYZjMSds5iw96pu/ICLiMoz9967jBi6umLHrvhMLbILIkeRLfLKLeLIiPBKSx2p4EtBg6Wu6z8Vb0KwCmjjFe5jWZLsGZ9zXFhkLTPuSp3KciiSK08tw2K5n7hLXyF14oZYHycjE3YgLr54ohdRJAlOkqLWM2uP5pI4sxfDqg5IKbabzJFuHtPn/jftHWOX7hHzInNPxBR4z2VM1O1frjyf0b7cgjA5MS7XdvNWBxwKF42UBRu8II7wcsalzUwAZSSXZzePBvDQad6Yp/SdA0ZwS7mFgPadsrVa0/Hnrx8zUOsbLTUtoo3BJHpYMRPJ5aqEjATikRxVAE+UOdrj6rNxNZb5qB3y+0CCSqWbtEeYpy9YddZNJi2vPZwBLMHVbgfkI8JljQpjZzo6UeaN0YpJZRrtqxEXbHU36ifstxyB1asbx0bzLGTCvp/CC/U5/z0CU5pjScZT1Ji9nHlp1W3wQFBm5+xHGYjIesgEM37DALfDlf2KylaZvzc62nEDQs+ChJbdkqC20yInKjDxVCOU+BXB2t5SYasUO5pK1o00grw4MQ7mgGvMdolah5BGGHogDhN2OrBkcV8PN5kTDRORstlDqBcS9e3N80AUL5soSH3RcyMKdlZ2b1wlcwiPUaiI7QwsYZ5iZkxWKqT+XqgQBCZnarG3OrnvZsxDU5z7qqy7CognTf6jBzoAsFft2rOJMdws0aOWNksFqk+AIjiB4rTNkAQ1+T3XY9BD1kvsgvaa351Z9AOspMRByJDzz8Ki7Qx3+WXZa1WZIU58fHNJQkC6xA/nE77JU2zvF41EPXKlaQwtlTayvYcX09ukU4mht/UJ+YGXYAsksk8pDdQ6xDBEtjLYdMpOVNNVIeljo8BEmPgHD/dLPGrpaD7DAHltvUMRd9y0Lu7gF8JDQ1/CuV0PW4z7YyNsx7iu880xbfQf8m186wItgOPS30LvfXzYQ6Q20FnH5k+FWXxwZKaptp72FR+WNvPDe8KfIBbL4j6o3wZEzvEPz79XXhzZGVaIQ9uu02PsenVjQNwsNyT1SJgr9XhhITkVGZhX4dXc6zLkz7F2e4PrJUVbqlpx7Om13iYTjKdINTFdJHqkIlxakpLBnc3FH+J+7wwbvXHBjH48l2yV3+BEXUkgbvjq+WWt/Ty+DTBUerN5BRKURbKA8ZAFZKK9k3Z/i1yvI9UFox6xPtnjgCqPWn28j2lX3sL1D1B1t5aFuMAWmyPfeJJxWxrBYpniNx+EgeO0gpDa4R3mEbmGU71MtXmHx1zIUkpQSoeNpjf9T+Ow9ic0rDin6jb69hAw0CXR6OUndpoc4h+JugqZfH/XFrdGg+v3835kYyoTvkbNilB738GlABeOo9+uAdPy3ln6knz4w9Ay7iuKxyFC9d9zg4ri6jqmzM7cp5Ve9/ZTF636bclG74bLNFe03ATMVs6M4ixai+F6uXfYNTnY8sE7yu/e0vbgW/LfKd3kuRahG4FGFFAbqLFvx1/ZyrsLeNf6cmSlH1E4At3ewBKdwUo2iHTZnXLwX83lwTllHpzIpoowcYS7DSiTxUKgMcrTDiRsCYN4GJCmSHjQfoKidKP+EReb8dmRRfyrhSk31xxdmFCKQSW6lwlYQPd97iS3wVOwMjpz7ijoxewE15vMXbwueYlpC4ruy3g+X6zt8EaoHWBKHBs6NnNUqDN6N4sZRKfzrK1iZQ8zICzX9tO4bd+b4y+oYkpv+8cQ8o/u9rHrEDhq1J6fS2+KHShAYczhBZ1Peg/X9Z3M2t+HOxYl/k3+EnsfGWhPWF9JfTTQIxoTMzdf5Oj/+r2sLet30U4lQI8Va5csa+LLqgjNAvOpmnG6FtD7XB6JDBIBzHFMsQqoKEsZ06eFy/ir9qDJOve2Rw2e7xOLkx013x/EgB6/9VsiPmBD1lMFZIAU8Sa5A9NNIYujeq1klBv6DyF7UGcDcRvV98cPcW8K57ABPbneb0dEnF6Yg9rjlPv4QugGl7+mn5wwiV3VisRLobfkX5iPDZUlnMWPU38x3UZVh6uoC9VOGjSy4bz2twNF5bKLIjBX2Ik2l3TScvrdcW/KSHs9n4WkTz9vq4+3Zd8rVDrMVKiTKHP7xtMAjFRForxREqEUWIdIn7hB1JXm9pWQmVWsUT2r7D3IukBpvn1n3a61Xx5+eLIAJALkbGqnOp0I7xC9qRG5IB4LbYgt1AxKKUyRp4PpQibId0jG+4lkZULleVxTLITDr+aIXQgMrR7a8Nx+DiqH2UXaZk2Z6VxqO5HSs4RFxV6kTGdtdB6GeGt3RP0mmmaEJYi2UcfO+5JHVUDoPDo0NerYb8DHRy4pANGviYbcpFSndhxO1+YoFPyNZRmFiWVke7TCeFfKGFlk5Zkjmwx9fkeztJRLJvT0tRx23IDYsFeOV1+PoNt3uNuto1dJSQG0SkxxefC98ce+dh3cF04pBy4+la0A4W5paXsoccf9+Lf8WthX4VHJNC7R8/YKZ8MitxvxizxeBfj3n8XtY40ZOeY2U8AYb1t4PlqX0hzVcuxCB609CLAVF3j0VBj7JLS4MXC7SOvDEoYAw75VSy4SIWWv46402sZIbgO44rqhYWtY5yzjimUnTbSIJG7WinCylJKNon75sk7lkscj2RoC0j7pYAcsxfwSILx494Z+MoZyL+9u3vPwzgkEUAQds8ratzbyVLaL961jxwhXicK2UalH8Jhw404VXvPCxUK//0EsfWtvb8kLCpWmNe7Q+lChJqXSS3jJFcIQ2/XpJjP4PygtC55OCkf0nz0ySYKJhnXu0=","transactionHash":"D3Np8CFa+txFk7ZK+LO7ONupCwERihslTwl6Dnl264Q=","transfer":{"addrsTo":["AQMAHWXX5ZrtXvvq5kJG4PMYTXxCQRQh6zhbow8sHABahevEQZz9"],"amounts":["100"]}}`

func BuildBlockConfigWithTxs(txJson string) BlockConfig {
	t := transactions.Transaction{}
	t.FromJSON(txJson)
	t.SetNonce(1)
	txs := []transactions.Transaction{t}
	configWithTxs := emptyBlock
	configWithTxs.txs = txs
	return configWithTxs
}

var blockWithTxs = BuildBlockConfigWithTxs(transferTx)

func NewBlock(config ...BlockConfig) *block.Block {
	var blockConfig BlockConfig
	if config == nil {
		blockConfig = emptyBlock
	} else {
		blockConfig = config[0]
	}

	b := block.CreateBlock(misc.Qaddress2Bin(blockConfig.minerAddress), blockConfig.blockNumber, blockConfig.prevBlockHeaderhash, blockConfig.prevBlockTimestamp, blockConfig.txs, blockConfig.timestamp)

	return b
}

func TestBlockCreate(t *testing.T) {
	b := NewBlock()
	assert.Equal(t, b.BlockNumber(), emptyBlock.blockNumber, "CreateBlock(attributes) should return a block with the same attributes")
	assert.Equal(t, b.PrevHeaderHash(), emptyBlock.prevBlockHeaderhash, "CreateBlock(attributes) should return a block with the same attributes")
	assert.Equal(t, b.Timestamp(), emptyBlock.timestamp, "CreateBlock(attributes) should return a block with the same attributes")
	assert.Len(t, b.Transactions(), 1, "put in 0 transactions - but the CoinbaseTxn should be in the block")

	bTx := NewBlock(blockWithTxs)
	assert.Len(t, bTx.Transactions(), 2, "This Block should have Coinbase and Transfer Transactions")
}

func TestBlockJSONSerialization(t *testing.T) {
	b1 := NewBlock()
	b2 := NewBlock(blockWithTxs)
	b1Json, _ := b1.JSON()
	b2Json, _ := b2.JSON()

	assert.NotEqual(t, b1Json, b2Json, "b2 should be different from b1 at this point")

	// Given that b1 and b2 are distinct: if b2.FromJSON(b1Json), now b2 should be the same as b1
	b2.FromJSON(b1Json)
	b2CopiedFromb1Json, _ := b2.JSON()
	assert.Equal(t, b1Json, b2CopiedFromb1Json, "b1 and b2 should now be the same (but not deeply equal)")
}

func TestBlockSerialization(t *testing.T) {
	b1 := NewBlock()
	b1Serialized, _ := b1.Serialize()

	b2, _ := block.DeSerializeBlock(b1Serialized)
	b2CopiedFromb1Serialized, _ := b2.Serialize()
	assert.Equal(t, b1Serialized, b2CopiedFromb1Serialized, "b1 and b2 should now be the same (but not deeply equal)")
}

func TestBlockValidate(t *testing.T) {
	b := NewBlock()
	parentBConfig := BlockConfig{
		minerAddress:        "Q010400d9f1efe5b272e042dcc8ef690f0e90ca8b0b6edba0d26f81e7aff12a6754b21788169f7f",
		blockNumber:         uint64(1),
		prevBlockHeaderhash: []byte("Unpublished Sci-Fi Book Title"),
		prevBlockTimestamp:  uint64(0),
		txs:                 []transactions.Transaction{},
		timestamp:           uint64(c.Genesis.GenesisTimestamp),
	}
	parentB := NewBlock(parentBConfig)

	var metaChildHeaderHashArray [][]byte
	childHeaderHash := []byte{84, 104}
	metaChildHeaderHashArray = append(metaChildHeaderHashArray, childHeaderHash)
	metaBlockDifficulty := make([]byte, 32)
	binary.BigEndian.PutUint64(metaBlockDifficulty, parentBConfig.timestamp)
	metaTotalDifficulty := make([]byte, 32)
	binary.BigEndian.PutUint64(metaBlockDifficulty, uint64(0))

	parentMetaData := metadata.CreateBlockMetadata(metaBlockDifficulty, metaTotalDifficulty, metaChildHeaderHashArray)

	mockNtp := new(MockNTP)
	mockNtp.On("Time").Return(1539008488)
	b.SetNTP(mockNtp)
	b.PBData().Header.HashHeaderPrev = parentB.HeaderHash()
	b.PBData().Header.HashHeader = b.GenerateHeaderHash()

	assert.True(t, b.Validate(nil, parentB, parentMetaData, uint64(0), nil))
	assert.False(t, b.Validate(parentB, parentB, parentMetaData, uint64(0), nil))
	assert.False(t, b.Validate(nil, nil, parentMetaData, uint64(0), nil))
}

func TestApplyStateChanges(t *testing.T) {
	qaddr := "Q010300a1da274e68c88b0ccf448e0b1916fa789b01eb2ed4e9ad565ce264c9390782a9c61ac02f"
	qaddrBin := misc.Qaddress2Bin(qaddr)
	a := make(map[string]*addressstate.AddressState)

	addressstateParams := struct {
		address            []byte
		nonce              uint64
		balance            uint64
		otsBitfield        [][]byte
		tokens             map[string]uint64
		slavePksAccessType map[string]uint32
		otsCounter         uint64
	}{
		qaddrBin,
		uint64(0),
		100000,
		make([][]byte, 1024),
		make(map[string]uint64),
		make(map[string]uint32),
		0,
	}
	a[misc.Bin2Qaddress(qaddrBin)] = addressstate.CreateAddressState(addressstateParams.address, addressstateParams.nonce, addressstateParams.balance, addressstateParams.otsBitfield, addressstateParams.tokens, addressstateParams.slavePksAccessType, addressstateParams.otsCounter)

	// Apply State Changes for a Block with only a Coinbase Transaction
	b := NewBlock()
	assert.True(t, b.ApplyStateChanges(a))

	// Apply State Changes for a Block with a Transfer Transaction
	bTxs := NewBlock(blockWithTxs)
	assert.True(t, bTxs.ApplyStateChanges(a))

	// Apply State Changes for a Block with an Invalid Transfer Transaction
	invalidTxJson := `{"fee":"1500","publicKey":"AQMAOOpjdQafgnLMGmYBs8dsIVGUVWA9NwA2uXx3mto1ZYVOOYO9VkKYxJri5/puKNS5VNjNWTmPEiWwjWFEhUruDg==","signature":"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","transactionHash":"D3Np8CFa+txFk7ZK+LO7ONupCwERihslTwl6Dnl264Q=","transfer":{"addrsTo":["AQMAHWXX5ZrtXvvq5kJG4PMYTXxCQRQh6zhbow8sHABahevEQZz9"],"amounts":["100"]}}`
	bTxsInvalidConfig := BuildBlockConfigWithTxs(invalidTxJson)
	bTxsInvalid := NewBlock(bTxsInvalidConfig)
	assert.False(t, bTxsInvalid.ApplyStateChanges(a))

	// Apply State Changes for a Block with an Valid Transfer Transaction, insufficient funds
	a[misc.Bin2Qaddress(qaddrBin)] = addressstate.CreateAddressState(addressstateParams.address, addressstateParams.nonce, 0, addressstateParams.otsBitfield, addressstateParams.tokens, addressstateParams.slavePksAccessType, addressstateParams.otsCounter)
	assert.False(t, bTxs.ApplyStateChanges(a))

	// Apply State Changes for a Block with an Valid Transfer Transaction, invalid transaction nonce (the same thing as AddressState nonce)
	a[misc.Bin2Qaddress(qaddrBin)] = addressstate.CreateAddressState(addressstateParams.address, 5, addressstateParams.balance, addressstateParams.otsBitfield, addressstateParams.tokens, addressstateParams.slavePksAccessType, addressstateParams.otsCounter)
	assert.False(t, bTxs.ApplyStateChanges(a))

	// Apply State Changes for a Block with a Valid Transfer Transaction, but with reused OTS key
	var bitfield = make([][]byte, 1024)
	for i := 0; i < 1024; i++ {
		bitfield[i] = make([]byte, 8)
		for j := 0; j < 8; j++ {
			bitfield[i][j] = 1
		}
	}
	a[misc.Bin2Qaddress(qaddrBin)] = addressstate.CreateAddressState(addressstateParams.address, addressstateParams.nonce, addressstateParams.balance, bitfield, addressstateParams.tokens, addressstateParams.slavePksAccessType, addressstateParams.otsCounter)
	assert.False(t, bTxs.ApplyStateChanges(a))
}

func TestPrepareAddressesList(t *testing.T) {
	b := NewBlock(blockWithTxs)
	assert.Len(t, b.PrepareAddressesList(), 4, "The Block has 1 Coinbase Transaction to the miner, and 1 Transfer Transaction from one party to another, so 4 affected addresses in total.")
}
