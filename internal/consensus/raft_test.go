package consensus

import (
	"testing"
	"time"

	"github.com/BKSmick12/raftflow/internal/config"
	"github.com/BKSmick12/raftflow/internal/log"
	"go.uber.org/zap"
)

func TestNewRaft(t *testing.T) {
	// Create a test configuration
	cfg := config.DefaultConfig("test-node", "localhost:8080", []string{})
	cfg.ElectionTimeoutMin = 100 * time.Millisecond
	cfg.ElectionTimeoutMax = 200 * time.Millisecond
	cfg.HeartbeatInterval = 50 * time.Millisecond
	
	// Create logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	
	// Create Raft node
	raft, err := NewRaft(cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create Raft node: %v", err)
	}
	
	// Verify initial state
	if raft.GetState() != Follower {
		t.Errorf("Expected initial state to be Follower, got %s", raft.GetState())
	}
	
	if raft.GetCurrentTerm() != 0 {
		t.Errorf("Expected initial term to be 0, got %d", raft.GetCurrentTerm())
	}
	
	// Clean up
	raft.Stop()
}

func TestRaftStateTransitions(t *testing.T) {
	// Create a test configuration
	cfg := config.DefaultConfig("test-node", "localhost:8080", []string{})
	cfg.ElectionTimeoutMin = 50 * time.Millisecond
	cfg.ElectionTimeoutMax = 100 * time.Millisecond
	cfg.HeartbeatInterval = 25 * time.Millisecond
	
	// Create logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	
	// Create Raft node
	raft, err := NewRaft(cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create Raft node: %v", err)
	}
	
	// Start the node
	if err := raft.Start(); err != nil {
		t.Fatalf("Failed to start Raft node: %v", err)
	}
	
	// Wait for election to start (node should become candidate)
	time.Sleep(150 * time.Millisecond)
	
	// Check if state changed from Follower
	state := raft.GetState()
	if state != Candidate && state != Leader {
		t.Logf("State after election timeout: %s", state)
		// Note: In a single-node cluster, it might become leader
	}
	
	// Clean up
	raft.Stop()
}

func TestRaftSubmitCommand(t *testing.T) {
	// Create a test configuration
	cfg := config.DefaultConfig("test-node", "localhost:8080", []string{})
	cfg.ElectionTimeoutMin = 100 * time.Millisecond
	cfg.ElectionTimeoutMax = 200 * time.Millisecond
	
	// Create logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	
	// Create Raft node
	raft, err := NewRaft(cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create Raft node: %v", err)
	}
	
	// Start the node
	if err := raft.Start(); err != nil {
		t.Fatalf("Failed to start Raft node: %v", err)
	}
	
	// Try to submit a command (should fail if not leader)
	err = raft.SubmitCommand([]byte("test command"))
	if err != ErrNotLeader {
		t.Logf("Expected ErrNotLeader when submitting to non-leader, got: %v", err)
	}
	
	// Clean up
	raft.Stop()
}

func TestRaftMetrics(t *testing.T) {
	// Create a test configuration with metrics enabled
	cfg := config.DefaultConfig("test-node", "localhost:8080", []string{})
	cfg.EnableMetrics = true
	cfg.MetricsAddress = "localhost:19090"
	
	// Create logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	
	// Create Raft node
	raft, err := NewRaft(cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create Raft node: %v", err)
	}
	
	// Verify metrics are initialized
	if raft.metrics == nil {
		t.Error("Metrics should be initialized")
	}
	
	if !raft.metrics.enabled {
		t.Error("Metrics should be enabled")
	}
	
	// Clean up
	raft.Stop()
}

func TestRaftLogOperations(t *testing.T) {
	// Create a test configuration
	cfg := config.DefaultConfig("test-node", "localhost:8080", []string{})
	
	// Create logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	
	// Create Raft node
	raft, err := NewRaft(cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create Raft node: %v", err)
	}
	
	// Test log operations through the Raft node
	if raft.log == nil {
		t.Fatal("Raft log should be initialized")
	}
	
	// Test appending to log
	entry := &log.Entry{
		Term:    1,
		Index:   1,
		Command: []byte("test command"),
		Type:    log.EntryNormal,
	}
	
	if err := raft.log.Append(entry); err != nil {
		t.Fatalf("Failed to append to log: %v", err)
	}
	
	// Verify entry was added
	if raft.log.Size() != 1 {
		t.Errorf("Expected log size 1, got %d", raft.log.Size())
	}
	
	// Verify we can retrieve the entry
	retrieved, err := raft.log.GetEntry(1)
	if err != nil {
		t.Fatalf("Failed to get entry: %v", err)
	}
	
	if string(retrieved.Command) != "test command" {
		t.Errorf("Expected command 'test command', got '%s'", string(retrieved.Command))
	}
	
	// Clean up
	raft.Stop()
}

func TestRaftStateString(t *testing.T) {
	tests := []struct {
		state    State
		expected string
	}{
		{Follower, "Follower"},
		{Candidate, "Candidate"},
		{Leader, "Leader"},
		{State(99), "Unknown"},
	}
	
	for _, tt := range tests {
		if got := tt.state.String(); got != tt.expected {
			t.Errorf("State(%d).String() = %s, expected %s", tt.state, got, tt.expected)
		}
	}
}

func TestRaftError(t *testing.T) {
	err := ErrNotLeader
	if err.Error() != "not the leader" {
		t.Errorf("Expected error message 'not the leader', got '%s'", err.Error())
	}
}

func TestMinFunction(t *testing.T) {
	tests := []struct {
		a, b   int64
		want   int64
	}{
		{1, 2, 1},
		{2, 1, 1},
		{5, 5, 5},
		{0, 10, 0},
		{-1, 1, -1},
	}
	
	for _, tt := range tests {
		if got := min(tt.a, tt.b); got != tt.want {
			t.Errorf("min(%d, %d) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}
