package raft

import (
	"net/rpc"
	"time"
    "fmt"
    "math/rand"
	"./Timer"
)

//Struct used during leader processing of AppendEntries replies
type AppendReplyAux struct {
	serverID int                  //Server who responded AppendEntry call
	reply    *AppendEntriesResult //Server reply
}

func (state *ServerState) ServerMainLoop() {
    rand.Seed(int64(state.id))

	//All servers start as followers
	state.curState = FOLLOWER

	//Register own rpc server
	go state.SetupRPCServer()
	fmt.Println("RPC services registered for ", state.id)

	//Get all client connections to send rpc calls to the other servers
	var clientConnections []*rpc.Client
	clientConnections = state.SetupRPCClients()
	fmt.Println("All RPC client connections dialed for", state.id)

	max_term := make(chan int, 1)

	//Election uses it's own timer
	electionTimer := time.NewTimer(0)
	<-electionTimer.C
	max_term <- 1

	//Leader auxiliary channels
	var replyChan chan *AppendReplyAux
	var abortChan chan int

	for {
		//Mutex is LOCKED right before checking state. Initial setups for state change should follow immediately and then unlock
		switch state.mux.Lock(); state.curState { //!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!
		//========================================
		// Follower routine
		//========================================
		case FOLLOWER:
		    fmt.Println("server ", state.id, "is now a follower")
			state.mux.Unlock() //!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!
			state.timer.Reset(Timer.GenRandom())
            fmt.Println(state.id, ": timer start")
			<-state.timer.C // Timer expired
			fmt.Println(state.id, ": timer expired")


			state.curState = CANDIDATE // Convert to candidate

		//========================================
		// Cadidate routine
		//========================================
		case CANDIDATE:
			//TODO: serialize vote processing
			fmt.Println("server ", state.id, "is now a cadidate")
			state.votedFor = state.id
			rcvdVotes := 1
			state.mux.Unlock() //!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!

			electionTimer.Reset(Timer.GenRandom())

			exit := make(chan int, nOfServers-1)
			done := make(chan int, nOfServers-1)

			for i, server := range clientConnections {
				if i != state.id {
				    go state.sendRequestVotes(server, done, exit, state.id)
				}
			}

			electionStillGoing := true
			//Receive votes
			for rcvdVotes < majority && electionStillGoing {
				select {
				case <-electionTimer.C:
					//Election timed out. Stop waiting for responses. Start new election
					for i := 0; i < nOfServers-1; i++ {
						exit <- 1
					}
					electionStillGoing = false

				case <-done:
				       fmt.Println("RN", rcvdVotes, majority)
					rcvdVotes++
				}
			}

			if rcvdVotes >= majority {
				state.curState = LEADER
			} else {
				state.votedFor = -1
			}

		//========================================
		// Leader routine
		//========================================

		case LEADER:
		    fmt.Println("=====> server ", state.id, "is now a leader <=======")

			//Updates lastIndex and matchIndex for all servers
			nOfServers := len(serverPorts)
			for i := 0; i < nOfServers; i++ {
				state.nextIndex[i] = len(state.log)
				state.matchIndex[i] = 0
			}

			//Channel to abort rpc call threads
			abortChan = make(chan int, len(clientConnections)-1)
			//Channel to pass rpc replies
			replyChan = make(chan *AppendReplyAux) //Receive replies from rpcs ONE AT A TIME extra carefully
			//Send initial empty AppendEntries for everyone
			for i, clientConn := range clientConnections { //TODO: lock mux before?
				if i != state.id { // TODO:  Should the server send AppendEntries to itself?
					aeArgs := &AppendEntriesArgs{
						term:         state.currentTerm,
						leaderID:     state.id,
						prevLogIndex: state.nextIndex[i] - 1,
						prevLogTerm:  state.log[state.nextIndex[i]-1].term,
						entries:      make([]LogEntry, 0), //First AppendEntries is always empty
						leaderCommit: state.commitIndex}

					go state.sendAppend(i, clientConn, aeArgs, replyChan, abortChan)

				}
			}
			state.mux.Unlock() //!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!

			remainingCalls := nOfServers - 1 //Ongoing calls

			leaderTimer := time.NewTimer(Timer.LeaderTimer()) //Leader will wait on this timer to send new AppendEntries

			for state.curState == LEADER { //Start leader loop
				select {

				case <-leaderTimer.C: //Time to send new Appends
					for i := 0; i < remainingCalls; i++ {
						abortChan <- 1
					}

					//Make new abort and reply channels
					abortChan = make(chan int, len(clientConnections)-1)
					replyChan = make(chan *AppendReplyAux)

					//Send new AppendEntries
					state.mux.Lock() //!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!
					if state.curState == LEADER {
						for i, clientConn := range clientConnections { //TODO: lock mux before?
							if i != state.id { // Do not send appendentries to yourself
								aeArgs := &AppendEntriesArgs{
									term:         state.currentTerm,
									leaderID:     state.id,
									prevLogIndex: state.nextIndex[i] - 1,
									prevLogTerm:  state.log[state.nextIndex[i]-1].term,
									entries:      nil,
									leaderCommit: state.commitIndex}

								if state.nextIndex[i] <= len(state.log)-1 { //Send new entries
									aeArgs.entries = state.log[state.nextIndex[i]:]
								} else { //Send just heartbeat
									aeArgs.entries = []LogEntry{}
								}
								go state.sendAppend(i, clientConn, aeArgs, replyChan, abortChan)
							}
						}
					}
					state.mux.Unlock() //!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!
					//Reset remainingCalls and timer
					remainingCalls = nOfServers - 1
					leaderTimer.Reset(Timer.LeaderTimer())

				case replyAux := <-replyChan: //Some server replied to the Append
					//process reply
					if replyAux.reply.success {
						state.nextIndex[replyAux.serverID] = len(state.log)
						state.matchIndex[replyAux.serverID] = len(state.log) - 1
					} else {
						state.nextIndex[replyAux.serverID]--
					}
					remainingCalls--

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
	case <-rpcCall.Done:
		//process reply at leader's main loop
		replyChan <- &AppendReplyAux{serverID, appendRPCReply}
	case <-abortChan:
		//abort call
	}

}

func (state *ServerState) sendRequestVotes(server *rpc.Client, done chan int, exit chan int, myID int) {
	vote := &RequestVoteResult{0, false}

	index := len(state.log) - 1
    
	reqVoteArgs := &RequestVoteArgs{
		Term:         state.currentTerm,
		CandidateID:  state.id,
		LastLogIndex: index,
		LastLogterm:  state.log[index].term}
    
	rpcCall := server.Go("ServerState.RequestVote", reqVoteArgs, vote, nil) // TODO: correct the call parameters
	fmt.Println("request sent")
	select {
	case err := <-rpcCall.Done:
		if err != nil {
		    fmt.Println(err)
		}
		state.mux.Lock()
		if state.curState == CANDIDATE {
		    if vote.VoteGranted {
			    done <- 1
		    }

		    state.currentTerm = vote.Term
		}
		state.mux.Unlock()
	case <-exit:
		fmt.Println("request exit chan")
		//leave select
	}
	fmt.Println("request finished")
}
