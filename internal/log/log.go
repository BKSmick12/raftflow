package log

import (
	"errors"
	"sync"

	"github.com/BKSmick12/raftflow/internal/storage"
)

// Entry represents a log entry
type Entry struct {
	// Term is the term when this entry was created
	Term int64
	
	// Index is the position of this entry in the log
	Index int64
	
	// Command is the command to be executed
	Command []byte
	
	// Type indicates the type of entry (normal, configuration change, etc.)
	Type EntryType
}

// EntryType represents the type of a log entry
type EntryType int

const (
	EntryNormal EntryType = iota
	EntryConfigChange
	EntrySnapshot
)

func (e EntryType) String() string {
	switch e {
	case EntryNormal:
		return "Normal"
	case EntryConfigChange:
		return "ConfigChange"
	case EntrySnapshot:
		return "Snapshot"
	default:
		return "Unknown"
	}
}

// Log represents the Raft log
type Log struct {
	storage *storage.Storage
	entries []*Entry
	mu      sync.RWMutex
}

// NewLog creates a new Log instance
func NewLog(storage *storage.Storage) (*Log, error) {
	l := &Log{
		storage: storage,
		entries: make([]*Entry, 0),
	}
	
	// Load existing entries from storage
	storedEntries, err := storage.GetLogEntries()
	if err != nil {
		return nil, err
	}
	
	for _, entry := range storedEntries {
		l.entries = append(l.entries, entry)
	}
	
	return l, nil
}

// Append adds an entry to the log
func (l *Log) Append(entry *Entry) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	// Set the index if not already set
	if entry.Index == 0 {
		entry.Index = l.GetLastIndex() + 1
	}
	
	// Append to in-memory log
	l.entries = append(l.entries, entry)
	
	// Persist to storage
	if err := l.storage.AppendLogEntry(entry); err != nil {
		// Remove from in-memory log if persistence fails
		l.entries = l.entries[:len(l.entries)-1]
		return err
	}
	
	return nil
}

// GetEntry retrieves an entry by index
func (l *Log) GetEntry(index int64) (*Entry, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	
	if index < 1 || index > int64(len(l.entries)) {
		return nil, ErrEntryNotFound
	}
	
	return l.entries[index-1], nil
}

// GetEntries retrieves a range of entries
func (l *Log) GetEntries(start, end int64) ([]*Entry, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	
	if start < 1 || end > int64(len(l.entries)) || start > end {
		return nil, ErrInvalidRange
	}
	
	return l.entries[start-1:end], nil
}

// GetLastIndex returns the index of the last entry
func (l *Log) GetLastIndex() int64 {
	l.mu.RLock()
	defer l.mu.RUnlock()
	
	if len(l.entries) == 0 {
		return 0
	}
	
	return l.entries[len(l.entries)-1].Index
}

// GetLastEntryInfo returns the index and term of the last entry
func (l *Log) GetLastEntryInfo() (int64, int64) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	
	if len(l.entries) == 0 {
		return 0, 0
	}
	
	lastEntry := l.entries[len(l.entries)-1]
	return lastEntry.Index, lastEntry.Term
}

// DeleteFrom deletes all entries from the given index onwards
func (l *Log) DeleteFrom(index int64) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	if index < 1 || index > int64(len(l.entries)) {
		return ErrInvalidIndex
	}
	
	// Truncate in-memory log
	l.entries = l.entries[:index-1]
	
	// Delete from storage
	if err := l.storage.TruncateLog(index - 1); err != nil {
		return err
	}
	
	return nil
}

// DeleteRange deletes a range of entries
func (l *Log) DeleteRange(start, end int64) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	if start < 1 || end > int64(len(l.entries)) || start > end {
		return ErrInvalidRange
	}
	
	// Create new slice without the range
	newEntries := make([]*Entry, 0, len(l.entries)-(int(end)-int(start)+1))
	newEntries = append(newEntries, l.entries[:start-1]...)
	newEntries = append(newEntries, l.entries[end:]...)
	
	l.entries = newEntries
	
	// Update indices of remaining entries
	for i := start - 1; i < len(l.entries); i++ {
		l.entries[i].Index = int64(i + 1)
	}
	
	// Delete from storage and re-write remaining entries
	if err := l.storage.TruncateLog(0); err != nil {
		return err
	}
	
	for _, entry := range l.entries {
		if err := l.storage.AppendLogEntry(entry); err != nil {
			return err
		}
	}
	
	return nil
}

// GetTerm returns the term for a given index
func (l *Log) GetTerm(index int64) (int64, error) {
	entry, err := l.GetEntry(index)
	if err != nil {
		return 0, err
	}
	return entry.Term, nil
}

// Size returns the number of entries in the log
func (l *Log) Size() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.entries)
}

// IsEmpty returns true if the log is empty
func (l *Log) IsEmpty() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.entries) == 0
}

// Clear removes all entries from the log
func (l *Log) Clear() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	l.entries = make([]*Entry, 0)
	
	if err := l.storage.TruncateLog(0); err != nil {
		return err
	}
	
	return nil
}

// ErrEntryNotFound is returned when an entry is not found
var ErrEntryNotFound = errors.New("entry not found")

// ErrInvalidRange is returned when an invalid range is specified
var ErrInvalidRange = errors.New("invalid range")

// ErrInvalidIndex is returned when an invalid index is specified
var ErrInvalidIndex = errors.New("invalid index")
