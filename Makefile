# Makefile for raftflow project

.PHONY: all build test clean docker docker-demo run run-demo lint fmt vet

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
BINARY_NAME=raftflow
DEMO_BINARY_NAME=raftflow-demo

# Build flags
LDFLAGS=-ldflags "-X main.Version=$(VERSION)"

# Directories
SRC_DIR=.
CMD_DIR=./cmd
RAFTFLOW_CMD=$(CMD_DIR)/raftflow
DEMO_CMD=$(CMD_DIR)/raftflow-demo
BIN_DIR=./bin
DIST_DIR=./dist

# Docker
DOCKER=docker
DOCKER_BUILD=$(DOCKER) build
DOCKER_TAG=ghcr.io/bksmick12/raftflow
DOCKER_DEMO_TAG=ghcr.io/bksmick12/raftflow-demo

# Kubernetes
KUBECTL=kubectl

all: build test

build: $(BIN_DIR)/$(BINARY_NAME) $(BIN_DIR)/$(DEMO_BINARY_NAME)

$(BIN_DIR)/$(BINARY_NAME):
	@mkdir -p $(BIN_DIR)
	$(GOBUILD) -o $@ $(LDFLAGS) $(RAFTFLOW_CMD)

$(BIN_DIR)/$(DEMO_BINARY_NAME):
	@mkdir -p $(BIN_DIR)
	$(GOBUILD) -o $@ $(LDFLAGS) $(DEMO_CMD)

test: test-all

test-all: test-unit test-integration

test-unit:
	$(GOTEST) -v -race ./internal/...

test-integration:
	$(GOTEST) -v -race ./test/...

clean:
	rm -rf $(BIN_DIR)
	rm -rf $(DIST_DIR)
	rm -rf ./data
	rm -rf ./demo-data

fmt:
	$(GOCMD) fmt ./...

lint:
	golangci-lint run ./...

vet:
	$(GOCMD) vet ./...

# Docker targets
docker: docker-build

docker-build: docker-build-main docker-build-demo

docker-build-main:
	$(DOCKER_BUILD) -t $(DOCKER_TAG):latest -f Dockerfile .

docker-build-demo:
	$(DOCKER_BUILD) -t $(DOCKER_DEMO_TAG):latest -f Dockerfile.demo .

docker-push: docker-push-main docker-push-demo

docker-push-main:
	$(DOCKER) push $(DOCKER_TAG):latest

docker-push-demo:
	$(DOCKER) push $(DOCKER_DEMO_TAG):latest

# Run targets
run: build
	@echo "Running raftflow node..."
	@$(BIN_DIR)/$(BINARY_NAME) --help

run-demo: build
	@echo "Running raftflow demo..."
	@$(BIN_DIR)/$(DEMO_BINARY_NAME) --help

# Kubernetes targets
k8s-apply: k8s-apply-raftflow k8s-apply-demo

k8s-apply-raftflow:
	$(KUBECTL) apply -f k8s/raftflow-deployment.yaml

k8s-apply-demo:
	$(KUBECTL) apply -f k8s/raftflow-demo-deployment.yaml

k8s-delete: k8s-delete-raftflow k8s-delete-demo

k8s-delete-raftflow:
	$(KUBECTL) delete -f k8s/raftflow-deployment.yaml

k8s-delete-demo:
	$(KUBECTL) delete -f k8s/raftflow-demo-deployment.yaml

# Dependency management
deps:
	$(GOMOD) download
	$(GOMOD) tidy

# Generate protobuf files (if needed)
# proto:
# 	protoc --go_out=. --go_opt=paths=source_relative \
# 		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
# 		proto/*.proto

# Build everything for release
release: clean build docker-build
	@mkdir -p $(DIST_DIR)
	cp $(BIN_DIR)/* $(DIST_DIR)/
	cp Dockerfile $(DIST_DIR)/
	cp Dockerfile.demo $(DIST_DIR)/
	cp -r k8s $(DIST_DIR)/

# Run a local demo cluster
local-demo:
	@echo "Starting local demo cluster..."
	@mkdir -p demo-data
	@echo "Starting 3 raftflow nodes..."
	@gnome-terminal --tab --title="Raft Node 1" -- bash -c "$(BIN_DIR)/$(BINARY_NAME) --node-id node-1 --address localhost:8081 --peers localhost:8082,localhost:8083 --log-dir ./demo-data/node-1/log --snapshot-dir ./demo-data/node-1/snapshot --enable-metrics --metrics-address :9091; exec bash"
	@gnome-terminal --tab --title="Raft Node 2" -- bash -c "$(BIN_DIR)/$(BINARY_NAME) --node-id node-2 --address localhost:8082 --peers localhost:8081,localhost:8083 --log-dir ./demo-data/node-2/log --snapshot-dir ./demo-data/node-2/snapshot --enable-metrics --metrics-address :9092; exec bash"
	@gnome-terminal --tab --title="Raft Node 3" -- bash -c "$(BIN_DIR)/$(BINARY_NAME) --node-id node-3 --address localhost:8083 --peers localhost:8081,localhost:8082 --log-dir ./demo-data/node-3/log --snapshot-dir ./demo-data/node-3/snapshot --enable-metrics --metrics-address :9093; exec bash"
	@echo "Nodes started in separate terminal tabs"
	@echo "Run the demo: $(BIN_DIR)/$(DEMO_BINARY_NAME) --cluster-size 3 --base-port 8081"

# Help
help:
	@echo "Available targets:"
	@echo "  all           - Build and test everything"
	@echo "  build         - Build the binaries"
	@echo "  test          - Run all tests"
	@echo "  test-unit     - Run unit tests"
	@echo "  test-integration - Run integration tests"
	@echo "  clean         - Clean build artifacts"
	@echo "  fmt           - Format code"
	@echo "  lint          - Run linter"
	@echo "  vet           - Run go vet"
	@echo "  docker        - Build Docker images"
	@echo "  docker-push   - Push Docker images"
	@echo "  run           - Run the raftflow node"
	@echo "  run-demo      - Run the raftflow demo"
	@echo "  k8s-apply     - Apply Kubernetes manifests"
	@echo "  k8s-delete    - Delete Kubernetes resources"
	@echo "  deps          - Download dependencies"
	@echo "  release       - Build release artifacts"
	@echo "  local-demo    - Start a local demo cluster"
	@echo "  help          - Show this help message"
