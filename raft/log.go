package raft

import (
	"errors"
	"fmt"
	"log"

	pb "github.com/ridwanmsharif/raft/pb"
)

// Raft Log
type raftLog struct {
	// storage contains all stable entries.
	storage Storage

	// committed is the highest log position that is known to be in
	// stable storage on a quorum of nodes.
	committed int64

	// applied is the highest log position that the application has
	// been instructed to apply to its state machine.
	// Invariant: applied <= committed
	applied int64

	logger Logger
}

// newLog returns log using the given storage. It recovers the log to the state
// that it just commits and applies the latest snapshot.
func newLog(storage Storage, logger Logger) *raftLog {
	if storage == nil {
		log.Panic("storage must not be nil")
	}
	log := &raftLog{
		storage: storage,
		logger:  logger,
	}
	firstIndex, err := storage.FirstIndex()
	if err != nil {
		panic(err)
	}
	// lastindex
	_, err = storage.LastIndex()
	if err != nil {
		panic(err)
	}
	log.committed = firstIndex - 1
	log.applied = firstIndex - 1

	return log
}

func (l *raftLog) String() string {
	return fmt.Sprintf("committed=%d, applied=%d", l.committed, l.applied)
}

// maybeAppend returns (0, false) if the entries cannot be appended. Otherwise,
// it returns (last index of new entries, true).
func (l *raftLog) maybeAppend(index, logTerm, committed int64, entries ...pb.LogEntry) (lastnewi int64, ok bool) {
	if l.matchTerm(index, logTerm) {
		lastnewi = index + int64(len(entries))
		ci := l.findConflict(entries)
		switch {
		case ci == 0:
		case ci <= l.committed:
			l.logger.Fatalf("entry %d conflict with committed entry [committed(%d)]", ci, l.committed)
		default:
			offset := index + 1
			l.append(entries[ci-offset:]...)
		}
		return lastnewi, true
	}
	return 0, false
}

// Find conflicts and returns index of conflict
func (l *raftLog) findConflict(entries []pb.LogEntry) int64 {
	for _, ne := range entries {
		if !l.matchTerm(ne.Index, ne.Term) {
			if ne.Index <= l.lastIndex() {
				l.logger.Printf("found conflict at index %d [existing term: %d, conflicting term: %d]",
					ne.Index, l.zeroTermOnErrCompacted(l.term(ne.Index)), ne.Term)
			}
			return ne.Index
		}
	}
	return 0
}

func (l *raftLog) append(entries ...pb.LogEntry) int64 {
	if len(entries) == 0 {
		return l.lastIndex()
	}
	if after := entries[0].Index - 1; after < l.committed {
		l.logger.Fatalf("after(%d) is out of range [committed(%d)]", after, l.committed)
	}
	l.storage.Append(entries)
	return l.lastIndex()
}

func (l *raftLog) firstIndex() int64 {
	index, err := l.storage.FirstIndex()
	if err != nil {
		panic(err)
	}
	return index
}

func (l *raftLog) lastIndex() int64 {
	index, err := l.storage.LastIndex()
	if err != nil {
		panic(err)
	}
	return index
}

func (l *raftLog) lastTerm() int64 {
	term, err := l.term(l.lastIndex())
	if err != nil {
		l.logger.Fatalf("unexpected error when getting the last term (%v)", err)
	}
	return term
}

func (l *raftLog) term(i int64) (int64, error) {
	// the valid term range is [index of dummy entry, last index]
	dummyIndex := l.firstIndex() - 1
	if i < dummyIndex || i > l.lastIndex() {
		return 0, nil
	}

	t, err := l.storage.Term(i)
	if err == nil {
		return t, nil
	}
	log.Printf("ERROR: (%v)", err)
	return 0, err
}

func (l *raftLog) entries(i, maxsize int64) ([]pb.LogEntry, error) {
	if i > l.lastIndex() {
		return nil, nil
	}
	return l.slice(i, l.lastIndex()+1, maxsize)
}

// allEntries returns all entries in the log.
func (l *raftLog) allEntries() []pb.LogEntry {
	ents, err := l.entries(l.firstIndex(), noLimit)
	if err == nil {
		return ents
	}
	return nil
}

// isUpToDate determines if the given (lastIndex,term) log is more up-to-date
// by comparing the index and term of the last entries in the existing logs.
// If the logs have last entries with different terms, then the log with the
// later term is more up-to-date. If the logs end with the same term, then
// whichever log has the larger lastIndex is more up-to-date. If the logs are
// the same, the given log is up-to-date.
func (l *raftLog) isUpToDate(lasti, term int64) bool {
	return term > l.lastTerm() || (term == l.lastTerm() && lasti >= l.lastIndex())
}

func (l *raftLog) matchTerm(i, term int64) bool {
	t, err := l.term(i)
	if err != nil {
		return false
	}
	return t == term
}

// slice returns a slice of log entries from lo through hi-1, inclusive.
func (l *raftLog) slice(lo, hi, maxSize int64) ([]pb.LogEntry, error) {
	err := l.mustCheckOutOfBounds(lo, hi)
	if err != nil {
		return nil, err
	}
	if lo == hi {
		return nil, nil
	}
	var entries []pb.LogEntry
	storedEnts, err := l.storage.Entries(lo, hi, maxSize)
	if err != nil {
		panic(err)
	}

	// check if ents has reached the size limitation
	if int64(len(storedEnts)) < hi-lo {
		return storedEnts, nil
	}

	entries = storedEnts
	return limitSize(entries, maxSize), nil
}

// l.firstIndex <= lo <= hi <= l.firstIndex + len(l.entries)
func (l *raftLog) mustCheckOutOfBounds(lo, hi int64) error {
	if lo > hi {
		l.logger.Fatalf("invalid slice %d > %d", lo, hi)
	}
	fi := l.firstIndex()
	if lo < fi {
		return errors.New("index could not be reached due to compaction")
	}

	length := l.lastIndex() + 1 - fi
	if lo < fi || hi > fi+length {
		l.logger.Fatalf("slice[%d,%d) out of bound [%d,%d]", lo, hi, fi, l.lastIndex())
	}
	return nil
}

func (l *raftLog) zeroTermOnErrCompacted(t int64, err error) int64 {
	if err == nil {
		return t
	}

	l.logger.Fatalf("unexpected error (%v)", err)
	return 0
}
