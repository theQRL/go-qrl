package p2p

import (
	"net"
	"sync"
	"errors"
	"github.com/cyyber/go-QRL/log"
	"fmt"
	"github.com/ethereum/go-ethereum/p2p/discover"
)

type conn struct {
	fd		net.Conn
	inbound	bool
}

type Server struct {
	listener	net.Listener
	lock		sync.Mutex

	running		bool
	loopWG 		sync.WaitGroup
	log			log.Logger

	exit          chan struct{}
	addpeer       chan *conn
	delpeer       chan peerDrop
}

type peerDrop struct {
	*Peer
	err       error
	requested bool // true if signaled by the peer
}

func (srv *Server) Start(log log.Logger) (err error) {
	srv.lock.Lock()
	defer srv.lock.Unlock()
	if srv.running {
		return errors.New("server is already running")
	}

	srv.exit = make(chan struct{})
	srv.log = log
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
		srv.addpeer <- &conn{c, true}
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
	listener, err := net.Listen("tcp", ":9000")  // Move to config
	if err != nil {
		return err
	}
	srv.listener = listener
	go srv.listenLoop(listener)
	return nil
}

func (srv *Server) run() {
	var (
		peers        = make(map[discover.NodeID]*Peer)
		inboundCount = 0
	)

	srv.loopWG.Add(1)
	defer srv.loopWG.Done()

running:
	for {
		select {
		case <-srv.exit:
			fmt.Print("Quitting...........")
			break running
		case c := <- srv.addpeer:
			p := newPeer(&c.fd, c.inbound, &srv.log)
			srv.log.Debug("Adding peer", "addr", c.fd.RemoteAddr())
			go srv.runPeer(p)
			if p.inbound {
				inboundCount++
			}
		case pd := <-srv.delpeer:
			pd.log.Debug("Removing Peer", "err", pd.err)
			if pd.inbound {
				inboundCount--
			}
		}
	}
	for _, p := range peers {
		p.Disconnect(DiscQuitting)
	}

	for len(peers) > 0 {
		p := <-srv.delpeer
		p.log.Trace("")
	}
}

func (srv *Server) runPeer(p *Peer) {
	remoteRequested, err := p.run()

	srv.delpeer <- peerDrop{p, err, remoteRequested}
}