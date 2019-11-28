package main

import ("./rpclib"
        "sync"
        "fmt"
        )

//Avoiding "imported and not used" compiler nonsense. Not to be used anywhere in the code.
func cancer() {
    var wg sync.WaitGroup
    wg.Add(10)
    fmt.Println("")
}

//Runs a number of clients concurrently and then exits
func main() {
    
    //Number of clients to be run
    nOfClients := 30
    quit := make(chan int)
    
    //Runs nOfClients clients concurrently
    for i:=0; i<nOfClients; i++ {
        go rpclib.ClientStart(i, quit)
    }
    
    for i:=0; i<nOfClients; i++ {
        n := <-quit
        fmt.Printf("Client %d done.\n", n)
    }
}