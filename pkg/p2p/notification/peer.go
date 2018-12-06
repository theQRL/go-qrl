package notification


import (
"encoding/binary"
"github.com/golang/protobuf/proto"
"github.com/theQRL/go-qrl/pkg/config"
"github.com/theQRL/go-qrl/pkg/core/block"
"github.com/theQRL/go-qrl/pkg/core/chain"
"github.com/theQRL/go-qrl/pkg/generated"
"github.com/theQRL/go-qrl/pkg/generated/notification"
"github.com/theQRL/go-qrl/pkg/log"
"github.com/theQRL/go-qrl/pkg/misc"
"github.com/theQRL/go-qrl/pkg/ntp"
"github.com/theQRL/go-qrl/pkg/p2p"
"io"
"net"
"sync"
"time"
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

	wg                  sync.WaitGroup
	disc                chan p2p.DiscReason
	writeErr			chan error
	disconnected        bool
	notificationEnabled bool
	exitPingLoop        chan struct{}
	exitRunLoop         chan struct{}
	log                 log.LoggerInterface
	config              *config.Config
	ntp                 ntp.NTPInterface
	lastPingReceived    uint64

	connectionTime uint64
}

func (p *Peer) Send(msg *notification.NotificationMessage) error {
	data, err := proto.Marshal(msg)
	if err != nil {
		log.GetLogger().Error("Error Parsing Data while Sending NotificationMessage",
			"Error", err.Error())
		return nil
	}
	bs := make([]byte, 4)
	binary.BigEndian.PutUint32(bs, uint32(len(data)))
	out := append(bs, data...)

	_, err = p.conn.Write(out)
	if err != nil {
		p.log.Error("Error while writing message on socket",
			"Error", err.Error())
		p.writeErr <- err
	}
	return err
}

func newPeer(conn *net.Conn, inbound bool, chain *chain.Chain) *Peer {
	p := &Peer{
		conn:           *conn,
		chain:          chain,
		disc:           make(chan p2p.DiscReason),
		writeErr:       make(chan error, 1),
		exitPingLoop:   make(chan struct{}),
		exitRunLoop:    make(chan struct{}),
		inbound:        inbound,
		log:            log.GetLogger(),
		config:         config.GetConfig(),
		ntp:            ntp.GetNTP(),
		connectionTime: ntp.GetNTP().Time(),
	}

	p.log.Info("New Peer connected",
		"Peer Addr", p.conn.RemoteAddr().String())
	return p
}

func (p *Peer) ReadMsg() (msg *notification.NotificationMessage, err error) {
	// TODO: Add Read timeout
	buf := make([]byte, 4)
	if _, err := io.ReadFull(p.conn, buf); err != nil {
		return msg, err
	}
	size := misc.ConvertBytesToLong(buf)
	buf = make([]byte, size)
	if _, err := io.ReadFull(p.conn, buf); err != nil {
		p.log.Info("Error while reading message",
			"error", err.Error())
		return nil, err
	}
	msg = &notification.NotificationMessage{}
	err = proto.Unmarshal(buf, msg)
	return msg, err  // 4 Byte Added for MetaData that includes the size of actual data
}

func (p *Peer) readLoop(errc chan<- error) {
	p.wg.Add(1)
	defer p.wg.Done()
	p.log.Debug("initiating readloop")

	for {
		msg, err := p.ReadMsg()
		if err != nil {
			errc <- err
			return
		}
		
		if err = p.handle(msg); err != nil {
			errc <- err
			return
		}
	}
}

func (p *Peer) IsNotificationEnabled() bool {
	p.lock.Lock()
	defer p.lock.Unlock()
	return p.notificationEnabled
}

func (p *Peer) NotifyNewBlock(headerHash []byte) error {
	if !p.IsNotificationEnabled() {
		return nil
	}
	bnData := &notification.BlockNotification{
		HeaderHash:headerHash,
	}
	out := &notification.NotificationMessage{
		FuncName:notification.NotificationMessage_BN,
		Data:&notification.NotificationMessage_BnData{
			BnData:bnData,
		},
	}
	return p.Send(out)
}

func (p *Peer) handle(msg *notification.NotificationMessage) error {
	switch msg.FuncName {

	case notification.NotificationMessage_NM:
		nmData := msg.GetNmData()
		if nmData == nil {
			return nil
		}
		p.notificationEnabled = nmData.Enable
	case notification.NotificationMessage_PING:
		pingData := msg.GetPingData()
		if pingData == nil {
			return nil
		}
		p.lastPingReceived = pingData.Timestamp
	case notification.NotificationMessage_BLOCKREQ:
		blockReqData := msg.GetBlockReqData()
		if blockReqData == nil {
			return nil
		}
		headerHash := blockReqData.GetHeaderHash()
		var b *block.Block
		var err error
		if headerHash != nil {
			b, err = p.chain.GetBlock(headerHash)
		} else {
			b, err = p.chain.GetBlockByNumber(blockReqData.GetBlockNumber())
		}
		blockRespData := &notification.BlockResponse{
			Found: err == nil,
		}
		if err == nil {
			blockRespData.Block = b.PBData()
		}
		out := &notification.NotificationMessage{
			FuncName:notification.NotificationMessage_BLOCKRESP,
			Data:&notification.NotificationMessage_BlockRespData{
				BlockRespData:blockRespData,
			},
		}
		p.Send(out)
	case notification.NotificationMessage_HEADERHASHESREQ:
		headerHashesReqData := msg.GetHeaderHashesReqData()
		if headerHashesReqData == nil {
			return nil
		}
		headerhashes, _ := p.chain.GetHeaderHashes(headerHashesReqData.StartBlockNumber, headerHashesReqData.Count)

		headerHashesRespData := &notification.HeaderHashesResp{
			Count:uint64(len(headerhashes.Headerhashes)),
			StartBlockNumber:headerhashes.BlockNumber,
			HeaderHashes:headerhashes.Headerhashes,
		}
		out := &notification.NotificationMessage{
			FuncName:notification.NotificationMessage_HEADERHASHESRESP,
			Data:&notification.NotificationMessage_HeaderHashesRespData{
				HeaderHashesRespData:headerHashesRespData,
			},
		}
		p.Send(out)
	case notification.NotificationMessage_HEIGHTREQ:
		heightReqData := msg.GetHeightReqData()
		if heightReqData == nil {
			return nil
		}
		height := p.chain.GetLastBlock().BlockNumber()
		heightRespData := &notification.HeightResp{
			Height:height,
		}
		out := &notification.NotificationMessage{
			FuncName:notification.NotificationMessage_HEIGHTRESP,
			Data:&notification.NotificationMessage_HeightRespData{
				HeightRespData:heightRespData,
			},
		}
		p.Send(out)
	}
	return nil
}

func (p *Peer) monitorPing() {
	for {
		select {
		case <-time.After(60*time.Second):
			currentTimestamp := p.ntp.Time()
			if currentTimestamp > p.lastPingReceived {
				p.Disconnect(p2p.DiscProtocolError)
			}
		case <-time.After(60*time.Second):
			pingData := &notification.Ping{
				Timestamp: p.ntp.Time(),
			}
			msg := &notification.NotificationMessage{
				FuncName:notification.NotificationMessage_PING,
				Data:&notification.NotificationMessage_PingData{
					PingData:pingData,
				},
			}
			p.Send(msg)
		case <-p.exitPingLoop:
			return
		}
	}
}

func (p *Peer) handshake() {

}

func (p *Peer) run() (remoteRequested bool, err error) {
	var (
		writeStart = make(chan struct{}, 1)
		readErr    = make(chan error, 1)
		reason     p2p.DiscReason
	)
	p.handshake()
	go p.readLoop(readErr)

loop:
	for {
		select {
		case err = <-p.writeErr:
			if err != nil {
				break loop
			}
			writeStart <- struct{}{}
		case err = <-readErr:
			if r, ok := err.(p2p.DiscReason); ok {
				break loop
				reason = r
			} else {
				reason = p2p.DiscNetworkError
			}
		case <-p.exitRunLoop:
			break loop
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
	close(p.exitPingLoop)
	close(p.exitRunLoop)
	p.conn.Close()
}

func (p *Peer) Disconnect(reason p2p.DiscReason) {
	p.log.Info("Disconnecting ",
		"Peer", p.conn.RemoteAddr().String())
	p.close(reason)
	p.wg.Wait()
}
