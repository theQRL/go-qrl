package p2p

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/theQRL/go-qrl/pkg/core/block"
	"github.com/theQRL/go-qrl/pkg/generated"
	"github.com/theQRL/go-qrl/pkg/misc"
	"github.com/theQRL/go-qrl/pkg/ntp"
	"io/ioutil"
	"net"
	"os"
	"path"
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

	chain *chain.Chain
	ntp   *ntp.NTP

	listener net.Listener
	lock     sync.Mutex

	running bool
	loopWG  sync.WaitGroup
	log     log.LoggerInterface

	exit                      chan struct{}
	mrDataConn                chan *MRDataConn
	blockAndPeerChan          chan *BlockAndPeer
	nodeHeaderHashAndPeerChan chan *NodeHeaderHashAndPeer
	addPeerToPeerList         chan *generated.PLData
	addpeer                   chan *conn
	delpeer                   chan peerDrop

	filter          *bloom.BloomFilter
	mr              *MessageReceipt
	downloader      *Downloader
	futureBlocks    map[string]*block.Block
	messagePriority map[generated.LegacyMessage_FuncName]uint64
}

type peerDrop struct {
	*Peer
	err       error
	requested bool // true if signaled by the peer
}

func (srv *Server) Start(chain *chain.Chain) (err error) {
	srv.lock.Lock()
	defer srv.lock.Unlock()
	if srv.running {
		return errors.New("server is already running")
	}

	srv.config = config.GetConfig()
	srv.exit = make(chan struct{})
	srv.addpeer = make(chan *conn)
	srv.delpeer = make(chan peerDrop)
	srv.log = log.GetLogger()
	srv.chain = chain
	srv.ntp = ntp.GetNTP()
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
	if err := srv.startListening(); err != nil {
		return err
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

func (srv *Server) ConnectPeers() {
	srv.loopWG.Add(1)
	defer srv.loopWG.Done()

	for _, peer := range srv.config.User.Node.PeerList {
		srv.log.Info("Connecting peer",
			"IP:PORT", peer)
		c, err := net.DialTimeout("tcp", peer, 10*time.Second)

		if err != nil {
			srv.log.Warn("Error while connecting to Peer",
				"IP:PORT", peer)
			continue
		}
		srv.log.Debug("Connected to peer",
			"IP:PORT", peer)
		srv.addpeer <- &conn{c, false}
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
	close(srv.exit)
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
		inboundCount = 0
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
			srv.log.Debug("Adding peer", "addr", c.fd.RemoteAddr())
			p := newPeer(&c.fd, c.inbound, srv.chain, srv.filter, srv.mr, srv.mrDataConn, srv.addPeerToPeerList, srv.blockAndPeerChan, srv.nodeHeaderHashAndPeerChan, srv.messagePriority)
			go srv.runPeer(p)
			peers[c.fd.RemoteAddr().String()] = p
			if p.inbound {
				inboundCount++
			}
		case pd := <-srv.delpeer:
			pd.log.Debug("Removing Peer", "err", pd.err)
			delete(peers, pd.conn.RemoteAddr().String())
			if pd.inbound {
				inboundCount--
			}
		case mrDataConn := <-srv.mrDataConn:
			// TODO: Process Message Recpt
			// Need to get connection too
			mrData := mrDataConn.mrData
			msgHash := misc.Bin2HStr(mrData.Hash)
			switch mrData.Type {
			case generated.LegacyMessage_BK:
				if mrData.BlockNumber > srv.chain.Height()+uint64(srv.config.Dev.MaxMarginBlockNumber) {
					srv.log.Debug("Skipping block #%s as beyond lead limit", "Block #", mrData.BlockNumber)
					break
				}
				if mrData.BlockNumber < srv.chain.Height()-uint64(srv.config.Dev.MinMarginBlockNumber) {
					srv.log.Debug("'Skipping block #%s as beyond the limit", "Block #", mrData.BlockNumber)
					break
				}
				_, err := srv.chain.GetBlock(mrData.PrevHeaderhash)
				if err != nil {
					srv.log.Debug("Missing Parent Block", "Block:", mrData.Hash,
						"Parent Block ", mrData.PrevHeaderhash)
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
				// Check transactions pool Size,
				// if full then ignore
			default:
				srv.log.Warn("Unknown Message Receipt Type")
				mrDataConn.peer.Disconnect(DiscProtocolError)
			}
		case blockAndPeer := <-srv.blockAndPeerChan:
			srv.BlockReceived(blockAndPeer.peer, blockAndPeer.block)
		case startSyncing := <-srv.nodeHeaderHashAndPeerChan:
			srv.log.Info("Running downloading thread")
			if srv.downloader.isSyncing {
				srv.log.Info("Node Already Syncing")
				break
			}
			srv.downloader.NewTargetNode(startSyncing.nodeHeaderHash, startSyncing.peer)
			go srv.downloader.BlockDownloader()
			srv.log.Info("Start Downloading Thread")
		case addPeerToPeerList := <-srv.addPeerToPeerList:
			srv.UpdatePeerList(addPeerToPeerList)
		}
	}

	for _, p := range peers {
		p.Disconnect(DiscQuitting)
	}
}

func (srv *Server) UpdatePeerList(pl *generated.PLData) error {
	peerFileName := path.Join(srv.config.User.DataDir(), srv.config.Dev.PeersFilename)
	jsonFile, err := os.Open(peerFileName)

	if err != nil {
		if err, ok := err.(*os.PathError); !ok {
			srv.log.Error("Error while opening ",
				"FileName", srv.config.Dev.PeersFilename,
				"Error", err.Error())
			return err
		}
	}

	byteValue, _ := ioutil.ReadAll(jsonFile)
	jsonFile.Close()

	var peersInfo PeersInfo
	json.Unmarshal([]byte(byteValue), &peersInfo)

	peers := make(map[string]*PeerInfo)

	for _, p := range peersInfo.PeersInfo {
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
		if !(port >= uint16(0) && port <= uint16(65535)) {
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
			peersInfo.PeersInfo = append(peersInfo.PeersInfo, *peerInfo)
		}
	}

	peersInfoJson, err := json.Marshal(peersInfo)
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

func (srv *Server) RequestFullMessage(mrData *generated.MRData) {
	outer:
	for ;; {
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

		for peer, requested := range requestedHash.peers {
			if requested {
				continue
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

			start := srv.ntp.Time()
			time.Sleep(time.Duration(start + uint64(srv.config.Dev.MessageReceiptTimeout)) * time.Second)

			continue outer
		}

		if ok {
			srv.mr.RemoveRequestedHash(msgHash)
		}
	}
}

func (srv *Server) BlockReceived(peer *Peer, b *block.Block) {
	// srv.downloader.lastPBTime = srv.ntp.Time()
	srv.log.Info(">>> Received Block",
		"Block Number", b.BlockNumber(),
		"HeaderHash", misc.Bin2HStr(b.HeaderHash()))

	srv.downloader.blockAndPeerChannel <- &BlockAndPeer{b, peer}
}

func (srv *Server) runPeer(p *Peer) {
	remoteRequested, err := p.run()

	srv.delpeer <- peerDrop{p, err, remoteRequested}
}
