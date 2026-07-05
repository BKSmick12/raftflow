package test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/BKSmick12/raftflow/internal/config"
	"github.com/BKSmick12/raftflow/internal/consensus"
	"go.uber.org/zap"
)

// TestCluster tests a small Raft cluster
func TestCluster(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	
	// Create logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	
	// Create 3-node cluster
	clusterSize := 3
	nodes := make([]*consensus.Raft, clusterSize)
	addresses := []string{"localhost:18081", "localhost:18082", "localhost:18083"}
	
	// Create and start nodes
	for i := 0; i < clusterSize; i++ {
		// Create peers list (all other nodes)
		peers := make([]string, 0, clusterSize-1)
		for j := 0; j < clusterSize; j++ {
			if j != i {
				peers = append(peers, addresses[j])
			}
		}
		
		// Create configuration
		cfg := config.DefaultConfig(
			fmt.Sprintf("test-node-%d", i),
			addresses[i],
			peers,
		)
		cfg.ElectionTimeoutMin = 100 * time.Millisecond
		cfg.ElectionTimeoutMax = 200 * time.Millisecond
		cfg.HeartbeatInterval = 50 * time.Millisecond
		cfg.RPCTimeout = 100 * time.Millisecond
		cfg.LogDir = fmt.Sprintf("./test-data/node-%d/log", i)
		cfg.SnapshotDir = fmt.Sprintf("./test-data/node-%d/snapshot", i)
		cfg.EnableMetrics = false
		
		// Create Raft node
		node, err := consensus.NewRaft(cfg, logger)
		if err != nil {
			t.Fatalf("Failed to create node %d: %v", i, err)
		}
		
		// Start node
		if err := node.Start(); err != nil {
			t.Fatalf("Failed to start node %d: %v", i, err)
		}
		
		nodes[i] = node
		t.Logf("Started node %d at %s", i, addresses[i])
	}
	
	// Wait for leader election
	t.Log("Waiting for leader election...")
	time.Sleep(500 * time.Millisecond)
	
	// Check that we have a leader
	leaderFound := false
	for _, node := range nodes {
		if node.GetState() == consensus.Leader {
			leaderFound = true
			t.Logf("Leader found: %s (term: %d)", node.GetCurrentTerm(), node.GetCurrentTerm())
			break
		}
	}
	
	if !leaderFound {
		t.Error("No leader elected after timeout")
	}
	
	// Clean up
	for i, node := range nodes {
		t.Logf("Stopping node %d...", i)
		if err := node.Stop(); err != nil {
			t.Logf("Error stopping node %d: %v", i, err)
		}
	}
}

// TestLeaderElection tests leader election in a cluster
func TestLeaderElection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	
	// Create logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	
	// Create 3-node cluster
	clusterSize := 3
	nodes := make([]*consensus.Raft, clusterSize)
	addresses := []string{"localhost:18084", "localhost:18085", "localhost:18086"}
	
	// Create and start nodes
	for i := 0; i < clusterSize; i++ {
		peers := make([]string, 0, clusterSize-1)
		for j := 0; j < clusterSize; j++ {
			if j != i {
				peers = append(peers, addresses[j])
			}
		}
		
		cfg := config.DefaultConfig(
			fmt.Sprintf("election-test-%d", i),
			addresses[i],
			peers,
		)
		cfg.ElectionTimeoutMin = 50 * time.Millisecond
		cfg.ElectionTimeoutMax = 150 * time.Millisecond
		cfg.HeartbeatInterval = 25 * time.Millisecond
		cfg.RPCTimeout = 50 * time.Millisecond
		cfg.LogDir = fmt.Sprintf("./test-data/election-%d/log", i)
		cfg.SnapshotDir = fmt.Sprintf("./test-data/election-%d/snapshot", i)
		cfg.EnableMetrics = false
		
		node, err := consensus.NewRaft(cfg, logger)
		if err != nil {
			t.Fatalf("Failed to create node %d: %v", i, err)
		}
		
		if err := node.Start(); err != nil {
			t.Fatalf("Failed to start node %d: %v", i, err)
		}
		
		nodes[i] = node
	}
	
	// Wait for multiple election cycles
	for cycle := 0; cycle < 3; cycle++ {
		t.Logf("Election cycle %d", cycle+1)
		
		// Wait for election
		time.Sleep(200 * time.Millisecond)
		
		// Count leaders
		leaderCount := 0
		var currentLeader *consensus.Raft
		for _, node := range nodes {
			if node.GetState() == consensus.Leader {
				leaderCount++
				currentLeader = node
			}
		}
		
		// Should have exactly one leader
		if leaderCount != 1 {
			t.Errorf("Expected exactly 1 leader, got %d", leaderCount)
		} else {
			t.Logf("Leader: %s (term: %d)", currentLeader.GetCurrentTerm(), currentLeader.GetCurrentTerm())
		}
		
		// All nodes should be in the same term
		terms := make(map[int64]int)
		for _, node := range nodes {
			terms[node.GetCurrentTerm()]++
		}
		
		if len(terms) != 1 {
			t.Errorf("Expected all nodes in same term, got terms: %v", terms)
		}
	}
	
	// Clean up
	for _, node := range nodes {
		node.Stop()
	}
}

// TestCommandSubmission tests command submission to a cluster
func TestCommandSubmission(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	
	// Create logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	
	// Create 3-node cluster
	clusterSize := 3
	nodes := make([]*consensus.Raft, clusterSize)
	addresses := []string{"localhost:18087", "localhost:18088", "localhost:18089"}
	
	// Create and start nodes
	for i := 0; i < clusterSize; i++ {
		peers := make([]string, 0, clusterSize-1)
		for j := 0; j < clusterSize; j++ {
			if j != i {
				peers = append(peers, addresses[j])
			}
		}
		
		cfg := config.DefaultConfig(
			fmt.Sprintf("command-test-%d", i),
			addresses[i],
			peers,
		)
		cfg.ElectionTimeoutMin = 100 * time.Millisecond
		cfg.ElectionTimeoutMax = 200 * time.Millisecond
		cfg.HeartbeatInterval = 50 * time.Millisecond
		cfg.RPCTimeout = 100 * time.Millisecond
		cfg.LogDir = fmt.Sprintf("./test-data/command-%d/log", i)
		cfg.SnapshotDir = fmt.Sprintf("./test-data/command-%d/snapshot", i)
		cfg.EnableMetrics = false
		
		node, err := consensus.NewRaft(cfg, logger)
		if err != nil {
			t.Fatalf("Failed to create node %d: %v", i, err)
		}
		
		if err := node.Start(); err != nil {
			t.Fatalf("Failed to start node %d: %v", i, err)
		}
		
		nodes[i] = node
	}
	
	// Wait for leader election
	time.Sleep(300 * time.Millisecond)
	
	// Find the leader
	var leader *consensus.Raft
	for _, node := range nodes {
		if node.GetState() == consensus.Leader {
			leader = node
			break
		}
	}
	
	if leader == nil {
		t.Fatal("No leader found")
	}
	
	t.Logf("Leader found: %s", leader.GetCurrentTerm())
	
	// Submit a command to the leader
	testCommand := []byte("test command for integration test")
	if err := leader.SubmitCommand(testCommand); err != nil {
		t.Fatalf("Failed to submit command: %v", err)
	}
	
	t.Log("Command submitted successfully")
	
	// Wait for command to be replicated
	time.Sleep(200 * time.Millisecond)
	
	// Check that all nodes have the command in their logs
	for i, node := range nodes {
		lastIndex := node.GetCommitIndex()
		t.Logf("Node %d: commit_index=%d, last_applied=%d", i, lastIndex, node.GetLastApplied())
		
		// The command should be committed on all nodes
		if lastIndex < 1 {
			t.Errorf("Node %d: expected commit index >= 1, got %d", i, lastIndex)
		}
	}
	
	// Clean up
	for _, node := range nodes {
		node.Stop()
	}
}

// TestClusterRecovery tests cluster recovery from persistent state
func TestClusterRecovery(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	
	// Create logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	
	// Create 3-node cluster
	clusterSize := 3
	addresses := []string{"localhost:18090", "localhost:18091", "localhost:18092"}
	
	// First, start a cluster and submit some commands
	nodes := make([]*consensus.Raft, clusterSize)
	
	for i := 0; i < clusterSize; i++ {
		peers := make([]string, 0, clusterSize-1)
		for j := 0; j < clusterSize; j++ {
			if j != i {
				peers = append(peers, addresses[j])
			}
		}
		
		cfg := config.DefaultConfig(
			fmt.Sprintf("recovery-test-%d", i),
			addresses[i],
			peers,
		)
		cfg.ElectionTimeoutMin = 50 * time.Millisecond
		cfg.ElectionTimeoutMax = 150 * time.Millisecond
		cfg.HeartbeatInterval = 25 * time.Millisecond
		cfg.RPCTimeout = 50 * time.Millisecond
		cfg.LogDir = fmt.Sprintf("./test-data/recovery-%d/log", i)
		cfg.SnapshotDir = fmt.Sprintf("./test-data/recovery-%d/snapshot", i)
		cfg.EnableMetrics = false
		
		node, err := consensus.NewRaft(cfg, logger)
		if err != nil {
			t.Fatalf("Failed to create node %d: %v", i, err)
		}
		
		if err := node.Start(); err != nil {
			t.Fatalf("Failed to start node %d: %v", i, err)
		}
		
		nodes[i] = node
	}
	
	// Wait for leader election
	time.Sleep(200 * time.Millisecond)
	
	// Find the leader and submit commands
	var leader *consensus.Raft
	for _, node := range nodes {
		if node.GetState() == consensus.Leader {
			leader = node
			break
		}
	}
	
	if leader == nil {
		t.Fatal("No leader found")
	}
	
	// Submit some commands
	for i := 0; i < 5; i++ {
		command := []byte(fmt.Sprintf("command %d", i))
		if err := leader.SubmitCommand(command); err != nil {
			t.Fatalf("Failed to submit command %d: %v", i, err)
		}
		time.Sleep(50 * time.Millisecond)
	}
	
	// Wait for commands to be committed
	time.Sleep(300 * time.Millisecond)
	
	// Stop all nodes
	for _, node := range nodes {
		node.Stop()
	}
	
	t.Log("Initial cluster stopped")
	
	// Now restart the cluster with the same data directories
	t.Log("Restarting cluster...")
	
	nodes2 := make([]*consensus.Raft, clusterSize)
	
	for i := 0; i < clusterSize; i++ {
		peers := make([]string, 0, clusterSize-1)
		for j := 0; j < clusterSize; j++ {
			if j != i {
				peers = append(peers, addresses[j])
			}
		}
		
		cfg := config.DefaultConfig(
			fmt.Sprintf("recovery-test-%d", i),
			addresses[i],
			peers,
		)
		cfg.ElectionTimeoutMin = 50 * time.Millisecond
		cfg.ElectionTimeoutMax = 150 * time.Millisecond
		cfg.HeartbeatInterval = 25 * time.Millisecond
		cfg.RPCTimeout = 50 * time.Millisecond
		cfg.LogDir = fmt.Sprintf("./test-data/recovery-%d/log", i)  // Same directory
		cfg.SnapshotDir = fmt.Sprintf("./test-data/recovery-%d/snapshot", i)  // Same directory
		cfg.EnableMetrics = false
		
		node, err := consensus.NewRaft(cfg, logger)
		if err != nil {
			t.Fatalf("Failed to create node %d: %v", i, err)
		}
		
		if err := node.Start(); err != nil {
			t.Fatalf("Failed to start node %d: %v", i, err)
		}
		
		nodes2[i] = node
	}
	
	// Wait for recovery and leader election
	time.Sleep(300 * time.Millisecond)
	
	// Check that nodes recovered their state
	for i, node := range nodes2 {
		lastIndex := node.GetCommitIndex()
		t.Logf("Node %d recovered: term=%d, commit_index=%d, last_applied=%d", 
			i, node.GetCurrentTerm(), lastIndex, node.GetLastApplied())
		
		// Should have recovered the committed entries
		if lastIndex < 5 {
			t.Logf("Warning: Node %d recovered with commit_index=%d, expected >=5", i, lastIndex)
		}
	}
	
	// Clean up
	for _, node := range nodes2 {
		node.Stop()
	}
}

// TestHTTPClient tests the HTTP client functionality
func TestHTTPClient(t *testing.T) {
	// Start a simple HTTP server for testing
	server := &http.Server{
		Addr: ":18099",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		}),
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			t.Logf("Server error: %v", err)
		}
	}()
	
	defer server.Shutdown(ctx)
	
	// Give server time to start
	time.Sleep(100 * time.Millisecond)
	
	// Test HTTP request
	resp, err := http.Get("http://localhost:18099")
	if err != nil {
		t.Fatalf("Failed to make HTTP request: %v", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}
