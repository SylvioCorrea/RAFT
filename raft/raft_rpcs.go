package raft

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/rpc"
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

//SetupRPCClients dials every other server and stores the client connections in the proper slice.
func (state *ServerState) SetupRPCClients() {
	for i, client := range state.clientConnections {
		if i != state.id && client == nil {
			go state.DialServer(i) //TODO: maybe this is a bad idea
		}
	}
}

//DialServer repeatedly attempts to establish a client connection with a server until it succeeds
func (state *ServerState) DialServer(serverID int) {
	client, err := rpc.DialHTTP("tcp", "127.0.0.1"+serverPorts[serverID])

	for err != nil { //If dial failed, try again until it succeeds. All servers are expected work on start
		fmt.Println("dialing: error. Trying again.")
		client, err = rpc.DialHTTP("tcp", "127.0.0.1"+serverPorts[serverID])
	}

	state.clientConnections[serverID] = client
}

//==============================================================================
// Structs for RPCs
//==============================================================================
// AppendEntriesArgs AppendEntry RPC parameters
type AppendEntriesArgs struct {
	Term         int
	LeaderID     int
	PrevLogIndex int
	PrevLogTerm  int
	Entries      []LogEntry
	LeaderCommit int
}

// RequestVoteArgs RequestVote RPC parameters
type RequestVoteArgs struct {
	Term         int
	CandidateID  int
	LastLogIndex int
	LastLogterm  int
}

// AppendEntriesResult AppendEntries RPC reply
type AppendEntriesResult struct {
	Term    int
	Success bool
}

// RequestVoteResult RequestVote RPC reply
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
		fmt.Println("RequestVoteRPC: FALSE -- candidate has lower term")
		// 2. If votedFor is null or candidateID, and candidate is up to date, grant vote

	} else if isUpToDate(state, candidate) {
		if state.CanGrantVote(candidate) {
			//Ignore timer timeout that hapenned during RPC processing if this receive should have stopped the timer.
			if !state.timer.Stop() { //Timer fired timeout
				//No need to drain the timer channel since the timeout signal is received by the FOLLOWER routine
				fmt.Println("RequestVoteRPC: Timed out during RPC processing!")
				state.shouldIgnoreTimeout = true
			}

			state.votedFor = candidate.CandidateID
			fmt.Println("RequestVoteRPC: TRUE -- vote granted for ", state.votedFor)
			state.currentTerm = candidate.Term
			vote.VoteGranted = true
			state.curState = FOLLOWER
			state.ResetStateTimer()

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

//AppendEntry function is structured in a way that checks cases where te reply should be false and returns
//immediately if any of them are true. Function logic is explained in the comments.
func (state *ServerState) AppendEntry(args *AppendEntriesArgs, rep *AppendEntriesResult) error {
	fmt.Println("AppendEntriesRPC: received")
	state.mux.Lock()
	defer state.mux.Unlock() //Will be called once function returns
	fmt.Println("AppendEntriesRPC: locked.")

	//Write standard negative reply.
	rep.Term = state.currentTerm
	rep.Success = false

	fmt.Println("AppendEntriesRPC: check term.")
	// 1.  Reply false if term < currentTerm
	if args.Term < state.currentTerm {
		fmt.Println("AppendEntriesRPC: FALSE. Leader's term is outdated")
		return nil
	}

	//Caller is on the same or higher term. Either way, this server should be on the same term from now on.
	state.currentTerm = args.Term
	rep.Term = state.currentTerm

	//At this point, if this server isn't already a follower it must become one, because the caller must
	//have been elected by the majority before sending AppendEntries calls and it's either this term's leader
	//or has a higher term.
	state.curState = FOLLOWER

	fmt.Println("AppendEntriesRPC: check log.")
	lastLogIndex := len(state.log) - 1
	// 2.  Reply false if log doesn’t contain an entry at prevLogIndex ...
	if lastLogIndex < args.PrevLogIndex {
		fmt.Println("AppendEntriesRPC: FALSE. Does not contain entry at prevLogIndex.")
		return nil
	}
	// ... whose term matches prevLogTerm
	if state.log[args.PrevLogIndex].Term != args.PrevLogTerm {
		fmt.Println("AppendEntriesRPC: FALSE. Entry at prevLogIndex has different term.")
		fmt.Printf("PrevLogindex: %d PrevLogTerm: %d\n", args.PrevLogIndex, args.PrevLogTerm)

		return nil
	}

	//Whatever the case from now on, this RPC should reply true.
	rep.Success = true

	if len(args.Entries) > 0 {
		//Not just a heartbeat
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

		//Ignore timer timeout that hapenned during RPC processing if this receive should have stopped the timer.
		if !state.timer.Stop() { //Timer fired timeout
			//No need to drain the timer channel since the timeout signal is received by the FOLLOWER routine
			fmt.Println("AppendEntriesRPC: Timed out during RPC processing!")
			state.shouldIgnoreTimeout = true
		}

		fmt.Println("AppendEntriesRPC: TRUE. Log replicated.")
		state.PrintLog()
	} else {
		//Just a heartbeat
		fmt.Println("AppendEntriesRPC: TRUE. Heartbeat reply.")
		state.PrintLog()
	}

	state.ResetStateTimer()
	return nil
}

//RequestJoin is called by servers on startup. It is used by the caller to signal other servers that it
//is ready to serve RPC requests. The true purpose of this RPC is to allow crashed servers to
//rejoin the system.
//When a server crashes, all client connections with it held by other servers get cut out.
//This means that if the crashed server recovers, other servers won't be able
//to call its RPCs unless they dial again. Receiving RequestJoin will prompt other servers to
//reestablish client connections with the caller.
//Returns true if the receiver successfully establishes a client connection with the caller.
func (state *ServerState) RequestJoin(serverID int, connectionEstablished bool) error {
	fmt.Println("RequestJoinRPC: received. Called by", serverID)

	//Check if this server has a client connection with caller
	if state.clientConnections[serverID] != nil {
		//TODO: avoid blocking here: make call assynchronous? timeout?
		n := 0
		ok := false
		callErr := state.clientConnections[serverID].Call("ServerState.ConnectionPulse", &n, &ok)

		//TODO: what if the error does not mean the connection is dead (other errors)?
		if callErr != nil { //Connection is dead. Close it and create another one
			fmt.Println("Client connection with", serverID, "is busted. Dialing again.")
			fmt.Println(callErr)
			closeErr := state.clientConnections[serverID].Close()
			if closeErr != nil {
				fmt.Println(closeErr)
			}

			//Dial another connection
			state.DialServer(serverID)
			fmt.Println("Client connection with", serverID, "reestablished.")

		} else {
			fmt.Println("Client connection with", serverID, "ok.")
		}

	} else {
		fmt.Println("Client connection with", serverID, "being established for the first time.")
		state.DialServer(serverID)
	}

	return nil
}

//ConnectionPulse is an RPC used by the caller to check if it's RPC client connection
//with the receiver is working properly, that is, if the connection returns any errors.
//If the receiver is able to receive the call at all, it returns true.
//This function is called from inside RequestJoin().
//Surely there must be a more sensible way to check the pulse of client connections,
//but this will do for now.
func (state *ServerState) ConnectionPulse(unusedArgs *int, ok *bool) error {
	*ok = true
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

/* CanGrantVote: If the candidate is up-to-date, it will receive a vote as long as these conditions are met.
   A reasonable implementation would avoid being so verbose, but this one is supposed
   to be understandable, not fast. */
func (state *ServerState) CanGrantVote(candidate *RequestVoteArgs) bool {
	hasNotVotedYet := state.votedFor == -1
	votedForCandidate := state.votedFor == candidate.CandidateID
	candidateHasHigherTerm := candidate.Term > state.currentTerm
	return hasNotVotedYet || votedForCandidate || candidateHasHigherTerm
}

//==============================================================================
