package main

import (
	"bufio"
	"os"
	"os/signal"
	"strings"

	"github.com/theQRL/go-qrl/pkg/core/chain"
	"github.com/theQRL/go-qrl/pkg/log"
	"github.com/theQRL/go-qrl/pkg/p2p"
)

var (
	server *p2p.Server
	input = bufio.NewReader(os.Stdin)
	logger = log.GetLogger()
)

func startServer() error {
	c, err := chain.CreateChain()
	if err != nil {
		return err
	}

	c.Load() // Loads Chain State

	err = server.Start(c)
	if err != nil {
		return err
	}
	return nil
}

func initialize() {
	server = &p2p.Server{}
	logger.Info("Server Initialized")
}

func run() {
	err := startServer()
	if err != nil {
		logger.Error("error while starting server", err)
		return
	}
	defer server.Stop()
	logger.Info("Connecting Peers")
	server.ConnectPeers()
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
	logger.Info("Shutting Down Server")
}

func sendLoop() {
	for {
		txt, err := input.ReadString('\n')
		if err != nil {
			logger.Error("input error: %s", err)
		}
		txt = strings.TrimRight(txt, "\n\r")
		if txt == "quit" {
			return
		}
	}
}

func main() {
	logger.Info("Starting")
	initialize()
	run()
	logger.Info("Shutting Down Node")
}