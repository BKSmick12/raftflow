package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/BKSmick12/raftflow/internal/config"
	"github.com/BKSmick12/raftflow/internal/consensus"
	"go.uber.org/zap"
)

// DemoNode represents a node in the demo cluster
type DemoNode struct {
	ID      string
	Address string
	Raft    *consensus.Raft
}

func main() {
	// Parse command line flags
	clusterSize := flag.Int("cluster-size", 3, "Number of nodes in the cluster")
	basePort := flag.Int("base-port", 8080, "Base port number for nodes")
	baseMetricsPort := flag.Int("base-metrics-port", 9090, "Base metrics port number for nodes")
	
	flag.Parse()
	
	// Initialize logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		fmt.Printf("Failed to create logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()
	
	// Create cluster
	nodes := make([]*DemoNode, *clusterSize)
	addresses := make([]string, *clusterSize)
	
	for i := 0; i < *clusterSize; i++ {
		address := fmt.Sprintf("localhost:%d", *basePort+i)
		addresses[i] = address
		
		// Create peers list (all other nodes)
		peers := make([]string, 0, *clusterSize-1)
		for j := 0; j < *clusterSize; j++ {
			if j != i {
				peers = append(peers, fmt.Sprintf("localhost:%d", *basePort+j))
			}
		}
		
		// Create configuration
		cfg := config.DefaultConfig(
			fmt.Sprintf("node-%d", i),
			address,
			peers,
		)
		cfg.LogDir = fmt.Sprintf("./demo-data/node-%d/log", i)
		cfg.SnapshotDir = fmt.Sprintf("./demo-data/node-%d/snapshot", i)
		cfg.EnableMetrics = true
		cfg.MetricsAddress = fmt.Sprintf("localhost:%d", *baseMetricsPort+i)
		
		// Create Raft node
		raft, err := consensus.NewRaft(cfg, logger)
		if err != nil {
			logger.Error("Failed to create Raft node",
				zap.String("node_id", fmt.Sprintf("node-%d", i)),
				zap.Error(err),
			)
			os.Exit(1)
		}
		
		// Start Raft node
		if err := raft.Start(); err != nil {
			logger.Error("Failed to start Raft node",
				zap.String("node_id", fmt.Sprintf("node-%d", i)),
				zap.Error(err),
			)
			os.Exit(1)
		}
		
		nodes[i] = &DemoNode{
			ID:      fmt.Sprintf("node-%d", i),
			Address: address,
			Raft:    raft,
		}
		
		logger.Info("Started node",
			zap.String("node_id", fmt.Sprintf("node-%d", i)),
			zap.String("address", address),
		)
	}
	
	// Start demo HTTP server for client interaction
	go startDemoServer(nodes, logger)
	
	// Print instructions
	fmt.Println("\n=== Raft Consensus Protocol Demo ===")
	fmt.Println("Cluster started with the following nodes:")
	for i, node := range nodes {
		fmt.Printf("  Node %d: %s (Metrics: localhost:%d)\n", 
			i, node.Address, *baseMetricsPort+i)
	}
	fmt.Println("\nCommands:")
	fmt.Println("  submit <command> - Submit a command to the current leader")
	fmt.Println("  state - Show cluster state")
	fmt.Println("  leader - Show current leader")
	fmt.Println("  help - Show this help message")
	fmt.Println("  exit - Exit the demo")
	fmt.Println("\nType 'help' for a list of commands.")
	
	// Start interactive console
	go startInteractiveConsole(nodes, logger)
	
	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	<-sigChan
	
	logger.Info("Shutting down demo...")
	
	// Stop all nodes
	for _, node := range nodes {
		if err := node.Raft.Stop(); err != nil {
			logger.Error("Failed to stop node",
				zap.String("node_id", node.ID),
				zap.Error(err),
			)
		}
	}
	
	logger.Info("Demo shutdown complete")
}

// startDemoServer starts an HTTP server for demo interaction
func startDemoServer(nodes []*DemoNode, logger *zap.Logger) {
	mux := http.NewServeMux()
	
	mux.HandleFunc("/submit", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		
		var request struct {
			Command string `json:"command"`
		}
		
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		
		// Find the leader
		leader := findLeader(nodes)
		if leader == nil {
			http.Error(w, "No leader available", http.StatusServiceUnavailable)
			return
		}
		
		// Submit command to leader
		if err := leader.Raft.SubmitCommand([]byte(request.Command)); err != nil {
			if err == consensus.ErrNotLeader {
				http.Error(w, "Not the leader", http.StatusServiceUnavailable)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Command submitted successfully to leader %s", leader.ID)
	})
	
	mux.HandleFunc("/state", func(w http.ResponseWriter, r *http.Request) {
		state := getClusterState(nodes)
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(state)
	})
	
	mux.HandleFunc("/leader", func(w http.ResponseWriter, r *http.Request) {
		leader := findLeader(nodes)
		if leader == nil {
			http.Error(w, "No leader available", http.StatusServiceUnavailable)
			return
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"leader": leader.ID})
	})
	
	mux.HandleFunc("/nodes", func(w http.ResponseWriter, r *http.Request) {
		nodeStates := make([]map[string]interface{}, len(nodes))
		for i, node := range nodes {
			nodeStates[i] = map[string]interface{}{
				"id":      node.ID,
				"address": node.Address,
				"state":   node.Raft.GetState().String(),
				"term":    node.Raft.GetCurrentTerm(),
				"commit_index": node.Raft.GetCommitIndex(),
				"last_applied":  node.Raft.GetLastApplied(),
			}
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(nodeStates)
	})
	
	logger.Info("Starting demo HTTP server on :8081")
	
	if err := http.ListenAndServe(":8081", mux); err != nil {
		logger.Error("Demo HTTP server error", zap.Error(err))
	}
}

// startInteractiveConsole starts an interactive console for the demo
func startInteractiveConsole(nodes []*DemoNode, logger *zap.Logger) {
	reader := bufio.NewReader(os.Stdin)
	
	for {
		fmt.Print("raft-demo> ")
		
		input, err := reader.ReadString('\n')
		if err != nil {
			logger.Error("Failed to read input", zap.Error(err))
			continue
		}
		
		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}
		
		parts := strings.SplitN(input, " ", 2)
		command := strings.ToLower(parts[0])
		
		switch command {
		case "submit", "sub":
			if len(parts) < 2 {
				fmt.Println("Usage: submit <command>")
				continue
			}
			
			leader := findLeader(nodes)
			if leader == nil {
				fmt.Println("Error: No leader available")
				continue
			}
			
			if err := leader.Raft.SubmitCommand([]byte(parts[1])); err != nil {
				if err == consensus.ErrNotLeader {
					fmt.Println("Error: Not the leader")
				} else {
					fmt.Printf("Error: %v\n", err)
				}
				continue
			}
			
			fmt.Printf("Command submitted to leader %s: %s\n", leader.ID, parts[1])
			
		case "state":
			clusterState := getClusterState(nodes)
			fmt.Printf("Cluster State:\n")
			fmt.Printf("  Leader: %s\n", clusterState.Leader)
			fmt.Printf("  Term: %d\n", clusterState.Term)
			fmt.Printf("  Nodes:\n")
			for _, nodeState := range clusterState.Nodes {
				fmt.Printf("    %s: %s (Term: %d, Commit: %d, Applied: %d)\n",
					nodeState.ID, nodeState.State, nodeState.Term, 
					nodeState.CommitIndex, nodeState.LastApplied)
			}
			
		case "leader":
			leader := findLeader(nodes)
			if leader == nil {
				fmt.Println("No leader available")
			} else {
				fmt.Printf("Current leader: %s (Term: %d)\n", 
					leader.ID, leader.Raft.GetCurrentTerm())
			}
			
		case "nodes":
			for _, node := range nodes {
				fmt.Printf("%s: %s (Term: %d, Commit: %d, Applied: %d)\n",
					node.ID, node.Raft.GetState().String(), 
					node.Raft.GetCurrentTerm(), 
					node.Raft.GetCommitIndex(), 
					node.Raft.GetLastApplied())
			}
			
		case "help":
			fmt.Println("Available commands:")
			fmt.Println("  submit <command> - Submit a command to the current leader")
			fmt.Println("  state - Show cluster state")
			fmt.Println("  leader - Show current leader")
			fmt.Println("  nodes - Show all node states")
			fmt.Println("  help - Show this help message")
			fmt.Println("  exit - Exit the demo")
			
		case "exit":
			fmt.Println("Exiting...")
			os.Exit(0)
			
		default:
			fmt.Printf("Unknown command: %s. Type 'help' for a list of commands.\n", command)
		}
	}
}

// ClusterState represents the state of the cluster
type ClusterState struct {
	Leader string         `json:"leader"`
	Term   int64          `json:"term"`
	Nodes  []NodeState    `json:"nodes"`
}

// NodeState represents the state of a single node
type NodeState struct {
	ID          string `json:"id"`
	Address     string `json:"address"`
	State       string `json:"state"`
	Term        int64  `json:"term"`
	CommitIndex int64  `json:"commit_index"`
	LastApplied int64  `json:"last_applied"`
}

// findLeader finds the current leader in the cluster
func findLeader(nodes []*DemoNode) *DemoNode {
	for _, node := range nodes {
		if node.Raft.GetState() == consensus.Leader {
			return node
		}
	}
	return nil
}

// getClusterState returns the current state of the cluster
func getClusterState(nodes []*DemoNode) *ClusterState {
	state := &ClusterState{
		Nodes: make([]NodeState, len(nodes)),
	}
	
	for i, node := range nodes {
		state.Nodes[i] = NodeState{
			ID:          node.ID,
			Address:     node.Address,
			State:       node.Raft.GetState().String(),
			Term:        node.Raft.GetCurrentTerm(),
			CommitIndex: node.Raft.GetCommitIndex(),
			LastApplied: node.Raft.GetLastApplied(),
		}
		
		// Update leader and term
		if node.Raft.GetState() == consensus.Leader {
			state.Leader = node.ID
			state.Term = node.Raft.GetCurrentTerm()
		}
	}
	
	return state
}

// startPeriodicStateCheck starts a goroutine to periodically check and display cluster state
func startPeriodicStateCheck(nodes []*DemoNode, logger *zap.Logger, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	
	for range ticker.C {
		leader := findLeader(nodes)
		if leader != nil {
			logger.Info("Cluster state",
				zap.String("leader", leader.ID),
				zap.Int64("term", leader.Raft.GetCurrentTerm()),
			)
		} else {
			logger.Info("No leader in cluster")
		}
		
		for _, node := range nodes {
			logger.Debug("Node state",
				zap.String("node_id", node.ID),
				zap.String("state", node.Raft.GetState().String()),
				zap.Int64("term", node.Raft.GetCurrentTerm()),
				zap.Int64("commit_index", node.Raft.GetCommitIndex()),
				zap.Int64("last_applied", node.Raft.GetLastApplied()),
			)
		}
	}
}
