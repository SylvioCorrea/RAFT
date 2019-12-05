package main

import (
        "./raft"
        )
        
func main() {
    
    server := raft.ServerStateInit(2)
    
    server.ServerMainLoop()
}
