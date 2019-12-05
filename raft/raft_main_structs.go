package raft

import (
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
	term  int
	value int
}

// Processes' state
type ServerState struct {
	//Extras (not in figure 2)
	id       int
	timer    *time.Timer
	curState int
	mux      sync.Mutex

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
		id:          id,
		timer:       time.NewTimer(0),
		curState:    FOLLOWER,
		mux:         sync.Mutex{},
		currentTerm: 0,
		votedFor:    -1, //nil
		log:         make([]LogEntry, 1),
		commitIndex: 0,
		lastApplied: 0,
		nextIndex:   make([]int, nOfServers),
		matchIndex:  make([]int, nOfServers)}

	//Timers should be emptied before resets. Even timers initialized with zero aren't empty
	<-server.timer.C

	//To avoid unnecessary headache, all servers start with one identical base log entry
	baseLogEntry := LogEntry{term: 0, value: 0}
	server.log[0] = baseLogEntry
	return server
}
