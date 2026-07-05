package log

import (
	"testing"
)

func TestLogAppend(t *testing.T) {
	// Create a mock storage
	mockStorage := &MockStorage{}
	
	log, err := NewLog(mockStorage)
	if err != nil {
		t.Fatalf("Failed to create log: %v", err)
	}
	
	// Test appending entries
	entry1 := &Entry{
		Term:    1,
		Index:   1,
		Command: []byte("command1"),
		Type:    EntryNormal,
	}
	
	if err := log.Append(entry1); err != nil {
		t.Errorf("Failed to append entry: %v", err)
	}
	
	if log.Size() != 1 {
		t.Errorf("Expected log size 1, got %d", log.Size())
	}
	
	// Test getting entry
	retrieved, err := log.GetEntry(1)
	if err != nil {
		t.Errorf("Failed to get entry: %v", err)
	}
	
	if retrieved.Term != 1 {
		t.Errorf("Expected term 1, got %d", retrieved.Term)
	}
	
	if string(retrieved.Command) != "command1" {
		t.Errorf("Expected command 'command1', got '%s'", string(retrieved.Command))
	}
}

func TestLogGetLastIndex(t *testing.T) {
	mockStorage := &MockStorage{}
	log, err := NewLog(mockStorage)
	if err != nil {
		t.Fatalf("Failed to create log: %v", err)
	}
	
	// Empty log
	if log.GetLastIndex() != 0 {
		t.Errorf("Expected last index 0 for empty log, got %d", log.GetLastIndex())
	}
	
	// Add entries
	for i := 1; i <= 5; i++ {
		entry := &Entry{
			Term:    1,
			Index:   int64(i),
			Command: []byte("command"),
			Type:    EntryNormal,
		}
		if err := log.Append(entry); err != nil {
			t.Fatalf("Failed to append entry %d: %v", i, err)
		}
	}
	
	if log.GetLastIndex() != 5 {
		t.Errorf("Expected last index 5, got %d", log.GetLastIndex())
	}
}

func TestLogGetLastEntryInfo(t *testing.T) {
	mockStorage := &MockStorage{}
	log, err := NewLog(mockStorage)
	if err != nil {
		t.Fatalf("Failed to create log: %v", err)
	}
	
	// Empty log
	index, term := log.GetLastEntryInfo()
	if index != 0 || term != 0 {
		t.Errorf("Expected (0, 0) for empty log, got (%d, %d)", index, term)
	}
	
	// Add entries
	for i := 1; i <= 3; i++ {
		entry := &Entry{
			Term:    int64(i),
			Index:   int64(i),
			Command: []byte("command"),
			Type:    EntryNormal,
		}
		if err := log.Append(entry); err != nil {
			t.Fatalf("Failed to append entry %d: %v", i, err)
		}
	}
	
	index, term = log.GetLastEntryInfo()
	if index != 3 {
		t.Errorf("Expected last index 3, got %d", index)
	}
	if term != 3 {
		t.Errorf("Expected last term 3, got %d", term)
	}
}

func TestLogDeleteFrom(t *testing.T) {
	mockStorage := &MockStorage{}
	log, err := NewLog(mockStorage)
	if err != nil {
		t.Fatalf("Failed to create log: %v", err)
	}
	
	// Add entries
	for i := 1; i <= 5; i++ {
		entry := &Entry{
			Term:    1,
			Index:   int64(i),
			Command: []byte("command"),
			Type:    EntryNormal,
		}
		if err := log.Append(entry); err != nil {
			t.Fatalf("Failed to append entry %d: %v", i, err)
		}
	}
	
	// Delete from index 3
	if err := log.DeleteFrom(3); err != nil {
		t.Fatalf("Failed to delete from index 3: %v", err)
	}
	
	if log.Size() != 2 {
		t.Errorf("Expected log size 2 after delete, got %d", log.Size())
	}
	
	// Check that we can still get entries 1 and 2
	if _, err := log.GetEntry(1); err != nil {
		t.Errorf("Failed to get entry 1 after delete: %v", err)
	}
	if _, err := log.GetEntry(2); err != nil {
		t.Errorf("Failed to get entry 2 after delete: %v", err)
	}
	
	// Check that entry 3 is gone
	if _, err := log.GetEntry(3); err == nil {
		t.Error("Expected error when getting deleted entry 3")
	}
}

func TestLogIsEmpty(t *testing.T) {
	mockStorage := &MockStorage{}
	log, err := NewLog(mockStorage)
	if err != nil {
		t.Fatalf("Failed to create log: %v", err)
	}
	
	if !log.IsEmpty() {
		t.Error("Expected empty log to return true for IsEmpty()")
	}
	
	// Add an entry
	entry := &Entry{
		Term:    1,
		Index:   1,
		Command: []byte("command"),
		Type:    EntryNormal,
	}
	if err := log.Append(entry); err != nil {
		t.Fatalf("Failed to append entry: %v", err)
	}
	
	if log.IsEmpty() {
		t.Error("Expected non-empty log to return false for IsEmpty()")
	}
}

func TestEntryTypeString(t *testing.T) {
	tests := []struct {
		entryType EntryType
		expected  string
	}{
		{EntryNormal, "Normal"},
		{EntryConfigChange, "ConfigChange"},
		{EntrySnapshot, "Snapshot"},
		{EntryType(99), "Unknown"},
	}
	
	for _, tt := range tests {
		if got := tt.entryType.String(); got != tt.expected {
			t.Errorf("EntryType(%d).String() = %s, expected %s", tt.entryType, got, tt.expected)
		}
	}
}

// MockStorage is a mock implementation of storage.Storage for testing
type MockStorage struct{}

func (m *MockStorage) SavePersistentState(term int64, votedFor string) error {
	return nil
}

func (m *MockStorage) GetPersistentState() (int64, string, error) {
	return 0, "", nil
}

func (m *MockStorage) AppendLogEntry(entry *Entry) error {
	return nil
}

func (m *MockStorage) GetLogEntries() ([]*Entry, error) {
	return nil, nil
}

func (m *MockStorage) GetLogEntry(index int64) (*Entry, error) {
	return nil, ErrEntryNotFound
}

func (m *MockStorage) TruncateLog(index int64) error {
	return nil
}

func (m *MockStorage) SaveSnapshot(snapshot interface{}) error {
	return nil
}

func (m *MockStorage) GetLatestSnapshot() (interface{}, error) {
	return nil, nil
}

func (m *MockStorage) GetSnapshot(index int64) (interface{}, error) {
	return nil, nil
}

func (m *MockStorage) Close() error {
	return nil
}
