package raft

import (
	"errors"
	"math"
	"math/rand"
	"time"

	// "log"
	pb "github.com/ridwanmsharif/raft/pb"
)

type StateType int64

const noLimit = math.MaxInt64

var stmap = [...]string{
	"StateFollower",
	"StateCandidate",
	"StateLeader",
}

func (st StateType) String() string {
	return stmap[uint64(st)]
}

type Config struct {
	// Raft Identity
	ID int64

	// peers contains the IDs of all the nodes (including self) in the cluster
	// To be set when a new raft cluster is started.
	Peers []int64

	// ElectionTick is the number of Node.Tick calls must pass between elections
	// It is the enumerator that triggers the step to a candidtade stage. If a
	// follower does not recieve any message from the current leader before ElectionTick
	// has completed, then it will start an election by stepping to a candidate stage.
	// ElectionTick = 10 * HeartbeatTick. We do this to avoid redundant leader elections
	ElectionTick int

	// HeartbeatTick is the number of Node.Tick calls that must pass between heartbeats
	// Leader sends heartbeats (messages) to maintain its state as the leader
	HeartbeatTick int

	// Storage for raft. raft generates entries and states to be stored in storage.
	// Persistent entries and storage is read by raft when required. Sate and config
	// info is used from storage when restarting
	Storage Storage

	// Applied is the index of the last applied entry. It's used only when restarting raft.
	// raft will not return entries to the application smaller or equal to applied.
	Applied int64

	// Logger is the logger used for the raft log.
	Logger Logger

	// All other configuration details to be added as required when need arises
	// Eg: Logger, MaxSizePerMsg, CheckQuorum, PreVote et cetera.
}

// Configuration validation
func (c *Config) Validate() error {
	// Check for valid ID - ID cannot be 0
	if c.ID == 0 {
		return errors.New("invalid ID, ID must be a positive integer")
	}

	// Check for valid HeartbeatTick
	if c.HeartbeatTick <= 0 {
		return errors.New("HeartbeatTick must be greater than 0")
	}

	// Check for valid ElectionTick
	if c.ElectionTick <= c.HeartbeatTick {
		return errors.New("ElectionTick must be greater than HeartbeatTick ")
	}

	return nil
}

const (
	StateFollower  StateType = 0
	StateCandidate StateType = 1
	StateLeader    StateType = 2
)

// Raft structure for each node in the cluster
type raft struct {
	// Raft ID
	ID int64

	// Current term
	Term int64

	// ID Node vored for (Must be a valid node ID)
	Vote int64

	// ID of leader
	lead int64

	// the raft log
	raftLog *raftLog

	// State of the raft node
	state StateType

	// number of ticks since it reached last electionTimeout when it is leader or candidate
	// number of ticks since it reached last elecctionTimeout or recieved a valid message
	// from a current leader when it is a follower
	electionElapsed int

	// number of ticks since it reached last heartbeatTimeout.
	// Kept by leader
	heartbeatElapsed int

	// Timeouts
	heartbeatTimeout int
	electionTimeout  int
	// randomizedElectionTimeout is a random number between election timeout and another
	// election timeout. It gets reset when there is a step change in state - that is when
	// raft changes its state to a follower or candidate
	randomizedElectionTimeout int

	// tick is the raft funtion that the Nodes will call when it'll tick. We will autonomously
	// switch between ticks depending on the state of the raft node.
	tick func()

	// Votes from all the peers
	votes map[int64]bool

	// Array of messages to be processed
	msgs []pb.Message

	// step function for raft
	step stepFunc

	// Logger for raft node
	logger Logger
}

func (r *raft) hasLeader() bool { return r.lead != 0 }

func newRaft(c *Config) *raft {
	// Check if config is valid
	if err := c.Validate(); err != nil {
		panic(err.Error())
	}
	raftlog := newLog(c.Storage, c.Logger)

	// Set peers
	// peers := c.Peers
	r := &raft{
		ID:               c.ID,
		lead:             0,
		electionTimeout:  c.ElectionTick,
		heartbeatTimeout: c.HeartbeatTick,
		raftLog:          raftlog,
		logger:           c.Logger,
	}

	r.logger.Printf("newRaft %x , term: %d, commit: %d, applied: %d, lastindex: %d, lastterm: %d]",
		r.ID, r.Term, r.raftLog.committed, r.raftLog.applied, r.raftLog.lastIndex(), r.raftLog.lastTerm())
	return r
}

func (r *raft) reset(term int64) {
	if r.Term != term {
		r.Term = term
		r.Vote = 0
	}
	r.lead = 0

	r.electionElapsed = 0
	r.heartbeatElapsed = 0
	r.resetRandomizedElectionTimeout()
	r.votes = make(map[int64]bool)
}

// Always promotable for now
func (r *raft) promotable() bool {
	return true
}

// tickElection is run by followers and candidates after r.electionTimeout.
func (r *raft) tickElection() {
	r.electionElapsed++

	if r.promotable() && r.pastElectionTimeout() {
		r.electionElapsed = 0
		r.logger.Printf("Past Election timeout")
	}
}

// tickHeartbeat is run by leaders to send a MsgBeat after r.heartbeatTimeout.
func (r *raft) tickHeartbeat() {
	r.heartbeatElapsed++
	r.electionElapsed++

	if r.electionElapsed >= r.electionTimeout {
		r.electionElapsed = 0
		r.logger.Printf("Past Election timeout")
	}

	if r.state != StateLeader {
		return
	}

	if r.heartbeatElapsed >= r.heartbeatTimeout {
		r.heartbeatElapsed = 0
		r.logger.Printf("Past heartbeat timeout")
	}
}

func (r *raft) becomeFollower(term int64, lead int64) {
	r.reset(term)
	r.tick = r.tickElection
	r.lead = lead
	r.state = StateFollower
	r.logger.Printf("%x became follower at term %d", r.ID, r.Term)
}

func (r *raft) becomeCandidate() {
	// TODO(xiangli) remove the panic when the raft implementation is stable
	if r.state == StateLeader {
		panic("invalid transition [leader -> candidate]")
	}
	r.reset(r.Term + 1)
	r.tick = r.tickElection
	r.Vote = r.ID
	r.state = StateCandidate
	r.logger.Printf("%x became candidate at term %d", r.ID, r.Term)
}

func (r *raft) becomeLeader() {
	// TODO(xiangli) remove the panic when the raft implementation is stable
	if r.state == StateFollower {
		panic("invalid transition [follower -> leader]")
	}
	r.reset(r.Term)
	r.tick = r.tickHeartbeat
	r.lead = r.ID
	r.state = StateLeader
	_, err := r.raftLog.entries(r.raftLog.committed+1, noLimit)
	if err != nil {
		r.logger.Fatalf("unexpected error getting uncommitted entries (%v)", err)
	}

	r.logger.Printf("%x became leader at term %d", r.ID, r.Term)
}

// pastElectionTimeout returns true iff r.electionElapsed is greater
// than or equal to the randomized election timeout in
// [electiontimeout, 2 * electiontimeout - 1].
func (r *raft) pastElectionTimeout() bool {
	return r.electionElapsed >= r.randomizedElectionTimeout
}

func (r *raft) resetRandomizedElectionTimeout() {
	random := rand.New(rand.NewSource(time.Now().UnixNano()))
	randomint := random.Intn(r.electionTimeout)
	r.randomizedElectionTimeout = r.electionTimeout + randomint
}

type stepFunc func(r *raft, m pb.Message)
