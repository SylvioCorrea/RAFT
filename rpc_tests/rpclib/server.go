package rpclib

import ("net"
        "net/rpc"
        "net/http"
        "log"
        "fmt"
        "os"
        "bufio"
        "sync"
        )

//Server process
func ServerStart() {
    
    state_ptr := &ServerState{
                ServerIntA: 0,
                ServerIntB: 0,
                Mutex: sync.Mutex{}}
    
    //Registers rpc function using golang sorcery
    rpc.Register(state_ptr)
    
    rpc.HandleHTTP()
    
    l, e := net.Listen("tcp", ":8001")
    if e != nil {
        log.Fatal("listen error:", e)
    }
    
    fmt.Println("Server online.")
    //Starts servicing
    go http.Serve(l, nil)
    fmt.Println("Waiting calls.")
    //Block forever
    reader := bufio.NewReader(os.Stdin)
    for {
        reader.ReadString('\n')
        fmt.Printf("ServerState ServerIntA is %d\n", state_ptr.ServerIntA)
    }
}

