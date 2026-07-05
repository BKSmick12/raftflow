package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/BKSmick12/raftflow/internal/config"
	"github.com/BKSmick12/raftflow/internal/consensus"
	"go.uber.org/zap"
)

func main() {
	// Parse command line flags
	nodeID := flag.String("node-id", "", "Unique identifier for this node")
	address := flag.String("address", "localhost:8080", "Network address for this node")
	peers := flag.String("peers", "", "Comma-separated list of peer addresses")
	logDir := flag.String("log-dir", "./data/log", "Directory for persistent logs")
	snapshotDir := flag.String("snapshot-dir", "./data/snapshot", "Directory for snapshots")
	enableMetrics := flag.Bool("enable-metrics", true, "Enable Prometheus metrics")
	metricsAddress := flag.String("metrics-address", ":9090", "Address for metrics server")
	
	flag.Parse()
	
	// Validate required flags
	if *nodeID == "" {
		fmt.Println("Error: --node-id is required")
		os.Exit(1)
	}
	
	// Parse peers
	var peerList []string
	if *peers != "" {
		peerList = splitAndTrim(*peers, ",")
	}
	
	// Initialize logger
	logger, err := zap.NewProduction()
	if err != nil {
		fmt.Printf("Failed to create logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()
	
	// Create configuration
	cfg := config.DefaultConfig(*nodeID, *address, peerList)
	cfg.LogDir = *logDir
	cfg.SnapshotDir = *snapshotDir
	cfg.EnableMetrics = *enableMetrics
	cfg.MetricsAddress = *metricsAddress
	
	// Validate configuration
	if err := cfg.Validate(); err != nil {
		logger.Error("Invalid configuration", zap.Error(err))
		os.Exit(1)
	}
	
	// Create and start Raft node
	raft, err := consensus.NewRaft(cfg, logger)
	if err != nil {
		logger.Error("Failed to create Raft node", zap.Error(err))
		os.Exit(1)
	}
	
	if err := raft.Start(); err != nil {
		logger.Error("Failed to start Raft node", zap.Error(err))
		os.Exit(1)
	}
	
	logger.Info("Raft node started successfully",
		zap.String("node_id", *nodeID),
		zap.String("address", *address),
		zap.Strings("peers", peerList),
	)
	
	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	<-sigChan
	
	logger.Info("Shutting down...")
	
	// Stop Raft node
	if err := raft.Stop(); err != nil {
		logger.Error("Failed to stop Raft node", zap.Error(err))
		os.Exit(1)
	}
	
	logger.Info("Shutdown complete")
}

// splitAndTrim splits a string by delimiter and trims whitespace from each element
func splitAndTrim(s, delim string) []string {
	var result []string
	for _, part := range split(s, delim) {
		trimmed := trimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// split is a simple string split function
func split(s, delim string) []string {
	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i:i+len(delim)] == delim {
			result = append(result, s[start:i])
			start = i + len(delim)
			i += len(delim) - 1
		}
	}
	result = append(result, s[start:])
	return result
}

// trimSpace trims leading and trailing whitespace
func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}
