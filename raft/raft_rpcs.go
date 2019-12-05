package raft

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/rpc"

	"./Timer"
)

//==============================================================================
// Server and client setups for RPCs
//==============================================================================
//Register and run rpc server
func (state *ServerState) SetupRPCServer() {
	rpc.Register(state)

	rpc.HandleHTTP()
	port := serverPorts[state.id]
	l, e := net.Listen("tcp", port)
	if e != nil {
		log.Fatal("listen error:", e)
	}

	fmt.Println("Server online.")
	//Starts servicing
	go http.Serve(l, nil)
	fmt.Println("Waiting calls.")
	select{} //Block forever
}

//Setup all rpc client connections and return them in a slice of pointers
func (state *ServerState) SetupRPCClients() []*rpc.Client {
	clientConnections := make([]*rpc.Client, len(serverPorts))

	for i := 0; i < len(clientConnections); i++ {
		client, err := rpc.DialHTTP("tcp", "127.0.0.1"+serverPorts[i])

		for err != nil { //If dial failed, try again until it succeeds. All servers are expected work on start
			fmt.Println("dialing: error")
			fmt.Println("Trying again.")
			client, err = rpc.DialHTTP("tcp", "127.0.0.1"+serverPorts[i])
		}

		clientConnections[i] = client //store client on slice
	}
	
	return clientConnections
}

//==============================================================================
// Structs for RPCs
//==============================================================================
// AppendEntry parameters
type AppendEntriesArgs struct {
	term         int
	leaderID     int
	prevLogIndex int
	prevLogTerm  int
	entries      []LogEntry
	leaderCommit int
}

// RequestVote parameters
type RequestVoteArgs struct {
	Term         int
	CandidateID  int
	LastLogIndex int
	LastLogterm  int
}

type AppendEntriesResult struct {
	term    int
	success bool
}

type RequestVoteResult struct {
	Term        int
	VoteGranted bool
}

//==============================================================================

//==============================================================================
// RPC functions
//==============================================================================

// RPC RequestVote (Args, Reply)
func (state *ServerState) RequestVote(candidate *RequestVoteArgs, vote *RequestVoteResult) error {
	fmt.Println("request vote received")
	state.mux.Lock()
	defer state.mux.Unlock() //Will be called once function returns

	vote.Term = state.currentTerm
	vote.VoteGranted = false

	// 1. Reply false if term < currentTerm
	if candidate.Term < state.currentTerm {
		vote.VoteGranted = false
        fmt.Println("vote false: candidate has lower term")
		// 2. If votedFor is null or candidateID, and candidate is up to date, grant vote
	} else if isUpToDate(state, candidate) {
		if (state.votedFor == -1 || state.votedFor == candidate.CandidateID) ||
			candidate.Term > state.currentTerm {
			fmt.Println("voted for ", state.votedFor)
			state.votedFor = candidate.CandidateID
			state.currentTerm = candidate.Term
			vote.VoteGranted = true
		} else {
		    fmt.Println("vote false: voteFor not nil or not candidate")
		}
	} else {
	    fmt.Println("vote false: not up-to-date")
	}
	//A rpc descrita no artigo não trata o caso em que um CANDIDATO que já votou em si recebe
	//RequestVote de outro candidato de TERMO MAIOR (caso comum que ocorre quando uma votação
	//termina sem lider e algum candidato é o primeiro a dar timeout e aumentar seu termo).
	//Se isso acontece, um candidato, mesmo já tendo votado em si, deve atualizar seu termo e
	//dar o voto para o candidato de maior termo que o pediu, desde que este esteja up-to-date.

	return nil
}

// RPC AppendEntry (Args, Reply)
func (state *ServerState) AppendEntry(args *AppendEntriesArgs, rep *AppendEntriesResult) error {
	state.mux.Lock()
	defer state.mux.Unlock() //Will be called once function returns

	rep.term = state.currentTerm
	rep.success = false

	if args.term < state.currentTerm { // 1.  Reply false if term < currentTerm
		return nil
	}

	if !state.timer.Stop() && state.curState == FOLLOWER {
		return nil
	}

	if len(state.log) < args.prevLogIndex || // 2.  Reply false if log doesn’t contain an entry at prevLogIndex
		state.log[args.prevLogIndex].term != args.prevLogTerm { // whose term matches prevLogTerm
		// return nil // restart timer
	} else {
		// if state.log[args.prevLogIndex].term == args.prevLogTerm {
		// 3. If an existing entry conflicts with a new one (same index but different terms)
		state.log = state.log[:args.prevLogIndex+1] // delete the existing entry and all that follow it
		// }

		// 4. Append any new entries not already in the log
		for _, entry := range args.entries {
			state.log = append(state.log, entry)
		}

		if args.leaderCommit > state.commitIndex {
			// 5. If leaderCommit > commitIndex, set commitIndex = min(leaderCommit, index of last new entry)
			if args.leaderCommit > len(state.log) {
				state.commitIndex = len(state.log)
			} else {
				state.commitIndex = args.leaderCommit
			}
		}
		state.currentTerm = rep.term
		state.curState = FOLLOWER // When receiving AppendEntries -> convert to follower
		rep.success = true
	}

	state.timer.Reset(Timer.GenRandom())
	return nil
}

//==============================================================================

//==============================================================================
// Miscellaneous
//==============================================================================

//Returns true if candidate requesting vote is at least as "up-to-date" as this server
/* From paper:
   Raft determines which of two logs is more up-to-date
   by comparing the index and term of the last entries in the
   logs. If the logs have last entries with different terms, then
   the log with the later term is more up-to-date. If the logs
   end with the same term, then whichever log is longer is
   more up-to-date. */
func isUpToDate(state *ServerState, candidate *RequestVoteArgs) bool {
	return state.log[len(state.log)-1].term <= candidate.LastLogterm &&
		len(state.log)-1 <= candidate.LastLogIndex
}

//==============================================================================
