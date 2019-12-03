package RPC

import (
	"sync"
	"time"

	"../Timer"
)

// Log Entry (appended $VALUE at $TERM)
type LogEntry struct {
	term  int
	value int
}

// Processes' state
type ServerState struct {
	// Persistent
	currentterm int
	votedFor    int //Should start as -1 since int cannot be nil
	log         []LogEntry

	// Volatile on all
	commitIndex int
	lastApplied int

	// Volatile on leader
	nextIndex  []int
	matchIndex []int

	// Aux
	timer    time.Timer
	curState int
	mux      sync.Mutex
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
	entries      []LogEntry
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
type AppendEntriesResult struct {
	term    int
	success bool
}

type RequestVoteResult struct {
	term        int
	voteGranted bool
}

// RPC RequestVote (Args, Reply)
func (state *ServerState) RequestVote(candidate *RequestVoteArgs, vote *RequestVoteResult) error {
	state.mux.Lock()
	defer state.mux.Unlock() //Will be called once function returns

	vote.term = state.currentterm
	vote.voteGranted = false

	if candidate.term < state.currentterm { // 1. Reply false if term < currentterm
		vote.voteGranted = false

	} else if state.votedFor == -1 || state.votedFor == candidate.candidateId { // 2. If votedFor is null or candidateId, grant vote
		state.votedFor = candidate.candidateId
		state.currentterm = candidate.term

		vote.voteGranted = true

	} else if len(state.log) <= candidate.lastLogIndex+1 {
		// 2. If candidate's log is at least as up-to-date as receiver's log, grant vote
		vote.voteGranted = true

		state.currentterm = candidate.term
		state.votedFor = candidate.candidateId
	}

	return nil
}

// RPC AppendEntry (Args, Reply)
func (state *ServerState) AppendEntry(args *AppendEntriesArgs, rep *AppendEntriesResult) error {
	state.mux.Lock()
	defer state.mux.Unlock() //Will be called once function returns

	rep.term = state.currentterm
	rep.success = false

	if args.term < state.currentterm { // 1.  Reply false if term < currentterm
		return nil // restart timer
	}

	if !state.timer.Stop() {
		return nil
	}

	if len(state.log) < args.prevLogIndex || // 2.  Reply false if log doesn’t contain an entry at prevLogIndex
		state.log[args.prevLogIndex].term != args.prevLogterm { // whose term matches prevLogterm
		// return nil // restart timer
	} else {
		// if state.log[args.prevLogIndex].term == args.prevLogterm {
		// 3. If an existing entry conflicts with a new one (same index but different terms)
		state.log = state.log[:args.prevLogIndex+1] // delete the existing entry and all that follow it
		// }

		// 4. Append any new entries not already in the log
		for _, entry := range args.entries {
			state.log = append(state.log, entry)
		}

		if args.leaderCommit > state.commitIndex {
			// 5. If leaderCommit > commitIndex, set commitIndex = min(leaderCommit, index of last new entry)
			if args.leaderCommit > len(state.log) {
				state.commitIndex = len(state.log)
			} else {
				state.commitIndex = args.leaderCommit
			}
		}

		state.curState = 0 // When receiving AppendEntries -> convert to follower
		rep.success = true
	}

	state.timer.Reset(Timer.GenRandom())
	return nil
}
