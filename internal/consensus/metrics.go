package consensus

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

// Metrics holds Prometheus metrics for Raft
type Metrics struct {
	enabled       bool
	clusterID     string
	nodeID        string
	server        *http.Server
	logger        *zap.Logger
	
	// Raft state metrics
	currentTerm   prometheus.Gauge
	currentState  *prometheus.GaugeVec
	
	// Election metrics
	electionsStarted prometheus.Counter
	electionsWon     prometheus.Counter
	electionsLost    prometheus.Counter
	
	// Vote metrics
	votesReceived prometheus.Counter
	votesGranted  prometheus.Counter
	
	// RPC metrics
	rpcRequestsReceived *prometheus.CounterVec
	rpcRequestsSent     *prometheus.CounterVec
	rpcFailures         *prometheus.CounterVec
	
	// Log metrics
	logEntriesAppended prometheus.Counter
	logEntriesCommitted prometheus.Counter
	logEntriesApplied prometheus.Counter
	
	// Heartbeat metrics
	heartbeatsSent prometheus.Counter
	heartbeatsReceived prometheus.Counter
	
	// Client metrics
	clientRequestsReceived prometheus.Counter
	clientRequestsSuccess  prometheus.Counter
	clientRequestsFailed   prometheus.Counter
	
	// Snapshot metrics
	snapshotsCreated prometheus.Counter
	snapshotsLoaded   prometheus.Counter
	
	// Timing metrics
	rpcLatency *prometheus.HistogramVec
	
	// Leader metrics
	isLeader prometheus.Gauge
}

// NewMetrics creates a new Metrics instance
func NewMetrics(enabled bool, clusterID, nodeID string) *Metrics {
	m := &Metrics{
		enabled:   enabled,
		clusterID: clusterID,
		nodeID:    nodeID,
		logger:    zap.L().Named("metrics"),
	}
	
	if !enabled {
		return m
	}
	
	// Raft state metrics
	m.currentTerm = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "raft",
		Subsystem: "state",
		Name:      "current_term",
		Help:      "Current term of the Raft node",
		ConstLabels: prometheus.Labels{
			"cluster_id": clusterID,
			"node_id":    nodeID,
		},
	})
	
	m.currentState = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "raft",
		Subsystem: "state",
		Name:      "current_state",
		Help:      "Current state of the Raft node (0=Follower, 1=Candidate, 2=Leader)",
		ConstLabels: prometheus.Labels{
			"cluster_id": clusterID,
			"node_id":    nodeID,
		},
	}, []string{"state"})
	
	// Election metrics
	m.electionsStarted = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "raft",
		Subsystem: "election",
		Name:      "started_total",
		Help:      "Total number of elections started",
		ConstLabels: prometheus.Labels{
			"cluster_id": clusterID,
			"node_id":    nodeID,
		},
	})
	
	m.electionsWon = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "raft",
		Subsystem: "election",
		Name:      "won_total",
		Help:      "Total number of elections won",
		ConstLabels: prometheus.Labels{
			"cluster_id": clusterID,
			"node_id":    nodeID,
		},
	})
	
	m.electionsLost = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "raft",
		Subsystem: "election",
		Name:      "lost_total",
		Help:      "Total number of elections lost",
		ConstLabels: prometheus.Labels{
			"cluster_id": clusterID,
			"node_id":    nodeID,
		},
	})
	
	// Vote metrics
	m.votesReceived = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "raft",
		Subsystem: "vote",
		Name:      "received_total",
		Help:      "Total number of votes received",
		ConstLabels: prometheus.Labels{
			"cluster_id": clusterID,
			"node_id":    nodeID,
		},
	})
	
	m.votesGranted = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "raft",
		Subsystem: "vote",
		Name:      "granted_total",
		Help:      "Total number of votes granted",
		ConstLabels: prometheus.Labels{
			"cluster_id": clusterID,
			"node_id":    nodeID,
		},
	})
	
	// RPC metrics
	m.rpcRequestsReceived = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "raft",
		Subsystem: "rpc",
		Name:      "requests_received_total",
		Help:      "Total number of RPC requests received",
		ConstLabels: prometheus.Labels{
			"cluster_id": clusterID,
			"node_id":    nodeID,
		},
	}, []string{"type"})
	
	m.rpcRequestsSent = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "raft",
		Subsystem: "rpc",
		Name:      "requests_sent_total",
		Help:      "Total number of RPC requests sent",
		ConstLabels: prometheus.Labels{
			"cluster_id": clusterID,
			"node_id":    nodeID,
		},
	}, []string{"type"})
	
	m.rpcFailures = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "raft",
		Subsystem: "rpc",
		Name:      "failures_total",
		Help:      "Total number of RPC failures",
		ConstLabels: prometheus.Labels{
			"cluster_id": clusterID,
			"node_id":    nodeID,
		},
	}, []string{"type"})
	
	// Log metrics
	m.logEntriesAppended = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "raft",
		Subsystem: "log",
		Name:      "entries_appended_total",
		Help:      "Total number of log entries appended",
		ConstLabels: prometheus.Labels{
			"cluster_id": clusterID,
			"node_id":    nodeID,
		},
	})
	
	m.logEntriesCommitted = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "raft",
		Subsystem: "log",
		Name:      "entries_committed_total",
		Help:      "Total number of log entries committed",
		ConstLabels: prometheus.Labels{
			"cluster_id": clusterID,
			"node_id":    nodeID,
		},
	})
	
	m.logEntriesApplied = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "raft",
		Subsystem: "log",
		Name:      "entries_applied_total",
		Help:      "Total number of log entries applied",
		ConstLabels: prometheus.Labels{
			"cluster_id": clusterID,
			"node_id":    nodeID,
		},
	})
	
	// Heartbeat metrics
	m.heartbeatsSent = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "raft",
		Subsystem: "heartbeat",
		Name:      "sent_total",
		Help:      "Total number of heartbeats sent",
		ConstLabels: prometheus.Labels{
			"cluster_id": clusterID,
			"node_id":    nodeID,
		},
	})
	
	m.heartbeatsReceived = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "raft",
		Subsystem: "heartbeat",
		Name:      "received_total",
		Help:      "Total number of heartbeats received",
		ConstLabels: prometheus.Labels{
			"cluster_id": clusterID,
			"node_id":    nodeID,
		},
	})
	
	// Client metrics
	m.clientRequestsReceived = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "raft",
		Subsystem: "client",
		Name:      "requests_received_total",
		Help:      "Total number of client requests received",
		ConstLabels: prometheus.Labels{
			"cluster_id": clusterID,
			"node_id":    nodeID,
		},
	})
	
	m.clientRequestsSuccess = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "raft",
		Subsystem: "client",
		Name:      "requests_success_total",
		Help:      "Total number of successful client requests",
		ConstLabels: prometheus.Labels{
			"cluster_id": clusterID,
			"node_id":    nodeID,
		},
	})
	
	m.clientRequestsFailed = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "raft",
		Subsystem: "client",
		Name:      "requests_failed_total",
		Help:      "Total number of failed client requests",
		ConstLabels: prometheus.Labels{
			"cluster_id": clusterID,
			"node_id":    nodeID,
		},
	})
	
	// Snapshot metrics
	m.snapshotsCreated = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "raft",
		Subsystem: "snapshot",
		Name:      "created_total",
		Help:      "Total number of snapshots created",
		ConstLabels: prometheus.Labels{
			"cluster_id": clusterID,
			"node_id":    nodeID,
		},
	})
	
	m.snapshotsLoaded = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "raft",
		Subsystem: "snapshot",
		Name:      "loaded_total",
		Help:      "Total number of snapshots loaded",
		ConstLabels: prometheus.Labels{
			"cluster_id": clusterID,
			"node_id":    nodeID,
		},
	})
	
	// Timing metrics
	m.rpcLatency = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "raft",
		Subsystem: "rpc",
		Name:      "latency_seconds",
		Help:      "RPC latency in seconds",
		ConstLabels: prometheus.Labels{
			"cluster_id": clusterID,
			"node_id":    nodeID,
		},
		Buckets: prometheus.ExponentialBuckets(0.001, 2, 10), // 1ms to ~1s
	}, []string{"type"})
	
	// Leader metrics
	m.isLeader = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "raft",
		Subsystem: "leader",
		Name:      "is_leader",
		Help:      "1 if this node is the leader, 0 otherwise",
		ConstLabels: prometheus.Labels{
			"cluster_id": clusterID,
			"node_id":    nodeID,
		},
	})
	
	// Register all metrics
	prometheus.MustRegister(
		m.currentTerm,
		m.currentState,
		m.electionsStarted,
		m.electionsWon,
		m.electionsLost,
		m.votesReceived,
		m.votesGranted,
		m.rpcRequestsReceived,
		m.rpcRequestsSent,
		m.rpcFailures,
		m.logEntriesAppended,
		m.logEntriesCommitted,
		m.logEntriesApplied,
		m.heartbeatsSent,
		m.heartbeatsReceived,
		m.clientRequestsReceived,
		m.clientRequestsSuccess,
		m.clientRequestsFailed,
		m.snapshotsCreated,
		m.snapshotsLoaded,
		m.rpcLatency,
		m.isLeader,
	)
	
	return m
}

// Start starts the metrics server
func (m *Metrics) Start(address string) {
	if !m.enabled {
		return
	}
	
	m.server = &http.Server{
		Addr:    address,
		Handler: promhttp.Handler(),
	}
	
	m.logger.Info("Starting metrics server", zap.String("address", address))
	
	go func() {
		if err := m.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			m.logger.Error("Metrics server error", zap.Error(err))
		}
	}()
}

// Stop stops the metrics server
func (m *Metrics) Stop() {
	if m.server != nil {
		m.logger.Info("Stopping metrics server")
		m.server.Close()
	}
}

// UpdateState updates the current state metric
func (m *Metrics) UpdateState(state State) {
	if !m.enabled {
		return
	}
	
	m.currentState.WithLabelValues(state.String()).Set(1)
	
	// Update leader metric
	if state == Leader {
		m.isLeader.Set(1)
	} else {
		m.isLeader.Set(0)
	}
}

// UpdateTerm updates the current term metric
func (m *Metrics) UpdateTerm(term int64) {
	if !m.enabled {
		return
	}
	
	m.currentTerm.Set(float64(term))
}

// ElectionStarted increments the elections started counter
func (m *Metrics) ElectionStarted() {
	if !m.enabled {
		return
	}
	
	m.electionsStarted.Inc()
}

// BecameLeader increments the elections won counter and updates state
func (m *Metrics) BecameLeader() {
	if !m.enabled {
		return
	}
	
	m.electionsWon.Inc()
	m.UpdateState(Leader)
}

// ElectionLost increments the elections lost counter
func (m *Metrics) ElectionLost() {
	if !m.enabled {
		return
	}
	
	m.electionsLost.Inc()
}

// VoteReceived increments the votes received counter
func (m *Metrics) VoteReceived() {
	if !m.enabled {
		return
	}
	
	m.votesReceived.Inc()
}

// VoteGranted increments the votes granted counter
func (m *Metrics) VoteGranted() {
	if !m.enabled {
		return
	}
	
	m.votesGranted.Inc()
}

// RPCRequestReceived increments the RPC requests received counter
func (m *Metrics) RPCRequestReceived(rpcType string) {
	if !m.enabled {
		return
	}
	
	m.rpcRequestsReceived.WithLabelValues(rpcType).Inc()
}

// RPCRequestSent increments the RPC requests sent counter
func (m *Metrics) RPCRequestSent(rpcType string) {
	if !m.enabled {
		return
	}
	
	m.rpcRequestsSent.WithLabelValues(rpcType).Inc()
}

// RPCFailure increments the RPC failures counter
func (m *Metrics) RPCFailure(rpcType string) {
	if !m.enabled {
		return
	}
	
	m.rpcFailures.WithLabelValues(rpcType).Inc()
}

// RPCLatency records the latency of an RPC
func (m *Metrics) RPCLatency(rpcType string, duration time.Duration) {
	if !m.enabled {
		return
	}
	
	m.rpcLatency.WithLabelValues(rpcType).Observe(duration.Seconds())
}

// AppendEntriesReceived increments the append entries received counter
func (m *Metrics) AppendEntriesReceived(count int) {
	if !m.enabled {
		return
	}
	
	m.logEntriesAppended.Add(float64(count))
}

// EntriesCommitted increments the log entries committed counter
func (m *Metrics) EntriesCommitted(count int) {
	if !m.enabled {
		return
	}
	
	m.logEntriesCommitted.Add(float64(count))
}

// CommandApplied increments the log entries applied counter
func (m *Metrics) CommandApplied() {
	if !m.enabled {
		return
	}
	
	m.logEntriesApplied.Inc()
}

// HeartbeatsSent increments the heartbeats sent counter
func (m *Metrics) HeartbeatsSent(count int) {
	if !m.enabled {
		return
	}
	
	m.heartbeatsSent.Add(float64(count))
}

// HeartbeatsReceived increments the heartbeats received counter
func (m *Metrics) HeartbeatsReceived() {
	if !m.enabled {
		return
	}
	
	m.heartbeatsReceived.Inc()
}

// ClientRequestReceived increments the client requests received counter
func (m *Metrics) ClientRequestReceived() {
	if !m.enabled {
		return
	}
	
	m.clientRequestsReceived.Inc()
}

// ClientRequestSuccess increments the client requests success counter
func (m *Metrics) ClientRequestSuccess() {
	if !m.enabled {
		return
	}
	
	m.clientRequestsSuccess.Inc()
}

// ClientRequestFailed increments the client requests failed counter
func (m *Metrics) ClientRequestFailed() {
	if !m.enabled {
		return
	}
	
	m.clientRequestsFailed.Inc()
}

// SnapshotCreated increments the snapshots created counter
func (m *Metrics) SnapshotCreated() {
	if !m.enabled {
		return
	}
	
	m.snapshotsCreated.Inc()
}

// SnapshotLoaded increments the snapshots loaded counter
func (m *Metrics) SnapshotLoaded() {
	if !m.enabled {
		return
	}
	
	m.snapshotsLoaded.Inc()
}
