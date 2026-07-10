# Anti-Entropy Repair by Gossip Protocol

[![Build](https://github.com/shehio/cassandra-playground/actions/workflows/build.yml/badge.svg)](https://github.com/shehio/cassandra-playground/actions/workflows/build.yml)

A playground implementation of Cassandra-style anti-entropy repair for eventual consistency. Nodes gossip their state to peers and use Merkle trees for efficient state comparison and synchronization.

## Features

- Distributed state management
- Merkle tree-based state comparison
- HTTP-based node communication
- Docker containerization support
- Multi-node testing capabilities

## Prerequisites

- Go 1.24 or later
- Docker and Docker Compose
- Bazel build system (optional — the plain Go toolchain works too)

## Building

To build the project:

```bash
bazel build //src/anti_entropy:anti_entropy
```

or with the Go toolchain directly:

```bash
go build ./...
```

## Running Tests

### Unit Tests

Run all unit tests:

```bash
bazel test //...
```

or:

```bash
go test ./...
```

### Integration Tests

Start the three-node cluster first (see Docker Setup below), then run the Merkle tree integration tests:

```bash
./test_merkle.sh
```

This script will:
1. Build the project
2. Run Merkle tree tests
3. Wait for the three nodes to be reachable
4. Add test data
5. Trigger gossip
6. Verify state consistency

## Docker Setup

The project includes Docker support for easy deployment and testing.

### Building the Docker Image

```bash
docker build -t anti-entropy .
```

### Running with Docker Compose

Start a cluster of three nodes:

```bash
docker compose up
```

This will start three nodes with the following configuration:
- Node 1: Port 8081
- Node 2: Port 8082
- Node 3: Port 8083

Each node syncs with a random peer every 5 seconds; gossip can also be triggered manually via `POST /gossip`.

## API Endpoints

Each node exposes the following HTTP endpoints:

- `GET /state` - Get current node state
- `POST /state` - Update node state (`{"key": "...", "value": "..."}`)
- `POST /gossip` - Trigger gossip with peers
- `GET /merkle/root` - Get Merkle tree root hash
- `POST /merkle/verify` - Verify data against Merkle tree (`{"data": "<base64-encoded bytes>"}`)
- `POST /sync` - Synchronize state with peers

## Testing Results

The system has been tested with the following results:
- All unit tests passing
- Successful state synchronization between nodes
- Identical Merkle tree root hashes across all nodes
- Consistent state across the cluster

## License

This project is licensed under the MIT License - see the LICENSE file for details.
