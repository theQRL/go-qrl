package api

import (
	"fmt"
	"github.com/theQRL/go-qrl/pkg/core/addressstate"
	"github.com/theQRL/go-qrl/pkg/core/block"
	"github.com/theQRL/go-qrl/pkg/core/chain"
	"github.com/theQRL/go-qrl/pkg/generated"
	"github.com/theQRL/go-qrl/pkg/misc"
	"strconv"
	"time"
)

type BlockResponse struct {
	HeaderHash  string `json:"headerHash"`
	BlockNumber uint64 `json:"blockNumber"`
	Timestamp   string `json:"timestamp"`
}

type TransactionResponse interface {
	GetSignerAddress() string
}

type BroadcastTransferTransactionRespose struct {
	TransactionHash string `json:"transactionHash"`
}

type TransferTransactionResponse struct {
	AddressFrom     string         `json:"addressFrom"`
	SignerAddress   string         `json:"signerAddress"`
	Fee             string         `json:"fee"`
	PublicKey       string         `json:"publicKey"`
	Signature       string         `json:"signature"`
	Nonce           string         `json:"nonce"`
	TransactionHash string         `json:"transactionHash"`
	TransactionType string         `json:"transactionType"`
	AddressesTo     []string       `json:"addressesTo"`
	Amounts         []string       `json:"amounts"`
	TotalAmount     string         `json:"totalAmount"`
	Block           *BlockResponse `json:"block"`
}

type AddressStateResponse struct {
	Address      string                `json:"address" bson:"address"`
	Balance      string                `json:"balance" bson:"balance"`
	Nonce        string                `json:"nonce" bson:"nonce"`
	OtsBitfield  []string              `json:"otsBitField" bson:"otsBitField"`
	Transactions []TransactionResponse `json:"transactions" bson:"transactions"`
	OtsCounter   string                `json:"otsCounter" bson:"otsCounter"`
}

func (tr *TransferTransactionResponse) GetSignerAddress() string {
	return tr.SignerAddress
}

func NewAddressStateResponse(a *addressstate.PlainAddressState, c *chain.Chain) (*AddressStateResponse, error) {
	transactions := make([]TransactionResponse, len(a.TransactionHashes))
	for i := range a.TransactionHashes {
		tm, err := c.GetTransactionMetaDataByHash(misc.HStr2Bin(a.TransactionHashes[i]))
		if err != nil {
			return nil, err
		}
		b, err := c.GetBlockByNumber(tm.BlockNumber)
		switch tm.Transaction.TransactionType.(type) {
		case *generated.Transaction_Transfer_:
			transactions[i] = NewTransferTransactionResponse(tm, b)
		case *generated.Transaction_Coinbase:
			transactions[i] = NewCoinBaseTransactionResponse(tm, b)
		case *generated.Transaction_Token_:
		case *generated.Transaction_TransferToken_:
		case *generated.Transaction_Message_:
		case *generated.Transaction_Slave_:
		}

	}
	ar := &AddressStateResponse{
		Address: a.Address,
		Balance: misc.ShorToQuanta(a.Balance),
		Nonce: strconv.FormatUint(a.Nonce, 10),
		OtsBitfield: a.OtsBitfield,
		Transactions: transactions,
		OtsCounter: strconv.FormatUint(a.OtsCounter, 10),
	}
	return ar, nil
}

func NewTransferTransactionResponse(tm *generated.TransactionMetadata, b *block.Block) *TransferTransactionResponse {
	addressFrom := misc.PK2Qaddress(tm.Transaction.PublicKey)
	if tm.Transaction.MasterAddr != nil {
		addressFrom = misc.Bin2Qaddress(tm.Transaction.MasterAddr)
	}
	totalAmount := uint64(0)
	for _, amount := range tm.Transaction.GetTransfer().Amounts {
		totalAmount += amount
	}
	t := time.Unix(int64(b.Timestamp()), 0)
	formattedTimestamp := fmt.Sprintf("%d-%02d-%02dT%02d:%02d:%02d.000Z",
		t.Year(), t.Month(), t.Day(),
		t.Hour(), t.Minute(), t.Second())
	fmt.Println(formattedTimestamp)
	tr := &TransferTransactionResponse{
		AddressFrom: addressFrom,
		SignerAddress: misc.PK2Qaddress(tm.Transaction.PublicKey),
		Fee: misc.ShorToQuanta(tm.Transaction.Fee),
		PublicKey: misc.Bin2HStr(tm.Transaction.PublicKey),
		Signature: misc.Bin2HStr(tm.Transaction.Signature),
		Nonce: strconv.FormatUint(tm.Transaction.Nonce, 10),
		TransactionHash: misc.Bin2HStr(tm.Transaction.TransactionHash),
		AddressesTo: misc.Bin2QAddresses(tm.Transaction.GetTransfer().AddrsTo),
		Amounts: misc.ShorsToQuantas(tm.Transaction.GetTransfer().GetAmounts()),
		TotalAmount: misc.ShorToQuanta(totalAmount),
		Block: &BlockResponse{
			HeaderHash: misc.Bin2HStr(b.HeaderHash()),
			BlockNumber: b.BlockNumber(),
			Timestamp: formattedTimestamp,
		},
	}
	return tr
}

func NewCoinBaseTransactionResponse(tm *generated.TransactionMetadata, b *block.Block) *TransferTransactionResponse {
	addressFrom := misc.Bin2Qaddress(tm.Transaction.MasterAddr)
	t := time.Unix(int64(b.Timestamp()), 0)
	formattedTimestamp := fmt.Sprintf("%d-%02d-%02dT%02d:%02d:%02d.000Z",
		t.Year(), t.Month(), t.Day(),
		t.Hour(), t.Minute(), t.Second())
	tr := &TransferTransactionResponse{
		AddressFrom: addressFrom,
		Nonce: strconv.FormatUint(tm.Transaction.Nonce, 10),
		TransactionHash: misc.Bin2HStr(tm.Transaction.TransactionHash),
		AddressesTo: []string{misc.Bin2Qaddress(tm.Transaction.GetCoinbase().AddrTo)},
		Amounts: []string{misc.ShorToQuanta(tm.Transaction.GetCoinbase().Amount)},
		TotalAmount: misc.ShorToQuanta(tm.Transaction.GetCoinbase().Amount),
		Block: &BlockResponse{
			HeaderHash: misc.Bin2HStr(b.HeaderHash()),
			BlockNumber: b.BlockNumber(),
			Timestamp: formattedTimestamp,
		},
	}
	return tr
}