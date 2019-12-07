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
	Term         int
	LeaderID     int
	PrevLogIndex int
	PrevLogTerm  int
	Entries      []LogEntry
	LeaderCommit int
}

// RequestVote parameters
type RequestVoteArgs struct {
	Term         int
	CandidateID  int
	LastLogIndex int
	LastLogterm  int
}

//TODO: MAKE FIELDS PUBLIC
type AppendEntriesResult struct {
	Term    int
	Success bool
}

type RequestVoteResult struct {
	Term        int
	VoteGranted bool
}

//==============================================================================

//==============================================================================
// RPC functions
//==============================================================================

func (state *ServerState) RequestVote(candidate *RequestVoteArgs, vote *RequestVoteResult) error {
	fmt.Println("RequestVoteRPC: received")
	state.mux.Lock()
	defer state.mux.Unlock() //Will be called once function returns

	vote.Term = state.currentTerm
	vote.VoteGranted = false

	// 1. Reply false if term < currentTerm
	if candidate.Term < state.currentTerm {
		vote.VoteGranted = false
		fmt.Println("RequestVoteRPC: FALSE -- candidate has lower term")
		// 2. If votedFor is null or candidateID, and candidate is up to date, grant vote

	} else if isUpToDate(state, candidate) {
		if state.CanGrantVote(candidate) {
			//One final test remains. The server will only grant the vote if it hasn't timed out in the meantime.
			if state.timer.Stop() {
				state.votedFor = candidate.CandidateID
				fmt.Println("RequestVoteRPC: TRUE --  vote granted for ", state.votedFor)
				state.currentTerm = candidate.Term
				vote.VoteGranted = true
				state.curState = FOLLOWER
				state.timer.Reset(Timer.GenRandom())
			} else {
				fmt.Println("RequestVoteRPC: FALSE -- timed out")
				//No need to drain the channel. The state machine is expecting it's signal.
			}
		} else {
			fmt.Println("RequestVoteRPC: FALSE -- voteFor not nil or not currently chosen candidate.")
		}

	} else {
		fmt.Println("RequestVoteRPC: FALSE -- candidate not up-to-date")
	}

	//A rpc descrita no artigo não trata o caso em que um CANDIDATO que já votou em si recebe
	//RequestVote de outro candidato de TERMO MAIOR (caso comum que ocorre quando uma votação
	//termina sem lider e algum candidato é o primeiro a dar timeout e aumentar seu termo).
	//Se isso acontece, um candidato, mesmo já tendo votado em si, deve atualizar seu termo e
	//dar o voto para o candidato de maior termo que o pediu, desde que este esteja up-to-date.

	return nil
}

func (state *ServerState) AppendEntry(args *AppendEntriesArgs, rep *AppendEntriesResult) error {
	fmt.Println("AppendEntriesRPC: received")
	state.mux.Lock()
	defer state.mux.Unlock() //Will be called once function returns
	fmt.Println("AppendEntriesRPC: locked.")

	rep.Term = state.currentTerm
	rep.Success = false

	fmt.Println("AppendEntriesRPC: check 1.")
	// 1.  Reply false if term < currentTerm
	if args.Term < state.currentTerm {
		fmt.Println("AppendEntriesRPC: FALSE. Leader's term is outdated")
		return nil
	}

	//At this point, if this server isn't already a follower it must become one, because the caller must
	//have been elected by the majority before sending AppendEntries calls
	state.curState = FOLLOWER

	fmt.Println("AppendEntriesRPC: check 2.")
	if !state.timer.Stop() { //Timeout ocurred just before receiving
		//No need to drain the timer channel since the timeout signal is received by the FOLLOWER routine
		fmt.Println("AppendEntriesRPC: FALSE. Timed out before receiving")
		return nil
	}
	//Stop successful

	fmt.Println("AppendEntriesRPC: check 3.")
	lastLogIndex := len(state.log) - 1
	// 2.  Reply false if log doesn’t contain an entry at prevLogIndex ...
	if lastLogIndex < args.PrevLogIndex {
		fmt.Println("AppendEntriesRPC: FALSE. Does not contain entry at prevLogIndex.")
		return nil
	}
	// ... whose term matches prevLogTerm
	if state.log[args.PrevLogIndex].Term != args.PrevLogTerm {
		fmt.Println("AppendEntriesRPC: FALSE. Entry at prevLogIndex has different term.")
		return nil

	}

	if len(args.Entries) > 0 { //Not just a heartbeat
		//Append all new entries overwriting outdated ones
		state.log = append(state.log[:args.PrevLogIndex+1], args.Entries...)

		if args.LeaderCommit > state.commitIndex { //TODO: check this
			// 5. If leaderCommit > commitIndex, set commitIndex = min(leaderCommit, index of last new entry)
			if args.LeaderCommit > len(state.log) {
				state.commitIndex = len(state.log)
			} else {
				state.commitIndex = args.LeaderCommit
			}
		}
		state.currentTerm = rep.Term
		state.curState = FOLLOWER // When receiving AppendEntries -> convert to follower
		rep.Success = true
		fmt.Println("AppendEntriesRPC: TRUE. Log replicated.")
		state.timer.Reset(Timer.GenRandom())
		return nil
	}

	//Just a heartbeat
	rep.Success = true
	fmt.Println("AppendEntriesRPC: TRUE. Heartbeat reply.")
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
	return state.log[len(state.log)-1].Term <= candidate.LastLogterm &&
		len(state.log)-1 <= candidate.LastLogIndex
}

/* If the candidate is up-to-date, it will receive a vote as long as these conditions are met.
   A reasonable implementation would avoid being so verbose, but this one is supposed
   to be understandable, not fast. */
func (state *ServerState) CanGrantVote(candidate *RequestVoteArgs) bool {
	hasNotVotedYet := state.votedFor == -1
	votedForCandidate := state.votedFor == candidate.CandidateID
	candidateHasHigherTerm := candidate.Term > state.currentTerm
	return hasNotVotedYet || votedForCandidate || candidateHasHigherTerm
}

//==============================================================================
