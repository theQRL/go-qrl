package api

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/theQRL/go-qrl/pkg/config"
	"github.com/theQRL/go-qrl/pkg/core/chain"
	"github.com/theQRL/go-qrl/pkg/log"
	"github.com/theQRL/go-qrl/pkg/misc"
	"github.com/theQRL/go-qrl/pkg/p2p/messages"
	"github.com/theQRL/go-qrl/pkg/pow/miner"
	"io"
	"net/http"
	"strconv"
)

type MiningAPIServer struct {
	chain  *chain.Chain
	miner  *miner.Miner
	config *config.Config
	log    log.LoggerInterface
}

func (m *MiningAPIServer) Start() error {
	c := config.GetConfig()

	router := mux.NewRouter()
	router.HandleFunc("/json_rpc", m.Handler).Methods("POST")

	allowedOrigins := handlers.AllowedOrigins([]string{"*"})
	allowedMethods := handlers.AllowedMethods([]string{"POST"})


	err := http.ListenAndServe(fmt.Sprintf("%s:%d", c.User.API.MiningAPI.Host, c.User.API.MiningAPI.Port), handlers.CORS(allowedOrigins, allowedMethods)(router))
	if err != nil {

	}

	return nil
}

type BlockHeader struct {
	Difficulty uint64  `json:"difficulty"`
	Height     uint64  `json:"height"`
	Timestamp  uint64  `json:"timestamp"`
	Reward     uint64  `json:"reward"`
	Hash       string  `json:"hash"`
	Depth      uint64  `json:"depth"`
}

type BlockTemplateRequest struct {
	ReserveSize   uint64 `json:"reserve_size"`
	WalletAddress string `json:"wallet_address"`
}

type BlockTemplate struct {
	Blob           string `json:"blocktemplate_blob"`
	Difficulty     uint64 `json:"difficulty"`
	Height         uint64 `json:"height"`
	ReservedOffset uint32 `json:"reserved_offset"`
	Status         string `json:"status"`
}

type BlockHeaderRequest struct {
	Height uint64 `json:"height"`
}

type BlockHeaderResponse struct {
	BlockHeader *BlockHeader `json:"block_header"`
	Status      string       `json:"status"`
}

type SubmitBlockResponse struct {
	err bool `json:"error"`
}

type RPCRequest struct {
	Id      string `json:"id"`
	JsonRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type RPCResponse struct {
	Result interface{} `json:"result"`
	Id string `json:"id"`
	JsonRPC string `json:"jsonrpc"`
}

func (m *MiningAPIServer) GetLastBlockHeader(w http.ResponseWriter) {
	b := m.chain.GetLastBlock()
	bh := b.BlockHeader()
	blockMetaData, err := m.chain.GetBlockMetaData(bh.HeaderHash())
	if err != nil {
		m.log.Error("[GetLastBlockHeader] Error while fetching BlockMetaData",
			"Error", err.Error())
		return
	}
	difficulty, err := strconv.ParseUint(misc.Bin2HStr(blockMetaData.BlockDifficulty()), 16, 64)
	if err != nil {
		m.log.Error("[GetLastBlockHeader] Error while parsing BlockDifficulty",
			"BlockDifficulty", blockMetaData.BlockDifficulty(),
			"Error", err.Error())
		return
	}
	blockHeader := &BlockHeader{
		Difficulty: difficulty,
		Height: bh.BlockNumber(),
		Timestamp: bh.Timestamp(),
		Reward: bh.BlockReward() + bh.FeeReward(),
		Hash: misc.Bin2HStr(bh.HeaderHash()),
		Depth: 1,
	}
	blockHeaderResponse := &BlockHeaderResponse{
		BlockHeader: blockHeader,
		Status: "OK",
	}
	response := &RPCResponse{
		Result:blockHeaderResponse,
		Id: "0",
		JsonRPC: "2.0",
	}

	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		m.log.Error("[GetLastBlockHeader] Error while Encoding Response",
			"response", blockHeaderResponse,
			"Error", err.Error())
		return
	}
}

func (m *MiningAPIServer) GetBlockHeaderByHeight(height uint64, w http.ResponseWriter) {
	b, err := m.chain.GetBlockByNumber(height)
	if err != nil {
		m.log.Error("[GetLastBlockHeader] Error by GetBlockByNumber",
			"height", height,
			"Error", err.Error())
		return
	}
	bh := b.BlockHeader()
	blockMetaData, err := m.chain.GetBlockMetaData(bh.HeaderHash())
	if err != nil {
		m.log.Error("[GetLastBlockHeader] Error while fetching BlockMetaData",
			"Error", err.Error())
		return
	}
	difficulty, err := strconv.ParseUint(misc.Bin2HStr(blockMetaData.BlockDifficulty()), 16, 64)
	if err != nil {
		m.log.Error("[GetLastBlockHeader] Error while parsing BlockDifficulty",
			"BlockDifficulty", blockMetaData.BlockDifficulty(),
			"Error", err.Error())
		return
	}
	blockHeader := &BlockHeader{
		Difficulty: difficulty,
		Height: bh.BlockNumber(),
		Timestamp: bh.Timestamp(),
		Reward: bh.BlockReward() + bh.FeeReward(),
		Hash: misc.Bin2HStr(bh.HeaderHash()),
		Depth: 1,
	}
	blockHeaderResponse := &BlockHeaderResponse{
		BlockHeader: blockHeader,
		Status: "OK",
	}
	response := &RPCResponse{
		Result:blockHeaderResponse,
		Id: "0",
		JsonRPC: "2.0",
	}
	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		m.log.Error("[GetBlockHeaderByHeight] Error while Encoding Response",
			"response", blockHeaderResponse,
			"Error", err.Error())
		return
	}
}

func (m *MiningAPIServer) GetBlockTemplate(walletAddress string, w http.ResponseWriter) {
	lastBlock := m.chain.GetLastBlock()
	blockMetaData, err := m.chain.GetBlockMetaData(lastBlock.HeaderHash())
	txPool := m.chain.GetTransactionPool()
	blob, currentDifficulty, err := m.miner.GetBlockToMine(walletAddress, lastBlock, blockMetaData.BlockDifficulty(), txPool)

	blockTemplate := &BlockTemplate{
		Blob: blob,
		Difficulty: currentDifficulty,
		Height: lastBlock.BlockNumber() + 1,
		ReservedOffset: uint32(m.config.Dev.ExtraNonceOffset),
		Status: "OK",
	}
	response := &RPCResponse{
		Result: blockTemplate,
		Id: "0",
		JsonRPC: "2.0",
	}
	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		m.log.Error("[GetBlockTemplate] Error while Encoding Response",
			"response", response,
			"Error", err.Error())
		return
	}
}

func (m *MiningAPIServer) SubmitBlock(blob string, w http.ResponseWriter) {
	submitBlockResponse := &SubmitBlockResponse{
		err:m.miner.SubmitMinedBlock(misc.HStr2Bin(blob)),
	}
	response := &RPCResponse{
		Result: submitBlockResponse,
		Id: "0",
		JsonRPC: "2.0",
	}
	err := json.NewEncoder(w).Encode(response)
	if err != nil {
		m.log.Error("[SubmitBlock] Error while Encoding Response",
			"response", response,
			"Error", err.Error())
		return
	}
}

func (m *MiningAPIServer) Transfer(w http.ResponseWriter, req *http.Request) {
	// TODO: Automatic Slave Support
}

func (m *MiningAPIServer) Handler(w http.ResponseWriter, req *http.Request) {
	byteBody := make([]byte, req.ContentLength)
	_, err := req.Body.Read(byteBody)
	if err != io.EOF && err != nil {
		m.log.Error("Error while reading Body",
			"ContentLength", req.ContentLength,
			"Error", err.Error())
		return
	}
	rpcRequest := &RPCRequest{}

	err = json.Unmarshal(byteBody, rpcRequest)
	if err != nil {
		m.log.Error("Error while Decoding JSON Body",
			"Error", err.Error())
		return
	}
	method := rpcRequest.Method
	fmt.Println(rpcRequest)
	fmt.Println(string(byteBody))
	switch method {
	case "getlastblockheader":
		m.GetLastBlockHeader(w)
	case "getblockheaderbyheight":
		blockHeaderRequest := &BlockHeaderRequest{}
		err = json.Unmarshal(rpcRequest.Params, blockHeaderRequest)
		if err != nil {
			m.log.Error("Error while Decoding JSON blockHeaderRequest",
				"Error", err.Error())
			return
		}
		m.GetBlockHeaderByHeight(blockHeaderRequest.Height, w)
	case "getblocktemplate":
		blockTemplateRequest := &BlockTemplateRequest{}
		err = json.Unmarshal(rpcRequest.Params, blockTemplateRequest)
		if err != nil {
			m.log.Error("Error while Decoding JSON blockTemplateRequest",
				"Error", err.Error())
			return
		}
		m.GetBlockTemplate(blockTemplateRequest.WalletAddress, w)
	case "submitblock":
		var blob []string
		err = json.Unmarshal(rpcRequest.Params, &blob)
		if err != nil {
			m.log.Error("Error while Decoding JSON submitBlockRequest",
				"Error", err.Error())
			return
		}
		m.SubmitBlock(blob[0], w)
	default:
	}
}

func NewMiningAPIServer(c *chain.Chain, registerAndBroadcastChan chan *messages.RegisterMessage) *MiningAPIServer {
	return &MiningAPIServer{
		chain: c,
		miner: miner.CreateMiner(c, registerAndBroadcastChan),
		config: config.GetConfig(),
		log: log.GetLogger(),
	}
}
