package p2p

import (
	"net"
	"sync"
	"errors"
	"fmt"
)

type Server struct {
	listener	net.Listener
	lock		sync.Mutex

	running		bool
	exit 		chan struct{}
	loopWG 		sync.WaitGroup
}

func (srv *Server) Start() (err error) {
	srv.lock.Lock()
	defer srv.lock.Unlock()
	if srv.running {
		return errors.New("server is already running")
	}

	srv.exit = make(chan struct{})
	srv.running = true

	if err := srv.startListening(); err != nil {
		return err
	}

	return nil
}

func (srv *Server) listenLoop(listener net.Listener) {
	srv.loopWG.Add(1)
	defer srv.loopWG.Done()
	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Read ERROR")
			return
		}
		conn.Write([] byte("Hello"))
		conn.Close()
		// Create New Peer here
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

//TODO: To be used later
//func (srv *Server) run() {
//	srv.loopWG.Add(1)
//	defer srv.loopWG.Done()
//
//running:
//	for {
//		select {
//		case <-srv.exit:
//			fmt.Print("Quitting...........")
//			break running
//		// Add peer
//		// Remove peer
//
//		}
//	}
//}