package main

import (
	"github.com/theQRL/go-qrl/pkg/config"
	"github.com/theQRL/go-qrl/pkg/core/chain"
	"github.com/theQRL/go-qrl/pkg/log"
	"github.com/theQRL/go-qrl/pkg/p2p"
	"github.com/theQRL/go-qrl/pkg/p2p/notification"
	"os"
	"os/signal"
)

var (
	server *p2p.Server
	notificationServer *notification.NotificationServer
	logger = log.GetLogger()
)

func startServer() error {
	c, err := chain.CreateChain()
	if err != nil {
		return err
	}

	c.Load() // Loads Chain State

	// Start Notification Server if enabled in config
	var newBlockNotificationChannel chan []byte
	if config.GetUserConfig().NotificationServerConfig.EnableNotificationServer {
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
}

func main() {
	//Enable only for debugging, Used for profiling
	//go func() {
	//	http.ListenAndServe("localhost:6060", nil)
	//}()
	logger.Info("Starting")
	initialize()
	run()
	logger.Info("Shutting Down Node")
}