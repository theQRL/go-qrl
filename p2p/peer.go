package p2p

import (
	"net"
	"github.com/cyyber/go-QRL/log"
	"sync"
	"io"
)

type Peer struct {
	conn		net.Conn

	wg			sync.WaitGroup
	closed		chan struct{}
	log			log.Logger

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
}

func (p *Peer) pingLoop() {

}

func (p* Peer) handle(msg Msg) error {
	switch {

	}
	return nil
}

func convertBytesToLong(b []byte) uint32 {
	return uint32(b[0]) << 24 | uint32(b[1]) << 16 | uint32(b[2]) << 8 | uint32(b[3])
}