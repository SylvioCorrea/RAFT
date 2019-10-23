package rpclib

import ("net"
        "net/rpc"
        "net/http"
        "log"
        "fmt"
        )

//Server process
func ServerStart() {
    
    state_ptr := &ServerState{
                ServerIntA: 0,
                ServerIntB: 0}
    
    //Registers rpc function using golang sorcery
    rpc.Register(state_ptr)
    
    rpc.HandleHTTP()
    
    l, e := net.Listen("tcp", ":8001")
    if e != nil {
        log.Fatal("listen error:", e)
    }
    
    fmt.Println("Server online.")
    
    //Starts servicing
    http.Serve(l, nil)
}

