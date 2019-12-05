package main

import (
        "./raft"
        )
        
func main() {
    
    server := raft.ServerStateInit(0)
    
    server.ServerMainLoop()
}
