package raft

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

//==============================================================================
// Global values known to every server
//==============================================================================
//State enum
const (
	LEADER = iota
	FOLLOWER
	CANDIDATE

	MINTIME     = 2000
	TIMERANGE   = 1000
	LEADERTIMER = MINTIME / 2
	TIMESCALE   = time.Millisecond
)

//Ports for all servers. Server ports and addresses are static and known to all.
//In this case, all servers are expected to work on loopback.
var serverPorts = []string{
	":8000",
	":8001",
	":8002"}

var nOfServers int = len(serverPorts)
var majority int = len(serverPorts)/2 + 1

//==============================================================================

//==============================================================================
// Server Structs
//==============================================================================
// Log Entry (appended $VALUE at $TERM)
type LogEntry struct {
	Term  int
	Value int
}

// ServerState holds all relevant variables for servers
type ServerState struct {
	//Extras (not in figure 2)
	id                  int
	timer               *time.Timer
	curState            int
	mux                 sync.Mutex
	timeoutChan         chan int
	shouldIgnoreTimeout bool

	// Persistent
	currentTerm int
	votedFor    int //Should start as -1 since int cannot be nil
	log         []LogEntry

	// Volatile on all
	commitIndex int
	lastApplied int

	// Volatile on leader
	nextIndex  []int
	matchIndex []int
}

//Convenience for building ServerState object
func ServerStateInit(id int) *ServerState {

	server := &ServerState{
		id:                  id,
		timer:               time.NewTimer(0),
		curState:            FOLLOWER,
		mux:                 sync.Mutex{},
		timeoutChan:         make(chan int, 1),
		shouldIgnoreTimeout: false,
		currentTerm:         0,
		votedFor:            -1, //nil
		log:                 make([]LogEntry, 1),
		commitIndex:         0,
		lastApplied:         0,
		nextIndex:           make([]int, nOfServers),
		matchIndex:          make([]int, nOfServers)}

	//Timers should be emptied before resets. Even timers initialized with zero aren't empty.
	<-server.timer.C

	//To avoid unnecessary headache, all servers start with one identical base log entry
	baseLogEntry := LogEntry{Term: 0, Value: 0}
	server.log[0] = baseLogEntry
	return server
}

func (state *ServerState) IsFollower() bool {
	return state.curState == FOLLOWER
}

func (state *ServerState) IsCandidate() bool {
	return state.curState == CANDIDATE
}

func (state *ServerState) IsLeader() bool {
	return state.curState == LEADER
}

//StartTimeoutManager handles the timeout of the state timer. It signals timeouts by writing on the timeoutChan
//No thread should check for timeouts directly on the timer. They should read from timeoutChan instead.
//This encapsulation is justified by the fact that timeouts can occur during RPC processing just before
//Stop() is called on the timer. If this happens and the RPC should have stopped the timer,
//then the timeout is considered to never have ocurred in the first place.
//The RPC function should signal this by changing shouldIgnoreTimeout to true.
func (state *ServerState) StartTimeoutManager() {
	fmt.Println("Timeout Manager start.")
	go func() {
		for {
			<-state.timer.C //Timer fired timeout

			state.mux.Lock() //!!!!!!!!!!!!!!!!!!!!

			if state.shouldIgnoreTimeout {
				state.shouldIgnoreTimeout = false
			} else {
				state.TimeoutStateTransition()
				state.timeoutChan <- 1
			}

			state.mux.Unlock() //!!!!!!!!!!!!!!!!!!!!
		}
	}()
}

//TimeoutStateTransition should be called while state is locked once a timeout has ocurred.
func (state *ServerState) TimeoutStateTransition() {

	if state.IsFollower() {
		state.curState = CANDIDATE
		state.votedFor = -1
		state.currentTerm++
	} else if state.IsCandidate() {
		//Candidates who timeout keep being candidates
		state.votedFor = -1
		state.currentTerm++
	} else if state.IsLeader() {
		fmt.Println("WARNING: timedout as a leader")
		//Leaders should not timeout
	}
}

//ResetStateTimer resets the timer for follower and candidate states
func (state *ServerState) ResetStateTimer() {
	t := time.Duration(MINTIME+rand.Intn(TIMERANGE)) * TIMESCALE
	fmt.Println(t)
	state.timer.Reset(t)
}
