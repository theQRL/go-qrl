package p2p

import (
	"errors"
	"fmt"
	"net"
	"sync"

	"github.com/willf/bloom"

	"github.com/theQRL/go-qrl/config"
	"github.com/theQRL/go-qrl/core"
	"github.com/theQRL/go-qrl/log"
)

type conn struct {
	fd		net.Conn
	inbound	bool
}

type Server struct {
	config *config.Config

	chain *core.Chain

	listener net.Listener
	lock     sync.Mutex

	running bool
	loopWG  sync.WaitGroup
	log     log.Logger

	exit    chan struct{}
	addpeer chan *conn
	delpeer chan peerDrop

	filter *bloom.BloomFilter
}

type peerDrop struct {
	*Peer
	err       error
	requested bool // true if signaled by the peer
}

func (srv *Server) Start(log log.Logger, config *config.Config, chain *core.Chain) (err error) {
	srv.lock.Lock()
	defer srv.lock.Unlock()
	if srv.running {
		return errors.New("server is already running")
	}

	srv.config = config
	srv.exit = make(chan struct{})
	srv.addpeer = make(chan *conn)
	srv.delpeer = make(chan peerDrop)
	srv.log = log
	srv.chain = chain

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
		srv.log.Debug("New Peer joined")
		if err != nil {
			srv.log.Error("Read ERROR", "Reason", err)
			return
		}
		srv.log.Debug("called addpeer")
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
			srv.log.Debug("Quitting!!!")
			break running
		case c := <-srv.addpeer:
			srv.log.Debug("Adding peer", "addr", c.fd.RemoteAddr())
			p := newPeer(&c.fd, c.inbound, srv.chain, &srv.log, srv.filter, srv.config)
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
