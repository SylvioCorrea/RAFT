package raft

import (
	"fmt"
	"math/rand"
	"net/rpc"
	"time"
)

//Struct used during leader processing of AppendEntries replies
type AppendReplyAux struct {
	serverID int //Server who responded AppendEntry call
	callInfo *rpc.Call
	reply    *AppendEntriesResult //Server reply
}

func (state *ServerState) ServerMainLoop() {
	rand.Seed(int64(state.id))

	//Lock to avoid processing RPCs during initial setup
	state.mux.Lock() //!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!
	//Register own rpc server
	fmt.Println("Registering RPC server.")
	state.SetupRPCServer()

	//Get all client connections to send rpc calls to the other servers
	fmt.Println("Establishing RPC client connections with the other servers.")
	state.SetupRPCClients()

	//Candidate auxiliary channels
	var voteChan chan *RequestVoteResult
	var electionAbortChan chan int

	//Leader auxiliary channels
	var replyChan chan *AppendReplyAux
	var abortChan chan int

	//All servers start as followers
	state.StartTimeoutManager()
	state.curState = FOLLOWER
	state.ResetStateTimer()
	fmt.Println("TIMER start.")
	state.mux.Unlock() //!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!

	fmt.Println("Starting state machine for server ", state.id)
	for {
		//Mutex is LOCKED right before checking state. Initial setups for state change should follow immediately and then unlock
		state.mux.Lock() //!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!
		switch state.curState {
		//========================================
		// Follower routine
		//========================================
		case FOLLOWER:
			fmt.Println("server ", state.id, "is now a follower")
			state.mux.Unlock() //!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!

			//The server MUST NOT become a follower without having resetted it's timer
			//Timer resets for followers happen upon receiving RPCs for which they can reply with TRUE
			<-state.timeoutChan // Timer expired
			fmt.Println(state.id, ": timer expired")
			//Timeout triggered state changes atomically in function TimeoutManager().

		//========================================
		// Cadidate routine
		//========================================
		case CANDIDATE:
			fmt.Println("server ", state.id, "is now a cadidate in term", state.currentTerm)
			state.votedFor = state.id
			rcvdVotes := 1

			voteChan = make(chan *RequestVoteResult, state.nOfServers-1)
			electionAbortChan = make(chan int, state.nOfServers-1)
			voteRequestsPending := 0
			//Request votes
			for i, server := range state.clientConnections {
				if i != state.id && server != nil { //Do not request to yourself
					voteRequestsPending++
					go state.sendRequestVotes(server, voteChan, electionAbortChan)
				}
			}

			state.ResetStateTimer() //Maybe do this somewhere else??

			state.mux.Unlock() //!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!

			electionStillGoing := true
			//Receive votes
			for state.IsCandidate() && electionStillGoing {
				select {
				case <-state.timeoutChan:
					//Election timed out.
					//State transitions triggered by timouts are handled by the TimeoutManager() fucntion.
					//Stop waiting for responses. Start new election.
					electionStillGoing = false

				case vote := <-voteChan:
					voteRequestsPending--
					if vote.VoteGranted {
						rcvdVotes++
						fmt.Println("Received votes: ", rcvdVotes)
					}
					state.mux.Lock()
					if rcvdVotes >= state.majority {
						//No need to check if still a candidate. No 2 leaders are elected in the same term
						//Even if the votes are late and a new leader has already been elected, that new leader
						//Would have to be of a higher term and the system would still be safe.

						//THIS SERVER WAS ELECTED LEADER
						state.curState = LEADER
						state.timer.Stop()
						//TODO: maybe do the whole "check if actually stopped" thing here too?
						//No need if TimeoutTransition() accounts for the leader case.
						electionStillGoing = false
					}
					state.mux.Unlock()

				default:
					//Go through loop check again to see if still a candidate
				}

			}
			//Election is over.

			//Abort pending requests
			for i := 0; i < voteRequestsPending; i++ {
				electionAbortChan <- 1
			}

		//========================================
		// Leader routine
		//========================================

		case LEADER:
			fmt.Println("=====> server ", state.id, "is now a leader! term(", state.currentTerm, ") <=======")

			//Updates lastIndex and matchIndex for all servers
			for i := 0; i < state.nOfServers; i++ {
				state.nextIndex[i] = len(state.log)
				state.matchIndex[i] = 0
			}

			//Channel to abort rpc call threads
			abortChan = make(chan int, state.nOfServers-1)
			//Channel to pass rpc replies
			replyChan = make(chan *AppendReplyAux) //Receive replies from rpcs ONE AT A TIME extra carefully
			remainingCalls := 0
			//Send initial empty AppendEntries for everyone
			for i, clientConn := range state.clientConnections {
				if i != state.id && clientConn != nil {
					aeArgs := &AppendEntriesArgs{
						Term:         state.currentTerm,
						LeaderID:     state.id,
						PrevLogIndex: state.nextIndex[i] - 1,
						PrevLogTerm:  state.log[state.nextIndex[i]-1].Term,
						Entries:      make([]LogEntry, 0), //First AppendEntries is always empty
						LeaderCommit: state.commitIndex}
					remainingCalls++
					go state.sendAppend(i, clientConn, aeArgs, replyChan, abortChan)

				}
			}
			fmt.Println("Initial AppendEntries sent.")
			state.mux.Unlock() //!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!

			leaderTimer := time.NewTimer(LEADERTIMER * TIMESCALE) //Leader will wait on this timer to send new AppendEntries

			for state.curState == LEADER { //Start leader loop
				select {

				case <-leaderTimer.C: //Time to send new Appends
					for i := 0; i < remainingCalls; i++ {
						abortChan <- 1
					}
					remainingCalls = 0

					//Make new abort and reply channels
					abortChan = make(chan int, state.nOfServers-1)
					replyChan = make(chan *AppendReplyAux)

					//Artificially generate a new LogEntry for the purpose of testing
					state.generateLogEntry()

					//Send new AppendEntries
					state.mux.Lock() //!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!
					if state.curState == LEADER {
						for i, clientConn := range state.clientConnections {
							if i != state.id && clientConn != nil { // Do not send appendentries to yourself
								aeArgs := &AppendEntriesArgs{
									Term:         state.currentTerm,
									LeaderID:     state.id,
									PrevLogIndex: state.nextIndex[i] - 1,
									PrevLogTerm:  state.log[state.nextIndex[i]-1].Term,
									Entries:      nil,
									LeaderCommit: state.commitIndex}

								if state.nextIndex[i] < len(state.log) { //Send new entries
									aeArgs.Entries = state.log[state.nextIndex[i]:]
								} else { //Send just heartbeat
									aeArgs.Entries = []LogEntry{}
								}
								remainingCalls++
								go state.sendAppend(i, clientConn, aeArgs, replyChan, abortChan)
							}
						}
					}
					fmt.Println("AppendEntries sent.")
					state.mux.Unlock() //!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!
					//Reset timer
					leaderTimer.Reset(LEADERTIMER * TIMESCALE)

				case replyAux := <-replyChan: //Some server replied to the Append
					remainingCalls--
					//process reply
					if replyAux.callInfo.Error != nil { //RPC error. Ignore results as if call never returned.
						fmt.Println("AppendEntries call error for server", replyAux.serverID, ":", replyAux.callInfo.Error)

					} else if replyAux.reply.Success {
						fmt.Println("AppendEntries reply: server", replyAux.serverID, "replied TRUE.")
						state.nextIndex[replyAux.serverID] = len(state.log)
						state.matchIndex[replyAux.serverID] = len(state.log) - 1
					} else if replyAux.reply.Term > state.currentTerm {
						fmt.Println("AppendEntries reply: server", replyAux.serverID, "replied FALSE. Reason: it is at a higher term.")
						//TODO become follower once a server with higher term is discovered
						//This might break the network if the server responding cannot
						//receive RPC calls from the other servers, only send them?
					} else {
						fmt.Println("AppendEntries reply: server", replyAux.serverID, "replied FALSE. Reason: log entries do not match.")
						state.nextIndex[replyAux.serverID]--
					}

				default:
					//Go through loop check again to avoid hanging in this select if no longer leader
				}
			}

			//Abort remaining calls if any
			for i := 0; i < remainingCalls; i++ {
				abortChan <- 1
			}
		}
	}
}

//Asynchronous call to AppendEntries RPC
func (state *ServerState) sendAppend(serverID int, server *rpc.Client, aeArgs *AppendEntriesArgs, replyChan chan *AppendReplyAux, abortChan chan int) {
	appendRPCReply := &AppendEntriesResult{0, false}
	rpcCall := server.Go("ServerState.AppendEntry", aeArgs, appendRPCReply, nil) // TODO: correct the call parameters
	select {
	case callInfo := <-rpcCall.Done:
		//process reply at leader's main loop
		replyChan <- &AppendReplyAux{serverID, callInfo, appendRPCReply}
	case <-abortChan:
		//abort call
	}

}

func (state *ServerState) sendRequestVotes(server *rpc.Client, voteChan chan *RequestVoteResult, electionAbortChan chan int) {
	vote := &RequestVoteResult{0, false}

	index := len(state.log) - 1

	reqVoteArgs := &RequestVoteArgs{ //TODO: this is a pointer... check what's being received on the other side
		Term:         state.currentTerm,
		CandidateID:  state.id,
		LastLogIndex: index,
		LastLogterm:  state.log[index].Term}

	rpcCall := server.Go("ServerState.RequestVote", reqVoteArgs, vote, nil) // TODO: correct the call parameters
	fmt.Println("request sent")
	select {
	case callInfo := <-rpcCall.Done:
		fmt.Println("request response received")
		if callInfo.Error != nil {
			fmt.Println(callInfo.Error)
		}
		voteChan <- vote
	case <-electionAbortChan:
		fmt.Println("request aborted")
		//leave select
	}
}

//GenerateLogEntry artificially generates a new log entry for the log of the server calling it.
//This function is used by the leader to simulate the receive of a log value for the purpose of
//testing the implementation.
func (state *ServerState) generateLogEntry() {
	le := LogEntry{Term: state.currentTerm, Value: rand.Intn(100)} //Generate a log entry with some value in [0, 100)
	state.log = append(state.log, le)
	state.PrintLog()
}
