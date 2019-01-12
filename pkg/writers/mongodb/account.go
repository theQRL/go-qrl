package mongodb

import (
	"fmt"
	"github.com/mongodb/mongo-go-driver/bson"
	"github.com/theQRL/go-qrl/pkg/config"
	"github.com/theQRL/go-qrl/pkg/generated"
	"github.com/theQRL/go-qrl/pkg/misc"
)

type Account struct {
	Address     []byte   `json:"address" bson:"address"`
	Balance     int64    `json:"balance" bson:"balance"`
	Nonce       int64    `json:"nonce" bson:"nonce"`
	OTSBitfield [][]byte `json:"ots_bit_field" bson:"ots_bit_field"`
	OTSCounter  int64    `json:"ots_counter" bson:"ots_counter"`
	BlockNumber int64    `json:"block_number" bson:"block_number"`
}

func (a *Account) AccountFromPBData(pbData *generated.AddressState) {
	a.Address = pbData.Address
	a.Balance = int64(pbData.Balance)
	a.Nonce = int64(pbData.Nonce)
	a.OTSBitfield = pbData.OtsBitfield
	a.OTSCounter = int64(pbData.OtsCounter)
}

func (a *Account) AddBalance(balance uint64) {
	a.Balance += int64(balance)
}

func (a *Account) SubBalance(balance uint64) {
	a.Balance -= int64(balance)
}

func (a *Account) SetOTSKey(otsKeyIndex uint64) {
	c := config.GetConfig()
	if otsKeyIndex < c.Dev.MaxOTSTracking {
		offset := otsKeyIndex >> 3
		relative := otsKeyIndex % 8
		bitfield := a.OTSBitfield[offset]
		a.OTSBitfield[offset][0] = bitfield[0] | (1 << relative)
	} else {
		a.OTSCounter = int64(otsKeyIndex)
	}
}

func (a *Account) ResetOTSKey(otsKeyIndex uint64) {
	c := config.GetConfig()
	if otsKeyIndex < c.Dev.MaxOTSTracking {
		offset := otsKeyIndex >> 3
		relative := otsKeyIndex % 8
		bitfield := a.OTSBitfield[offset]
		a.OTSBitfield[offset][0] = bitfield[0] & ^(1 << relative)
	} else {
		fmt.Println("NO SUPPORT beyond MAX OTS Tracking")
	}
}

func CreateAccount(address []byte, blockNumber int64) *Account {
	a := &Account{}
	a.Address = address
	c := config.GetConfig()
	a.OTSBitfield = make([][]byte, c.Dev.OtsBitFieldSize)
	for i := 0; i < int(c.Dev.OtsBitFieldSize); i++ {
		a.OTSBitfield[i] = make([]byte, 8)
	}
	a.BlockNumber = blockNumber
	return a
}

func LoadAccount(m *MongoProcessor, address []byte, accounts map[string]*Account, blockNumber int64) {
	qAddress := misc.Bin2Qaddress(address)
	if _, ok := accounts[qAddress]; ok {
		return
	}
	cursor, err := m.accountsCollection.Find(m.ctx, bson.D{{"address", address}})
	if err != nil {
		//TODO: Do Something
		fmt.Println("Error while Finding Account", err)
	}
	accounts[qAddress] = CreateAccount(address, blockNumber)
	for cursor.Next(m.ctx) {
		err := cursor.Decode(accounts[qAddress])
		if err != nil {
			//TODO: Do Something
			fmt.Println("Error while Decoding Account", err)
		}
	}
}