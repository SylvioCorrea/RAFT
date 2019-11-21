package STATE_MACHINE

import . "./RPC"
import . "./Timer"
import "time"

func (state *ServerState) transitions() error {
	following   := make(chan int)
	elected     := make(chan int)
	candidature := make(chan int)


	electionTimer = NewTimer(0)

	following <- 1

	for {
		select{
			case <- following: // As follower
				// state.timer.Reset(genRandom())

				<- state.timer.C   // Timer expired

				state.curState = 1 // Convert to candidate
				candidate <- 1

			case <- candidature: // As candidate
				finished := false
				
				for !finished {
					electionTimer.Reset(genRandom())

					if state.timer.Stop(){ // A new leader has already been elected
						remainingTime := <- state.timer.C
						state.timer.Reset(remainingTime)
						following <- 1
						break
					}

					exit := make(chan int, len(servers))
					done := make(chan int)
					
					rcvdVotes := 1

					for each other server {
						go state.sendVotes(done, exit)
					}

					cont := true

					for rcvdVotes < majority && cont{
						select{
							case <- electionTimer.C:
								for i := 0; i < len(server); i++ {
									exit <- 1
								}
								cont = false

							case <- done:
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


			case <- elected: // As leader
			
				for{ // Start leader procedure
					// Send Append
					
				}
		}
	}
}


func (state *ServerState) sendVotes(done chan int, exit chan int) {
	voteInfo = ReplyInfo{state.term, false}
	args = RequestVoteArgs{
		state.term   ,
		state.myID   ,
		lastLogIndex ,
		lastLogterm 
	}

	send = client.RequestVote()
	select {
		case <- send.Done:
			if voteInfo.reply{
				done <- 1
			}

		case <- exit:
			return
	}	
}