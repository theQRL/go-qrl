package main

import (
	"bufio"
	"os"
	"strings"
	"github.com/cyyber/go-QRL/p2p"
	"github.com/cyyber/go-QRL/log"
	"github.com/cyyber/go-QRL/core"
)

var (
	server *p2p.Server
	config *core.Config
	input = bufio.NewReader(os.Stdin)
	logger = log.New()
)

func startServer() error {
	err := server.Start(logger, config)
	if err != nil {
		return err
	}
	return nil
}

func initialize() {
	config = core.GetConfig()
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