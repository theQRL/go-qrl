package gqrl

import (
	"bufio"
	"os"
	"strings"

	"github.com/theQRL/go-qrl/pkg/config"
	"github.com/theQRL/go-qrl/pkg/core/chain"
	"github.com/theQRL/go-qrl/pkg/log"
	"github.com/theQRL/go-qrl/pkg/p2p"
)

var (
	server *p2p.Server
	conf *config.Config
	input = bufio.NewReader(os.Stdin)
	logger = log.New()
)

func startServer() error {
	c, err := chain.CreateChain(&logger, conf)
	if err != nil {
		return err
	}

	err = server.Start(logger, conf, c)
	if err != nil {
		return err
	}
	return nil
}

func initialize() {
	conf = config.GetConfig()
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