# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the main application
RUN CGO_ENABLED=0 GOOS=linux go build -o raftflow ./cmd/raftflow

# Runtime stage
FROM alpine:3.18

WORKDIR /app

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

# Copy binary from builder
COPY --from=builder /app/raftflow .

# Create data directories
RUN mkdir -p /data/log /data/snapshot

# Copy configuration (if any)
COPY --from=builder /app/config.yaml .

# Set environment variables
ENV NODE_ID=""
ENV ADDRESS="0.0.0.0:8080"
ENV PEERS=""
ENV LOG_DIR="/data/log"
ENV SNAPSHOT_DIR="/data/snapshot"
ENV ENABLE_METRICS="true"
ENV METRICS_ADDRESS=":9090"

EXPOSE 8080 9090

CMD ["./raftflow"]
