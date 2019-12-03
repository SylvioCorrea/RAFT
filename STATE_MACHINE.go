package STATE_MACHINE

import . "./RPC"
import . "./Timer"
import "time"

func (state *ServerState) transitions() error {
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

				state.curState = 1 // Convert to candidate
				candidate <- 1

			
			
			
			//========================================
			// Cadidate routine
			//========================================
			case <- candidature:
				finished := false
				
				for !finished {
					electionTimer.Reset(genRandom())

					<- state.sema
					if state.timer.Stop(){ // A new leader has already been elected
						remainingTime := <- state.timer.C
						state.timer.Reset(remainingTime)
						following <- 1
						state.sema <- 1
						break
					}
					state.sema <- 1

					exit := make(chan int, len(servers))
					done := make(chan int, len(servers))
					
					rcvdVotes := 1

					for each other server {
						go state.sendVotes(done, exit, max_term)
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
			   repeat during idle periods toprevent election timeouts (§5.2)
			   
			   O lider manda o AppendEntry inicial vazio para saber, para cada outro servidor,
			   seu prevLogindex
			*/
			
			case <- elected: // As leader
				leader_timer := NewTimer(0)
				for{ // Start leader procedure
					<- leader_timer.C

					<- state.sema
					if state.timer.Stop(){ // A new leader has already been elected
						remainingTime := <- state.timer.C
						state.timer.Reset(remainingTime)
						following <- 1
						state.sema <- 1
						break
					}
					state.sema <- 1

					
					for each other server {
						manda appendentries
					}


					leader_timer.Reset(Timer.LeaderTimer())
					
				}
		}
	}
}


func (state *ServerState) sendAppend(done chan int, exit chan int, term chan int) {
	voteInfo = ReplyInfo{state.term, false}

	send = client.AppendEntries()
}

func (state *ServerState) sendVotes(done chan int, exit chan int, term chan int) {
	voteInfo = ReplyInfo{state.term, false}
	args = RequestVoteArgs{
		state.term   ,
		state.myID   ,
		lastLogIndex ,
		lastLogterm 
	}

	send = client.Go("state.RequestVote", args, voteInfo)
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
