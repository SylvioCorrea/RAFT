package RPC


import "errors"
import "time"
import "math/rand"
import . "../Timer"

// Log Entry (appended $VALUE at $TERM)
type LogEntry struct {
	term 		int
	value 		int
}

// Processes' state
type RecvrInfo struct {
	// Persistent
	currentterm  int
	votedFor     int
	log			 make([] LogEntry)

	// Volatile on all
	commitIndex  int
	lastApplied  int

	// Volatile on leader
	nextIndex    make([] int)
	matchIndex   make([] int)

	// Aux
	timer		 time.Timer
	curState 	 int
	/*
		curState :
			- 0 == Follower
			- 1 == Candidate
			- 2 == Leader
	*/
}

/*
	State init::
	currentterm <- 0
	votedFor    <- nil
	commitIndex <- 0
	lastApplied <- 0

	timer 		<- NewTimer(2*RANGE*TIMESCALE)
	curState 	<- 0
*/


// AppendEntry parameters
type AppendEntriesArgs struct {
	term         int
	leaderId     int
	prevLogIndex int
	prevLogterm  int
	entries		 make([] LogEntry)
	leaderCommit int
}

// RequestVote parameters
type RequestVoteArgs struct {
	term         int
	candidateId  int
	lastLogIndex int
	lastLogterm  int
}

// RPCs response
type ReplyInfo struct {
	term 		 int
	reply        bool
}


// RPC RequestVote (Args, Reply)
func (state *RecvrInfo) RequestVote(candidate *RequestVoteArgs, vote *ReplyInfo) error{
	// add semaphore
	if state.votedFor == nil {  // 2. If votedFor is null or candidateId, grant vote
		state.votedFor     = candidate.candidateId
		state.currentterm  = candidate.term

		vote.term  = state.currentterm+1
		vote.reply = true
		return nil
	}

	if candidate.term < state.currentterm {  // 1. Reply false if term < currentterm
		vote.term  = state.currentterm+1
		vote.reply = false
		return nil
	}

	if len(state.log) <= candidate.lastLogIndex {  // 2. If candidate's log is at least as up-to-date as receiver's log, grant vote
		vote.term  = state.currentterm+1
		vote.reply = true

		state.currentterm  = candidate.term
		state.votedFor     = candidate.candidateId
		return nil
	}
	// end semaphore

	vote.term  = state.currentterm+1
	vote.reply = false
	return nil
}

// RPC AppendEntry (Args, Reply)
func (state *RecvrInfo) AppendEntry(args *AppendEntriesArgs, rep *Reply) error{
	if !state.timer.Stop(){
		return nil
	}

	rep.term  = state.currentterm
	rep.reply = false

	if args.term < state.currentterm {  // 1.  Reply false if term < currentterm
		return nil // restart timer
	}

	if len(state.log) < args.prevLogIndex || // 2.  Reply false if log doesn’t contain an entry at prevLogIndex
			state.log[args.prevLogIndex].term != args.prevLogterm { // whose term matches prevLogterm
		return nil // restart timer
	}

	// if state.log[args.prevLogIndex].term == args.prevLogterm { // 3. If an existing entry conflicts with a new one (same index
	// 													// but different terms)
		state.log = state.log[:args.prevLogIndex+1] // delete the existing entry and all that follow it
	// }

	// 4. Append any new entries not already in the log
	for _, entry := range args.entries {
		state.log = append(state.log, entry)
	}

	if args.leaderCommit > state.commitIndex {
		// 5. If leaderCommit > commitIndex, set commitIndex = min(leaderCommit, index of last new entry)
		if args.leaderCommit > len(state.log){
			state.commitIndex = len(state.log)
		}
		else{
			state.commitIndex = args.leaderCommit
		}
	}

	remaining := <- state.timer.C
	remaining.Add(GenRandom())
	state.timer.Reset(remaining)

	rep.reply = true
}