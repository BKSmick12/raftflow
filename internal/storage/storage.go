package storage

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/BKSmick12/raftflow/internal/log"
)

// Storage provides persistent storage for Raft state
type Storage struct {
	logDir      string
	snapshotDir string
	mu          sync.RWMutex
}

// NewStorage creates a new Storage instance
func NewStorage(logDir, snapshotDir string) (*Storage, error) {
	// Create directories if they don't exist
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}
	
	if err := os.MkdirAll(snapshotDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create snapshot directory: %w", err)
	}
	
	return &Storage{
		logDir:      logDir,
		snapshotDir: snapshotDir,
	}, nil
}

// SavePersistentState saves the persistent state (current term and votedFor)
func (s *Storage) SavePersistentState(term int64, votedFor string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	state := PersistentState{
		CurrentTerm: term,
		VotedFor:    votedFor,
	}
	
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	
	stateFile := filepath.Join(s.logDir, "persistent_state.json")
	
	// Write to temporary file first, then rename for atomicity
	tmpFile := stateFile + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return err
	}
	
	if err := os.Rename(tmpFile, stateFile); err != nil {
		os.Remove(tmpFile)
		return err
	}
	
	return nil
}

// GetPersistentState retrieves the persistent state
func (s *Storage) GetPersistentState() (int64, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	stateFile := filepath.Join(s.logDir, "persistent_state.json")
	
	data, err := os.ReadFile(stateFile)
	if err != nil {
		if os.IsNotExist(err) {
			// Return default values if file doesn't exist
			return 0, "", nil
		}
		return 0, "", err
	}
	
	var state PersistentState
	if err := json.Unmarshal(data, &state); err != nil {
		return 0, "", err
	}
	
	return state.CurrentTerm, state.VotedFor, nil
}

// AppendLogEntry appends a log entry to persistent storage
func (s *Storage) AppendLogEntry(entry *log.Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// Marshal entry to JSON
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	
	// Create entry file name: term-index.json
	filename := fmt.Sprintf("term-%d-index-%d.json", entry.Term, entry.Index)
	entryFile := filepath.Join(s.logDir, filename)
	
	// Write to temporary file first, then rename for atomicity
	tmpFile := entryFile + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return err
	}
	
	if err := os.Rename(tmpFile, entryFile); err != nil {
		os.Remove(tmpFile)
		return err
	}
	
	return nil
}

// GetLogEntries retrieves all log entries from storage
func (s *Storage) GetLogEntries() ([]*log.Entry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	entries := make([]*log.Entry, 0)
	
	// Read all .json files in the log directory
	files, err := os.ReadDir(s.logDir)
	if err != nil {
		return nil, err
	}
	
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		
		filename := file.Name()
		if filename == "persistent_state.json" || filename == "persistent_state.json.tmp" {
			continue
		}
		
		// Check if it's a log entry file (term-*-index-*.json)
		var term, index int64
		if _, err := fmt.Sscanf(filename, "term-%d-index-%d.json", &term, &index); err == nil {
			data, err := os.ReadFile(filepath.Join(s.logDir, filename))
			if err != nil {
				continue // Skip files we can't read
			}
			
			var entry log.Entry
			if err := json.Unmarshal(data, &entry); err != nil {
				continue // Skip files we can't parse
			}
			
			entries = append(entries, &entry)
		}
	}
	
	// Sort entries by index
	for i := 0; i < len(entries); i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[i].Index > entries[j].Index {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}
	
	return entries, nil
}

// GetLogEntry retrieves a specific log entry by index
func (s *Storage) GetLogEntry(index int64) (*log.Entry, error) {
	entries, err := s.GetLogEntries()
	if err != nil {
		return nil, err
	}
	
	for _, entry := range entries {
		if entry.Index == index {
			return entry, nil
		}
	}
	
	return nil, ErrEntryNotFound
}

// TruncateLog truncates the log up to the specified index
func (s *Storage) TruncateLog(index int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	entries, err := s.GetLogEntries()
	if err != nil {
		return err
	}
	
	// Delete all entries with index > specified index
	for _, entry := range entries {
		if entry.Index > index {
			filename := fmt.Sprintf("term-%d-index-%d.json", entry.Term, entry.Index)
			entryFile := filepath.Join(s.logDir, filename)
			if err := os.Remove(entryFile); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
	}
	
	return nil
}

// SaveSnapshot saves a snapshot to persistent storage
func (s *Storage) SaveSnapshot(snapshot *Snapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	data, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	
	// Create snapshot file name: snapshot-term-index.json
	filename := fmt.Sprintf("snapshot-term-%d-index-%d.json", snapshot.LastIncludedTerm, snapshot.LastIncludedIndex)
	snapshotFile := filepath.Join(s.snapshotDir, filename)
	
	// Write to temporary file first, then rename for atomicity
	tmpFile := snapshotFile + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return err
	}
	
	if err := os.Rename(tmpFile, snapshotFile); err != nil {
		os.Remove(tmpFile)
		return err
	}
	
	// Clean up old snapshots
	// Keep only the most recent snapshot
	files, err := os.ReadDir(s.snapshotDir)
	if err != nil {
		return err
	}
	
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		
		filename := file.Name()
		if filename != snapshotFile && filename != snapshotFile+".tmp" {
			os.Remove(filepath.Join(s.snapshotDir, filename))
		}
	}
	
	return nil
}

// GetLatestSnapshot retrieves the latest snapshot
func (s *Storage) GetLatestSnapshot() (*Snapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	files, err := os.ReadDir(s.snapshotDir)
	if err != nil {
		return nil, err
	}
	
	var latestSnapshot *Snapshot
	var latestIndex int64 = 0
	
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		
		filename := file.Name()
		var term, index int64
		if _, err := fmt.Sscanf(filename, "snapshot-term-%d-index-%d.json", &term, &index); err == nil {
			if index > latestIndex {
				data, err := os.ReadFile(filepath.Join(s.snapshotDir, filename))
				if err != nil {
					continue
				}
				
				var snapshot Snapshot
				if err := json.Unmarshal(data, &snapshot); err != nil {
					continue
				}
				
				latestSnapshot = &snapshot
				latestIndex = index
			}
		}
	}
	
	if latestSnapshot == nil {
		return nil, ErrNoSnapshot
	}
	
	return latestSnapshot, nil
}

// GetSnapshot retrieves a specific snapshot by index
func (s *Storage) GetSnapshot(index int64) (*Snapshot, error) {
	files, err := os.ReadDir(s.snapshotDir)
	if err != nil {
		return nil, err
	}
	
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		
		filename := file.Name()
		var term, idx int64
		if _, err := fmt.Sscanf(filename, "snapshot-term-%d-index-%d.json", &term, &idx); err == nil {
			if idx == index {
				data, err := os.ReadFile(filepath.Join(s.snapshotDir, filename))
				if err != nil {
					return nil, err
				}
				
				var snapshot Snapshot
				if err := json.Unmarshal(data, &snapshot); err != nil {
					return nil, err
				}
				
				return &snapshot, nil
			}
		}
	}
	
	return nil, ErrNoSnapshot
}

// Close closes the storage
func (s *Storage) Close() error {
	// Nothing to do for now
	return nil
}

// PersistentState represents the persistent state that must be saved
// according to the Raft paper
type PersistentState struct {
	CurrentTerm int64  `json:"current_term"`
	VotedFor    string `json:"voted_for"`
}

// Snapshot represents a snapshot of the system state
type Snapshot struct {
	// LastIncludedIndex is the index of the last entry included in the snapshot
	LastIncludedIndex int64 `json:"last_included_index"`
	
	// LastIncludedTerm is the term of the last entry included in the snapshot
	LastIncludedTerm int64 `json:"last_included_term"`
	
	// Data is the snapshot data (state machine state)
	Data []byte `json:"data"`
	
	// Configuration is the cluster configuration at the time of the snapshot
	Configuration []string `json:"configuration"`
}

// WAL provides write-ahead logging functionality
type WAL struct {
	file     *os.File
	encoder  *json.Encoder
	decoder  *json.Decoder
	position int64
	mu       sync.Mutex
}

// NewWAL creates a new WAL instance
func NewWAL(dir string) (*WAL, error) {
	walFile := filepath.Join(dir, "wal.log")
	
	file, err := os.OpenFile(walFile, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}
	
	// Get current file position
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, err
	}
	
	return &WAL{
		file:     file,
		encoder:  json.NewEncoder(file),
		decoder:  json.NewDecoder(file),
		position: info.Size(),
	}, nil
}

// Write writes an entry to the WAL
func (w *WAL) Write(entry *log.Entry) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	
	// Write the entry length first (4 bytes)
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	
	length := uint32(len(data))
	if err := binary.Write(w.file, binary.BigEndian, length); err != nil {
		return err
	}
	
	// Write the entry data
	if _, err := w.file.Write(data); err != nil {
		return err
	}
	
	// Flush to ensure data is written to disk
	if err := w.file.Sync(); err != nil {
		return err
	}
	
	w.position += int64(4 + len(data))
	
	return nil
}

// Read reads entries from the WAL starting from the given position
func (w *WAL) Read() ([]*log.Entry, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	
	// Seek to the beginning
	if _, err := w.file.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	
	entries := make([]*log.Entry, 0)
	
	for {
		// Read the length
		var length uint32
		if err := binary.Read(w.file, binary.BigEndian, &length); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		
		// Read the entry data
		data := make([]byte, length)
		if _, err := io.ReadFull(w.file, data); err != nil {
			return nil, err
		}
		
		var entry log.Entry
		if err := json.Unmarshal(data, &entry); err != nil {
			return nil, err
		}
		
		entries = append(entries, &entry)
	}
	
	return entries, nil
}

// Close closes the WAL
func (w *WAL) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	
	if err := w.file.Close(); err != nil {
		return err
	}
	
	return nil
}

// ErrEntryNotFound is returned when an entry is not found
var ErrEntryNotFound = errors.New("entry not found")

// ErrNoSnapshot is returned when no snapshot exists
var ErrNoSnapshot = errors.New("no snapshot found")
