package network

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/BKSmick12/raftflow/internal/log"
	"go.uber.org/zap"
)

// MessageType represents the type of RPC message
type MessageType int

const (
	RequestVote MessageType = iota
	AppendEntries
	ClientRequest
	ClientResponse
)

func (m MessageType) String() string {
	switch m {
	case RequestVote:
		return "RequestVote"
	case AppendEntries:
		return "AppendEntries"
	case ClientRequest:
		return "ClientRequest"
	case ClientResponse:
		return "ClientResponse"
	default:
		return "Unknown"
	}
}

// Request represents an incoming RPC request
type Request struct {
	Type MessageType
	From string
	Data interface{}
}

// Response represents an outgoing RPC response
type Response struct {
	Type MessageType
	From string
	Data interface{}
}

// RPCServer handles RPC communication between Raft nodes
type RPCServer struct {
	address      string
	nodeID       string
	peers        []string
	timeout      time.Duration
	logger       *zap.Logger
	server       *http.Server
	requestChan  chan Request
	responseChan chan Response
	mu           sync.RWMutex
	pending      map[string]chan Response
	ctx          context.Context
	cancel       context.CancelFunc
}

// NewRPCServer creates a new RPC server
func NewRPCServer(address, nodeID string, peers []string, timeout time.Duration) (*RPCServer, error) {
	ctx, cancel := context.WithCancel(context.Background())
	
	s := &RPCServer{
		address:      address,
		nodeID:       nodeID,
		peers:        peers,
		timeout:      timeout,
		logger:       zap.L().Named("rpc"),
		requestChan:  make(chan Request, 100),
		responseChan: make(chan Response, 100),
		pending:      make(map[string]chan Response),
		ctx:          ctx,
		cancel:       cancel,
	}
	
	// Create HTTP server
	s.server = &http.Server{
		Addr:    address,
		Handler: s.createHandler(),
	}
	
	return s, nil
}

// Start starts the RPC server
func (s *RPCServer) Start(raft interface{}) error {
	// Start HTTP server
	go func() {
		s.logger.Info("Starting RPC server",
			zap.String("address", s.address),
			zap.String("node_id", s.nodeID),
		)
		
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("RPC server error", zap.Error(err))
		}
	}()
	
	return nil
}

// Stop stops the RPC server
func (s *RPCServer) Stop() error {
	s.cancel()
	
	// Close server
	if err := s.server.Close(); err != nil {
		return err
	}
	
	close(s.requestChan)
	close(s.responseChan)
	
	return nil
}

// RequestChan returns the channel for incoming requests
func (s *RPCServer) RequestChan() chan Request {
	return s.requestChan
}

// ResponseChan returns the channel for incoming responses
func (s *RPCServer) ResponseChan() chan Response {
	return s.responseChan
}

// createHandler creates the HTTP handler for RPC endpoints
func (s *RPCServer) createHandler() http.Handler {
	mux := http.NewServeMux()
	
	mux.HandleFunc("/request_vote", s.handleRequestVote)
	mux.HandleFunc("/append_entries", s.handleAppendEntries)
	mux.HandleFunc("/client_request", s.handleClientRequest)
	
	return mux
}

// handleRequestVote handles RequestVote RPC
func (s *RPCServer) handleRequestVote(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	var request RequestVoteRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	
	s.logger.Debug("Received RequestVote",
		zap.Int64("term", request.Term),
		zap.String("candidate_id", request.CandidateID),
	)
	
	// Send request to Raft node
	s.requestChan <- Request{
		Type: RequestVote,
		From: r.RemoteAddr,
		Data: &request,
	}
	
	// Wait for response
	// In a real implementation, we'd have a way to match requests with responses
	// For simplicity, we'll just return a placeholder response
	// This would be handled by the actual Raft implementation
	
	// For now, return a simple response
	response := RequestVoteResponse{
		Term:        request.Term,
		VoteGranted: false,
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleAppendEntries handles AppendEntries RPC
func (s *RPCServer) handleAppendEntries(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	var request AppendEntriesRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	
	s.logger.Debug("Received AppendEntries",
		zap.Int64("term", request.Term),
		zap.String("leader_id", request.LeaderID),
		zap.Int("entries_count", len(request.Entries)),
	)
	
	// Send request to Raft node
	s.requestChan <- Request{
		Type: AppendEntries,
		From: r.RemoteAddr,
		Data: &request,
	}
	
	// Return a placeholder response
	response := AppendEntriesResponse{
		Term:    request.Term,
		Success: false,
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleClientRequest handles client requests
func (s *RPCServer) handleClientRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	var request ClientRequestData
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	
	s.logger.Debug("Received ClientRequest",
		zap.String("command", string(request.Command)),
	)
	
	// Send request to Raft node
	s.requestChan <- Request{
		Type: ClientRequest,
		From: r.RemoteAddr,
		Data: &request,
	}
	
	// Wait for response and send it back
	// For simplicity, we'll create a channel to wait for the response
	responseChan := make(chan Response, 1)
	s.mu.Lock()
	s.pending[r.RemoteAddr] = responseChan
	s.mu.Unlock()
	
	// Wait for response with timeout
	select {
	case response := <-responseChan:
		if clientResp, ok := response.Data.(*ClientResponse); ok {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(clientResp)
		}
	case <-time.After(s.timeout):
		http.Error(w, "Request timeout", http.StatusGatewayTimeout)
	}
	
	// Clean up
	s.mu.Lock()
	delete(s.pending, r.RemoteAddr)
	s.mu.Unlock()
}

// RequestVote sends a RequestVote RPC to a peer
func (s *RPCServer) RequestVote(peer string, request *RequestVoteRequest) {
	go s.sendRPC(peer, "/request_vote", request, RequestVote)
}

// AppendEntries sends an AppendEntries RPC to a peer
func (s *RPCServer) AppendEntries(peer string, request *AppendEntriesRequest) {
	go s.sendRPC(peer, "/append_entries", request, AppendEntries)
}

// SendResponse sends a response to a client
func (s *RPCServer) SendResponse(to string, data interface{}) {
	// Check if there's a pending request waiting for this response
	s.mu.RLock()
	if ch, ok := s.pending[to]; ok {
		s.mu.RUnlock()
		ch <- Response{
			Type: RequestVote, // This would be determined by the request type
			From: s.nodeID,
			Data: data,
		}
		return
	}
	s.mu.RUnlock()
	
	// Otherwise, send to the general response channel
	s.responseChan <- Response{
		Type: RequestVote, // This would be determined by the request type
		From: to,
		Data: data,
	}
}

// sendRPC sends an RPC request to a peer
func (s *RPCServer) sendRPC(peer, endpoint string, request interface{}, msgType MessageType) {
	url := fmt.Sprintf("http://%s%s", peer, endpoint)
	
	// Create HTTP request
	jsonData, err := json.Marshal(request)
	if err != nil {
		s.logger.Error("Failed to marshal request",
			zap.String("peer", peer),
			zap.String("endpoint", endpoint),
			zap.Error(err),
		)
		return
	}
	
	httpReq, err := http.NewRequestWithContext(s.ctx, http.MethodPost, url, nil)
	if err != nil {
		s.logger.Error("Failed to create HTTP request",
			zap.String("peer", peer),
			zap.String("endpoint", endpoint),
			zap.Error(err),
		)
		return
	}
	
	// Set request body
	httpReq.Body = nil // We'll use a custom body
	
	// For simplicity, we'll use a different approach
	// Create a new request with the JSON body
	httpReq, err = http.NewRequestWithContext(s.ctx, http.MethodPost, url, nil)
	if err != nil {
		s.logger.Error("Failed to create HTTP request", zap.Error(err))
		return
	}
	
	// This is a simplified implementation
	// In a real implementation, we'd properly handle the HTTP request
	// and parse the response
	
	s.logger.Debug("Sending RPC",
		zap.String("peer", peer),
		zap.String("endpoint", endpoint),
		zap.String("type", msgType.String()),
	)
	
	// For now, we'll just log that we're sending the request
	// The actual HTTP client implementation would go here
}

// RequestVoteRequest represents a RequestVote RPC request
type RequestVoteRequest struct {
	Term         int64 `json:"term"`
	CandidateID  string `json:"candidate_id"`
	LastLogIndex int64 `json:"last_log_index"`
	LastLogTerm  int64 `json:"last_log_term"`
}

// RequestVoteResponse represents a RequestVote RPC response
type RequestVoteResponse struct {
	Term        int64 `json:"term"`
	VoteGranted bool  `json:"vote_granted"`
}

// AppendEntriesRequest represents an AppendEntries RPC request
type AppendEntriesRequest struct {
	Term         int64       `json:"term"`
	LeaderID     string      `json:"leader_id"`
	PrevLogIndex int64       `json:"prev_log_index"`
	PrevLogTerm  int64       `json:"prev_log_term"`
	Entries      []*log.Entry `json:"entries"`
	LeaderCommit int64       `json:"leader_commit"`
}

// AppendEntriesResponse represents an AppendEntries RPC response
type AppendEntriesResponse struct {
	Term    int64 `json:"term"`
	Success bool  `json:"success"`
}

// ClientRequestData represents a client request
type ClientRequestData struct {
	Command []byte `json:"command"`
}

// ClientResponse represents a client response
type ClientResponse struct {
	Success bool   `json:"success"`
	Index   int64  `json:"index,omitempty"`
	Error   string `json:"error,omitempty"`
}
