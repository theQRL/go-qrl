package main

import (
	"bufio"
	"os"
	"strings"

	c "github.com/theQRL/go-qrl/config"
	"github.com/theQRL/go-qrl/core"
	"github.com/theQRL/go-qrl/log"
	"github.com/theQRL/go-qrl/p2p"
)

var (
	server *p2p.Server
	config *c.Config
	input = bufio.NewReader(os.Stdin)
	logger = log.New()
)

func startServer() error {
	chain, err := core.CreateChain(&logger, config)
	if err != nil {
		return err
	}

	err = server.Start(logger, config, chain)
	if err != nil {
		return err
	}
	return nil
}

func initialize() {
	config = c.GetConfig()
	server = &p2p.Server{}
}

func run() {
	err := startServer()
	if err != nil {
		logger.Error("error while starting server", err)
		return
	}
	defer server.Stop()

	sendLoop()
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
	logger.Info("quitting..............")
}