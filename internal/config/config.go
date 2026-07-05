package config

import (
	"time"
)

// Config holds the configuration for a Raft node
type Config struct {
	// NodeID is the unique identifier for this node
	NodeID string
	
	// ClusterID is the identifier for the cluster
	ClusterID string
	
	// Address is the network address for this node
	Address string
	
	// Peers is the list of other nodes in the cluster
	Peers []string
	
	// ElectionTimeoutMin is the minimum election timeout
	ElectionTimeoutMin time.Duration
	
	// ElectionTimeoutMax is the maximum election timeout
	ElectionTimeoutMax time.Duration
	
	// HeartbeatInterval is how often the leader sends heartbeats
	HeartbeatInterval time.Duration
	
	// RPCTimeout is the timeout for RPC calls
	RPCTimeout time.Duration
	
	// SnapshotInterval is how often to create snapshots
	SnapshotInterval time.Duration
	
	// SnapshotThreshold is the minimum number of entries before snapshotting
	SnapshotThreshold int
	
	// LogDir is the directory for persistent logs
	LogDir string
	
	// SnapshotDir is the directory for snapshots
	SnapshotDir string
	
	// EnableMetrics enables Prometheus metrics
	EnableMetrics bool
	
	// MetricsAddress is the address for the metrics server
	MetricsAddress string
}

// DefaultConfig returns a default configuration
func DefaultConfig(nodeID, address string, peers []string) *Config {
	return &Config{
		NodeID:             nodeID,
		ClusterID:          "raft-cluster",
		Address:            address,
		Peers:              peers,
		ElectionTimeoutMin: 1500 * time.Millisecond,
		ElectionTimeoutMax: 3000 * time.Millisecond,
		HeartbeatInterval:  500 * time.Millisecond,
		RPCTimeout:         1000 * time.Millisecond,
		SnapshotInterval:   30 * time.Second,
		SnapshotThreshold:  1000,
		LogDir:             "./data/log",
		SnapshotDir:        "./data/snapshot",
		EnableMetrics:      true,
		MetricsAddress:     ":9090",
	}
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.NodeID == "" {
		return ErrEmptyNodeID
	}
	if c.Address == "" {
		return ErrEmptyAddress
	}
	if c.ElectionTimeoutMin <= 0 {
		return ErrInvalidElectionTimeout
	}
	if c.ElectionTimeoutMax < c.ElectionTimeoutMin {
		return ErrInvalidElectionTimeoutRange
	}
	if c.HeartbeatInterval <= 0 {
		return ErrInvalidHeartbeatInterval
	}
	if c.HeartbeatInterval >= c.ElectionTimeoutMin {
		return ErrHeartbeatTooLong
	}
	return nil
}

// ErrEmptyNodeID is returned when NodeID is empty
var ErrEmptyNodeID = &ConfigError{"node ID cannot be empty"}

// ErrEmptyAddress is returned when Address is empty
var ErrEmptyAddress = &ConfigError{"address cannot be empty"}

// ErrInvalidElectionTimeout is returned when election timeout is invalid
var ErrInvalidElectionTimeout = &ConfigError{"election timeout must be positive"}

// ErrInvalidElectionTimeoutRange is returned when max timeout < min timeout
var ErrInvalidElectionTimeoutRange = &ConfigError{"max election timeout must be >= min election timeout"}

// ErrInvalidHeartbeatInterval is returned when heartbeat interval is invalid
var ErrInvalidHeartbeatInterval = &ConfigError{"heartbeat interval must be positive"}

// ErrHeartbeatTooLong is returned when heartbeat interval is too long
var ErrHeartbeatTooLong = &ConfigError{"heartbeat interval must be less than election timeout"}

// ConfigError represents a configuration error
type ConfigError struct {
	Message string
}

func (e *ConfigError) Error() string {
	return e.Message
}
