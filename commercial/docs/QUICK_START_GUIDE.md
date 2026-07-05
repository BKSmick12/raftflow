# 🚀 RaftFlow Quick Start Guide

**Version**: 1.0  
**Last Updated**: January 1, 2025

Welcome to RaftFlow! This guide will help you get started with RaftFlow quickly, whether you're evaluating it for your project or ready to deploy to production.

---

## 📋 Prerequisites

Before you begin, ensure you have the following:

### System Requirements
| Requirement | Minimum | Recommended |
|-------------|---------|-------------|
| **Operating System** | Linux, macOS, Windows 10+ | Linux (Ubuntu 20.04+, CentOS 8+) |
| **CPU** | 2 cores | 4+ cores |
| **Memory** | 2GB RAM | 4GB+ RAM |
| **Storage** | 10GB disk | 50GB+ SSD |
| **Network** | 100Mbps | 1Gbps+ |

### Software Requirements
- **Go**: 1.21+ (for building from source)
- **Docker**: 20.10+ (for container deployment)
- **Git**: 2.30+
- **Make**: 4.3+

### Network Requirements
- **Ports**: 8080 (Raft), 9090 (Metrics), 8081 (Demo)
- **Firewall**: Allow TCP traffic between nodes
- **DNS**: Proper hostname resolution

---

## 🎯 Installation Options

Choose the installation method that best fits your needs:

### Option 1: Docker (Recommended)

**Quickest way to get started**

```bash
# Pull the latest image
docker pull ghcr.io/bksmick12/raftflow:latest

# Run a single node
docker run -d \
  --name raftflow-node-1 \
  -p 8080:8080 \
  -p 9090:9090 \
  -e NODE_ID=node-1 \
  -e ADDRESS=0.0.0.0:8080 \
  -e PEERS="" \
  -v ./data:/data \
  ghcr.io/bksmick12/raftflow:latest
```

### Option 2: Docker Compose (3-Node Cluster)

**Best for local development and testing**

1. Create `docker-compose.yml`:

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
```

2. Start the cluster:

```bash
# Create data directories
mkdir -p data/node-{1,2,3}/log data/node-{1,2,3}/snapshot

# Start the cluster
docker-compose up -d

# View logs
docker-compose logs -f
```

### Option 3: Build from Source

**Best for development and customization**

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

### Option 4: Kubernetes

**Best for production deployments**

```bash
# Apply the deployment
kubectl apply -f k8s/raftflow-deployment.yaml

# Check the pods
kubectl get pods -l app=raftflow

# View logs
kubectl logs -l app=raftflow -f
```

---

## 🏃‍♂️ Running Your First Cluster

### Step 1: Start 3 Nodes

Open three terminal windows and run:

**Terminal 1 (Node 1):**
```bash
./raftflow --node-id node-1 \
  --address localhost:8081 \
  --peers localhost:8082,localhost:8083 \
  --log-dir ./data/node-1/log \
  --snapshot-dir ./data/node-1/snapshot \
  --enable-metrics \
  --metrics-address :9091
```

**Terminal 2 (Node 2):**
```bash
./raftflow --node-id node-2 \
  --address localhost:8082 \
  --peers localhost:8081,localhost:8083 \
  --log-dir ./data/node-2/log \
  --snapshot-dir ./data/node-2/snapshot \
  --enable-metrics \
  --metrics-address :9092
```

**Terminal 3 (Node 3):**
```bash
./raftflow --node-id node-3 \
  --address localhost:8083 \
  --peers localhost:8081,localhost:8082 \
  --log-dir ./data/node-3/log \
  --snapshot-dir ./data/node-3/snapshot \
  --enable-metrics \
  --metrics-address :9093
```

### Step 2: Verify Cluster Health

```bash
# Check which node is the leader
curl http://localhost:8081/leader
curl http://localhost:8082/leader
curl http://localhost:8083/leader

# Get cluster state
curl http://localhost:8081/state

# Get node information
curl http://localhost:8081/nodes
```

### Step 3: Submit Your First Command

```bash
# Submit a command to the leader
curl -X POST http://localhost:8081/submit \
  -H "Content-Type: application/json" \
  -d '{"command": "set key=value"}'

# Expected response
# {"success": true, "index": 1}
```

---

## 🎮 Using the Demo

The demo provides an interactive way to explore RaftFlow:

```bash
# Start the demo
./raftflow-demo --cluster-size 3 --base-port 8080
```

### Demo Commands

| Command | Description |
|---------|-------------|
| `submit <command>` | Submit a command to the current leader |
| `state` | Show cluster state |
| `leader` | Show current leader |
| `nodes` | Show all node states |
| `help` | Show help message |
| `exit` | Exit the demo |

### Example Demo Session

```
raft-demo> state
Cluster State:
  Leader: node-1
  Term: 2
  Nodes:
    node-1: Leader (Term: 2, Commit: 5, Applied: 5)
    node-2: Follower (Term: 2, Commit: 5, Applied: 5)
    node-3: Follower (Term: 2, Commit: 5, Applied: 5)

raft-demo> submit "set user:123 = {name: 'John', age: 30}"
Command submitted to leader node-1: set user:123 = {name: 'John', age: 30}

raft-demo> leader
Current leader: node-1 (Term: 2)

raft-demo> exit
```

---

## 📊 Monitoring and Metrics

RaftFlow provides comprehensive Prometheus metrics out of the box.

### Access Metrics

```bash
# Access metrics endpoint
curl http://localhost:9091/metrics

# Or use Prometheus
scrape_configs:
  - job_name: 'raftflow'
    static_configs:
      - targets: ['localhost:9091', 'localhost:9092', 'localhost:9093']
```

### Key Metrics to Monitor

| Metric | Description | Target Value |
|--------|-------------|--------------|
| `raft_state_current_term` | Current term | Should be consistent across cluster |
| `raft_state_current_state` | Node state (0=Follower, 1=Candidate, 2=Leader) | One leader per cluster |
| `raft_election_started_total` | Elections started | Low frequency |
| `raft_election_won_total` | Elections won | One per leader |
| `raft_log_entries_appended_total` | Log entries appended | High and increasing |
| `raft_log_entries_committed_total` | Log entries committed | Should match appended |
| `raft_log_entries_applied_total` | Log entries applied | Should match committed |
| `raft_rpc_requests_received_total` | RPC requests received | High throughput |
| `raft_rpc_latency_seconds` | RPC latency | Low (P99 < 10ms) |
| `raft_leader_is_leader` | Is node the leader | 1 for leader, 0 for others |

### Grafana Dashboard

Import our pre-built Grafana dashboard:

```json
{
  "dashboard": {
    "title": "RaftFlow Cluster",
    "panels": [
      {
        "title": "Cluster Health",
        "type": "stat",
        "targets": [
          {
            "expr": "count(raft_leader_is_leader == 1)",
            "legendFormat": "Leaders"
          }
        ]
      },
      {
        "title": "Current Term",
        "type": "stat",
        "targets": [
          {
            "expr": "max(raft_state_current_term)",
            "legendFormat": "Term"
          }
        ]
      },
      {
        "title": "Throughput",
        "type": "graph",
        "targets": [
          {
            "expr": "rate(raft_log_entries_appended_total[1m])",
            "legendFormat": "Entries/s"
          }
        ]
      },
      {
        "title": "Latency",
        "type": "graph",
        "targets": [
          {
            "expr": "histogram_quantile(0.99, sum(rate(raft_rpc_latency_seconds_bucket[5m])) by (le))",
            "legendFormat": "P99 Latency"
          }
        ]
      }
    ]
  }
}
```

---

## 🔧 Configuration

### Command Line Flags

| Flag | Description | Default | Required |
|------|-------------|---------|----------|
| `--node-id` | Unique identifier for this node | - | ✅ |
| `--address` | Network address for this node | `localhost:8080` | ❌ |
| `--peers` | Comma-separated list of peer addresses | `""` | ❌ |
| `--log-dir` | Directory for persistent logs | `./data/log` | ❌ |
| `--snapshot-dir` | Directory for snapshots | `./data/snapshot` | ❌ |
| `--election-timeout-min` | Minimum election timeout | `1500ms` | ❌ |
| `--election-timeout-max` | Maximum election timeout | `3000ms` | ❌ |
| `--heartbeat-interval` | Heartbeat interval | `500ms` | ❌ |
| `--rpc-timeout` | RPC timeout | `1000ms` | ❌ |
| `--snapshot-interval` | Snapshot interval | `30s` | ❌ |
| `--snapshot-threshold` | Minimum entries before snapshotting | `1000` | ❌ |
| `--enable-metrics` | Enable Prometheus metrics | `true` | ❌ |
| `--metrics-address` | Address for metrics server | `:9090` | ❌ |

### Environment Variables

The same configuration can be set via environment variables:

```bash
# Set via environment variables
export NODE_ID=node-1
export ADDRESS=localhost:8080
export PEERS=localhost:8081,localhost:8082
export LOG_DIR=./data/log
export SNAPSHOT_DIR=./data/snapshot
export ENABLE_METRICS=true
export METRICS_ADDRESS=:9090

# Then run
./raftflow
```

### Configuration File (Future)

We're working on YAML configuration file support:

```yaml
# raftflow.yaml
node_id: node-1
address: localhost:8080
peers:
  - localhost:8081
  - localhost:8082

log_dir: ./data/log
snapshot_dir: ./data/snapshot

election_timeout:
  min: 1500ms
  max: 3000ms

heartbeat_interval: 500ms
rpc_timeout: 1000ms

snapshot:
  interval: 30s
  threshold: 1000

metrics:
  enabled: true
  address: :9090
```

---

## 🛠️ Development

### Building from Source

```bash
# Clone the repository
git clone https://github.com/BKSmick12/raftflow.git
cd raftflow

# Install dependencies
go mod download

# Build the binaries
make build

# Run tests
make test

# Run with race detector
go run -race ./cmd/raftflow
```

### Project Structure

```
raftflow/
├── cmd/
│   ├── raftflow/          # Main command
│   │   └── main.go
│   └── raftflow-demo/     # Demo command
│       └── main.go
├── internal/
│   ├── config/           # Configuration
│   ├── consensus/        # Core Raft implementation
│   ├── log/              # Log management
│   ├── network/          # RPC layer
│   ├── snapshot/         # Snapshot management
│   ├── storage/          # Persistent storage
│   └── util/             # Utilities
├── k8s/                 # Kubernetes manifests
├── test/                # Integration tests
├── Dockerfile           # Main Dockerfile
├── Dockerfile.demo      # Demo Dockerfile
├── Makefile             # Build automation
└── README.md            # Documentation
```

### Contributing

We welcome contributions! See [CONTRIBUTING.md](../CONTRIBUTING.md) for details.

---

## 🚨 Troubleshooting

### Common Issues

#### Issue: Node fails to start

**Symptoms**: Node exits immediately with error

**Solutions**:
1. Check that the `--node-id` is unique
2. Verify the `--address` is available
3. Ensure the `--log-dir` and `--snapshot-dir` exist and are writable
4. Check for port conflicts

**Debug**:
```bash
# Run with debug logging
./raftflow --node-id node-1 --address localhost:8080 --log-level debug
```

#### Issue: No leader elected

**Symptoms**: All nodes remain in Follower or Candidate state

**Solutions**:
1. Verify all nodes can communicate (check firewall, network connectivity)
2. Check that peer addresses are correct
3. Ensure election timeouts are not too long
4. Verify all nodes have the same cluster configuration

**Debug**:
```bash
# Check node states
curl http://localhost:8081/nodes

# Check logs for election activity
journalctl -u raftflow -f
```

#### Issue: High latency

**Symptoms**: Slow command processing, high RPC latency

**Solutions**:
1. Check network connectivity between nodes
2. Verify system resources (CPU, memory, disk I/O)
3. Reduce snapshot threshold if snapshots are frequent
4. Check for disk I/O bottlenecks

**Debug**:
```bash
# Check metrics
curl http://localhost:9091/metrics | grep raft_rpc_latency

# Check system resources
top
iostat -x 1
df -h
```

#### Issue: Disk space full

**Symptoms**: Node crashes, "no space left on device" errors

**Solutions**:
1. Clean up old log files
2. Reduce snapshot interval
3. Increase disk space
4. Configure log rotation

**Debug**:
```bash
# Check disk usage
du -sh ./data/

# Clean up old snapshots
rm -rf ./data/snapshot/*.json
```

### Getting Help

1. **Documentation**: https://raftflow.io/docs
2. **FAQ**: https://raftflow.io/faq
3. **Community Forums**: https://community.raftflow.io
4. **GitHub Issues**: https://github.com/BKSmick12/raftflow/issues
5. **Support**: support@raftflow.io (for paid customers)

---

## 📚 Next Steps

### After Getting Started

1. **Explore the Demo**: Run the interactive demo to understand RaftFlow
2. **Read the Documentation**: Dive deeper into features and configuration
3. **Join the Community**: Connect with other users and contributors
4. **Consider Production**: Evaluate RaftFlow for your production needs
5. **Contact Sales**: Discuss enterprise options and pricing

### Learning Resources

- **Documentation**: https://raftflow.io/docs
- **Tutorials**: https://raftflow.io/tutorials
- **API Reference**: https://raftflow.io/api
- **Examples**: https://github.com/BKSmick12/raftflow/tree/main/examples
- **Blog**: https://raftflow.io/blog
- **Webinars**: https://raftflow.io/webinars

### Production Checklist

- [ ] Evaluate performance with your workload
- [ ] Test failure scenarios (node failures, network partitions)
- [ ] Configure monitoring and alerting
- [ ] Set up backup and disaster recovery
- [ ] Plan for scaling (adding/removing nodes)
- [ ] Review security configuration
- [ ] Choose the right licensing plan
- [ ] Contact support for production guidance

---

## 🎉 Congratulations!

You've successfully gotten started with RaftFlow! Whether you're evaluating it for a project or ready to deploy to production, you now have a solid foundation for building reliable distributed systems.

**Remember**:
- Start small with a 3-node cluster
- Monitor your cluster health
- Test failure scenarios
- Scale as needed
- Reach out for help when needed

**We're here to help you succeed with RaftFlow!**

---

**Need More Help?**
- 📧 Email: support@raftflow.io
- 💬 Chat: https://raftflow.io/chat
- 📞 Phone: +1 (555) RAFT-SUPPORT
- 🌐 Web: https://raftflow.io

**Happy Consensus Building!** 🎊
