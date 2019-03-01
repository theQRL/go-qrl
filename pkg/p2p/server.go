package p2p

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/theQRL/go-qrl/api"
	"github.com/theQRL/go-qrl/pkg/core/block"
	"github.com/theQRL/go-qrl/pkg/generated"
	"github.com/theQRL/go-qrl/pkg/misc"
	"github.com/theQRL/go-qrl/pkg/ntp"
	"github.com/theQRL/go-qrl/pkg/p2p/messages"
	"github.com/theQRL/go-qrl/pkg/pow/miner"
	"io/ioutil"
	"net"
	"os"
	"path"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/willf/bloom"

	"github.com/theQRL/go-qrl/pkg/config"
	"github.com/theQRL/go-qrl/pkg/core/chain"
	"github.com/theQRL/go-qrl/pkg/log"
)

type conn struct {
	fd      net.Conn
	inbound bool
}

type peerDrop struct {
	*Peer
	err       error
	requested bool // true if signaled by the peer
}

type PeerInfo struct {
	IP                      string `json:"IP"`
	Port                    uint16 `json:"Port"`
	LastConnectionTimestamp uint64 `json:"LastConnectionTimestamp"`
}

type PeersInfo struct {
	PeersInfo []PeerInfo `json:"PeersInfo"`
}

type Server struct {
	config *config.Config

	miner        *miner.Miner
	chain        *chain.Chain
	ntp          ntp.NTPInterface
	peersInfo    *PeersInfo
	ipCount      map[string]int
	inboundCount uint16

	listener     net.Listener
	lock         sync.Mutex
	peerInfoLock sync.Mutex

	running bool
	loopWG  sync.WaitGroup
	log     log.LoggerInterface

	exit                      chan struct{}
	connectPeersExit          chan struct{}
	mrDataConn                chan *MRDataConn
	blockAndPeerChan          chan *BlockAndPeer
	nodeHeaderHashAndPeerChan chan *NodeHeaderHashAndPeer
	addPeerToPeerList         chan *generated.PLData
	addpeer                   chan *conn
	delpeer                   chan *peerDrop
	registerAndBroadcastChan  chan *messages.RegisterMessage

	filter          *bloom.BloomFilter
	mr              *MessageReceipt
	downloader      *Downloader
	futureBlocks    map[string]*block.Block
	messagePriority map[generated.LegacyMessage_FuncName]uint64
}

func (srv *Server) Start(chain *chain.Chain) (err error) {
	srv.lock.Lock()
	defer srv.lock.Unlock()
	if srv.running {
		return errors.New("server is already running")
	}

	srv.config = config.GetConfig()
	srv.exit = make(chan struct{})
	srv.connectPeersExit = make(chan struct{})
	srv.addpeer = make(chan *conn)
	srv.delpeer = make(chan *peerDrop)
	srv.registerAndBroadcastChan = make(chan *messages.RegisterMessage, 100)
	srv.log = log.GetLogger()
	srv.chain = chain
	srv.ntp = ntp.GetNTP()
	srv.ipCount = make(map[string]int)
	srv.mr = CreateMR()
	srv.downloader = CreateDownloader(chain)
	srv.futureBlocks = make(map[string]*block.Block)
	srv.mrDataConn = make(chan *MRDataConn)
	srv.blockAndPeerChan = make(chan *BlockAndPeer)
	srv.nodeHeaderHashAndPeerChan = make(chan *NodeHeaderHashAndPeer)
	srv.addPeerToPeerList = make(chan *generated.PLData)

	srv.messagePriority = make(map[generated.LegacyMessage_FuncName]uint64)
	srv.messagePriority[generated.LegacyMessage_VE] = 0
	srv.messagePriority[generated.LegacyMessage_PL] = 0
	srv.messagePriority[generated.LegacyMessage_PONG] = 0

	srv.messagePriority[generated.LegacyMessage_MR] = 2
	srv.messagePriority[generated.LegacyMessage_SFM] = 1

	srv.messagePriority[generated.LegacyMessage_BK] = 1
	srv.messagePriority[generated.LegacyMessage_FB] = 0
	srv.messagePriority[generated.LegacyMessage_PB] = 0
	srv.messagePriority[generated.LegacyMessage_BH] = 1

	srv.messagePriority[generated.LegacyMessage_TX] = 1
	srv.messagePriority[generated.LegacyMessage_MT] = 1
	srv.messagePriority[generated.LegacyMessage_TK] = 1
	srv.messagePriority[generated.LegacyMessage_TT] = 1
	srv.messagePriority[generated.LegacyMessage_LT] = 1
	srv.messagePriority[generated.LegacyMessage_SL] = 1

	srv.messagePriority[generated.LegacyMessage_EPH] = 3

	srv.messagePriority[generated.LegacyMessage_SYNC] = 0
	srv.messagePriority[generated.LegacyMessage_CHAINSTATE] = 0
	srv.messagePriority[generated.LegacyMessage_HEADERHASHES] = 1
	srv.messagePriority[generated.LegacyMessage_P2P_ACK] = 0

	srv.filter = bloom.New(200000, 5)
	chain.GetTransactionPool().SetRegisterAndBroadcastChan(srv.registerAndBroadcastChan)
	if err := srv.startListening(); err != nil {
		return err
	}

	if srv.config.User.Miner.MiningEnabled {
		srv.miner = miner.CreateMiner(srv.chain, srv.registerAndBroadcastChan)
		go srv.miner.StartMining()
	} else if srv.config.User.API.MiningAPI.Enabled {
		miningServer := api.NewMiningAPIServer(srv.chain, srv.registerAndBroadcastChan)
		go miningServer.Start()
	}

	srv.running = true
	go srv.run()

	return nil
}

func (srv *Server) listenLoop(listener net.Listener) {
	srv.loopWG.Add(1)
	defer srv.loopWG.Done()

	for {
		c, err := listener.Accept()

		if err != nil {
			srv.log.Error("Read ERROR", "Reason", err)
			return
		}
		srv.log.Debug("New Peer joined")
		srv.addpeer <- &conn{c, true}
	}
}

func (srv *Server) ConnectPeer(peer string) error {
	ip, _, _ := net.SplitHostPort(peer)
	if _, ok := srv.ipCount[ip]; ok {
		return nil
	}

	c, err := net.DialTimeout("tcp", peer, 10*time.Second)

	if err != nil {
		srv.log.Warn("Error while connecting to Peer",
			"IP:PORT", peer)
		return err
	}
	srv.log.Debug("Connected to peer",
		"IP:PORT", peer)
	srv.addpeer <- &conn{c, false}

	return nil
}

func (srv *Server) ConnectPeers() {
	srv.loopWG.Add(1)
	defer srv.loopWG.Done()

	for _, peer := range srv.config.User.Node.PeerList {
		srv.log.Info("Connecting peer",
			"IP:PORT", peer)
		srv.ConnectPeer(peer)
		// TODO: Update last connection time
	}

	for {
		select {
		case <-time.After(15*time.Second):
			srv.peerInfoLock.Lock()
			if srv.inboundCount > srv.config.User.Node.MaxPeersLimit {
				srv.peerInfoLock.Unlock()
				break
			}

			maxConnectionTry := 10
			peerList := make([]string, 0)
			removePeerList := make([]string, 0)

			count := 0

			for _, p := range srv.peersInfo.PeersInfo {

				if count > maxConnectionTry {
					break
				}

				if _, ok := srv.ipCount[p.IP]; ok {
					continue
				}

				count += 1
				peerList = append(peerList, net.JoinHostPort(p.IP, strconv.FormatInt(int64(p.Port), 10)))
				p.LastConnectionTimestamp = srv.ntp.Time()
			}
			srv.peerInfoLock.Unlock()

			for _, ipPort := range peerList {
				if !srv.running {
					break
				}
				fmt.Println("Trying to Connect",
					"Peer", ipPort)
				err := srv.ConnectPeer(ipPort)
				if err != nil {
					removePeerList = append(removePeerList, ipPort)
				}
			}

			srv.peerInfoLock.Lock()
			for _, ipPort := range removePeerList {
				ip, port, _ := net.SplitHostPort(ipPort)
				var index int
				for i, p := range srv.peersInfo.PeersInfo {
					if p.IP == ip && strconv.FormatInt(int64(p.Port), 10) == port {
						index = i
						break
					}
				}
				srv.peersInfo.PeersInfo = append(srv.peersInfo.PeersInfo[:index], srv.peersInfo.PeersInfo[index+1:]...)
			}

			srv.WritePeerList()
			srv.peerInfoLock.Unlock()
		case <-srv.connectPeersExit:
			return
		}
	}
}

func (srv *Server) Stop() {
	srv.lock.Lock()
	defer srv.lock.Unlock()
	if !srv.running {
		return
	}
	srv.running = false
	if srv.listener != nil {
		srv.listener.Close()
	}
	close(srv.downloader.done)
	close(srv.exit)
	close(srv.connectPeersExit)
	srv.loopWG.Wait()
}

func (srv *Server) startListening() error {
	bindingAddress := fmt.Sprintf("%s:%d", srv.config.User.Node.BindingIP, srv.config.User.Node.LocalPort)
	listener, err := net.Listen("tcp", bindingAddress)
	if err != nil {
		return err
	}
	srv.listener = listener
	go srv.listenLoop(listener)
	return nil
}

func (srv *Server) run() {
	var (
		peers        = make(map[string]*Peer)
	)

	srv.loopWG.Add(1)
	defer srv.loopWG.Done()

running:
	for {
		select {
		case <-srv.exit:
			srv.log.Debug("Shutting Down Server")
			break running
		case c := <-srv.addpeer:
			srv.peerInfoLock.Lock()

			srv.log.Debug("Adding peer", "addr", c.fd.RemoteAddr())
			p := newPeer(
				&c.fd,
				c.inbound,
				srv.chain,
				srv.filter,
				srv.mr,
				srv.mrDataConn,
				srv.registerAndBroadcastChan,
				srv.addPeerToPeerList,
				srv.blockAndPeerChan,
				srv.nodeHeaderHashAndPeerChan,
				srv.messagePriority)
			go srv.runPeer(p)
			peers[c.fd.RemoteAddr().String()] = p

			ip, _, _ := net.SplitHostPort(c.fd.RemoteAddr().String())
			srv.ipCount[ip] += 1
			if p.inbound {
				srv.inboundCount++
			}
			if srv.ipCount[ip] > srv.config.User.Node.MaxRedundantConnections {
				p.Disconnect(DiscAlreadyConnected)
				// TODO: Ban peer
			}

			srv.peerInfoLock.Unlock()
		case pd := <-srv.delpeer:
			srv.peerInfoLock.Lock()

			pd.log.Debug("Removing Peer", "err", pd.err)
			delete(peers, pd.conn.RemoteAddr().String())
			if pd.inbound {
				srv.inboundCount--
			}
			ip, _, _ := net.SplitHostPort(pd.conn.RemoteAddr().String())
			srv.ipCount[ip] -= 1

			srv.peerInfoLock.Unlock()
		case mrDataConn := <-srv.mrDataConn:
			// TODO: Process Message Recpt
			// Need to get connection too
			mrData := mrDataConn.mrData
			msgHash := misc.Bin2HStr(mrData.Hash)
			switch mrData.Type {
			case generated.LegacyMessage_BK:
				if mrData.BlockNumber > srv.chain.Height()+uint64(srv.config.Dev.MaxMarginBlockNumber) {
					srv.log.Debug("Skipping block #%s as beyond lead limit",
						"Block #", mrData.BlockNumber)
					break
				}
				if mrData.BlockNumber < srv.chain.Height()-uint64(srv.config.Dev.MinMarginBlockNumber) {
					srv.log.Debug("'Skipping block #%s as beyond the limit",
						"Block #", mrData.BlockNumber)
					break
				}
				_, err := srv.chain.GetBlock(mrData.PrevHeaderhash)
				if err != nil {
					srv.log.Debug("Missing Parent Block",
						"#", mrData.BlockNumber,
						"Block:", misc.Bin2HStr(mrData.Hash),
						"Parent Block ", misc.Bin2HStr(mrData.PrevHeaderhash))
					break
				}
				if srv.mr.contains(mrData.Hash, mrData.Type) {
					break
				}

				srv.mr.addPeer(mrData, mrDataConn.peer)

				value, _ := srv.mr.GetRequestedHash(msgHash)
				if value.GetRequested() {
					break
				}

				go srv.RequestFullMessage(mrData)
				// Request for full message
				// Check if its already being feeded by any other peer
			case generated.LegacyMessage_TX:
				srv.HandleTransaction(mrData)
			case generated.LegacyMessage_MT:
				srv.HandleTransaction(mrData)
			case generated.LegacyMessage_TK:
				srv.HandleTransaction(mrData)
			case generated.LegacyMessage_TT:
				srv.HandleTransaction(mrData)
			case generated.LegacyMessage_SL:
				srv.HandleTransaction(mrData)
			default:
				srv.log.Warn("Unknown Message Receipt Type",
					"Type", mrData.Type)
				mrDataConn.peer.Disconnect(DiscProtocolError)
			}
		case blockAndPeer := <-srv.blockAndPeerChan:
			srv.BlockReceived(blockAndPeer.peer, blockAndPeer.block)
		case startSyncing := <-srv.nodeHeaderHashAndPeerChan:
			if srv.downloader.isSyncing {
				srv.downloader.AddPeer(startSyncing.peer) // Added new Peer
				srv.log.Info("Node Already Syncing")
				break
			}
			srv.downloader.NewTargetNode(startSyncing.nodeHeaderHash, startSyncing.peer)
			//go srv.downloader.BlockDownloader()
			srv.downloader.Initialize(startSyncing.peer)
			srv.log.Info("Start Downloading Thread")
		case addPeerToPeerList := <-srv.addPeerToPeerList:
			srv.UpdatePeerList(addPeerToPeerList)
		case registerAndBroadcast := <-srv.registerAndBroadcastChan:
			srv.mr.Register(registerAndBroadcast.MsgHash, registerAndBroadcast.Msg)
			out := &Msg{
				msg: &generated.LegacyMessage{
					FuncName: registerAndBroadcast.Msg.MessageType,
					Data: registerAndBroadcast.Msg.Msg,
				},
			}
			ignorePeers := make(map[*Peer]bool, 0)
			if msgRequest, ok := srv.mr.GetRequestedHash(registerAndBroadcast.MsgHash); ok {
				ignorePeers = msgRequest.peers
			}
			for _, p := range peers {
				if _, ok := ignorePeers[p]; !ok {
					p.Send(out)
				}
			}
		}
	}
	for _, p := range peers {
		p.Disconnect(DiscQuitting)
	}
}

func (srv *Server) HandleTransaction(mrData *generated.MRData) {
	if srv.downloader.isSyncing {
		return
	}
	if srv.chain.GetTransactionPool().IsFull() {
		return
	}
	go srv.RequestFullMessage(mrData)
}

func (srv *Server) LoadPeerList() {
	srv.peersInfo = &PeersInfo{}

	peerFileName := path.Join(srv.config.User.DataDir(), srv.config.Dev.PeersFilename)
	jsonFile, err := os.Open(peerFileName)

	if err != nil {
		if err, ok := err.(*os.PathError); !ok {
			srv.log.Error("Error while opening ",
				"FileName", srv.config.Dev.PeersFilename,
				"Error", err.Error())
			return
		}
	}
	defer jsonFile.Close()
	byteValue := make([]byte, 0)

	// Parse only when peers file is found
	if err == nil {
		byteValue, err = ioutil.ReadAll(jsonFile)
		if err != nil {
			srv.log.Error("Error while parsing JsonFile",
				"Error", err.Error())
			return
		}
	}
	json.Unmarshal([]byte(byteValue), srv.peersInfo)
}

func (srv *Server) WritePeerList() error {
	peerFileName := path.Join(srv.config.User.DataDir(), srv.config.Dev.PeersFilename)
	peersInfoJson, err := json.Marshal(*srv.peersInfo)
	if err != nil {
		srv.log.Info("Error while parsing before writing peersInfo",
			"Error", err.Error())
		return err
	}

	err = ioutil.WriteFile(peerFileName, peersInfoJson, 0644)
	if err != nil {
		srv.log.Info("Error while writing file",
			"FileName", peerFileName,
			"Error", err.Error())
		return err
	}

	return nil
}

func (srv *Server) UpdatePeerList(pl *generated.PLData) error {
	srv.peerInfoLock.Lock()
	defer srv.peerInfoLock.Unlock()

	peers := make(map[string]*PeerInfo)

	for _, p := range srv.peersInfo.PeersInfo {
		if _, ok := peers[p.IP]; !ok {
			peers[p.IP] = &p
		}
	}

	var ip string
	var port uint16

	for _, p := range pl.GetPeerIps() {
		ipPort := strings.Split(p, ":")
		if len(ipPort) == 1 {
			ip = ipPort[0]
			port = 19000
		} else if len(ipPort) == 2 {
			ip = ipPort[0]
			port64, err := strconv.ParseUint(ipPort[1], 10, 64)
			if err != nil {
				// TODO: Invalid Port
				continue
			}
			port = uint16(port64)
		} else {
			// TODO: Invalid Peer List
			continue
		}

		// TODO: Validate IP, should not be local ip
		// Ignores Ephemeral Port Range of Linux
		if (port >= uint16(32768) && port <= uint16(61000)) || port <= 0 || port > 65535 {
			// TODO: Invalid Port Ban peer
			continue
		}

		if _, ok := peers[ip]; !ok {
			peerInfo := &PeerInfo{
				IP: ip,
				Port: port,
				LastConnectionTimestamp: 0,
			}
			peers[ip] = peerInfo
			srv.peersInfo.PeersInfo = append(srv.peersInfo.PeersInfo, *peerInfo)
		}
	}

	return srv.WritePeerList()
}

func (srv *Server) RequestFullMessage(mrData *generated.MRData) {
	for {
		msgHash := misc.Bin2HStr(mrData.Hash)
		_, ok := srv.mr.GetHashMsg(msgHash)
		if ok {
			if _, ok = srv.mr.GetRequestedHash(msgHash); ok {
				srv.mr.RemoveRequestedHash(msgHash)
			}
			return
		}

		requestedHash, ok := srv.mr.GetRequestedHash(msgHash)
		if !ok {
			return
		}

		peer := requestedHash.GetPeer()
		if peer == nil {
			return
		}
		requestedHash.SetPeer(peer, true)
		mrData := &generated.MRData{
			Hash: mrData.Hash,
			Type: mrData.Type,
		}
		out := &Msg{}
		out.msg = &generated.LegacyMessage{
			FuncName: generated.LegacyMessage_SFM,
			Data: &generated.LegacyMessage_MrData{
				MrData: mrData,
			},
		}
		peer.Send(out)

		time.Sleep(time.Duration(uint64(srv.config.Dev.MessageReceiptTimeout)) * time.Second)
	}
}

func (srv *Server) BlockReceived(peer *Peer, b *block.Block) {
	headerHash := misc.Bin2HStr(b.HeaderHash())
	srv.log.Info(">>> Received Block",
		"Block Number", b.BlockNumber(),
		"HeaderHash", headerHash)

	select {
		case srv.downloader.blockAndPeerChannel <- &BlockAndPeer{b, peer}:
		case <-time.After(5 * time.Second):
			srv.log.Info("Timeout for Received Block",
				"#", b.BlockNumber(),
				"HeaderHash", headerHash)
	}
	if srv.miner != nil {
		if reflect.DeepEqual(srv.chain.GetLastBlock().HeaderHash(), b.HeaderHash()) {
			srv.miner.StopMiningChan <- true
		}
	}
}

func (srv *Server) runPeer(p *Peer) {
	remoteRequested, err := p.run()

	srv.delpeer <- &peerDrop{p, err, remoteRequested}
}
