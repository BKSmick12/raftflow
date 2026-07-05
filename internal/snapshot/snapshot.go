package snapshot

import (
	"time"

	"github.com/BKSmick12/raftflow/internal/log"
	"github.com/BKSmick12/raftflow/internal/storage"
	"go.uber.org/zap"
)

// Manager manages snapshot creation and restoration
type Manager struct {
	storage           *storage.Storage
	threshold        int
	interval         time.Duration
	logger           *zap.Logger
	lastSnapshotIndex int64
	lastSnapshotTerm  int64
}

// NewManager creates a new snapshot manager
func NewManager(storage *storage.Storage, threshold int) (*Manager, error) {
	return &Manager{
		storage:           storage,
		threshold:        threshold,
		logger:           zap.L().Named("snapshot"),
		lastSnapshotIndex: 0,
		lastSnapshotTerm:  0,
	}, nil
}

// Start starts the snapshot manager
func (m *Manager) Start(interval time.Duration, raftLog *log.Log, storage *storage.Storage) {
	m.interval = interval
	
	// Load latest snapshot info
	latestSnapshot, err := m.storage.GetLatestSnapshot()
	if err == nil && latestSnapshot != nil {
		m.lastSnapshotIndex = latestSnapshot.LastIncludedIndex
		m.lastSnapshotTerm = latestSnapshot.LastIncludedTerm
	}
	
	// Start snapshot ticker
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			m.CheckAndCreate(raftLog, raftLog.GetLastIndex())
		}
	}
}

// CheckAndCreate checks if a snapshot should be created and creates one if needed
func (m *Manager) CheckAndCreate(raftLog *log.Log, currentIndex int64) {
	// Check if we've reached the threshold
	if currentIndex-m.lastSnapshotIndex >= int64(m.threshold) {
		m.Create(raftLog, currentIndex)
	}
}

// Create creates a new snapshot
func (m *Manager) Create(raftLog *log.Log, lastIndex int64) error {
	m.logger.Info("Creating snapshot",
		zap.Int64("last_index", lastIndex),
		zap.Int64("last_snapshot_index", m.lastSnapshotIndex),
	)
	
	// Get the last entry info
	lastTerm, err := raftLog.GetTerm(lastIndex)
	if err != nil {
		m.logger.Error("Failed to get last term", zap.Error(err))
		return err
	}
	
	// Create snapshot data
	// In a real implementation, this would include the state machine state
	snapshotData := []byte("snapshot data") // Placeholder
	
	// Create snapshot
	snapshot := &storage.Snapshot{
		LastIncludedIndex: lastIndex,
		LastIncludedTerm:  lastTerm,
		Data:             snapshotData,
		// Configuration would be included in a real implementation
	}
	
	// Save snapshot
	if err := m.storage.SaveSnapshot(snapshot); err != nil {
		m.logger.Error("Failed to save snapshot", zap.Error(err))
		return err
	}
	
	// Update last snapshot info
	m.lastSnapshotIndex = lastIndex
	m.lastSnapshotTerm = lastTerm
	
	// Compact log
	if err := raftLog.DeleteFrom(lastIndex); err != nil {
		m.logger.Error("Failed to compact log", zap.Error(err))
		return err
	}
	
	m.logger.Info("Snapshot created successfully",
		zap.Int64("last_included_index", lastIndex),
		zap.Int64("last_included_term", lastTerm),
	)
	
	return nil
}

// Load loads a snapshot
func (m *Manager) Load(snapshot *storage.Snapshot) error {
	m.logger.Info("Loading snapshot",
		zap.Int64("last_included_index", snapshot.LastIncludedIndex),
		zap.Int64("last_included_term", snapshot.LastIncludedTerm),
	)
	
	// Update last snapshot info
	m.lastSnapshotIndex = snapshot.LastIncludedIndex
	m.lastSnapshotTerm = snapshot.LastIncludedTerm
	
	// In a real implementation, we would restore the state machine state
	// from snapshot.Data
	
	return nil
}

// GetLastSnapshotIndex returns the index of the last snapshot
func (m *Manager) GetLastSnapshotIndex() int64 {
	return m.lastSnapshotIndex
}

// GetLastSnapshotTerm returns the term of the last snapshot
func (m *Manager) GetLastSnapshotTerm() int64 {
	return m.lastSnapshotTerm
}

// NeedsSnapshot returns true if a snapshot should be created
func (m *Manager) NeedsSnapshot(currentIndex int64) bool {
	return currentIndex-m.lastSnapshotIndex >= int64(m.threshold)
}
