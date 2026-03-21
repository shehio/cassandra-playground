#!/bin/bash

# Build the project
bazel build //src/anti_entropy/merkle:merkle_test

# Run the Merkle tree tests
bazel test //src/anti_entropy/merkle:merkle_test

# Check if tests passed
if [ $? -eq 0 ]; then
    echo "Merkle tree tests passed successfully"
else
    echo "Merkle tree tests failed"
    exit 1
fi

# Start the cluster
docker-compose up -d

wait_for_service() {
    local host=$1
    local port=$2
    echo "Waiting for $host:$port to be ready..."
    while ! nc -z $host $port; do
        sleep 1
    done
    echo "$host:$port is ready!"
}

# Wait for all nodes to be ready
wait_for_service localhost 8081
wait_for_service localhost 8082
wait_for_service localhost 8083

# Add some test data to each node
echo "Adding test data to node1..."
curl -X POST -H "Content-Type: application/json" -d '{"key":"weather","value":"sunny"}' http://localhost:8081/state
curl -X POST -H "Content-Type: application/json" -d '{"key":"temperature","value":"25°C"}' http://localhost:8081/state

echo "Adding test data to node2..."
curl -X POST -H "Content-Type: application/json" -d '{"key":"humidity","value":"65%"}' http://localhost:8082/state
curl -X POST -H "Content-Type: application/json" -d '{"key":"wind_speed","value":"12 km/h"}' http://localhost:8082/state

echo "Adding test data to node3..."
curl -X POST -H "Content-Type: application/json" -d '{"key":"pressure","value":"1013 hPa"}' http://localhost:8083/state
curl -X POST -H "Content-Type: application/json" -d '{"key":"visibility","value":"10 km"}' http://localhost:8083/state

# Trigger gossip on all nodes
echo "Triggering gossip on all nodes..."
curl -X POST http://localhost:8081/gossip
curl -X POST http://localhost:8082/gossip
curl -X POST http://localhost:8083/gossip

# Wait for gossip to complete
sleep 5

# Get Merkle tree root hashes from all nodes
echo "Getting Merkle tree root hashes..."
echo "Node 1 root hash:"
curl http://localhost:8081/merkle/root

echo "Node 2 root hash:"
curl http://localhost:8082/merkle/root

echo "Node 3 root hash:"
curl http://localhost:8083/merkle/root

# Get states from all nodes to verify consistency
echo "Getting states from all nodes..."
echo "Node 1 state:"
curl http://localhost:8081/state

echo "Node 2 state:"
curl http://localhost:8082/state

echo "Node 3 state:"
curl http://localhost:8083/state
