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
		select{
			//========================================
			// Follower routine
			//========================================
			case <- following:
				// state.timer.Reset(genRandom())

				<- state.timer.C   // Timer expired

				state.curState = CANDIDATE // Convert to candidate
				candidate <- 1

			
			
			
			//========================================
			// Cadidate routine
			//========================================
			case <- candidature:
				finished := false
				
				for !finished {
					electionTimer.Reset(genRandom())

					state.mux.Lock()
					if state.timer.Stop(){ // A new leader has already been elected
						remainingTime := <- state.timer.C
						state.timer.Reset(remainingTime)
						following <- 1
						state.mux.Unlock()
						break
					}
					state.mux.Unlock()

					exit := make(chan int, len(servers))
					done := make(chan int, len(servers))
					
					rcvdVotes := 1

					for server := range servers {
						go state.sendVotes(server, done, exit, max_term, myID)
					}

					cont := true
					tmpTerm := <- max_term
					if tmpTerm > state.currentterm { 
						state.currentterm = tmpTerm
					}
					max_term <- state.currentterm

					for rcvdVotes < majority && cont{
						select{
							case <- electionTimer.C:case tmp:= <- done
								for i := 0; i < len(server); i++ {
									exit <- 1
								}
								cont = false

							case tmp:= <- done:
								rcvdVotes++
						}
					}

					if rcvdVotes >= majority{
						finished = true
					}
				}

				if finished{
					elected <- 1
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
			
			case <- elected: // As leader
				leader_timer := NewTimer(0)
				for{ // Start leader procedure
					<- leader_timer.C

					state.mux.Lock()
					if state.timer.Stop(){ // A new leader has already been elected
						remainingTime := <- state.timer.C
						state.timer.Reset(remainingTime)
						following <- 1
						state.mux.Unlock()
						break
					}
					state.mux.Unlock()

					
					for server := range servers {
						sendAppend(server, max_term)
					}


					leader_timer.Reset(Timer.LeaderTimer())
					
				}
		}
	}
}


func (state *ServerState) sendAppend(server net.Conn, term chan int, myID int) {
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
