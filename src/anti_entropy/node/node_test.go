package node

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestNewNode(t *testing.T) {
	node := NewNode(1)
	if node.GetID() != 1 {
		t.Errorf("Expected node ID to be 1, got %d", node.GetID())
	}
	state := node.GetState()
	if len(state) != 0 {
		t.Errorf("Expected empty state, got %v", state)
	}
}

func TestUpdateState(t *testing.T) {
	node := NewNode(1)
	node.UpdateState("test", "value")
	state := node.GetState()
	if state["test"] != "value" {
		t.Errorf("Expected state[test] to be 'value', got %s", state["test"])
	}
}

func TestConcurrentAccess(t *testing.T) {
	// Simulates concurrent HTTP handlers and sync goroutines touching the
	// same node, as happens in main.go. Run with -race to catch regressions.
	node := NewNode(1)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				node.UpdateState(fmt.Sprintf("key%d", i), "value")
				_ = node.GetState()
				_ = node.GetVersion()
			}
		}(i)
	}
	wg.Wait()
}

func TestGossip(t *testing.T) {
	node1 := NewNode(1)
	node2 := NewNode(2)
	node1.AddPeer(node2)
	node2.AddPeer(node1)

	// First update from node1
	node1.UpdateState("test", "value1")
	time.Sleep(100 * time.Millisecond)

	// Second update from node2 with a newer value
	node2.UpdateState("test", "value2")
	node2.UpdateState("test", "value2") // Update twice to ensure higher version

	// Let nodes gossip multiple times to ensure state propagation
	for i := 0; i < 5; i++ { // Increased number of rounds
		node1.Gossip()
		time.Sleep(1100 * time.Millisecond)
		node2.Gossip()
		time.Sleep(1100 * time.Millisecond)

		// Check intermediate states
		state1 := node1.GetState()
		state2 := node2.GetState()
		if state1["test"] == "value2" && state2["test"] == "value2" {
			// States have converged, we can exit early
			return
		}
	}

	// Final state check
	state1 := node1.GetState()
	state2 := node2.GetState()
	if state1["test"] != "value2" {
		t.Errorf("Expected node1 state[test] to be 'value2', got %s", state1["test"])
	}
	if state2["test"] != "value2" {
		t.Errorf("Expected node2 state[test] to be 'value2', got %s", state2["test"])
	}
}

func TestMultipleKeys(t *testing.T) {
	node1 := NewNode(1)
	node2 := NewNode(2)
	node1.AddPeer(node2)
	node2.AddPeer(node1)

	// Set initial states with different keys
	node1.UpdateState("key1", "value1")
	node1.UpdateState("key2", "value2")
	node2.UpdateState("key3", "value3")

	// Let nodes gossip multiple times to ensure state propagation
	for i := 0; i < 5; i++ { // Increased number of rounds
		node1.Gossip()
		time.Sleep(1100 * time.Millisecond)
		node2.Gossip()
		time.Sleep(1100 * time.Millisecond)

		// Check intermediate states
		state1 := node1.GetState()
		state2 := node2.GetState()
		allSynced := true
		for _, key := range []string{"key1", "key2", "key3"} {
			if state1[key] != state2[key] {
				allSynced = false
				break
			}
		}
		if allSynced {
			// States have converged, we can exit early
			return
		}
	}

	// Final state check
	state1 := node1.GetState()
	state2 := node2.GetState()

	expectedKeys := []string{"key1", "key2", "key3"}
	expectedValues := []string{"value1", "value2", "value3"}

	for i, key := range expectedKeys {
		if state1[key] != expectedValues[i] {
			t.Errorf("Node1: Expected %s to be %s, got %s", key, expectedValues[i], state1[key])
		}
		if state2[key] != expectedValues[i] {
			t.Errorf("Node2: Expected %s to be %s, got %s", key, expectedValues[i], state2[key])
		}
	}
} 