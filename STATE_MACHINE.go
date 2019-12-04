package STATE_MACHINE

import (
		. "./RPC"
		. "./Timer"
		"time"
		"net"
		)

		
func (state *ServerState) transitions(servers []net.Conn, dataChan chan int, myID int) error {
	following   := make(chan int, 1)
	elected     := make(chan int, 1)
	candidature := make(chan int, 1)
	max_term    := make(chan int, 1)

	electionTimer = NewTimer(0)

	following <- 1
	max_term  <- 1

	for {
		switch state.curState {
			//========================================
			// Follower routine
			//========================================
			case FOLLOWER:
				// state.timer.Reset(genRandom())

				<- state.timer.C   // Timer expired

				state.curState = CANDIDATE // Convert to candidate

			
			
			
			//========================================
			// Cadidate routine
			//========================================
			case CANDIDATE:
				

				state.mux.Lock() //This server might receive AppendEntries or RequestVotes during this operation
				max_term := make(chan int, len(servers))
				tmpTerm := <- max_term
				if tmpTerm > state.currentterm { 
					state.currentterm = tmpTerm
				}
				max_term <- state.currentterm
				rcvdVotes := 1
				votedFor = myID
				state.mux.Unlock()
				
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
			/* IMPORTANTE:
			   
			   Upon election: send initial empty AppendEntries RPCs(heartbeat) to each server;
			   repeat during idle periods to prevent election timeouts (§5.2)
			   
			   O lider manda o AppendEntry inicial vazio para saber, para cada outro servidor,
			   seu prevLogindex
			*/
			
			case LEADER: // As leader
				/*
				type AppendEntriesArgs struct {
					term         int
					leaderId     int
					prevLogIndex int
					prevLogterm  int
					entries      []LogEntry
					leaderCommit int
				}
				*/

				for server := range servers { //Colocar isso dentro de um semaforo
					ae := AppendEntriesArgs {
						term: state.term
						leaderId: state.myID
						prevLogindex: nextIndex[server.myID] - 1
						prevLogTerm: state.log[ nextIndex[server.myID] - 1 ].term
						entries:
						leaderCommit: state.commitIndex
					}
				}





				leader_timer := NewTimer(0)
				// for{ // Start leader procedure

					state.mux.Lock()
					// if state.timer.Stop() { // A new leader has already been elected
					// 	remainingTime := <- state.timer.C
					// 	state.timer.Reset(remainingTime)
					// 	following <- 1
					// 	state.mux.Unlock()
					// 	break
					// }
					myterm = state.currentterm
					state.mux.Unlock()

					switch {
						case <- leader_timer.C:
							for server := range servers {
								sendAppend(server, myterm, myID)
							}

						case data := <- dataChan:
							// aplica log
							for server := range servers {
								sendAppend(server, myterm, myID)
							}							

						default:

					}
					leader_timer.Reset(Timer.LeaderTimer())
					

					
					
				// }
		}
	}
}


func (state *ServerState) sendAppend(server net.Conn, term int, myID int) {
	voteInfo = ReplyInfo{state.term, false}

	index := len(state.log)
	args = AppendEntriesArgs {
		state.term        ,
		myID              ,
		index             ,
		state.log[index]  ,
		state.log         ,
		state.commitIndex
	}

	send = server.Call("AppendEntry", args, voteInfo) // TODO: correct the call parameters
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

	send = server.Go("state.RequestVote", args, voteInfo) // TODO: correct the call parameters
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
