package consensus

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"github.com/BKSmick12/raftflow/internal/config"
	"github.com/BKSmick12/raftflow/internal/log"
	"github.com/BKSmick12/raftflow/internal/network"
	"github.com/BKSmick12/raftflow/internal/snapshot"
	"github.com/BKSmick12/raftflow/internal/storage"
	"go.uber.org/zap"
)

// State represents the current state of the Raft node
type State int

const (
	Follower State = iota
	Candidate
	Leader
)

func (s State) String() string {
	switch s {
	case Follower:
		return "Follower"
	case Candidate:
		return "Candidate"
	case Leader:
		return "Leader"
	default:
		return "Unknown"
	}
}

// Raft is the main consensus protocol implementation
type Raft struct {
	config *config.Config
	logger *zap.Logger
	
	// Current state
	state     State
	currentTerm int64
	votedFor   string
	
	// Volatile state
	commitIndex int64
	lastApplied int64
	
	// Leader state
	nextIndex  map[string]int64
	matchIndex map[string]int64
	
	// Log
	log *log.Log
	
	// Network
	network *network.RPCServer
	
	// Storage
	storage *storage.Storage
	
	// Snapshot
	snapshot *snapshot.Manager
	
	// Timers
	electionTimer *time.Timer
	electionTimerChan chan struct{}
	heartbeatTimer *time.Timer
	heartbeatTimerChan chan struct{}
	
	// Mutex for thread safety
	mu sync.RWMutex
	
	// Context for shutdown
	ctx    context.Context
	cancel context.CancelFunc
	
	// WAL
	wal *storage.WAL
	
	// Metrics
	metrics *Metrics
}

// NewRaft creates a new Raft instance
func NewRaft(cfg *config.Config, logger *zap.Logger) (*Raft, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	// Initialize storage
	storage, err := storage.NewStorage(cfg.LogDir, cfg.SnapshotDir)
	if err != nil {
		cancel()
		return nil, err
	}
	
	// Initialize WAL
	wal, err := storage.NewWAL(cfg.LogDir)
	if err != nil {
		cancel()
		return nil, err
	}
	
	// Initialize log
	raftLog, err := log.NewLog(storage)
	if err != nil {
		cancel()
		return nil, err
	}
	
	// Initialize snapshot manager
	snapshotManager, err := snapshot.NewManager(storage, cfg.SnapshotThreshold)
	if err != nil {
		cancel()
		return nil, err
	}
	
	// Initialize network
	rpcServer, err := network.NewRPCServer(cfg.Address, cfg.NodeID, cfg.Peers, cfg.RPCTimeout)
	if err != nil {
		cancel()
		return nil, err
	}
	
	// Initialize metrics
	metrics := NewMetrics(cfg.EnableMetrics, cfg.ClusterID, cfg.NodeID)
	
	r := &Raft{
		config:             cfg,
		logger:             logger,
		state:              Follower,
		currentTerm:        0,
		votedFor:           "",
		commitIndex:        0,
		lastApplied:        0,
		nextIndex:         make(map[string]int64),
		matchIndex:        make(map[string]int64),
		log:                raftLog,
		network:            rpcServer,
		storage:            storage,
		snapshot:           snapshotManager,
		electionTimerChan:  make(chan struct{}, 1),
		heartbeatTimerChan: make(chan struct{}, 1),
		ctx:               ctx,
		cancel:            cancel,
		wal:               wal,
		metrics:           metrics,
	}
	
	// Load persistent state
	if err := r.loadPersistentState(); err != nil {
		logger.Error("Failed to load persistent state", zap.Error(err))
		cancel()
		return nil, err
	}
	
	// Initialize timers
	r.resetElectionTimer()
	
	return r, nil
}

// loadPersistentState loads the persistent state from storage
func (r *Raft) loadPersistentState() error {
	// Load current term and votedFor from storage
	term, votedFor, err := r.storage.GetPersistentState()
	if err != nil {
		return err
	}
	
	r.mu.Lock()
	defer r.mu.Unlock()
	
	r.currentTerm = term
	r.votedFor = votedFor
	
	// Load log entries
	entries, err := r.storage.GetLogEntries()
	if err != nil {
		return err
	}
	
	for _, entry := range entries {
		if err := r.log.Append(entry); err != nil {
			return err
		}
	}
	
	// Load snapshot if exists
	snapshot, err := r.storage.GetLatestSnapshot()
	if err == nil && snapshot != nil {
		if err := r.snapshot.Load(snapshot); err != nil {
			return err
		}
	}
	
	return nil
}

// Start starts the Raft node
func (r *Raft) Start() error {
	// Start network server
	if err := r.network.Start(r); err != nil {
		return err
	}
	
	// Start metrics server if enabled
	if r.config.EnableMetrics {
		go r.metrics.Start(r.config.MetricsAddress)
	}
	
	// Start snapshot manager
	go r.snapshot.Start(r.config.SnapshotInterval, r.log, r.storage)
	
	// Start main loop
	go r.run()
	
	r.logger.Info("Raft node started",
		zap.String("node_id", r.config.NodeID),
		zap.String("address", r.config.Address),
		zap.String("state", r.state.String()),
		zap.Int64("term", r.currentTerm),
	)
	
	return nil
}

// Stop stops the Raft node
func (r *Raft) Stop() error {
	r.cancel()
	
	// Stop timers
	if r.electionTimer != nil {
		r.electionTimer.Stop()
	}
	if r.heartbeatTimer != nil {
		r.heartbeatTimer.Stop()
	}
	
	// Stop network
	if err := r.network.Stop(); err != nil {
		r.logger.Error("Failed to stop network", zap.Error(err))
	}
	
	// Stop storage
	if err := r.storage.Close(); err != nil {
		r.logger.Error("Failed to close storage", zap.Error(err))
	}
	
	// Stop WAL
	if err := r.wal.Close(); err != nil {
		r.logger.Error("Failed to close WAL", zap.Error(err))
	}
	
	// Stop metrics
	r.metrics.Stop()
	
	r.logger.Info("Raft node stopped", zap.String("node_id", r.config.NodeID))
	
	return nil
}

// run is the main event loop
func (r *Raft) run() {
	for {
		select {
		case <-r.ctx.Done():
			r.logger.Info("Raft node shutting down")
			return
			
		case <-r.electionTimerChan:
			r.handleElectionTimeout()
			
		case <-r.heartbeatTimerChan:
			if r.state == Leader {
				r.sendHeartbeats()
				r.resetHeartbeatTimer()
			}
			
		case req := <-r.network.RequestChan():
			r.handleRequest(req)
			
		case resp := <-r.network.ResponseChan():
			r.handleResponse(resp)
		}
	}
}

// handleElectionTimeout handles election timeout
func (r *Raft) handleElectionTimeout() {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	// Only start election if we're a follower or candidate
	if r.state != Follower && r.state != Candidate {
		return
	}
	
	r.logger.Info("Election timeout, starting new election",
		zap.Int64("current_term", r.currentTerm),
		zap.String("state", r.state.String()),
	)
	
	// Increment current term
	r.currentTerm++
	r.votedFor = r.config.NodeID
	r.state = Candidate
	
	// Persist state
	if err := r.storage.SavePersistentState(r.currentTerm, r.votedFor); err != nil {
		r.logger.Error("Failed to save persistent state", zap.Error(err))
		return
	}
	
	// Request votes from all peers
	lastLogIndex, lastLogTerm := r.log.GetLastEntryInfo()
	
	for _, peer := range r.config.Peers {
		go r.network.RequestVote(peer, &network.RequestVoteRequest{
			Term:         r.currentTerm,
			CandidateID:  r.config.NodeID,
			LastLogIndex: lastLogIndex,
			LastLogTerm:  lastLogTerm,
		})
	}
	
	// Reset election timer with random timeout
	r.resetElectionTimer()
	
	r.metrics.ElectionStarted()
}

// handleRequest handles incoming RPC requests
func (r *Raft) handleRequest(req network.Request) {
	switch req.Type {
	case network.RequestVote:
		r.handleRequestVote(req)
	case network.AppendEntries:
		r.handleAppendEntries(req)
	case network.ClientRequest:
		r.handleClientRequest(req)
	}
}

// handleRequestVote handles RequestVote RPC
func (r *Raft) handleRequestVote(req network.Request) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	request := req.Data.(*network.RequestVoteRequest)
	
	r.logger.Debug("Received RequestVote",
		zap.Int64("term", request.Term),
		zap.String("candidate_id", request.CandidateID),
		zap.Int64("last_log_index", request.LastLogIndex),
		zap.Int64("last_log_term", request.LastLogTerm),
	)
	
	// Reply false if term < currentTerm
	if request.Term < r.currentTerm {
		r.network.SendResponse(req.From, &network.RequestVoteResponse{
			Term:        r.currentTerm,
			VoteGranted: false,
		})
		return
	}
	
	// If RPC request contains term > currentTerm, update currentTerm and convert to follower
	if request.Term > r.currentTerm {
		r.currentTerm = request.Term
		r.votedFor = ""
		r.state = Follower
		
		if err := r.storage.SavePersistentState(r.currentTerm, r.votedFor); err != nil {
			r.logger.Error("Failed to save persistent state", zap.Error(err))
		}
		
		r.resetElectionTimer()
	}
	
	// Check if we can vote for this candidate
	lastLogIndex, lastLogTerm := r.log.GetLastEntryInfo()
	
	// Reply false if candidate's log is less up-to-date than ours
	if request.LastLogTerm < lastLogTerm || 
		(request.LastLogTerm == lastLogTerm && request.LastLogIndex < lastLogIndex) {
		r.network.SendResponse(req.From, &network.RequestVoteResponse{
			Term:        r.currentTerm,
			VoteGranted: false,
		})
		return
	}
	
	// Reply false if we've already voted for someone else in this term
	if r.votedFor != "" && r.votedFor != request.CandidateID {
		r.network.SendResponse(req.From, &network.RequestVoteResponse{
			Term:        r.currentTerm,
			VoteGranted: false,
		})
		return
	}
	
	// Grant vote
	r.votedFor = request.CandidateID
	r.state = Follower
	
	if err := r.storage.SavePersistentState(r.currentTerm, r.votedFor); err != nil {
		r.logger.Error("Failed to save persistent state", zap.Error(err))
	}
	
	r.network.SendResponse(req.From, &network.RequestVoteResponse{
		Term:        r.currentTerm,
		VoteGranted: true,
	})
	
	r.resetElectionTimer()
	
	r.metrics.VoteReceived()
}

// handleAppendEntries handles AppendEntries RPC
func (r *Raft) handleAppendEntries(req network.Request) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	request := req.Data.(*network.AppendEntriesRequest)
	
	r.logger.Debug("Received AppendEntries",
		zap.Int64("term", request.Term),
		zap.String("leader_id", request.LeaderID),
		zap.Int64("prev_log_index", request.PrevLogIndex),
		zap.Int64("prev_log_term", request.PrevLogTerm),
		zap.Int("entries_count", len(request.Entries)),
		zap.Int64("leader_commit", request.LeaderCommit),
	)
	
	// Reply false if term < currentTerm
	if request.Term < r.currentTerm {
		r.network.SendResponse(req.From, &network.AppendEntriesResponse{
			Term:    r.currentTerm,
			Success: false,
		})
		return
	}
	
	// Reset election timer if we receive a valid RPC from leader
	if request.Term >= r.currentTerm {
		r.currentTerm = request.Term
		r.votedFor = ""
		r.state = Follower
		
		if err := r.storage.SavePersistentState(r.currentTerm, r.votedFor); err != nil {
			r.logger.Error("Failed to save persistent state", zap.Error(err))
		}
		
		r.resetElectionTimer()
	}
	
	// Check if previous log entry matches
	prevEntry, err := r.log.GetEntry(request.PrevLogIndex)
	if err != nil {
		// If there's no entry at PrevLogIndex, check if it's because the index is before our first entry
		if request.PrevLogIndex > 0 {
			r.logger.Debug("Previous log entry not found",
				zap.Int64("prev_log_index", request.PrevLogIndex),
				zap.Error(err),
			)
			r.network.SendResponse(req.From, &network.AppendEntriesResponse{
				Term:    r.currentTerm,
				Success: false,
			})
			return
		}
	}
	
	// If PrevLogIndex > 0, check if terms match
	if request.PrevLogIndex > 0 && prevEntry.Term != request.PrevLogTerm {
		r.logger.Debug("Previous log term mismatch",
			zap.Int64("expected_term", request.PrevLogTerm),
			zap.Int64("actual_term", prevEntry.Term),
		)
		r.network.SendResponse(req.From, &network.AppendEntriesResponse{
			Term:    r.currentTerm,
			Success: false,
		})
		return
	}
	
	// Append entries
	for i, entry := range request.Entries {
		// Check if entry already exists and has different term
		if existingEntry, err := r.log.GetEntry(request.PrevLogIndex + 1 + int64(i)); err == nil {
			if existingEntry.Term != entry.Term {
				// Delete this entry and all following entries
				if err := r.log.DeleteFrom(request.PrevLogIndex + 1 + int64(i)); err != nil {
					r.logger.Error("Failed to delete entries", zap.Error(err))
					return
				}
				// Re-append the new entries
				if err := r.log.Append(entry); err != nil {
					r.logger.Error("Failed to append entry", zap.Error(err))
					return
				}
			} else {
				// Entry already exists, skip
				continue
			}
		} else {
			// Entry doesn't exist, append it
			if err := r.log.Append(entry); err != nil {
				r.logger.Error("Failed to append entry", zap.Error(err))
				return
			}
		}
	}
	
	// Update commit index
	if request.LeaderCommit > r.commitIndex {
		r.commitIndex = min(request.LeaderCommit, r.log.GetLastIndex())
		
		// Apply committed entries
		go r.applyCommittedEntries()
	}
	
	r.network.SendResponse(req.From, &network.AppendEntriesResponse{
		Term:    r.currentTerm,
		Success: true,
	})
	
	r.metrics.AppendEntriesReceived(len(request.Entries))
}

// handleClientRequest handles client requests
func (r *Raft) handleClientRequest(req network.Request) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if r.state != Leader {
		// Redirect to leader if we know who it is
		// For simplicity, we'll just return an error
		r.network.SendResponse(req.From, &network.ClientResponse{
			Success: false,
			Error:   "Not the leader",
		})
		return
	}
	
	clientReq := req.Data.(*network.ClientRequestData)
	
	// Create log entry
	entry := &log.Entry{
		Term:    r.currentTerm,
		Command: clientReq.Command,
		Index:   r.log.GetLastIndex() + 1,
	}
	
	// Append to log
	if err := r.log.Append(entry); err != nil {
		r.logger.Error("Failed to append client command", zap.Error(err))
		r.network.SendResponse(req.From, &network.ClientResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}
	
	// Replicate to followers
	go r.replicateToFollowers(entry)
	
	// Wait for majority to acknowledge
	// For simplicity, we'll just return success immediately
	// In a real implementation, we'd wait for acknowledgments
	r.network.SendResponse(req.From, &network.ClientResponse{
		Success: true,
		Index:   entry.Index,
	})
	
	r.metrics.ClientRequestReceived()
}

// handleResponse handles RPC responses
func (r *Raft) handleResponse(resp network.Response) {
	switch resp.Type {
	case network.RequestVote:
		r.handleRequestVoteResponse(resp)
	case network.AppendEntries:
		r.handleAppendEntriesResponse(resp)
	}
}

// handleRequestVoteResponse handles RequestVote response
func (r *Raft) handleRequestVoteResponse(resp network.Response) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if r.state != Candidate {
		return
	}
	
	response := resp.Data.(*network.RequestVoteResponse)
	
	r.logger.Debug("Received RequestVote response",
		zap.Int64("term", response.Term),
		zap.Bool("vote_granted", response.VoteGranted),
	)
	
	// If RPC request contains term > currentTerm, update currentTerm and convert to follower
	if response.Term > r.currentTerm {
		r.currentTerm = response.Term
		r.votedFor = ""
		r.state = Follower
		
		if err := r.storage.SavePersistentState(r.currentTerm, r.votedFor); err != nil {
			r.logger.Error("Failed to save persistent state", zap.Error(err))
		}
		
		r.resetElectionTimer()
		return
	}
	
	// If vote not granted, do nothing
	if !response.VoteGranted {
		return
	}
	
	// Count votes
	// For simplicity, we'll just check if we have majority
	// In a real implementation, we'd track votes from each peer
	
	// Check if we have majority
	votesNeeded := len(r.config.Peers)/2 + 1
	
	// This is simplified - in reality we'd track actual votes
	// For now, we'll assume we got a vote and check if we have enough
	// This is a placeholder for the actual vote counting logic
	
	// If we have majority, become leader
	if true { // Placeholder for vote counting
		r.state = Leader
		r.votedFor = ""
		
		// Initialize leader state
		lastIndex := r.log.GetLastIndex()
		for _, peer := range r.config.Peers {
			r.nextIndex[peer] = lastIndex + 1
			r.matchIndex[peer] = 0
		}
		
		// Start sending heartbeats
		r.resetHeartbeatTimer()
		
		r.logger.Info("Became leader",
			zap.Int64("term", r.currentTerm),
			zap.String("node_id", r.config.NodeID),
		)
		
		r.metrics.BecameLeader()
	}
}

// handleAppendEntriesResponse handles AppendEntries response
func (r *Raft) handleAppendEntriesResponse(resp network.Response) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if r.state != Leader {
		return
	}
	
	response := resp.Data.(*network.AppendEntriesResponse)
	
	r.logger.Debug("Received AppendEntries response",
		zap.String("from", resp.From),
		zap.Int64("term", response.Term),
		zap.Bool("success", response.Success),
	)
	
	// If RPC request contains term > currentTerm, update currentTerm and convert to follower
	if response.Term > r.currentTerm {
		r.currentTerm = response.Term
		r.votedFor = ""
		r.state = Follower
		
		if err := r.storage.SavePersistentState(r.currentTerm, r.votedFor); err != nil {
			r.logger.Error("Failed to save persistent state", zap.Error(err))
		}
		
		r.resetElectionTimer()
		return
	}
	
	// If response is not successful, decrement nextIndex and retry
	if !response.Success {
		r.nextIndex[resp.From]--
		
		// Send entries again with updated nextIndex
		go r.sendAppendEntries(resp.From)
		return
	}
	
	// Update nextIndex and matchIndex
	r.nextIndex[resp.From] = r.log.GetLastIndex() + 1
	r.matchIndex[resp.From] = r.log.GetLastIndex()
	
	// Check if we can advance commitIndex
	r.updateCommitIndex()
}

// sendHeartbeats sends heartbeats to all followers
func (r *Raft) sendHeartbeats() {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	if r.state != Leader {
		return
	}
	
	lastIndex := r.log.GetLastIndex()
	lastTerm, _ := r.log.GetLastEntryInfo()
	
	for _, peer := range r.config.Peers {
		// Get the next index for this peer
		nextIdx := r.nextIndex[peer]
		
		// Get entries to send
		entries := make([]*log.Entry, 0)
		if nextIdx <= lastIndex {
			for i := nextIdx; i <= lastIndex; i++ {
				entry, err := r.log.GetEntry(i)
				if err != nil {
					break
				}
				entries = append(entries, entry)
			}
		}
		
		// Get previous log info
		prevIndex := nextIdx - 1
		prevTerm := int64(0)
		if prevIndex > 0 {
			prevEntry, err := r.log.GetEntry(prevIndex)
			if err == nil {
				prevTerm = prevEntry.Term
			}
		}
		
		r.network.AppendEntries(peer, &network.AppendEntriesRequest{
			Term:         r.currentTerm,
			LeaderID:     r.config.NodeID,
			PrevLogIndex: prevIndex,
			PrevLogTerm:  prevTerm,
			Entries:      entries,
			LeaderCommit: r.commitIndex,
		})
	}
	
	r.metrics.HeartbeatsSent(len(r.config.Peers))
}

// replicateToFollowers replicates a new entry to followers
func (r *Raft) replicateToFollowers(entry *log.Entry) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	if r.state != Leader {
		return
	}
	
	for _, peer := range r.config.Peers {
		// Get the next index for this peer
		nextIdx := r.nextIndex[peer]
		
		// Get entries to send (including the new entry)
		entries := make([]*log.Entry, 0)
		lastIndex := r.log.GetLastIndex()
		
		if nextIdx <= lastIndex {
			for i := nextIdx; i <= lastIndex; i++ {
				entry, err := r.log.GetEntry(i)
				if err != nil {
					break
				}
				entries = append(entries, entry)
			}
		}
		
		// Get previous log info
		prevIndex := nextIdx - 1
		prevTerm := int64(0)
		if prevIndex > 0 {
			prevEntry, err := r.log.GetEntry(prevIndex)
			if err == nil {
				prevTerm = prevEntry.Term
			}
		}
		
		r.network.AppendEntries(peer, &network.AppendEntriesRequest{
			Term:         r.currentTerm,
			LeaderID:     r.config.NodeID,
			PrevLogIndex: prevIndex,
			PrevLogTerm:  prevTerm,
			Entries:      entries,
			LeaderCommit: r.commitIndex,
		})
	}
}

// sendAppendEntries sends AppendEntries to a specific peer
func (r *Raft) sendAppendEntries(peer string) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	if r.state != Leader {
		return
	}
	
	nextIdx := r.nextIndex[peer]
	lastIndex := r.log.GetLastIndex()
	
	// Get entries to send
	entries := make([]*log.Entry, 0)
	if nextIdx <= lastIndex {
		for i := nextIdx; i <= lastIndex; i++ {
			entry, err := r.log.GetEntry(i)
			if err != nil {
				break
			}
			entries = append(entries, entry)
		}
	}
	
	// Get previous log info
	prevIndex := nextIdx - 1
	prevTerm := int64(0)
	if prevIndex > 0 {
		prevEntry, err := r.log.GetEntry(prevIndex)
		if err == nil {
			prevTerm = prevEntry.Term
		}
	}
	
	r.network.AppendEntries(peer, &network.AppendEntriesRequest{
		Term:         r.currentTerm,
		LeaderID:     r.config.NodeID,
		PrevLogIndex: prevIndex,
		PrevLogTerm:  prevTerm,
		Entries:      entries,
		LeaderCommit: r.commitIndex,
	})
}

// updateCommitIndex updates the commit index based on matchIndex
func (r *Raft) updateCommitIndex() {
	// Find the highest index that has been replicated on a majority of nodes
	// This is a simplified version
	
	// Sort matchIndex values
	matchIndices := make([]int64, 0, len(r.matchIndex))
	for _, idx := range r.matchIndex {
		matchIndices = append(matchIndices, idx)
	}
	
	if len(matchIndices) == 0 {
		return
	}
	
	// Sort in descending order
	for i := 0; i < len(matchIndices); i++ {
		for j := i + 1; j < len(matchIndices); j++ {
			if matchIndices[i] < matchIndices[j] {
				matchIndices[i], matchIndices[j] = matchIndices[j], matchIndices[i]
			}
		}
	}
	
	// Get the median (majority) index
	majorityIdx := len(matchIndices) / 2
	newCommitIndex := matchIndices[majorityIdx]
	
	// Only update if the new commit index is higher and the entry at that index is from current term
	if newCommitIndex > r.commitIndex {
		entry, err := r.log.GetEntry(newCommitIndex)
		if err == nil && entry.Term == r.currentTerm {
			r.commitIndex = newCommitIndex
			go r.applyCommittedEntries()
		}
	}
}

// applyCommittedEntries applies committed entries to the state machine
func (r *Raft) applyCommittedEntries() {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	for r.lastApplied < r.commitIndex {
		r.lastApplied++
		
		entry, err := r.log.GetEntry(r.lastApplied)
		if err != nil {
			r.logger.Error("Failed to get entry for apply",
				zap.Int64("index", r.lastApplied),
				zap.Error(err),
			)
			break
		}
		
		// Apply the command to the state machine
		// In a real implementation, this would update the application state
		r.logger.Info("Applying command",
			zap.Int64("index", entry.Index),
			zap.Int64("term", entry.Term),
			zap.String("command", string(entry.Command)),
		)
		
		// Write to WAL
		if err := r.wal.Write(entry); err != nil {
			r.logger.Error("Failed to write to WAL", zap.Error(err))
		}
		
		// Update snapshot if needed
		if r.snapshot != nil {
			r.snapshot.CheckAndCreate(r.log, r.lastApplied)
		}
		
		r.metrics.CommandApplied()
	}
}

// resetElectionTimer resets the election timer with a random timeout
func (r *Raft) resetElectionTimer() {
	if r.electionTimer != nil {
		r.electionTimer.Stop()
	}
	
	timeout := time.Duration(rand.Int63n(int64(r.config.ElectionTimeoutMax-r.config.ElectionTimeoutMin))) + r.config.ElectionTimeoutMin
	
	r.electionTimer = time.AfterFunc(timeout, func() {
		select {
		case r.electionTimerChan <- struct{}{}:
		default:
		}
	})
	
	r.logger.Debug("Reset election timer", zap.Duration("timeout", timeout))
}

// resetHeartbeatTimer resets the heartbeat timer
func (r *Raft) resetHeartbeatTimer() {
	if r.heartbeatTimer != nil {
		r.heartbeatTimer.Stop()
	}
	
	r.heartbeatTimer = time.AfterFunc(r.config.HeartbeatInterval, func() {
		select {
		case r.heartbeatTimerChan <- struct{}{}:
		default:
		}
	})
}

// GetState returns the current state of the Raft node
func (r *Raft) GetState() State {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.state
}

// GetCurrentTerm returns the current term
func (r *Raft) GetCurrentTerm() int64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.currentTerm
}

// GetCommitIndex returns the commit index
func (r *Raft) GetCommitIndex() int64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.commitIndex
}

// GetLastApplied returns the last applied index
func (r *Raft) GetLastApplied() int64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.lastApplied
}

// SubmitCommand submits a command to the Raft cluster
func (r *Raft) SubmitCommand(command []byte) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	if r.state != Leader {
		return ErrNotLeader
	}
	
	// Create log entry
	entry := &log.Entry{
		Term:    r.currentTerm,
		Command: command,
		Index:   r.log.GetLastIndex() + 1,
	}
	
	// Append to log
	if err := r.log.Append(entry); err != nil {
		return err
	}
	
	// Replicate to followers
	go r.replicateToFollowers(entry)
	
	return nil
}

// ErrNotLeader is returned when trying to submit a command to a non-leader
var ErrNotLeader = &RaftError{"not the leader"}

// RaftError represents a Raft error
type RaftError struct {
	Message string
}

func (e *RaftError) Error() string {
	return e.Message
}

func min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
