package rpclib

import ("fmt"
        "sync")

type ServerState struct {
    ServerIntA int
    ServerIntB int
    Mutex sync.Mutex
}

//RPC call to increment serverIntA in ServerState
func (s *ServerState) IncA(arg *int, reply *bool) error {
    //Locks access to the state before incrementing
    s.Mutex.Lock()
        s.ServerIntA++
        fmt.Printf("ServerIntA is now %d\n", s.ServerIntA)
    s.Mutex.Unlock()
    
    *reply = true
    return nil
}

//RPC call to increment serverIntB in ServerState
func (s *ServerState) IncB(arg *int, reply *bool) error {
    //Locks access to the state before incrementing
    s.Mutex.Lock()
        s.ServerIntB++
        fmt.Printf("ServerIntB is now %d\n", s.ServerIntB)
    s.Mutex.Unlock()
    
    *reply = true
    return nil
}