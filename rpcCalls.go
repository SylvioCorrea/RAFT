package server


import "errors"


// Log Entry (appended $VALUE at $TERM)
type LogEntry struct {
	Term 		int
	Value 		int
}

// AppendEntry parameters
type AppendInfo struct {
	Term         int
	leaderID     int
	prevLogIndex int
	prevLogTerm  int
	leaderCommit int
	entries		 make([] LogEntry)
}

// Processes' state
type RecvrInfo struct {
	Term         int
	candidateID  int
	lastLogIndex int
	lastLogTerm  int
	commitIndex  int
	log			 make([] LogEntry)
}

// RequestVote parameters
type CandidateInfo struct {
	Term         int
	candidateID  int
	lastLogIndex int
	lastLogTerm  int
}

// RPCs response
type ReplyInfo struct {
	Term 		 int
	Reply        int
}


// RPC RequestVote (Args, Reply)
func (state *RecvrInfo) RequestVote(candidate *CandidateInfo, vote *ReplyInfo) error{
	if state.candidateID == nil {  // 2. If votedFor is null or candidateId, grant vote
		vote.Term  = state.Term+1
		vote.Reply = 1

		state.Term 		   = candidate.Term
		state.candidateID  = candidate.candidateID
		state.lastLogIndex = candidate.lastLogIndex
		state.lastLogTerm  = candidate.lastLogTerm
		return nil
	}

	if state.Term >= candidate.Term {  // 1. Reply false if term < currentTerm
		vote.Term = state.Term+1
		vote.Reply = 0
		return nil
	}

	if state.lastLogIndex <= candidate.lastLogIndex {  // 2. If candidate's log is at least as up-to-date as receiver's log, grant vote
		vote.Term = state.Term+1
		vote.Reply = 1

		state.Term 		   = candidate.Term
		state.candidateID  = candidate.candidateID
		state.lastLogIndex = candidate.lastLogIndex
		state.lastLogTerm  = candidate.lastLogTerm
		return nil
	}

	vote.Term = state.Term+1
	vote.Reply = 0
	return nil
}

// RPC AppendEntry (Args, Reply)
func (state *RecvrInfo) AppendEntry(entry *AppendInfo, rep *Reply) error{
	if entry.Term < state.Term {  // 1.  Reply false if term < currentTerm
		rep.Term  = state.Term
		rep.Reply = 0
		return nil
	}

	if state.lastLogIndex < entry.prevLogIndex || // 2.  Reply false if log doesn’t contain an entry at prevLogIndex
		state.log[entry.prevLogIndex].Term != entry.prevLogTerm{ // whose term matches prevLogTerm
		rep.Term  = state.Term
		rep.Reply = 0
		return nil
	}

	if state.log[entry.prevLogIndex+1].Term != entry.prevLogTerm{ // 3. If an existing entry conflicts with a new one (same index
														// but different terms), delete the existing entry and all that follow it
		// TODO
	}

	// 4. Append any new entries not already in the log
	// TODO

	if entry.leaderCommit > state.commitIndex {
		// 5. If leaderCommit > commitIndex, set commitIndex = min(leaderCommit, index of last new entry)
		if entry.leaderCommit > len(state.log){
			commitIndex = len(state.log)
		}
		else{
			commitIndex = entry.leaderCommit
		}
	}
}