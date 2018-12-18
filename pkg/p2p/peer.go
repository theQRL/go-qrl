package p2p

import (
	"github.com/theQRL/go-qrl/pkg/core/block"
	"github.com/theQRL/go-qrl/pkg/misc"
	"github.com/theQRL/go-qrl/pkg/ntp"
	"github.com/theQRL/go-qrl/pkg/p2p/messages"
	"io"
	"math/big"
	"net"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/willf/bloom"

	"github.com/theQRL/go-qrl/pkg/config"
	"github.com/theQRL/go-qrl/pkg/core/chain"
	"github.com/theQRL/go-qrl/pkg/generated"
	"github.com/theQRL/go-qrl/pkg/log"
)

type MRDataConn struct {
	mrData *generated.MRData
	peer   *Peer
}

type Peer struct {
	conn    net.Conn
	inbound bool

	lock sync.Mutex

	chain *chain.Chain

	wg                        sync.WaitGroup
	disc                      chan DiscReason
	writeErr                  chan error
	disconnected              bool
	exitMonitorChainState     chan struct{}
	log                       log.LoggerInterface
	filter                    *bloom.BloomFilter
	mr                        *MessageReceipt
	config                    *config.Config
	ntp                       ntp.NTPInterface
	chainState                *generated.NodeChainState
	addPeerToPeerList         chan *generated.PLData
	blockAndPeerChan          chan *BlockAndPeer
	nodeHeaderHashAndPeerChan chan *NodeHeaderHashAndPeer
	mrDataConn                chan *MRDataConn
	registerAndBroadcastChan  chan *messages.RegisterMessage

	inCounter           uint64
	outCounter          uint64
	lastRateLimitUpdate uint64
	bytesSent           uint64
	connectionTime      uint64
	messagePriority     map[generated.LegacyMessage_FuncName]uint64
	outgoingQueue       *PriorityQueue

	isPLShared     bool // Flag to mark once peer list has been received by the peer
	isDownloadPeer bool // FLag is Peer is in the list of Downloader's Peer List
}

func newPeer(conn *net.Conn, inbound bool, chain *chain.Chain, filter *bloom.BloomFilter, mr *MessageReceipt, mrDataConn chan *MRDataConn, registerAndBroadcastChan chan *messages.RegisterMessage, addPeerToPeerList chan *generated.PLData, blockAndPeerChan chan *BlockAndPeer, nodeHeaderHashAndPeerChan chan *NodeHeaderHashAndPeer, messagePriority map[generated.LegacyMessage_FuncName]uint64) *Peer {
	p := &Peer{
		conn:                  *conn,
		inbound:               inbound,
		chain:                 chain,
		disc:                  make(chan DiscReason),
		writeErr:              make(chan error, 1),
		disconnected:          false,
		exitMonitorChainState: make(chan struct{}),
		log:                       log.GetLogger(),
		filter:                    filter,
		mr:                        mr,
		config:                    config.GetConfig(),
		ntp:                       ntp.GetNTP(),
		mrDataConn:                mrDataConn,
		registerAndBroadcastChan:  registerAndBroadcastChan,
		addPeerToPeerList:         addPeerToPeerList,
		blockAndPeerChan:          blockAndPeerChan,
		nodeHeaderHashAndPeerChan: nodeHeaderHashAndPeerChan,
		connectionTime:            ntp.GetNTP().Time(),
		messagePriority:           messagePriority,
		outgoingQueue:             &PriorityQueue{},
	}

	p.log.Info("New Peer connected",
		"Peer Addr", p.conn.RemoteAddr().String())
	return p
}

func (p *Peer) ID() string {
	return p.conn.RemoteAddr().String()
}

func (p *Peer) SetDownloaderPeerList(value bool) {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.isDownloadPeer = value
}

func (p *Peer) GetCumulativeDifficulty() []byte {
	p.lock.Lock()
	defer p.lock.Unlock()

	if p.chainState == nil {
		return nil
	}

	return p.chainState.CumulativeDifficulty
}

func (p *Peer) updateCounters() {
	timeDiff := p.ntp.Time() - p.lastRateLimitUpdate
	if timeDiff > 60 {
		p.outCounter = 0
		p.inCounter = 0
		p.lastRateLimitUpdate = p.ntp.Time()
	}
}

func (p *Peer) Send(msg *Msg) error {
	priority, ok := p.messagePriority[msg.msg.FuncName]
	if !ok {
		p.log.Warn("Unexpected FuncName while SEND",
			"FuncName", msg.msg.FuncName)
		return nil
	}
	outgoingMsg := CreateOutgoingMessage(priority, msg.msg)
	if p.outgoingQueue.Full() {
		p.log.Info("Outgoing Queue Full: Skipping Message")
		return nil
	}
	p.outgoingQueue.Push(outgoingMsg)
	return p.SendNext()
}

func (p *Peer) SendNext() error {
	p.lock.Lock()
	defer p.lock.Unlock()

	if p.disconnected {
		return DiscUselessPeer
	}

	p.updateCounters()
	if float32(p.outCounter) >= float32(p.config.User.Node.PeerRateLimit) * 0.9 {
		p.log.Info("Send Next Cancelled as",
			"p.outcounter", p.outCounter,
			"rate limit", float32(p.config.User.Node.PeerRateLimit) * 0.9)
		return nil
	}

	for p.bytesSent < p.config.Dev.MaxBytesOut {
		data := p.outgoingQueue.Pop()
		if data == nil {
			return nil
		}
		om := data.(*OutgoingMessage)
		outgoingBytes, _ := om.bytesMessage, om.msg

		if outgoingBytes == nil {
			p.log.Info("Outgoing bytes Nil")
			return nil
		}
		p.bytesSent += uint64(len(outgoingBytes))
		_, err := p.conn.Write(outgoingBytes)

		if err != nil {
			p.log.Error("Error while writing message on socket", "error", err)
			p.writeErr <- err
			return err
		}
	}

	return nil
}

func (p *Peer) ReadMsg() (msg *Msg, size uint32, err error) {
	// TODO: Add Read timeout
	msg = &Msg{}
	buf := make([]byte, 4)
	if _, err := io.ReadFull(p.conn, buf); err != nil {
		return msg, 0, err
	}
	size = misc.ConvertBytesToLong(buf)
	buf = make([]byte, size)
	if _, err := io.ReadFull(p.conn, buf); err != nil {
		return nil, 0, err
	}
	message := &generated.LegacyMessage{}
	err = proto.Unmarshal(buf, message)
	msg.msg = message
	return msg, size+4, err  // 4 Byte Added for MetaData that includes the size of actual data
}

func (p *Peer) readLoop(errc chan<- error) {
	p.wg.Add(1)
	defer p.wg.Done()
	p.log.Debug("initiating readloop")

	for {
		p.updateCounters()
		totalBytesRead := uint32(0)
		msg, size, err := p.ReadMsg()
		if err != nil {
			errc <- err
			return
		}
		msg.ReceivedAt = time.Now()
		if err = p.handle(msg); err != nil {
			errc <- err
			return
		}
		p.inCounter += 1
		if float32(p.inCounter) > 2.2 * float32(p.config.User.Node.PeerRateLimit) {
			p.log.Warn("Rate Limit Hit")
			p.Disconnect(DiscProtocolError)
			return
		}

		totalBytesRead += size
		if msg.msg.FuncName != generated.LegacyMessage_P2P_ACK {
			p2pAck := &generated.P2PAcknowledgement{
				BytesProcessed: totalBytesRead,
			}
			out := &Msg{}
			out.msg = &generated.LegacyMessage{
				FuncName: generated.LegacyMessage_P2P_ACK,
				Data: &generated.LegacyMessage_P2PAckData{
					P2PAckData: p2pAck,
				},
			}
			err = p.Send(out)
		}
	}
}

func (p *Peer) monitorChainState() error {
	p.wg.Add(1)
	defer p.wg.Done()
	for {
		select {
		case <-time.After(30 * time.Second):
			currentTime := p.ntp.Time()
			delta := int64(currentTime)
			if p.chainState != nil {
				delta -= int64(p.chainState.Timestamp)
			} else {
				delta -= int64(p.connectionTime)
			}
			if delta > int64(p.config.User.ChainStateTimeout) {
				p.log.Warn("Disconnecting Peer due to Ping Timeout",
					"delta", delta,
					"currentTime", currentTime,
					"peer", p.ID())
				p.Disconnect(DiscProtocolError)
				return nil
			}

			lastBlock := p.chain.GetLastBlock()
			blockMetaData, err := p.chain.GetBlockMetaData(lastBlock.HeaderHash())
			if err != nil {
				p.log.Warn("Ping Failed Disconnecting",
					"Peer", p.conn.RemoteAddr().String())
				p.Disconnect(DiscNetworkError)
				return err
			}
			chainStateData := &generated.NodeChainState{
				BlockNumber:          lastBlock.BlockNumber(),
				HeaderHash:           lastBlock.HeaderHash(),
				CumulativeDifficulty: blockMetaData.TotalDifficulty(),
				Version:              p.config.Dev.Version,
				Timestamp:            p.ntp.Time(),
			}
			out := &Msg{}
			out.msg = &generated.LegacyMessage{
				FuncName: generated.LegacyMessage_CHAINSTATE,
				Data: &generated.LegacyMessage_ChainStateData{
					ChainStateData: chainStateData,
				},
			}

			err = p.Send(out)
			if err != nil {
				p.log.Info("Error while sending ChainState",
					"peer", p.conn.RemoteAddr().String())
				return err
			}

			if p.chainState == nil {
				continue
			}
			// If Peer is already in download peer list then skip further processing
			if p.isDownloadPeer {
				continue
			}
			// Ignore syncing if difference between blockheight is 3
			if lastBlock.BlockNumber() - p.chainState.BlockNumber < 3 {
				continue
			}
			peerCumulativeDifficulty := big.NewInt(0).SetBytes(p.chainState.CumulativeDifficulty)
			localCumulativeDifficulty := big.NewInt(0).SetBytes(blockMetaData.TotalDifficulty())
			if peerCumulativeDifficulty.Cmp(localCumulativeDifficulty) > 0 {
				startBlockNumber := uint64(0)
				maxStartBlockNumber := uint64(0)
				if lastBlock.BlockNumber() > p.config.Dev.ReorgLimit {
					maxStartBlockNumber = lastBlock.BlockNumber() - p.config.Dev.ReorgLimit
				}
				if maxStartBlockNumber > startBlockNumber {
					startBlockNumber = maxStartBlockNumber
				}
				nodeHeaderHash := &generated.NodeHeaderHash{
					BlockNumber: startBlockNumber,
				}
				out := &Msg{}
				out.msg = &generated.LegacyMessage{
					FuncName: generated.LegacyMessage_HEADERHASHES,
					Data: &generated.LegacyMessage_NodeHeaderHash{
						NodeHeaderHash: nodeHeaderHash,
					},
				}
				p.Send(out)
			}
		case <-p.exitMonitorChainState:
			return nil
		}
	}
}

func (p *Peer) handle(msg *Msg) error {
	switch msg.msg.FuncName {

	case generated.LegacyMessage_VE:
		p.log.Debug("Received VE MSG")
		if msg.msg.GetVeData() == nil {
			out := &Msg{}
			veData := &generated.VEData{
				Version:         "",
				GenesisPrevHash: []byte("0"),
				RateLimit:       100,
			}
			out.msg = &generated.LegacyMessage{
				FuncName: generated.LegacyMessage_VE,
				Data: &generated.LegacyMessage_VeData{
					VeData: veData,
				},
			}
			err := p.Send(out)
			return err
		}
		veData := msg.msg.GetVeData()
		p.log.Info("", "version:", veData.Version,
			"GenesisPrevHash:", veData.GenesisPrevHash, "RateLimit:", veData.RateLimit)

	case generated.LegacyMessage_PL:
		p.log.Debug("Received PL MSG")
		if p.isPLShared {
			p.log.Debug("Peer list already shared before")
			return nil
		}
		p.isPLShared = true
		p.addPeerToPeerList <- msg.msg.GetPlData()

	case generated.LegacyMessage_PONG:
		p.log.Debug("Received PONG MSG")

	case generated.LegacyMessage_MR:
		mrData := msg.msg.GetMrData()
		mrDataConn := &MRDataConn{
			mrData,
			p,
		}
		p.mrDataConn <- mrDataConn

	case generated.LegacyMessage_SFM:
		mrData := msg.msg.GetMrData()
		msg := p.mr.Get(&mrData.Type, mrData.Hash)
		if msg != nil {
			out := &Msg{}
			out.msg = msg
			p.Send(out)
		}

	case generated.LegacyMessage_BK:
		b := msg.msg.GetBlock()
		p.HandleBlock(b)

	case generated.LegacyMessage_FB:
		fbData := msg.msg.GetFbData()
		blockNumber := fbData.Index
		p.log.Info("Fetch Block Request",
			"BlockNumber", blockNumber,
			"Peer", p.conn.RemoteAddr().String())

		h := p.chain.Height()
		if blockNumber > h {
			p.log.Info("Disconnecting Peer, as peer requested for block more than current block height",
				"Requested Block Number", blockNumber,
				"Current Chain Height", h)
			p.Disconnect(DiscProtocolError)
		}

		b, err := p.chain.GetBlockByNumber(blockNumber)
		if err == nil {
			p.log.Info("Disconnecting Peer, as GetBlockByNumber returned nil")
			p.Disconnect(DiscProtocolError)
		}
		pbData := &generated.PBData{
			Block: b.PBData(),
		}
		out := &Msg{}
		out.msg = &generated.LegacyMessage{
			FuncName: generated.LegacyMessage_PB,
			Data: &generated.LegacyMessage_PbData{
				PbData: pbData,
			},
		}
		p.Send(out)

	case generated.LegacyMessage_PB:
		pbData := msg.msg.GetPbData()
		if pbData.Block == nil {
			p.log.Info("Disconnecting Peer, as no block sent for Push Block")
			p.Disconnect(DiscProtocolError)
		}

		b := block.BlockFromPBData(pbData.Block)
		p.blockAndPeerChan <- &BlockAndPeer{b, p}

	case generated.LegacyMessage_BH:
		p.log.Warn("BH has not been Implemented <<<< --- ")
	case generated.LegacyMessage_TX:
	case generated.LegacyMessage_LT:
	case generated.LegacyMessage_EPH:
	case generated.LegacyMessage_MT:
	case generated.LegacyMessage_TK:
	case generated.LegacyMessage_TT:
	case generated.LegacyMessage_SL:
	case generated.LegacyMessage_SYNC:
		p.log.Warn("SYNC has not been Implemented <<<< --- ")
	case generated.LegacyMessage_CHAINSTATE:
		chainStateData := msg.msg.GetChainStateData()
		p.HandleChainState(chainStateData)
	case generated.LegacyMessage_HEADERHASHES:
		p.log.Info(">>>Received HEADERHASHES")
		nodeHeaderHash := msg.msg.GetNodeHeaderHash()
		if len(nodeHeaderHash.Headerhashes) == 0 {
			outNodeHeaderHash, err := p.chain.GetHeaderHashes(nodeHeaderHash.BlockNumber, p.config.Dev.ReorgLimit)
			if err != nil {
				p.log.Warn("Error in GetHeaderHashes",
					"Blocknumber", nodeHeaderHash.BlockNumber,
					"peer", p.conn.RemoteAddr().String())
				return nil
			}
			out := &Msg{}
			out.msg = &generated.LegacyMessage{
				FuncName: generated.LegacyMessage_HEADERHASHES,
				Data: &generated.LegacyMessage_NodeHeaderHash{
					NodeHeaderHash: outNodeHeaderHash,
				},
			}
			p.Send(out)
		} else {
			p.log.Info(">>>>>Triggering Download")
			nodeHeaderHashAndPeer := &NodeHeaderHashAndPeer {
				nodeHeaderHash,
				p,
			}
			p.nodeHeaderHashAndPeerChan <- nodeHeaderHashAndPeer
		}
	case generated.LegacyMessage_P2P_ACK:
		p2pAckData := msg.msg.GetP2PAckData()
		p.bytesSent -= uint64(p2pAckData.BytesProcessed)
		if p.bytesSent < 0 {
			p.log.Warn("Disconnecting Peer due to negative bytes sent",
				"bytesSent", p.bytesSent,
				"BytesProcessed", p2pAckData.BytesProcessed)
			p.Disconnect(DiscProtocolError)
			return DiscProtocolError
		}
		p.SendNext()
	}
	return nil
}

func (p *Peer) HandleBlock(pbBlock *generated.Block) {
	// TODO: Validate Message
	b := block.BlockFromPBData(pbBlock)
	p.log.Info("Received Block from ip:port block_number block_headerhash")
	parentBlock, err := p.chain.GetBlock(b.PrevHeaderHash())
	if err != nil {
		// TODO: Think of how to handle such cases
		p.log.Warn("Parent Block Not Found")
		return
	}
	parentMetaData, err := p.chain.GetBlockMetaData(b.PrevHeaderHash())
	if err != nil {
		p.log.Warn("Impossible Error : ParentMetaData Not Found")
		return
	}

	blockFromState, _ := p.chain.GetBlock(b.HeaderHash())
	measurement, err := p.chain.GetMeasurement(uint32(b.Timestamp()), b.PrevHeaderHash(), parentMetaData)
	if !b.Validate(blockFromState, parentBlock, parentMetaData, measurement, nil) {
		p.log.Warn("Block Validation Failed",
			"BlockNumber", b.BlockNumber(),
			"Headerhash", misc.Bin2HStr(b.HeaderHash()))
		return
	}

	if !p.chain.AddBlock(b) {
		p.log.Warn("Failed To Add Block")
		return
	}

	msg := &generated.Message{
		Msg:&generated.LegacyMessage_Block{
			Block:b.PBData(),
		},
		MessageType:generated.LegacyMessage_BK,
	}

	registerMessage := &messages.RegisterMessage{
		MsgHash:misc.Bin2HStr(b.HeaderHash()),
		Msg:msg,
	}

	select {
	case p.registerAndBroadcastChan <- registerMessage:
	case <-time.After(10*time.Second):
		p.log.Warn("[HandleBlock] RegisterAndBroadcastChan Timeout",
			"Peer", p.ID())
	}
}

func (p *Peer) HandleChainState(nodeChainState *generated.NodeChainState) {
	p.chainState = nodeChainState
	p.chainState.Timestamp = p.ntp.Time()
	p.log.Info("Chain State updated",
		"Peer", p.ID())
}

func (p *Peer) SendFetchBlock(blockNumber uint64) error {
	p.log.Info("Fetching",
		"Block #", blockNumber,
		"Peer", p.conn.RemoteAddr().String())
	out := &Msg{}
	fbData := &generated.FBData{
		Index:blockNumber,
	}
	out.msg = &generated.LegacyMessage{
		FuncName: generated.LegacyMessage_FB,
		Data: &generated.LegacyMessage_FbData{
			FbData: fbData,
		},
	}
	return p.Send(out)
}

func (p *Peer) SendPeerList() {
	out := &Msg{}
	plData := &generated.PLData{
		PeerIps:[]string{},
		PublicPort:19000,
	}
	out.msg = &generated.LegacyMessage{
		FuncName: generated.LegacyMessage_PL,
		Data: &generated.LegacyMessage_PlData{
			PlData: plData,
		},
	}
	p.Send(out)
}

func (p *Peer) SendVersion() {
	out := &Msg{}
	veData := &generated.VEData{
		Version:p.config.Dev.Version,
		GenesisPrevHash:p.config.Dev.Genesis.GenesisPrevHeadehash,
		RateLimit:p.config.User.Node.PeerRateLimit,
	}
	out.msg = &generated.LegacyMessage{
		FuncName: generated.LegacyMessage_PL,
		Data: &generated.LegacyMessage_VeData{
			VeData: veData,
		},
	}
	p.Send(out)
}

func (p *Peer) SendSync() {
	out := &Msg{}
	syncData := &generated.SYNCData{
		State: "Synced",
	}
	out.msg = &generated.LegacyMessage{
		FuncName: generated.LegacyMessage_SYNC,
		Data: &generated.LegacyMessage_SyncData{
			SyncData: syncData,
		},
	}
	p.Send(out)
}

func (p *Peer) handshake() {
	p.SendPeerList()
	// p.SendVersion()
	p.SendSync()
}

func (p *Peer) run() (remoteRequested bool, err error) {
	var (
		writeStart = make(chan struct{}, 1)
		readErr    = make(chan error, 1)
		reason     DiscReason
	)
	p.handshake()
	go p.readLoop(readErr)
	go p.monitorChainState()
loop:
	for {
		select {
		case err = <-p.writeErr:
			if err != nil {
				break loop
			}
			writeStart <- struct{}{}
		case err = <-readErr:
			if r, ok := err.(DiscReason); ok {
				break loop
				reason = r
			} else {
				reason = DiscNetworkError
			}
		}
	}
	p.close(reason)
	p.wg.Wait()
	return remoteRequested, err
}

func (p *Peer) close(err error) {
	p.lock.Lock()
	defer p.lock.Unlock()
	if p.disconnected {
		p.log.Info("Disconnected ",
			"Peer", p.conn.RemoteAddr().String())
		return
	}
	p.disconnected = true
	close(p.exitMonitorChainState)
	p.conn.Close()
}

func (p *Peer) Disconnect(reason DiscReason) {
	p.log.Info("Disconnecting ",
		"Peer", p.conn.RemoteAddr().String())
	p.close(reason)
	p.wg.Wait()
}
