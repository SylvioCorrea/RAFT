package RPC


import "errors"
import "time"
import "math/rand"
import . "../Timer"

// Log Entry (appended $VALUE at $TERM)
type LogEntry struct {
	Term 		int
	Value 		int
}

// Processes' state
type RecvrInfo struct {
	// Persistent
	currentTerm  int
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
	currentTerm <- 0
	votedFor    <- nil
	commitIndex <- 0
	lastApplied <- 0

	timer 		<- NewTimer(2*RANGE*TIMESCALE)
	curState 	<- 0
*/


// AppendEntry parameters
type AppendInfo struct {
	term         int
	leaderId     int
	prevLogIndex int
	prevLogTerm  int
	entries		 make([] LogEntry)
	leaderCommit int
}

// RequestVote parameters
type CandidateInfo struct {
	term         int
	candidateId  int
	lastLogIndex int
	lastLogTerm  int
}

// RPCs response
type ReplyInfo struct {
	term 		 int
	reply        int
}


// RPC RequestVote (Args, Reply)
func (state *RecvrInfo) RequestVote(candidate *CandidateInfo, vote *ReplyInfo) error{
	if state.votedFor == nil {  // 2. If votedFor is null or candidateId, grant vote
		vote.term  = state.currentTerm+1
		vote.reply = 1

		state.currentTerm  = candidate.term
		state.votedFor     = candidate.candidateId
		return nil
	}

	if state.currentTerm >= candidate.term {  // 1. Reply false if term < currentTerm
		vote.term  = state.currentTerm+1
		vote.reply = 0
		return nil
	}

	if len(state.log) <= candidate.lastLogIndex {  // 2. If candidate's log is at least as up-to-date as receiver's log, grant vote
		vote.term  = state.currentTerm+1
		vote.reply = 1

		state.currentTerm  = candidate.term
		state.votedFor     = candidate.candidateId
		return nil
	}

	vote.term  = state.currentTerm+1
	vote.reply = 0
	return nil
}

// RPC AppendEntry (Args, Reply)
func (state *RecvrInfo) AppendEntry(entry *AppendInfo, rep *Reply) error{
	if entry.term < state.currentTerm {  // 1.  Reply false if term < currentTerm
		rep.term  = state.currentTerm
		rep.reply = 0
		return nil
	}

	if !state.timer.Stop(){
		return nil
	}

	if len(state.log) < entry.prevLogIndex || // 2.  Reply false if log doesn’t contain an entry at prevLogIndex
		state.log[entry.prevLogIndex].Term != entry.prevLogTerm { // whose term matches prevLogTerm
		rep.term  = state.currentTerm
		rep.reply = 0
		return nil
	}

	if state.log[entry.prevLogIndex+1].Term != entry.prevLogTerm { // 3. If an existing entry conflicts with a new one (same index
														// but different terms), delete the existing entry and all that follow it
		// TODO
	}

	// 4. Append any new entries not already in the log
	// TODO

	if entry.leaderCommit > state.commitIndex {
		// 5. If leaderCommit > commitIndex, set commitIndex = min(leaderCommit, index of last new entry)
		if entry.leaderCommit > len(state.log){
			state.commitIndex = len(state.log)
		}
		else{
			state.commitIndex = entry.leaderCommit
		}
	}

	remaining := <- state.timer.C
	remaining.Add(GenRandom())
	state.timer.Reset(remaining)
}