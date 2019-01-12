package api

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/theQRL/go-qrl/pkg/config"
	"github.com/theQRL/go-qrl/pkg/core/addressstate"
	"github.com/theQRL/go-qrl/pkg/core/block"
	"github.com/theQRL/go-qrl/pkg/core/chain"
	"github.com/theQRL/go-qrl/pkg/core/transactions"
	"github.com/theQRL/go-qrl/pkg/log"
	"github.com/theQRL/go-qrl/pkg/misc"
	"net/http"
	"strconv"
)

type GetHeightResponse struct {
	Height uint64  `json:"height"`
}

type PublicAPIServer struct {
	chain  *chain.Chain
	config *config.Config
	log    log.LoggerInterface
}

func (p *PublicAPIServer) Start() error {
	c := config.GetConfig()

	router := mux.NewRouter()
	router.HandleFunc("/api/GetBlockByNumber", p.GetBlockByNumber).Methods("GET")
	router.HandleFunc("/api/GetBlockByHash", p.GetBlockByHash).Methods("GET")
	router.HandleFunc("/api/GetLastBlock", p.GetLastBlock).Methods("GET")
	router.HandleFunc("/api/GetTxByHash", p.GetTxByHash).Methods("GET")
	router.HandleFunc("/api/GetAddressState", p.GetAddressState).Methods("GET")
	router.HandleFunc("/api/GetHeight", p.GetHeight).Methods("GET")

	allowedOrigins := handlers.AllowedOrigins([]string{"*"})
	allowedMethods := handlers.AllowedMethods([]string{"POST"})

	err := http.ListenAndServe(fmt.Sprintf("%s:%d", c.User.API.PublicAPI.Host, c.User.API.PublicAPI.Port), handlers.CORS(allowedOrigins, allowedMethods)(router))
	if err != nil {

	}

	return nil
}

func (p *PublicAPIServer) GetBlockByNumber(w http.ResponseWriter, r *http.Request) {
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
	b := p.chain.GetLastBlock()

	response := &block.PlainBlock{}
	response.BlockFromPBData(b.PBData())
	json.NewEncoder(w).Encode(response)
}

func (p *PublicAPIServer) GetTxByHash(w http.ResponseWriter, r *http.Request) {
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

func (p *PublicAPIServer) GetHeight(w http.ResponseWriter, r *http.Request) {
	resp := &GetHeightResponse{Height:p.chain.Height()}
	json.NewEncoder(w).Encode(resp)
}

func (p *PublicAPIServer) GetNetworkStats(w http.ResponseWriter, r *http.Request) {

}

func (p *PublicAPIServer) PushRawTxn(w http.ResponseWriter, r *http.Request) {

}

func NewPublicAPIServer(c *chain.Chain) *PublicAPIServer {
	return &PublicAPIServer{
		chain: c,
		config: config.GetConfig(),
		log: log.GetLogger(),
	}
}
