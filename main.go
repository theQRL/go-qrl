package main

import (
	"go-QRL/p2p"
	"fmt"
	"bufio"
	"os"
	"strings"
)

var (
	server *p2p.Server
	input = bufio.NewReader(os.Stdin)
)

func startServer() error {
	err := server.Start()
	if err != nil {
		return err
	}
	return nil
}

func initialize() {
	server = &p2p.Server{}
}

func run() {
	err := startServer()
	if err != nil {
		fmt.Println(err)
		return
	}
	defer server.Stop()

	sendLoop()
}

func sendLoop() {
	for {
		txt, err := input.ReadString('\n')
		if err != nil {
			fmt.Print("input error: ", err)
		}
		txt = strings.TrimRight(txt, "\n\r")
		fmt.Println(txt)
		if txt == "quit" {
			return
		}
	}
}

func main() {
	initialize()
	run()
	fmt.Println("qutting..............")
}