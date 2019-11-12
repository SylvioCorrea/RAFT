package STATE_MACHINE

import . "./RPC"
import . "./Timer"
import "time"

func (state *ServerState) transitions() error {
	following   := make(chan int)
	elected     := make(chan int)
	candidature := make(chan int)

	state.timer = NewTimer(0)

	following <- 1

	for {
		select{
			case <- following: // As follower
				state.timer.Reset(genRandom())

				<- state.timer.C   // Timer expired

				state.curState = 1 // Convert to candidate
				candidate <- 1

			case <- candidature: // As candidate
				state.timer.Reset(genRandom())
				for {
					rcvdVotes := 1

					for each other server {
						go sendVotes()
					}

					// Send ReqstVote
					if (rcvdVotes > majority){
						elected <- 1
					}
				}
			case <- elected: // As leader
			
				for{ // Start leader procedure
					// Send Append
					
				}
		}
	}
}


func sendVotes() {
	send = client.Vote()
	select {
		case <- send.Done:
			rcvdVotes += 1 
		case <- state.timer.C:
			return
	}	
}