package notification

import (
	"errors"
	"fmt"
	"github.com/theQRL/go-qrl/pkg/config"
	"github.com/theQRL/go-qrl/pkg/core/chain"
	"github.com/theQRL/go-qrl/pkg/log"
	"github.com/theQRL/go-qrl/pkg/ntp"
	"github.com/theQRL/go-qrl/pkg/p2p"
	"net"
	"sync"
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

type NotificationServer struct {
	config *config.Config

	chain *chain.Chain
	ntp   *ntp.NTP

	listener net.Listener
	lock     sync.Mutex

	running bool
	loopWG  sync.WaitGroup
	log     log.LoggerInterface

	exit                      chan struct{}
	addpeer                   chan *conn
	delPeer                   chan *peerDrop

	newBlockNotificationChannel chan []byte
}

func (ns *NotificationServer) Start(chain *chain.Chain) (err error) {
	ns.lock.Lock()
	defer ns.lock.Unlock()
	if ns.running {
		return errors.New("server is already running")
	}

	ns.config = config.GetConfig()
	ns.exit = make(chan struct{})
	ns.addpeer = make(chan *conn)
	ns.delPeer = make(chan *peerDrop)
	ns.newBlockNotificationChannel = make(chan []byte, 100)
	ns.log = log.GetLogger()
	ns.chain = chain
	ns.ntp = ntp.GetNTP()

	if err := ns.startListening(); err != nil {
		return err
	}
	ns.running = true
	go ns.run()
	return nil
}

func (ns *NotificationServer) GetNewBlockNotificationChannel() chan []byte {
	return ns.newBlockNotificationChannel
}

func (ns *NotificationServer) listenLoop(listener net.Listener) {
	ns.loopWG.Add(1)
	defer ns.loopWG.Done()

	for {
		c, err := listener.Accept()

		if err != nil {
			ns.log.Error("Read ERROR", "Reason", err)
			return
		}
		ns.log.Debug("New Notification Client Joined")
		ns.addpeer <- &conn{c, true}
	}
}

func (ns *NotificationServer) Stop() {
	ns.lock.Lock()
	defer ns.lock.Unlock()
	if !ns.running {
		return
	}
	ns.running = false
	if ns.listener != nil {
		ns.listener.Close()
	}
	close(ns.exit)
	ns.loopWG.Wait()
}

func (ns *NotificationServer) startListening() error {
	bindingAddress := fmt.Sprintf("%s:%d", ns.config.User.NotificationServerConfig.BindingIP, ns.config.User.NotificationServerConfig.LocalPort)
	listener, err := net.Listen("tcp", bindingAddress)
	if err != nil {
		return err
	}
	ns.listener = listener
	go ns.listenLoop(listener)
	return nil
}

func (ns *NotificationServer) run() {
	var (
		peers        = make(map[string]*Peer)
		inboundCount = 0
	)

	ns.loopWG.Add(1)
	defer ns.loopWG.Done()

running:
	for {
		select {
		case <-ns.exit:
			ns.log.Debug("Quitting!!!")
			break running
		case c := <-ns.addpeer:
			ns.log.Debug("Adding peer", "addr", c.fd.RemoteAddr())
			p := newPeer(&c.fd, c.inbound, ns.chain)
			go ns.runPeer(p)
			peers[c.fd.RemoteAddr().String()] = p
			if p.inbound {
				inboundCount++
			}
		case pd := <-ns.delPeer:
			pd.log.Debug("Removing Peer", "err", pd.err)
			delete(peers, pd.conn.RemoteAddr().String())
			if pd.inbound {
				inboundCount--
			}
		case headerHash := <-ns.newBlockNotificationChannel:
			for _, peer := range peers {
				err := peer.NotifyNewBlock(headerHash)
				if err != nil {
					ns.log.Error("Error while notifying",
						"Peer", peer.conn.RemoteAddr().String(),
						"Error", err.Error())
				}
			}
		}
	}
	for _, p := range peers {
		p.Disconnect(p2p.DiscQuitting)
	}
}

func (ns *NotificationServer) runPeer(p *Peer) {
	remoteRequested, err := p.run()

	ns.delPeer <- &peerDrop{p, err, remoteRequested}
}
