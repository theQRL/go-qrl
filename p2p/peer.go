package p2p

import (
	"net"
	"github.com/cyyber/go-QRL/log"
	"sync"
	"io"
	"github.com/cyyber/go-QRL/generated"
	"github.com/golang/protobuf/proto"
	"time"
)

type Peer struct {
	conn		net.Conn
	inbound		bool

	wg			sync.WaitGroup
	closed		chan struct{}
	log			log.Logger
}

func newPeer(conn *net.Conn, inbound bool, log *log.Logger) *Peer {
	p := &Peer {
		conn: conn,
		inbound: inbound,
		log: log,
	}
	return p
}

func (p *Peer) WriteMsg(msg Msg) error {
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

	return msg, err
}

func (p *Peer) readLoop(errc chan<- error) {
	defer p.wg.Done()
	for {
		msg, err := p.ReadMsg()
		if err != nil {
			errc <- err
			return
		}
		msg.ReceivedAt = time.Now()
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
	switch {

	}
	return nil
}

func (p *Peer) run() (remoteRequested bool, err error) {
	p.wg.Add(2)
	readErr := make(chan error, 1)
	go p.readLoop(readErr)
	go p.pingLoop()
}

func convertBytesToLong(b []byte) uint32 {
	return uint32(b[0]) << 24 | uint32(b[1]) << 16 | uint32(b[2]) << 8 | uint32(b[3])
}