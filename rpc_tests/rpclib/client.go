package rpclib

import ("fmt"
        "log"
        "net/rpc")

//Client process
func ClientStart(id int) {
    
    //Server address is the loopback ip
    serverAddressAndPort := "127.0.0.1:8001"
    
    client, err := rpc.DialHTTP("tcp", serverAddressAndPort)
    if err != nil {
        log.Fatal("dialing:", err)
    }
    
    var reply bool
    
    //Performs 1000 calls to server rpc IncA
    for i:=0; i<1000; i++ {
        err = client.Call("ServerState.IncA", 0, &reply)
        if err != nil {
            log.Fatal("rpc error:", err)
        }
    }
    
    fmt.Printf("Client %d done.\n", id)
}