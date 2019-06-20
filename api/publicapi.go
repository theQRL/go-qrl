package api

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/rs/cors"
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
	Height uint64 `json:"height"`
}

type GetEstimatedNetworkFeeResponse struct {
	Fee string `json:"fee"`
}

type GetVersionResponse struct {
	Version string `json:"version"`
}

type Response struct {
	Error        uint        `json:"error"`
	ErrorMessage string      `json:"errorMessage"`
	Data         interface{} `json:"data"`
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
	router.HandleFunc("/api/GetVersion", p.GetVersion).Methods("GET")
	router.HandleFunc("/api/GetBlockByNumber", p.GetBlockByNumber).Methods("GET")
	router.HandleFunc("/api/GetBlockByHash", p.GetBlockByHash).Methods("GET")
	router.HandleFunc("/api/GetLastBlock", p.GetLastBlock).Methods("GET")
	router.HandleFunc("/api/GetTxByHash", p.GetTxByHash).Methods("GET")
	router.HandleFunc("/api/GetAddressState", p.GetAddressState).Methods("GET")
	router.HandleFunc("/api/GetBalance", p.GetBalance).Methods("GET")
	router.HandleFunc("/api/GetUnusedOTS", p.GetUnusedOTSIndex).Methods("GET")
	router.HandleFunc("/api/GetEstimatedNetworkFee", p.GetEstimatedNetworkFee).Methods("GET")
	router.HandleFunc("/api/BroadcastTransferTx", p.BroadcastTransferTx).Methods("POST")
	router.HandleFunc("/api/GetHeight", p.GetHeight).Methods("GET")
	//handler := cors.Default().Handler(router)
	co := cors.New(cors.Options{
		AllowedOrigins: []string{"*"}, //you service is available and allowed for this base url
		AllowedMethods: []string{
			http.MethodGet,//http methods for your app
			http.MethodPost,
			http.MethodPut,
			http.MethodPatch,
			http.MethodDelete,
			http.MethodOptions,
			http.MethodHead,
			"post",
			"*",
			"*/*",
		},

		AllowedHeaders: []string{
			"*",//or you can your header key values which you are using in your application

		},
	})
	router.StrictSlash(false)
	err := http.ListenAndServe(fmt.Sprintf("%s:%d", c.User.API.PublicAPI.Host, c.User.API.PublicAPI.Port), co.Handler(router))
	if err != nil {

	}
	return nil
}

func (p *PublicAPIServer) prepareResponse(errorCode uint, errorMessage string, data interface{}) *Response {
	r := &Response{
		Error: errorCode,
		ErrorMessage: errorMessage,
		Data: data,
	}
	return r
}

func (p *PublicAPIServer) GetVersion(w http.ResponseWriter, r *http.Request) {
	// Check Rate Limit
	if !p.visitors.isAllowed(r.RemoteAddr) {
		w.WriteHeader(429)
		return
	}
	fmt.Println("Get version called");
	getVersionResponse := &GetVersionResponse{
		Version: p.config.Dev.Version,
	}
	json.NewEncoder(w).Encode(p.prepareResponse(0, "", getVersionResponse))
}

func (p *PublicAPIServer) GetBlockByNumber(w http.ResponseWriter, r *http.Request) {
	// Check Rate Limit
	if !p.visitors.isAllowed(r.RemoteAddr) {
		w.WriteHeader(429)
		return
	}
	param, found := r.URL.Query()["blockNumber"]
	if !found {
		json.NewEncoder(w).Encode(p.prepareResponse(1,
			"blockNumber not found in parameter",
			nil))
		return
	}
	blockNumber, err := strconv.ParseUint(param[0], 10, 64)
	if err != nil {
		json.NewEncoder(w).Encode(p.prepareResponse(1,
			"Failed to Parse blockNumber",
			nil))
		return
	}
	b, err := p.chain.GetBlockByNumber(blockNumber)
	response := &block.PlainBlock{}
	response.BlockFromPBData(b.PBData())
	json.NewEncoder(w).Encode(p.prepareResponse(0, "", response))
}

func (p *PublicAPIServer) GetBlockByHash(w http.ResponseWriter, r *http.Request) {
	// Check Rate Limit
	if !p.visitors.isAllowed(r.RemoteAddr) {
		w.WriteHeader(429)
		return
	}
	param, found := r.URL.Query()["headerHash"]
	if !found {
		json.NewEncoder(w).Encode(p.prepareResponse(1,
			"headerHash not found in parameter",
			nil))
		return
	}
	headerHash, err := hex.DecodeString(param[0])
	if err != nil {
		json.NewEncoder(w).Encode(p.prepareResponse(1,
			fmt.Sprintf("Error Decoding headerHash\n %s", err.Error()),
			nil))
		return
	}
	b, err := p.chain.GetBlock(headerHash)
	if err != nil {
		json.NewEncoder(w).Encode(p.prepareResponse(1,
			fmt.Sprintf("Error in GetBlock for headerHash %s\n %s", param[0], err.Error()),
			nil))
		return
	}
	response := &block.PlainBlock{}
	response.BlockFromPBData(b.PBData())
	json.NewEncoder(w).Encode(p.prepareResponse(0,
		"",
		response))
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
	json.NewEncoder(w).Encode(p.prepareResponse(0,
		"",
		response))
}

func (p *PublicAPIServer) GetTxByHash(w http.ResponseWriter, r *http.Request) {
	// Check Rate Limit
	if !p.visitors.isAllowed(r.RemoteAddr) {
		w.WriteHeader(429)
		return
	}
	param, found := r.URL.Query()["txHash"]
	if !found {
		json.NewEncoder(w).Encode(p.prepareResponse(1,
			"txHash not found in parameter",
			nil))
		return
	}
	txHash, err := hex.DecodeString(param[0])
	if err != nil {
		json.NewEncoder(w).Encode(p.prepareResponse(1,
			fmt.Sprintf("Error Decoding txHash\n %s", err.Error()),
			nil))
		return
	}
	tx, err := p.chain.GetTransactionByHash(txHash)
	if err != nil {
		json.NewEncoder(w).Encode(p.prepareResponse(1,
			fmt.Sprintf("Error in GetTransactionByHash for txHash %s\n %s", param[0], err.Error()),
			nil))
		return
	}
	response := transactions.ProtoToPlainTransaction(tx)
	json.NewEncoder(w).Encode(p.prepareResponse(0,
		"",
		response))
}

func (p *PublicAPIServer) GetAddressState(w http.ResponseWriter, r *http.Request) {
	// Check Rate Limit
	if !p.visitors.isAllowed(r.RemoteAddr) {
		w.WriteHeader(429)
		return
	}
	param, found := r.URL.Query()["address"]
	if !found {
		json.NewEncoder(w).Encode(p.prepareResponse(1,
			"address not found in parameter",
			nil))
		return
	}
	addressState, err := p.chain.GetAddressState(misc.Qaddress2Bin(param[0]))
	if err != nil {
		json.NewEncoder(w).Encode(p.prepareResponse(1,
			fmt.Sprintf("Error Decoding address %s\n %s", param[0], err.Error()),
			nil))
		return
	}
	a := &addressstate.PlainAddressState{}
	a.AddressStateFromPBData(addressState.PBData())
	response, err := NewAddressStateResponse(a, p.chain)
	if err != nil {
		json.NewEncoder(w).Encode(p.prepareResponse(1,
			fmt.Sprintf("Error in NewAddressStateResponse %s", err.Error()),
			nil))
		return
	}
	json.NewEncoder(w).Encode(p.prepareResponse(0,
		"",
		response))
}

func (p *PublicAPIServer) GetBalance(w http.ResponseWriter, r *http.Request) {
	// Check Rate Limit
	if !p.visitors.isAllowed(r.RemoteAddr) {
		w.WriteHeader(429)
		return
	}
	param, found := r.URL.Query()["address"]
	if !found {
		json.NewEncoder(w).Encode(p.prepareResponse(1,
			"address not found in parameter",
			nil))
		return
	}
	addressState, err := p.chain.GetAddressState(misc.Qaddress2Bin(param[0]))
	if err != nil {
		json.NewEncoder(w).Encode(p.prepareResponse(1,
			fmt.Sprintf("Error Decoding address %s\n %s", param[0], err.Error()),
			nil))
		return
	}
	response := addressstate.PlainBalance{}
	response.Balance = strconv.FormatUint(addressState.Balance(), 10)
	json.NewEncoder(w).Encode(p.prepareResponse(0,
		"",
		response))
}

func (p *PublicAPIServer) GetUnusedOTSIndex(w http.ResponseWriter, r *http.Request) {
	// Check Rate Limit
	if !p.visitors.isAllowed(r.RemoteAddr) {
		w.WriteHeader(429)
		return
	}
	var err error
	startFrom := uint64(0)
	param, found := r.URL.Query()["startFrom"]
	if found {
		startFrom, err = strconv.ParseUint(param[0], 10, 64)
		if err != nil {
			json.NewEncoder(w).Encode(err.Error())
			return
		}
	}
	param, found = r.URL.Query()["address"]
	if !found {
		json.NewEncoder(w).Encode(p.prepareResponse(1,
			"address not found in parameter",
			nil))
		return
	}
	addressState, err := p.chain.GetAddressState(misc.Qaddress2Bin(param[0]))
	if err != nil {
		json.NewEncoder(w).Encode(p.prepareResponse(1,
			fmt.Sprintf("Error Decoding address %s\n %s", param[0], err.Error()),
			nil))
		return
	}
	unusedOTSIndex, found := addressState.GetUnusedOTSIndex(uint64(startFrom))
	response := addressstate.NextUnusedOTS{}
	response.UnusedOTSIndex = unusedOTSIndex
	response.Found = found
	json.NewEncoder(w).Encode(p.prepareResponse(0,
		"",
		response))
}

func (p *PublicAPIServer) GetHeight(w http.ResponseWriter, r *http.Request) {
	// Check Rate Limit
	if !p.visitors.isAllowed(r.RemoteAddr) {
		w.WriteHeader(429)
		return
	}
	response := &GetHeightResponse{Height:p.chain.Height()}
	json.NewEncoder(w).Encode(p.prepareResponse(0,
		"",
		response))
}

func (p *PublicAPIServer) GetNetworkStats(w http.ResponseWriter, r *http.Request) {
	// Check Rate Limit
	if !p.visitors.isAllowed(r.RemoteAddr) {
		w.WriteHeader(429)
		return
	}
}

func (p *PublicAPIServer) GetEstimatedNetworkFee(w http.ResponseWriter, r *http.Request) {
	// Check Rate Limit
	if !p.visitors.isAllowed(r.RemoteAddr) {
		w.WriteHeader(429)
		return
	}
	// TODO: Fee needs to be calcuated by mean, median or mode
	response := &GetEstimatedNetworkFeeResponse{Fee:"1"}
	json.NewEncoder(w).Encode(p.prepareResponse(0,
		"",
		response))
}

func (p *PublicAPIServer) BroadcastTransferTx(w http.ResponseWriter, r *http.Request) {
	// Check Rate Limit
	if !p.visitors.isAllowed(r.RemoteAddr) {
		w.WriteHeader(429)
		return
	}
	//buf, _ := ioutil.ReadAll(r.Body);
	//rdr1 := ioutil.NopCloser(bytes.NewBuffer(buf))
	//fmt.Printf("%q", rdr1);
	decoder := json.NewDecoder(r.Body)
	var jsonTransferTransactionRequest transactions.JSONTransferTransactionRequest
	err := decoder.Decode(&jsonTransferTransactionRequest)
	if err != nil {
		json.NewEncoder(w).Encode(p.prepareResponse(1,
			fmt.Sprintf("Error Decoding JSONTransferTransactionRequest \n%s", err.Error()),
			nil))
		return
	}
	plainTransferTransaction, err := jsonTransferTransactionRequest.ToPlainTransferTransaction()
	if err != nil {
		json.NewEncoder(w).Encode(p.prepareResponse(1,
			fmt.Sprintf("Error Parsing PlainTransferTransaction from JSONTransferTransactionRequest \n%s", err.Error()),
			nil))
		return
	}
	tx, err := plainTransferTransaction.ToTransferTransactionObject()
	if err != nil {
		json.NewEncoder(w).Encode(p.prepareResponse(1,
			fmt.Sprintf("Error parsing ToTransferTransactionObject\n %s", err.Error()),
			nil))
		return
	}
	if !tx.Validate(true) {
		json.NewEncoder(w).Encode(p.prepareResponse(1,
			"Transaction Validation Failed",
			nil))
		return
	}
	addrFromState, err := p.chain.GetAddressState(tx.AddrFrom())
	if err != nil {
		json.NewEncoder(w).Encode(p.prepareResponse(1,
			fmt.Sprintf("Error Getting Address State for Source Address \n %s", err.Error()),
			nil))
		return
	}
	addrFromPKState := addrFromState
	addrFromPK := tx.GetSlave()
	if addrFromPK != nil {
		addrFromPKState, err = p.chain.GetAddressState(addrFromPK)
		if err != nil {
			json.NewEncoder(w).Encode(p.prepareResponse(1,
				fmt.Sprintf("Error Getting Address State for Slave Address \n %s", err.Error()),
				nil))
			return
		}
	}

	if false && !tx.ValidateExtended(addrFromState, addrFromPKState) {
		json.NewEncoder(w).Encode(p.prepareResponse(1,
			"Transfer Transaction ValidationExtended Failed",
			nil))
		return
	}

	err = p.chain.GetTxPool().Add(
		tx,
		p.chain.GetLastBlock().BlockNumber(),
		p.ntp.Time())
	if err != nil {
		json.NewEncoder(w).Encode(p.prepareResponse(1,
			fmt.Sprintf("Failed to Add Txn into txn pool \n %s", err.Error()),
			nil))
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
		json.NewEncoder(w).Encode(p.prepareResponse(1,
			"Transaction Broadcast Timeout",
			nil))
		return
	}
	response := &BroadcastTransferTransactionRespose{
		TransactionHash: misc.Bin2HStr(tx.Txhash()),
	}
	json.NewEncoder(w).Encode(p.prepareResponse(0,
		"",
		response))
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
