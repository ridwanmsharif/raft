package raft

import (
	pb "github.com/ridwanmsharif/raft/pb"
)

// Ready encapsulates the entries and messages that are ready to read,
// be saved to stable storage, committed or sent to other peers.
// All fields in Ready are read-only.
type Ready struct {
	// Entries specifies entries to be saved to stable storage BEFORE
	// Messages are sent.
	Entries []pb.LogEntry

	// CommittedEntries specifies entries to be committed to a
	// store/state-machine. These have previously been committed to stable
	// store.
	CommittedEntries []pb.LogEntry

	// Messages specifies outbound messages to be sent AFTER Entries are
	// committed to stable storage.
	// If it contains a MsgSnap message, the application MUST report back to raft
	// when the snapshot has been received or has failed by calling ReportSnapshot.
	Messages []pb.Message
}

func (rd Ready) containsUpdates() bool {
	return len(rd.Entries) > 0 || len(rd.CommittedEntries) > 0 ||
		len(rd.Messages) > 0
}

// Node represents a node in a raft cluster.
type Node interface {
	// Tick increments the internal logical clock for the Node by a single tick. Election
	// timeouts and heartbeat timeouts are in units of ticks.
	Tick()

	// Ready returns a channel that returns the current point-in-time state.
	// Users of the Node must call Advance after retrieving the state returned by Ready.
	Ready() <-chan Ready

	// Notifies the Node that the application has saved progress up to the last Ready.
	// It prepares the node to return the next available Ready.
	// The application should generally call after it applies the entries in last Ready.
	Advance()

	// Stop performs any necessary termination of the Node.
	Stop()
}

type Peer struct {
	ID      uint64
	Context []byte
}

// StartNode returns a new Node given configuration and a list of raft peers.
// It appends a ConfChangeAddNode entry for each given peer to the initial log.
func StartNode(c *Config, peers []Peer) Node {
	r := newRaft(c)
	// become the follower at term 1 and apply initial configuration
	// entries of term 1
	r.becomeFollower(1, 0)
	// Mark these initial entries as committed.
	r.raftLog.committed = r.raftLog.lastIndex()

	n := newNode()
	n.logger = c.Logger
	go n.run(r)
	return &n
}

// RestartNode is similar to StartNode but does not take a list of peers.
// The current membership of the cluster will be restored from the Storage.
// If the caller has an existing state machine, pass in the last log index that
// has been applied to it; otherwise use zero.
func RestartNode(c *Config) Node {
	r := newRaft(c)

	n := newNode()
	n.logger = c.Logger
	go n.run(r)
	return &n
}

// node is the canonical implementation of the Node interface
type node struct {
	propc    chan pb.Message
	recvc    chan pb.Message
	readyc   chan Ready
	advancec chan struct{}
	tickc    chan struct{}
	done     chan struct{}
	stop     chan struct{}

	logger Logger
}

func newNode() node {
	return node{
		propc:    make(chan pb.Message),
		recvc:    make(chan pb.Message),
		readyc:   make(chan Ready),
		advancec: make(chan struct{}),
		tickc:    make(chan struct{}, 128),
		done:     make(chan struct{}),
		stop:     make(chan struct{}),
	}
}

func (n *node) Stop() {
	select {
	case n.stop <- struct{}{}:
		// Not already stopped, so trigger it
	case <-n.done:
		// Node has already been stopped - no need to do anything
		return
	}
	// Block until the stop has been acknowledged by run()
	<-n.done
}

func (n *node) run(r *raft) {
	var propc chan pb.Message
	var readyc chan Ready
	var advancec chan struct{}
	var rd Ready

	lead := int64(0)
	for {
		if advancec != nil {
			readyc = nil
		} else {
			rd = newReady(r)
			if rd.containsUpdates() {
				readyc = n.readyc
			} else {
				readyc = nil
			}
		}

		if lead != r.lead {
			if r.hasLeader() {
				if lead == 0 {
					r.logger.Printf("raft.node: %x elected leader %x at term %d", r.ID, r.lead, r.Term)
				} else {
					r.logger.Printf("raft.node: %x changed leader from %x to %x at term %d", r.ID, lead, r.lead, r.Term)
				}
				propc = n.propc
			} else {
				r.logger.Printf("raft.node: %x lost leader %x at term %d", r.ID, lead, r.Term)
				propc = nil
			}
			lead = r.lead
		}

		select {
		case _ = <-propc:
			//m.From = r.id
			r.logger.Printf("Message from %d", r.ID)
		case _ = <-n.recvc:
			r.logger.Printf("Message recieved ")
		case <-n.tickc:
			r.tick()
		case readyc <- rd:
			if len(rd.Entries) > 0 {
				r.logger.Printf("More than 0 readies")
			}

			r.msgs = nil
			advancec = n.advancec
		case <-advancec:
			advancec = nil
		case <-n.stop:
			close(n.done)
			return
		}
	}
}

// Tick increments the internal logical clock for this Node. Election timeouts
// and heartbeat timeouts are in units of ticks.
func (n *node) Tick() {
	select {
	case n.tickc <- struct{}{}:
	case <-n.done:
	default:
		n.logger.Printf("A tick missed to fire. Node blocks too long!")
	}
}

func (n *node) Ready() <-chan Ready { return n.readyc }

func (n *node) Advance() {
	select {
	case n.advancec <- struct{}{}:
	case <-n.done:
	}
}

func newReady(r *raft) Ready {
	rd := Ready{
		Entries:          r.raftLog.allEntries(),
		CommittedEntries: nil,
		Messages:         r.msgs,
	}
	return rd
}
