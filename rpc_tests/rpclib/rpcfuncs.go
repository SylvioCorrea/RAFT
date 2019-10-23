package rpclib

import ("fmt")

type ServerState struct {
    ServerIntA int
    ServerIntB int
}

//RPC call to increment serverIntA in ServerState
func (s *ServerState) IncA(arg *int, reply *bool) error {
    s.ServerIntA++
    fmt.Printf("ServerIntA is now %d\n", s.ServerIntA)
    *reply = true
    return nil
}

//RPC call to increment serverIntB in ServerState
func (s *ServerState) IncB(arg *int, reply *bool) error {
    s.ServerIntB++
    *reply = true
    return nil
}