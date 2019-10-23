package main

import ("./rpclib"
        "sync"
        )

//Runs a number of clients concurrently and then exits
func main() {
    
    //Number of clients to be run
    nOfClients := 20
    
    //Barrier that will block main until all client processes are done
    var wg sync.WaitGroup
    wg.Add(nOfClients)
    
    //Runs nOfClients clients concurrently
    for i:=0; i<nOfClients; i++ {
        go func(n int) {
           //Runs a client process
           rpclib.ClientStart(n)
           //Signals WaitGroup the go routine is done
           wg.Done()
        }(i) //Passing i as argument n, otherwise i could be incremented by the loop before calling ClientStart()
    }
    
    //Waits on nOfClients client processes
    wg.Wait()
}