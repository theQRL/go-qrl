package main

import (
	"github.com/theQRL/go-qrl/api"
	"github.com/theQRL/go-qrl/pkg/config"
	"github.com/theQRL/go-qrl/pkg/core/chain"
	"github.com/theQRL/go-qrl/pkg/core/state"
	"github.com/theQRL/go-qrl/pkg/genesis"
	"github.com/theQRL/go-qrl/pkg/log"
	"github.com/theQRL/go-qrl/pkg/p2p"
	"github.com/theQRL/go-qrl/pkg/p2p/notification"
	"github.com/theQRL/go-qrl/pkg/writers/mongodb"
	"os"
	"os/signal"
	"sync"
)

var (
	server *p2p.Server
	publicAPIServer *api.PublicAPIServer
	notificationServer *notification.NotificationServer
	mongoProcessor *mongodb.MongoProcessor
	loopWG sync.WaitGroup
	logger = log.GetLogger()
)

func startServer() error {
	s, err := state.CreateState()
	if err != nil {
		return err
	}

	c := chain.CreateChain(s)
	if err != nil {
		return err
	}

	genesisBlock, err := genesis.CreateGenesisBlock()
	if err != nil {
		logger.Warn("Error Loading Genesis Block")
		return err
	}

	c.Load(genesisBlock) // Loads Chain State

	conf := config.GetConfig()

	// Start Notification Server if enabled in config
	var newBlockNotificationChannel chan []byte
	if conf.User.NotificationServerConfig.EnableNotificationServer {
		notificationServer = &notification.NotificationServer{}
		err := notificationServer.Start(c)
		if err != nil {
			logger.Error("Failed to start Notification Server!!!",
				"Error", err.Error())
		} else {
			newBlockNotificationChannel = notificationServer.GetNewBlockNotificationChannel()
		}
	}

	if newBlockNotificationChannel != nil {
		c.SetNewBlockNotificationChannel(newBlockNotificationChannel)
	}

	server = &p2p.Server{}

	err = server.Start(c)
	if err != nil {
		return err
	}

	if conf.User.API.PublicAPI.Enabled {
		publicAPIServer = api.NewPublicAPIServer(c, server.GetRegisterAndBroadcastChan())
		go publicAPIServer.Start()
	}
	mongoProcessorConfig := conf.User.MongoProcessorConfig
	if mongoProcessorConfig.Enabled {
		mongoProcessor, err = mongodb.CreateMongoProcessor(mongoProcessorConfig.DBName, c)
		if err != nil {
			return err
		}
		go mongoProcessor.Run()
	}

	return nil
}

func initialize() {
	logger.Info("Server Initialized")
}

func run() {
	err := startServer()
	if err != nil {
		logger.Error("error while starting server", err)
		return
	}
	defer server.Stop()
	if notificationServer != nil {
		defer notificationServer.Stop()
	}

	logger.Info("Connecting Peers")
	server.LoadPeerList()
	go server.ConnectPeers()
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit

	logger.Info("Shutting Down Server")

	if mongoProcessor != nil {
		logger.Info("Closing MongoProcessor")
		close(mongoProcessor.Exit)
		mongoProcessor.LoopWG.Wait()
		logger.Info("MongoProcessor Successfully Closed")
	}
}

func ConfigCheck() bool {
	c := config.GetConfig()
	// Both config cannot be True
	if c.User.Miner.MiningEnabled && c.User.API.MiningAPI.Enabled {
		logger.Error("Both MiningEnabled & MiningAPI.Enabled are true")
		logger.Error("Disable either of them")
		return false
	}

	// Some checks only when Mining is Enabled
	if c.User.Miner.MiningEnabled {
		// TODO: Check if Valid QRL Address
		//if c.User.Miner.MiningAddress
	}
	return true
}

func main() {
	//Enable only for debugging, Used for profiling
	//go func() {
	//	http.ListenAndServe("localhost:6060", nil)
	//}()
	if !ConfigCheck() {
		logger.Info("Invalid Config")
		return
	}
	logger.Info("Starting")
	initialize()
	run()
	logger.Info("Shutting Down Node")
}