package rpclib

import ("fmt"
        "log"
        "net/rpc")

//Avoiding "imported and not used" compiler nonsense. Not to be used anywhere in the code.
func cancer() {
    fmt.Println("")
}

//Client process
func ClientStart(id int, quit chan int) {
    
    //Server address is the loopback ip
    serverAddressAndPort := "127.0.0.1:8001"
    
    client, err := rpc.DialHTTP("tcp", serverAddressAndPort)
    if err != nil {
        log.Fatal("dialing:", err)
    }
    
    var reply bool
    
    //Performs 3000 calls to server rpc IncA
    for i:=0; i<3000; i++ {
        err = client.Call("ServerState.IncA", 0, &reply)
        if err != nil {
            log.Fatal("rpc error:", err)
        }
    }
    
    quit <- id
}