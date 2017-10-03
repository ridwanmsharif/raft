package raft

import (
	"errors"
	"log"
	"sync"

	pb "github.com/ridwanmsharif/raft/pb"
)

// Storage is an interface that may be implemented by the users
// of the distributed application to retrieve log entries from storage.
// If any Storage method returns an error, the raft instance will
// become inoperable and refuse to participate in elections; the
// application is responsible for cleanup and recovery in this case.
type Storage interface {
	// Entries returns a slice of log entries in the range [lo,hi).
	// MaxSize limits the total size of the log entries returned, but
	// Entries returns at least one entry if any.
	Entries(lo, hi, maxSize int64) ([]pb.LogEntry, error)

	// Term returns the term of entry i, which must be in the range
	// [FirstIndex()-1, LastIndex()]. The term of the entry before
	// FirstIndex is retained for matching purposes even though the
	// rest of that entry may not be available.
	Term(i int64) (int64, error)

	// LastIndex returns the index of the last entry in the log.
	LastIndex() (int64, error)

	// FirstIndex returns the index of the first log entry that is
	// possibly available via Entries (older entries have been incorporated
	// into the latest Snapshot; if storage only contains the dummy entry the
	// first log entry is not available).
	FirstIndex() (int64, error)
}

// MemoryStorage implements the Storage interface backed by an
// in-memory array.
type DefaultStorage struct {
	// Protects access to all fields. Most methods of MemoryStorage are
	// run on the raft goroutine, but Append() is run on an application
	// goroutine.
	sync.Mutex

	// entries[i] has raft log position i
	entries []pb.LogEntry
}

// NewMemoryStorage creates an empty MemoryStorage.
func NewStorage() *DefaultStorage {
	return &DefaultStorage{
		// When starting from scratch populate the list with a dummy entry at term zero.
		entries: make([]pb.LogEntry, 1),
	}
}

// Limit Size of slice of log entries
func limitSize(entries []pb.LogEntry, maxSize int64) []pb.LogEntry {
	if len(entries) == 0 {
		return entries
	}
	var limit int
	for limit = 0; limit < len(entries); limit++ {
		if limit > int(maxSize) {
			break
		}
	}
	return entries[:limit]
}

// Entries implements the Storage interface.
func (ds *DefaultStorage) Entries(lo, hi, maxSize int64) ([]pb.LogEntry, error) {
	ds.Lock()
	defer ds.Unlock()
	offset := ds.entries[0].Index
	if lo <= int64(offset) {
		return nil, errors.New("requested index is unavailable due to compaction")
	}

	if hi > ds.lastIndex()+1 {
		// raftLogger.Panicf
		log.Panicf("entries' hi(%d) is out of bound lastindex(%d)", hi, ds.lastIndex())
	}
	// only contains dummy entries.
	if len(ds.entries) == 1 {
		return nil, errors.New("requested entry at index is unavailable")
	}

	entries := ds.entries[lo-offset : hi-offset]
	return limitSize(entries, maxSize), nil
}

// Term implements the Storage interface.
func (ds *DefaultStorage) Term(i int64) (int64, error) {
	ds.Lock()
	defer ds.Unlock()
	offset := ds.entries[0].Index
	if i < offset {
		return 0, errors.New("requested index is unavailable due to compaction")
	}
	if int(i-offset) >= len(ds.entries) {
		return 0, errors.New("requested entry at index is unavailable")
	}
	return ds.entries[i-offset].Term, nil
}

// LastIndex implements the Storage interface.
func (ds *DefaultStorage) LastIndex() (int64, error) {
	ds.Lock()
	defer ds.Unlock()
	return ds.lastIndex(), nil
}

func (ds *DefaultStorage) lastIndex() int64 {
	return ds.entries[0].Index + int64(len(ds.entries)) - 1
}

// FirstIndex implements the Storage interface.
func (ds *DefaultStorage) FirstIndex() (int64, error) {
	ds.Lock()
	defer ds.Unlock()
	return ds.firstIndex(), nil
}

func (ds *DefaultStorage) firstIndex() int64 {
	return ds.entries[0].Index + 1
}

// Append the new entries to storage.
// entries[0].Index > ds.entries[0].Index
func (ds *DefaultStorage) Append(entries []pb.LogEntry) error {
	if len(entries) == 0 {
		return nil
	}

	ds.Lock()
	defer ds.Unlock()

	first := ds.firstIndex()
	last := entries[0].Index + int64(len(entries)) - 1

	// shortcut if there is no new entry.
	if last < first {
		return nil
	}
	// truncate compacted entries
	// Read entries in te new array whic weren't already in storage
	if first > entries[0].Index {
		entries = entries[first-entries[0].Index:]
	}

	offset := entries[0].Index - ds.entries[0].Index
	switch {
	case int64(len(ds.entries)) > offset:
		ds.entries = append([]pb.LogEntry{}, ds.entries[:offset]...)
		ds.entries = append(ds.entries, entries...)
	case int64(len(ds.entries)) == offset:
		ds.entries = append(ds.entries, entries...)
	default:
		ds.entries = ds.entries
		//	raftLogger.Panicf("missing log entry [last: %d, append at: %d]",
		//		ds.lastIndex(), entries[0].Index)
	}
	return nil
}
