package raft

import (
	"errors"
	// "fmt"
	// "log"
	pb "github.com/ridwanmsharif/raft/pb"
)

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

	// number of ticks since it reached last electionTimeout when it is leader or candidate
	// number of ticks since it reached last elecctionTimeout or recieved a valid message
	// from a current leader when it is a follower
	electionElapsed int

	// number of ticks since it reached last heartbeatTimeout.
	// Kept by leader
	heartbeatElapsed int

	// randomizedElectionTimeout is a random number between election timeout and another
	// election timeout. It gets reset when there is a step change in state - that is when
	// raft changes its state to a follower or candidate
	randomizedElectionTimeout int

	// tick is the raft funtion that the Nodes will call when it'll tick. We will autonomously
	// switch between ticks depending on the state of the raft node.
	tick func()

	// step function for raft
	step stepFunc

	// Logger for raft node
	logger Logger
}

type stepFunc func(r *raft, m pb.Message)
