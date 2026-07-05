# raftflow

A production-ready Raft consensus protocol implementation in Go.

## Overview

Raftflow is a complete implementation of the Raft distributed consensus protocol, designed for building reliable, fault-tolerant distributed systems. It provides leader election, log replication, snapshotting, and cluster membership changes out of the box.

## Features

- **Leader Election**: Randomized election timeouts with proper term management
- **Log Replication**: Reliable log replication with consistency guarantees
- **Snapshotting**: Automatic log compaction and state snapshots
- **Cluster Membership**: Dynamic cluster membership changes
- **Custom RPC Layer**: Efficient network communication
- **Write-Ahead Log**: Persistent storage with WAL for durability
- **Metrics & Tracing**: Comprehensive Prometheus metrics and distributed tracing
- **Kubernetes Support**: Ready-to-use Kubernetes manifests

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        Raft Node                               │
├─────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐      │
│  │   Config    │    │   Log       │    │  Snapshot    │      │
│  └─────────────┘    └─────────────┘    └─────────────┘      │
│          │               │                   │                │
│          ▼               ▼                   ▼                │
│  ┌───────────────────────────────────────────────────────┐  │
│  │                    Consensus Core                       │  │
│  │  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐ │  │
│  │  │  State      │    │  Election    │    │  Replication │ │  │
│  │  │  Machine    │    │  Manager    │    │  Manager    │ │  │
│  │  └─────────────┘    └─────────────┘    └─────────────┘ │  │
│  └───────────────────────────────────────────────────────┘  │
│                          │││││                                  │
│                          ▼▼▼▼▼                                  │
│  ┌───────────────────────────────────────────────────────┐  │
│  │                    Network Layer                        │  │
│  │  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐ │  │
│  │  │  RPC Server  │    │  RPC Client  │    │  Transport   │ │  │
│  │  └─────────────┘    └─────────────┘    └─────────────┘ │  │
│  └───────────────────────────────────────────────────────┘  │
│                          │││││                                  │
│                          ▼▼▼▼▼                                  │
│  ┌───────────────────────────────────────────────────────┐  │
│  │                    Storage Layer                        │  │
│  │  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐ │  │
│  │  │    WAL      │    │    Log      │    │  Snapshot    │ │  │
│  │  └─────────────┘    └─────────────┘    └─────────────┘ │  │
│  └───────────────────────────────────────────────────────┘  │
│                                                                  │
└─────────────────────────────────────────────────────────────┘
```

## Quick Start

### Prerequisites

- Go 1.21+
- Docker (optional, for containerized deployment)
- Kubernetes (optional, for Kubernetes deployment)

### Building

```bash
# Clone the repository
git clone https://github.com/BKSmick12/raftflow.git
cd raftflow

# Build the binaries
make build

# Or build manually
go build -o raftflow ./cmd/raftflow
go build -o raftflow-demo ./cmd/raftflow-demo
```

### Running a Single Node

```bash
# Start a single Raft node
./raftflow --node-id node-1 --address localhost:8080 --peers ""
```

### Running a Cluster

Open three terminal windows and run:

```bash
# Node 1
./raftflow --node-id node-1 --address localhost:8081 --peers localhost:8082,localhost:8083

# Node 2
./raftflow --node-id node-2 --address localhost:8082 --peers localhost:8081,localhost:8083

# Node 3
./raftflow --node-id node-3 --address localhost:8083 --peers localhost:8081,localhost:8082
```

### Running the Demo

```bash
# Start the demo (automatically starts a 3-node cluster)
./raftflow-demo --cluster-size 3 --base-port 8080

# Or use the make target
make run-demo
```

The demo provides an interactive console where you can:
- Submit commands to the cluster
- Check cluster state
- View current leader
- Monitor node states

## Docker Deployment

### Building Docker Images

```bash
# Build the main raftflow image
docker build -t ghcr.io/bksmick12/raftflow:latest .

# Build the demo image
docker build -t ghcr.io/bksmick12/raftflow-demo:latest -f Dockerfile.demo .

# Or use make
docker build -t ghcr.io/bksmick12/raftflow:latest -f Dockerfile .
docker build -t ghcr.io/bksmick12/raftflow-demo:latest -f Dockerfile.demo .
```

### Running with Docker Compose

Create a `docker-compose.yml` file:

```yaml
version: '3.8'

services:
  raftflow-1:
    image: ghcr.io/bksmick12/raftflow:latest
    ports:
      - "8081:8080"
      - "9091:9090"
    environment:
      - NODE_ID=node-1
      - ADDRESS=raftflow-1:8080
      - PEERS=raftflow-2:8080,raftflow-3:8080
      - LOG_DIR=/data/log
      - SNAPSHOT_DIR=/data/snapshot
    volumes:
      - ./data/node-1:/data

  raftflow-2:
    image: ghcr.io/bksmick12/raftflow:latest
    ports:
      - "8082:8080"
      - "9092:9090"
    environment:
      - NODE_ID=node-2
      - ADDRESS=raftflow-2:8080
      - PEERS=raftflow-1:8080,raftflow-3:8080
      - LOG_DIR=/data/log
      - SNAPSHOT_DIR=/data/snapshot
    volumes:
      - ./data/node-2:/data

  raftflow-3:
    image: ghcr.io/bksmick12/raftflow:latest
    ports:
      - "8083:8080"
      - "9093:9090"
    environment:
      - NODE_ID=node-3
      - ADDRESS=raftflow-3:8080
      - PEERS=raftflow-1:8080,raftflow-2:8080
      - LOG_DIR=/data/log
      - SNAPSHOT_DIR=/data/snapshot
    volumes:
      - ./data/node-3:/data

  raftflow-demo:
    image: ghcr.io/bksmick12/raftflow-demo:latest
    ports:
      - "8084:8081"
    environment:
      - CLUSTER_SIZE=3
      - BASE_PORT=8080
      - BASE_METRICS_PORT=9090
    depends_on:
      - raftflow-1
      - raftflow-2
      - raftflow-3
```

Then run:

```bash
docker-compose up -d
```

## Kubernetes Deployment

### Prerequisites

- Kubernetes cluster
- kubectl configured
- (Optional) Prometheus for metrics

### Deploying the Cluster

```bash
# Apply the Raft cluster deployment
kubectl apply -f k8s/raftflow-deployment.yaml

# Check the pods
kubectl get pods -l app=raftflow

# View logs
kubectl logs -l app=raftflow -f

# Access metrics (if Prometheus is configured)
kubectl port-forward svc/raftflow-metrics 9090:9090
```

### Deploying the Demo

```bash
# Apply the demo deployment
kubectl apply -f k8s/raftflow-demo-deployment.yaml

# Access the demo interface
kubectl port-forward svc/raftflow-demo 8081:8081
```

## API Reference

### RPC Endpoints

#### RequestVote

**Request:**
```json
{
  "term": 1,
  "candidate_id": "node-1",
  "last_log_index": 10,
  "last_log_term": 2
}
```

**Response:**
```json
{
  "term": 1,
  "vote_granted": true
}
```

#### AppendEntries

**Request:**
```json
{
  "term": 1,
  "leader_id": "node-1",
  "prev_log_index": 10,
  "prev_log_term": 2,
  "entries": [
    {
      "term": 2,
      "index": 11,
      "command": "set x=5",
      "type": 0
    }
  ],
  "leader_commit": 10
}
```

**Response:**
```json
{
  "term": 1,
  "success": true
}
```

### Client API

#### Submit Command

**Request:**
```bash
POST /submit
Content-Type: application/json

{
  "command": "set x=5"
}
```

**Response:**
```json
{
  "success": true,
  "index": 11
}
```

#### Get Cluster State

**Request:**
```bash
GET /state
```

**Response:**
```json
{
  "leader": "node-1",
  "term": 2,
  "nodes": [
    {
      "id": "node-1",
      "address": "localhost:8081",
      "state": "Leader",
      "term": 2,
      "commit_index": 11,
      "last_applied": 11
    },
    {
      "id": "node-2",
      "address": "localhost:8082",
      "state": "Follower",
      "term": 2,
      "commit_index": 11,
      "last_applied": 11
    },
    {
      "id": "node-3",
      "address": "localhost:8083",
      "state": "Follower",
      "term": 2,
      "commit_index": 11,
      "last_applied": 11
    }
  ]
}
```

#### Get Current Leader

**Request:**
```bash
GET /leader
```

**Response:**
```json
{
  "leader": "node-1"
}
```

## Configuration

### Command Line Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--node-id` | Unique identifier for this node | (required) |
| `--address` | Network address for this node | `localhost:8080` |
| `--peers` | Comma-separated list of peer addresses | `""` |
| `--log-dir` | Directory for persistent logs | `./data/log` |
| `--snapshot-dir` | Directory for snapshots | `./data/snapshot` |
| `--election-timeout-min` | Minimum election timeout | `1500ms` |
| `--election-timeout-max` | Maximum election timeout | `3000ms` |
| `--heartbeat-interval` | Heartbeat interval | `500ms` |
| `--rpc-timeout` | RPC timeout | `1000ms` |
| `--snapshot-interval` | Snapshot interval | `30s` |
| `--snapshot-threshold` | Minimum entries before snapshotting | `1000` |
| `--enable-metrics` | Enable Prometheus metrics | `true` |
| `--metrics-address` | Address for metrics server | `:9090` |

### Environment Variables

The same configuration can be set via environment variables:

- `NODE_ID`
- `ADDRESS`
- `PEERS`
- `LOG_DIR`
- `SNAPSHOT_DIR`
- `ENABLE_METRICS`
- `METRICS_ADDRESS`

## Metrics

Raftflow exposes comprehensive Prometheus metrics on the configured metrics address.

### Available Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `raft_state_current_term` | Gauge | Current term of the Raft node |
| `raft_state_current_state` | Gauge | Current state (0=Follower, 1=Candidate, 2=Leader) |
| `raft_election_started_total` | Counter | Total elections started |
| `raft_election_won_total` | Counter | Total elections won |
| `raft_election_lost_total` | Counter | Total elections lost |
| `raft_vote_received_total` | Counter | Total votes received |
| `raft_vote_granted_total` | Counter | Total votes granted |
| `raft_rpc_requests_received_total` | Counter | RPC requests received (by type) |
| `raft_rpc_requests_sent_total` | Counter | RPC requests sent (by type) |
| `raft_rpc_failures_total` | Counter | RPC failures (by type) |
| `raft_log_entries_appended_total` | Counter | Log entries appended |
| `raft_log_entries_committed_total` | Counter | Log entries committed |
| `raft_log_entries_applied_total` | Counter | Log entries applied |
| `raft_heartbeat_sent_total` | Counter | Heartbeats sent |
| `raft_heartbeat_received_total` | Counter | Heartbeats received |
| `raft_client_requests_received_total` | Counter | Client requests received |
| `raft_client_requests_success_total` | Counter | Successful client requests |
| `raft_client_requests_failed_total` | Counter | Failed client requests |
| `raft_snapshot_created_total` | Counter | Snapshots created |
| `raft_snapshot_loaded_total` | Counter | Snapshots loaded |
| `raft_rpc_latency_seconds` | Histogram | RPC latency |
| `raft_leader_is_leader` | Gauge | 1 if node is leader, 0 otherwise |

## Testing

### Running Tests

```bash
# Run all tests
make test

# Run unit tests only
make test-unit

# Run integration tests only
make test-integration

# Run with race detector
go test -race ./...
```

### Test Coverage

```bash
# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Generate coverage report for specific package
go test -coverprofile=coverage.out ./internal/consensus
go tool cover -html=coverage.out
```

## Project Structure

```
raftflow/
├── cmd/
│   ├── raftflow/          # Main raftflow command
│   │   └── main.go
│   └── raftflow-demo/     # Demo command
│       └── main.go
├── internal/
│   ├── config/           # Configuration management
│   │   └── config.go
│   ├── consensus/         # Core Raft consensus implementation
│   │   ├── raft.go        # Main Raft implementation
│   │   └── metrics.go     # Prometheus metrics
│   ├── log/              # Log implementation
│   │   └── log.go
│   ├── network/          # Network/RPC layer
│   │   └── rpc.go
│   ├── snapshot/         # Snapshot management
│   │   └── snapshot.go
│   ├── storage/          # Persistent storage
│   │   └── storage.go
│   └── util/             # Utility functions
│       └── util.go
├── k8s/                 # Kubernetes manifests
│   ├── raftflow-deployment.yaml
│   └── raftflow-demo-deployment.yaml
├── test/                # Integration tests
├── Dockerfile           # Main Dockerfile
├── Dockerfile.demo      # Demo Dockerfile
├── Makefile             # Makefile
└── README.md            # This file
```

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/your-feature`)
3. Commit your changes (`git commit -am 'Add some feature'`)
4. Push to the branch (`git push origin feature/your-feature`)
5. Open a Pull Request

### Development Guidelines

- Follow Go coding standards
- Add tests for new functionality
- Update documentation
- Keep commits atomic and well-described
- Use meaningful commit messages

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- Inspired by the [Raft paper](https://raft.github.io/raft.pdf) by Diego Ongaro and John Ousterhout
- Built with [Go](https://golang.org/)
- Metrics powered by [Prometheus](https://prometheus.io/)
- Logging with [zap](https://github.com/uber-go/zap)

## References

- [Raft Consensus Algorithm](https://raft.github.io/)
- [Raft Paper](https://raft.github.io/raft.pdf)
- [Raft Visualization](http://thesecretlivesofdata.com/raft/)
