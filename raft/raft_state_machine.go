package raft

import (
	"net/rpc"
	"time"

	"./Timer"
)

//Struct used during leader processing of AppendEntries replies
type AppendReplyAux struct {
	serverID int                  //Server who responded AppendEntry call
	reply    *AppendEntriesResult //Server reply
}

func (state *ServerState) ServerMainLoop() error {

	//All servers start as followers
	state.curState = FOLLOWER

	//Register own rpc server
	state.SetupRPCServer()

	//Get all client connections to send rpc calls to the other servers
	var clientConnections []*rpc.Client
	clientConnections = state.SetupRPCClients()

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
			state.mux.Unlock() //!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!
			// state.timer.Reset(genRandom())

			<-state.timer.C // Timer expired

			state.curState = CANDIDATE // Convert to candidate

		//========================================
		// Cadidate routine
		//========================================
		case CANDIDATE:
			//TODO: serialize vote processing
			max_term := make(chan int, nOfServers)
			tmpTerm := <-max_term
			if tmpTerm > state.currentTerm {
				state.currentTerm = tmpTerm
			}
			max_term <- state.currentTerm
			rcvdVotes := 1
			state.votedFor = state.id
			state.mux.Unlock() //!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!

			electionTimer.Reset(Timer.GenRandom())

			exit := make(chan int, nOfServers-1)
			done := make(chan int, nOfServers-1)

			for _, server := range clientConnections {
				go state.sendRequestVotes(server, done, exit, max_term, state.id)
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
					rcvdVotes++
				}
			}

			if rcvdVotes >= majority {
				state.curState = LEADER
			}

		//========================================
		// Leader routine
		//========================================

		case LEADER:

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
						prevLogTerm:  state.log[state.nextIndex[i-1]].term,
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

func (state *ServerState) sendRequestVotes(server *rpc.Client, done chan int, exit chan int, term chan int, myID int) {
	vote := &RequestVoteResult{0, false}

	index := len(state.log) - 1

	reqVoteArgs := &RequestVoteArgs{
		term:         state.currentTerm,
		candidateID:  state.id,
		lastLogIndex: index,
		lastLogterm:  state.log[index].term}

	rpcCall := server.Go("ServerState.RequestVote", reqVoteArgs, vote, nil) // TODO: correct the call parameters
	select {
	case <-rpcCall.Done:
		if vote.voteGranted {
			done <- 1
		}

		cur := <-term
		if cur < vote.term {
			cur = vote.term
		}
		term <- cur

	case <-exit:
		return
	}
}
