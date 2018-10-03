package p2p

import (
	"encoding/binary"
	"io"
	"net"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/willf/bloom"

	"github.com/theQRL/go-qrl/config"
	"github.com/theQRL/go-qrl/core"
	"github.com/theQRL/go-qrl/generated"
	"github.com/theQRL/go-qrl/log"
)

type Peer struct {
	conn    net.Conn
	inbound bool

	chain *core.Chain

	wg     sync.WaitGroup
	closed chan struct{}
	disc   chan DiscReason
	log    log.Logger
	filter *bloom.BloomFilter
	config *config.Config
}

func newPeer(conn *net.Conn, inbound bool, chain *core.Chain, log *log.Logger, filter *bloom.BloomFilter, config *config.Config) *Peer {
	p := &Peer {
		conn: *conn,
		inbound: inbound,
		chain: chain,
		log: *log,
		filter: filter,
		config: config,
	}
	return p
}

func (p *Peer) WriteMsg(msg Msg) error {
	data, err := proto.Marshal(msg.msg)
	if err != nil {
		p.log.Error("Error Parsing Data")
		return err
	}

	bs := make([]byte, 4)
	binary.BigEndian.PutUint32(bs, uint32(len(data)))
	out := append(bs, data...)

	_, err = p.conn.Write(out)
	if err != nil {
		p.log.Error("Error while writing message on socket", "error", err)
	}
	return nil
}

func (p *Peer) ReadMsg() (msg Msg, err error){
	buf := make([]byte, 4)
	if _, err := io.ReadFull(p.conn, buf); err != nil {
		return msg, err
	}
	size := convertBytesToLong(buf)
	buf = make([]byte, size)
	if _, err := io.ReadFull(p.conn, buf); err != nil {
		return msg, err
	}
	message := &generated.LegacyMessage{}
	err = proto.Unmarshal(buf, message)
	msg.msg = message
	return msg, err
}

func (p *Peer) readLoop(errc chan<- error) {
	defer p.wg.Done()
	p.log.Debug("initiating readloop")
	for {
		msg, err := p.ReadMsg()
		if err != nil {
			errc <- err
			return
		}
		msg.ReceivedAt = time.Now()
		p.log.Debug("Received msg")
		if err = p.handle(msg); err != nil {
			errc <- err
			return
		}
	}
}

func (p *Peer) pingLoop() {
	defer p.wg.Done()

}

func (p* Peer) handle(msg Msg) error {
	switch msg.msg.FuncName {
	case generated.LegacyMessage_VE:
		p.log.Debug("Received VE MSG")
		if msg.msg.GetVeData() == nil {
			out := Msg{}
			veData := generated.VEData{
				Version: "",
				GenesisPrevHash: []byte("0"),
				RateLimit: 100,
			}
			out.msg = &generated.LegacyMessage {
				FuncName: generated.LegacyMessage_VE,
				Data: &generated.LegacyMessage_VeData{
					VeData: &veData,
				},
			}
			err := p.WriteMsg(out)
			return err
		}
		veData := msg.msg.GetVeData()
		p.log.Info("", "version:", veData.Version,
			"GenesisPrevHash:", veData.GenesisPrevHash, "RateLimit:", veData.RateLimit)

	case generated.LegacyMessage_PL:
		p.log.Debug("Received PL MSG")
	case generated.LegacyMessage_PONG:
		p.log.Debug("Received PONG MSG")
	case generated.LegacyMessage_MR:
		mrData := msg.msg.GetMrData()
		if p.filter.Test(mrData.Hash) {
			return nil
		}

		switch mrData.Type {
		case generated.LegacyMessage_BK:
			if mrData.BlockNumber > p.chain.Height() + uint64(p.config.Dev.MaxMarginBlockNumber) {
				p.log.Debug("Skipping block #%s as beyond lead limit", "Block #", mrData.BlockNumber)
				return nil
			}
			if mrData.BlockNumber < p.chain.Height() - uint64(p.config.Dev.MinMarginBlockNumber) {
				p.log.Debug("'Skipping block #%s as beyond the limit", "Block #", mrData.BlockNumber)
				return nil
			}
			_, err := p.chain.GetBlock(mrData.PrevHeaderhash)
			if err != nil {
				p.log.Debug("Missing Parent Block", "Block:", mrData.Hash,
					"Parent Block ", mrData.PrevHeaderhash)
				return nil
			}
			// Request for full message
			// Check if its already being feeded by any other peer
		case generated.LegacyMessage_TX:
			// Check transactions pool Size,
			// if full then ignore
		default:
			//connection lost
		}
	case generated.LegacyMessage_SFM:
	case generated.LegacyMessage_BK:
	case generated.LegacyMessage_FB:
	case generated.LegacyMessage_PB:
	case generated.LegacyMessage_BH:
	case generated.LegacyMessage_TX:
	case generated.LegacyMessage_LT:
	case generated.LegacyMessage_EPH:
	case generated.LegacyMessage_MT:
	case generated.LegacyMessage_TK:
	case generated.LegacyMessage_TT:
	case generated.LegacyMessage_SL:
	case generated.LegacyMessage_SYNC:
	case generated.LegacyMessage_CHAINSTATE:
	case generated.LegacyMessage_HEADERHASHES:
	case generated.LegacyMessage_P2P_ACK:
	}
	return nil
}

func (p *Peer) run() (remoteRequested bool, err error) {
	var (
		writeStart = make(chan struct{}, 1)
		writeErr = make(chan error, 1)
		readErr	 = make(chan error, 1)
		reason 	 DiscReason
	)
	p.wg.Add(2)
	go p.readLoop(readErr)
	go p.pingLoop()

loop:
	for {
		select {
		case err = <-writeErr:
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
	close(p.closed)
	p.close(reason)
	p.wg.Wait()
	return remoteRequested, err
}

func (p *Peer) close(err error) {
	p.conn.Close()
}

func (p *Peer) Disconnect(reason DiscReason) {
	select {
	case p.disc <- reason:
	case <-p.closed:
	}
}

func convertBytesToLong(b []byte) uint32 {
	return uint32(b[0]) << 24 | uint32(b[1]) << 16 | uint32(b[2]) << 8 | uint32(b[3])
}
