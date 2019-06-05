package api

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"github.com/theQRL/go-qrl/pkg/config"
	"github.com/theQRL/go-qrl/pkg/core/addressstate"
	"github.com/theQRL/go-qrl/pkg/core/block"
	"github.com/theQRL/go-qrl/pkg/core/chain"
	"github.com/theQRL/go-qrl/pkg/core/transactions"
	"github.com/theQRL/go-qrl/pkg/generated"
	"github.com/theQRL/go-qrl/pkg/log"
	"github.com/theQRL/go-qrl/pkg/misc"
	"github.com/theQRL/go-qrl/pkg/ntp"
	"github.com/theQRL/go-qrl/pkg/p2p/messages"
	"net/http"
	"strconv"
	"time"
)

type GetHeightResponse struct {
	Height uint64  `json:"height"`
}

type PublicAPIServer struct {
	chain                    *chain.Chain
	ntp                      ntp.NTPInterface
	config                   *config.Config
	log                      log.LoggerInterface
	visitors                 *visitors
	registerAndBroadcastChan chan *messages.RegisterMessage
}

func (p *PublicAPIServer) Start() error {
	c := config.GetConfig()

	router := mux.NewRouter()
	router.HandleFunc("/api/GetBlockByNumber", p.GetBlockByNumber).Methods("GET")
	router.HandleFunc("/api/GetBlockByHash", p.GetBlockByHash).Methods("GET")
	router.HandleFunc("/api/GetLastBlock", p.GetLastBlock).Methods("GET")
	router.HandleFunc("/api/GetTxByHash", p.GetTxByHash).Methods("GET")
	router.HandleFunc("/api/GetAddressState", p.GetAddressState).Methods("GET")
	router.HandleFunc("/api/GetBalance", p.GetBalance).Methods("GET")
	router.HandleFunc("/api/GetUnusedOTS", p.GetUnusedOTSIndex).Methods("GET")
	//router.HandleFunc("/api/GetEstimatedTxFee", p.GetEstimatedTxFee).Methods("GET")
	router.HandleFunc("/api/BroadcastTransferTx", p.BroadcastTransferTx).Methods("POST")
	router.HandleFunc("/api/GetHeight", p.GetHeight).Methods("GET")

	allowedOrigins := handlers.AllowedOrigins([]string{"*"})
	allowedMethods := handlers.AllowedMethods([]string{"POST"})

	err := http.ListenAndServe(fmt.Sprintf("%s:%d", c.User.API.PublicAPI.Host, c.User.API.PublicAPI.Port), handlers.CORS(allowedOrigins, allowedMethods)(router))
	if err != nil {

	}
	return nil
}

func (p *PublicAPIServer) GetBlockByNumber(w http.ResponseWriter, r *http.Request) {
	// Check Rate Limit
	if !p.visitors.isAllowed(r.RemoteAddr) {
		w.WriteHeader(429)
		return
	}
	param, found := r.URL.Query()["blocknumber"]
	if !found {
		json.NewEncoder(w).Encode(nil)
		return
	}
	blockNumber, err := strconv.ParseUint(param[0], 10, 64)
	if err != nil {
		json.NewEncoder(w).Encode(nil)
		return
	}
	b, err := p.chain.GetBlockByNumber(blockNumber)
	response := &block.PlainBlock{}
	response.BlockFromPBData(b.PBData())
	json.NewEncoder(w).Encode(response)
}

func (p *PublicAPIServer) GetBlockByHash(w http.ResponseWriter, r *http.Request) {
	// Check Rate Limit
	if !p.visitors.isAllowed(r.RemoteAddr) {
		w.WriteHeader(429)
		return
	}
	param, found := r.URL.Query()["headerhash"]
	if !found {
		json.NewEncoder(w).Encode(nil)
		return
	}
	headerHash, err := hex.DecodeString(param[0])
	if err != nil {
		json.NewEncoder(w).Encode(err.Error())
		return
	}
	b, err := p.chain.GetBlock(headerHash)
	if err != nil {
		json.NewEncoder(w).Encode(err.Error())
		return
	}
	response := &block.PlainBlock{}
	response.BlockFromPBData(b.PBData())
	json.NewEncoder(w).Encode(response)
}

func (p *PublicAPIServer) GetLastBlock(w http.ResponseWriter, r *http.Request) {
	// Check Rate Limit
	if !p.visitors.isAllowed(r.RemoteAddr) {
		w.WriteHeader(429)
		return
	}
	b := p.chain.GetLastBlock()

	response := &block.PlainBlock{}
	response.BlockFromPBData(b.PBData())
	json.NewEncoder(w).Encode(response)
}

func (p *PublicAPIServer) GetTxByHash(w http.ResponseWriter, r *http.Request) {
	// Check Rate Limit
	if !p.visitors.isAllowed(r.RemoteAddr) {
		w.WriteHeader(429)
		return
	}
	param, found := r.URL.Query()["txhash"]
	if !found {
		json.NewEncoder(w).Encode(nil)
		return
	}
	txHash, err := hex.DecodeString(param[0])
	if err != nil {
		json.NewEncoder(w).Encode(err.Error())
		return
	}
	tx, err := p.chain.GetTransactionByHash(txHash)
	if err != nil {
		json.NewEncoder(w).Encode(err.Error())
		return
	}
	response := transactions.ProtoToPlainTransaction(tx)
	json.NewEncoder(w).Encode(response)
}

func (p *PublicAPIServer) GetAddressState(w http.ResponseWriter, r *http.Request) {
	// Check Rate Limit
	if !p.visitors.isAllowed(r.RemoteAddr) {
		w.WriteHeader(429)
		return
	}
	param, found := r.URL.Query()["address"]
	if !found {
		json.NewEncoder(w).Encode(nil)
		return
	}
	addressState, err := p.chain.GetAddressState(misc.Qaddress2Bin(param[0]))
	if err != nil {
		json.NewEncoder(w).Encode(err.Error())
		return
	}
	response := addressstate.PlainAddressState{}
	response.AddressStateFromPBData(addressState.PBData())
	json.NewEncoder(w).Encode(response)
}

func (p *PublicAPIServer) GetBalance(w http.ResponseWriter, r *http.Request) {
	// Check Rate Limit
	if !p.visitors.isAllowed(r.RemoteAddr) {
		w.WriteHeader(429)
		return
	}
	param, found := r.URL.Query()["address"]
	if !found {
		json.NewEncoder(w).Encode(nil)
		return
	}
	addressState, err := p.chain.GetAddressState(misc.Qaddress2Bin(param[0]))
	if err != nil {
		json.NewEncoder(w).Encode(err.Error())
		return
	}
	response := addressstate.PlainBalance{}
	response.Balance = addressState.Balance()
	json.NewEncoder(w).Encode(response)
}

func (p *PublicAPIServer) GetUnusedOTSIndex(w http.ResponseWriter, r *http.Request) {
	// Check Rate Limit
	if !p.visitors.isAllowed(r.RemoteAddr) {
		w.WriteHeader(429)
		return
	}
	var err error
	startFrom := uint64(0)
	param, found := r.URL.Query()["start_from"]
	if found {
		startFrom, err = strconv.ParseUint(param[0], 10, 64)
		if err != nil {
			json.NewEncoder(w).Encode(err.Error())
			return
		}
	}
	addressState, err := p.chain.GetAddressState(misc.Qaddress2Bin(param[0]))
	if err != nil {
		json.NewEncoder(w).Encode(err.Error())
		return
	}
	unusedOTSIndex, found := addressState.GetUnusedOTSIndex(uint64(startFrom))
	response := addressstate.NextUnusedOTS{}
	response.UnusedOTSIndex = unusedOTSIndex
	response.Found = found
	json.NewEncoder(w).Encode(response)
}

func (p *PublicAPIServer) GetHeight(w http.ResponseWriter, r *http.Request) {
	// Check Rate Limit
	if !p.visitors.isAllowed(r.RemoteAddr) {
		w.WriteHeader(429)
		return
	}
	resp := &GetHeightResponse{Height:p.chain.Height()}
	json.NewEncoder(w).Encode(resp)
}

func (p *PublicAPIServer) GetNetworkStats(w http.ResponseWriter, r *http.Request) {
	// Check Rate Limit
	if !p.visitors.isAllowed(r.RemoteAddr) {
		w.WriteHeader(429)
		return
	}
}

func (p *PublicAPIServer) BroadcastTransferTx(w http.ResponseWriter, r *http.Request) {
	// Check Rate Limit
	if !p.visitors.isAllowed(r.RemoteAddr) {
		w.WriteHeader(429)
		return
	}
	decoder := json.NewDecoder(r.Body)
	var plainTransferTransaction transactions.PlainTransferTransaction
	err := decoder.Decode(&plainTransferTransaction)
	if err != nil {
		json.NewEncoder(w).Encode(err.Error())
		return
	}
	tx, err := plainTransferTransaction.ToTransferTransactionObject()
	if err != nil {
		json.NewEncoder(w).Encode(err.Error())
		return
	}
	if !tx.Validate(true) {
		json.NewEncoder(w).Encode(errors.New("Transfer Transaction Validation Failed").Error())
		return
	}
	addrFromState, err := p.chain.GetAddressState(tx.AddrFrom())
	if err != nil {
		json.NewEncoder(w).Encode(err.Error())
		return
	}
	addrFromPKState := addrFromState
	addrFromPK := tx.GetSlave()
	if addrFromPK != nil {
		addrFromPKState, err = p.chain.GetAddressState(addrFromPK)
		if err != nil {
			json.NewEncoder(w).Encode(err.Error())
			return
		}
	}

	if !tx.ValidateExtended(addrFromState, addrFromPKState) {
		json.NewEncoder(w).Encode(errors.New("Transfer Transaction ValidateExtended Failed").Error())
		return
	}

	err = p.chain.GetTxPool().Add(
		tx,
		p.chain.GetLastBlock().BlockNumber(),
		p.ntp.Time())
	if err != nil {
		json.NewEncoder(w).Encode(err.Error())
		return
	}
	msg2 := &generated.Message{
		Msg:&generated.LegacyMessage_TtData{
			TtData:tx.PBData(),
		},
		MessageType:generated.LegacyMessage_TT,
	}
	registerMessage := &messages.RegisterMessage{
		MsgHash:misc.Bin2HStr(tx.Txhash()),
		Msg:msg2,
	}
	select {
	case p.registerAndBroadcastChan <- registerMessage:
	case <-time.After(10*time.Second):
		json.NewEncoder(w).Encode(errors.New("Transaction Broadcast Timeout").Error())
	}
	json.NewEncoder(w).Encode("Successful")
}

func NewPublicAPIServer(c *chain.Chain, registerAndBroadcastChan chan *messages.RegisterMessage) *PublicAPIServer {
	return &PublicAPIServer{
		chain:                    c,
		ntp:                      ntp.GetNTP(),
		config:                   config.GetConfig(),
		log:                      log.GetLogger(),
		visitors:                 newVisitors(),
		registerAndBroadcastChan: registerAndBroadcastChan,
	}
}
