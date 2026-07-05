package config

import (
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	peers := []string{"peer1:8080", "peer2:8080"}
	cfg := DefaultConfig("node1", "localhost:8080", peers)
	
	if cfg.NodeID != "node1" {
		t.Errorf("Expected NodeID 'node1', got '%s'", cfg.NodeID)
	}
	
	if cfg.Address != "localhost:8080" {
		t.Errorf("Expected Address 'localhost:8080', got '%s'", cfg.Address)
	}
	
	if len(cfg.Peers) != 2 {
		t.Errorf("Expected 2 peers, got %d", len(cfg.Peers))
	}
	
	if cfg.ElectionTimeoutMin != 1500*time.Millisecond {
		t.Errorf("Expected ElectionTimeoutMin 1500ms, got %v", cfg.ElectionTimeoutMin)
	}
	
	if cfg.ElectionTimeoutMax != 3000*time.Millisecond {
		t.Errorf("Expected ElectionTimeoutMax 3000ms, got %v", cfg.ElectionTimeoutMax)
	}
	
	if cfg.HeartbeatInterval != 500*time.Millisecond {
		t.Errorf("Expected HeartbeatInterval 500ms, got %v", cfg.HeartbeatInterval)
	}
	
	if cfg.ClusterID != "raft-cluster" {
		t.Errorf("Expected ClusterID 'raft-cluster', got '%s'", cfg.ClusterID)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: &Config{
				NodeID:             "node1",
				Address:            "localhost:8080",
				ElectionTimeoutMin: 1500 * time.Millisecond,
				ElectionTimeoutMax: 3000 * time.Millisecond,
				HeartbeatInterval:  500 * time.Millisecond,
			},
			wantErr: false,
		},
		{
			name: "empty node ID",
			config: &Config{
				NodeID:             "",
				Address:            "localhost:8080",
				ElectionTimeoutMin: 1500 * time.Millisecond,
				ElectionTimeoutMax: 3000 * time.Millisecond,
				HeartbeatInterval:  500 * time.Millisecond,
			},
			wantErr: true,
		},
		{
			name: "empty address",
			config: &Config{
				NodeID:             "node1",
				Address:            "",
				ElectionTimeoutMin: 1500 * time.Millisecond,
				ElectionTimeoutMax: 3000 * time.Millisecond,
				HeartbeatInterval:  500 * time.Millisecond,
			},
			wantErr: true,
		},
		{
			name: "invalid election timeout",
			config: &Config{
				NodeID:             "node1",
				Address:            "localhost:8080",
				ElectionTimeoutMin: 0,
				ElectionTimeoutMax: 3000 * time.Millisecond,
				HeartbeatInterval:  500 * time.Millisecond,
			},
			wantErr: true,
		},
		{
			name: "max timeout less than min",
			config: &Config{
				NodeID:             "node1",
				Address:            "localhost:8080",
				ElectionTimeoutMin: 3000 * time.Millisecond,
				ElectionTimeoutMax: 1500 * time.Millisecond,
				HeartbeatInterval:  500 * time.Millisecond,
			},
			wantErr: true,
		},
		{
			name: "invalid heartbeat interval",
			config: &Config{
				NodeID:             "node1",
				Address:            "localhost:8080",
				ElectionTimeoutMin: 1500 * time.Millisecond,
				ElectionTimeoutMax: 3000 * time.Millisecond,
				HeartbeatInterval:  0,
			},
			wantErr: true,
		},
		{
			name: "heartbeat too long",
			config: &Config{
				NodeID:             "node1",
				Address:            "localhost:8080",
				ElectionTimeoutMin: 1500 * time.Millisecond,
				ElectionTimeoutMax: 3000 * time.Millisecond,
				HeartbeatInterval:  2000 * time.Millisecond,
			},
			wantErr: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConfigError(t *testing.T) {
	err := ErrEmptyNodeID
	if err.Error() != "node ID cannot be empty" {
		t.Errorf("Expected error message 'node ID cannot be empty', got '%s'", err.Error())
	}
}
