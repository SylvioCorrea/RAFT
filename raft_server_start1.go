package main

import (
        "./raft"
        )
        
func main() {

    
    server := raft.ServerStateInit(1)
    
    server.ServerMainLoop()
}
