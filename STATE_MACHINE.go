package STATE_MACHINE

import (
		. "./RPC"
		. "./Timer"
		"time"
		"net/rpc"
		)



		
func (state *ServerState) ServerMainLoop(servers []net.Conn, dataChan chan int, myID int) error {
	
	//All servers start as followers
	state.curState = FOLLOWER

	//Register own rpc server
	state.SetupRPCServer()

	//Get all client connections to send rpc calls to the other servers
	var clientConnections []*rpc.Client
	clientConnections = state.SetupRPCClients()

	max_term    := make(chan int, 1)

	//Election uses it's own timer
	electionTimer = NewTimer(0)
	max_term  <- 1

	for {
		switch state.mux.Lock(); state.curState { //!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!
			//========================================
			// Follower routine
			//========================================
			case FOLLOWER:
				state.mux.Unlock()
				//!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!
				// state.timer.Reset(genRandom())

				<- state.timer.C   // Timer expired

				state.curState = CANDIDATE // Convert to candidate

			
			
			
			//========================================
			// Cadidate routine
			//========================================
			case CANDIDATE:
				max_term := make(chan int, len(servers))
				tmpTerm := <- max_term
				if tmpTerm > state.currentterm { 
					state.currentterm = tmpTerm
				}
				max_term <- state.currentterm
				rcvdVotes := 1
				votedFor = myID
				state.mux.Unlock()
				//!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!
				
				electionTimer.Reset(genRandom())

				exit := make(chan int, len(servers))
				done := make(chan int, len(servers))
				


				for server := range servers {
					go state.sendVotes(server, done, exit, max_term, myID)
				}


				electionStillGoing := true
				//Receive votes
				for rcvdVotes < majority && electionStillGoing{
					select{
						case <- electionTimer.C:
							//Election timed out. Stop waiting for responses. Start new election
							for i := 0; i < len(server); i++ {
								exit <- 1
							}
							electionStillGoing = false

						case tmp:= <- done:
							rcvdVotes++
					}
				}

				if rcvdVotes >= majority{
					state.curState = LEADER
				}
				


			
			
			
			//========================================
			// Leader routine
			//========================================
			
			case LEADER: // As leader
				
				//Updates lastIndex and matchIndex for all servers
				nOfServers := len(serverPorts)
				for i := 0; i<nOfServers; i++ {
					state.lastIndex[i] = len(state.log)
					state.matchIndex[i] = 0
                }
				
				//Channel to abort rpc call threads
				abortChan := make(chan int, len(clientConnections)-1)
				//Channel to pass rpc replies
				replyChan := make(chan *AppendEntriesResult) //Receive replies from rpcs ONE AT A TIME extra carefully
				//Send initial empty AppendEntries for everyone
				for i, clientConn := range(clientConnections) { //TODO: lock mux before?
					if i != state.id { // TODO:  Should the server send AppendEntries to itself?
						aeArgs := &AppendEntriesArgs {
							term: 			state.term,
							leaderId: 		state.id,
							prevLogindex: 	nextIndex[server.myID] - 1,
							prevLogTerm: 	state.log[ nextIndex[server.myID] - 1 ].term,
							entries: 		make([]LogEntry, 0), //First AppendEntries is always empty
							leaderCommit: 	state.commitIndex }
							
							go state.sendAppend(clientConn, aeArgs, replyChan, abortChan)
						}
					}
				}
				state.mux.Unlock()
				//!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!
				
				remainingCalls := nOfServers - 1 //Ongoing calls
				
				leader_timer := time.NewTimer(LeaderTimer()) //Leader will wait on this timer to send new AppendEntries
				
				for state.curState == LEADER { //Start leader loop
					select {
					
					case <- leader_timer.C: //Time to send new Appends
						for i:=0; i<remainingCalls; i++ {
							abortChan <- 1
						}
						
						//Make new abort and reply channels
						abortChan = make(chan int, len(clientConnections)-1)
						replyChan = make(chan *AppendEntriesResult)
						
						//Send new AppendEntries
						state.mux.Lock()
						if state.curState == LEADER {
							for i, clientConn := range(clientConnections) { //TODO: lock mux before?
								if i != state.id { // TODO:  Should the server send AppendEntries to itself?
									aeArgs := &AppendEntriesArgs {
										term: 			state.term,
										leaderId: 		state.id,
										prevLogindex: 	nextIndex[i] - 1,
										prevLogTerm: 	state.log[ nextIndex[i] - 1 ].term,
										entries: 		nil,
										leaderCommit: 	state.commitIndex }
										
									
									if nextIndex[i] <= len(log) - 1 { //Send new entries
										aeArgs.entries = state.log(state.nextIndex[i]:)
									} else { //Send just heartbeat
										aeArgs.entries = []LogEntry{}
									}
									go state.sendAppend(clientConn, aeArgs)
								}
							}
						}
						state.mux.Unlock()
						//Reset remainingCalls and timer
						remainingCalls = nOfServers - 1
						leader_timer.Reset(Timer.LeaderTimer())

					
						
					case reply := <- replyChan: //Some server replied to the Append
						//process reply
						remainingCalls--
						
					default:
						//Go through loop check again to avoid hanging in this select if no longer leader
					}
				}

				//Abort remaining calls if any
				for i:=0; i<remainingCalls; i++ {
					abortChan <- 1
				}
		}
	}
}

//Asynchronous call to AppendEntries RPC
func (state *ServerState) sendAppend(server *Client, aeArgs *AppendEntriesArgs, replyChan chan *AppendEntriesResult, abortChan chan int) {
	
	rpcCall = server.Go("ServerState.AppendEntry", aeArgs, voteInfo) // TODO: correct the call parameters
	select {
	case replyChan <- rpcCall.Done:
		//process reply at leader's main loop
	case <- abortChan:
		//abort call
	}

}




func (state *ServerState) sendVotes(server net.Conn, done chan int, exit chan int, term chan int, myID int) {
	voteInfo = ReplyInfo{state.term, false}

	index := len(state.log)
	args = RequestVoteArgs{
		state.term       ,
		myID             ,
		index            ,
		state.log[index]
	}

	send = server.Go("ServerState.RequestVote", args, voteInfo) // TODO: correct the call parameters
	select {
		case <- send.Done:
			if voteInfo.reply{
				done <- 1
			}

			cur  := <- term 
			if cur < voteInfo.term{
				cur = voteInfo.term
			}
			term <- cur

		case <- exit:
			return
	}	
}
