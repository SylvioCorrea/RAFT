package main

import (
	"fmt"
	"os"
	"strconv"

	"./raft"
)

func errorCheck(e error) {
	if e != nil {
		panic(e)
	}
}

func main() {

	if len(os.Args) < 2 {
		fmt.Println("Usage:\n>go run raft_server_start.go n")
		fmt.Println("where n is the id of this node in the network.")
		os.Exit(1)
	}
	serverID, err := strconv.Atoi(os.Args[1])
	errorCheck(err)

	server := raft.ServerStateInit(serverID)

	server.ServerMainLoop()
}
