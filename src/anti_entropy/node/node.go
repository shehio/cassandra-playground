package node

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

type Node struct {
    id       uint64
    mu       sync.Mutex
    state    map[string]string
    version  map[string]int
    knownPeers    []*Node
    lastGossip time.Time
}

func NewNode(id uint64) *Node {
    node := &Node{
        id:       id,
        state:    make(map[string]string),
        version:  make(map[string]int),
        knownPeers:    make([]*Node, 0),
        lastGossip: time.Now(),
    }
    return node
}

func (n *Node) AddPeer(peer *Node) {
    n.mu.Lock()
    defer n.mu.Unlock()
    n.knownPeers = append(n.knownPeers, peer)
}

func (n *Node) GetID() uint64 {
    return n.id
}

// GetState returns a copy of the node's state so callers can iterate it
// safely while other goroutines update the node.
func (n *Node) GetState() map[string]string {
    n.mu.Lock()
    defer n.mu.Unlock()
    state := make(map[string]string, len(n.state))
    for key, value := range n.state {
        state[key] = value
    }
    return state
}

// GetVersion returns a copy of the node's version map, see GetState.
func (n *Node) GetVersion() map[string]int {
    n.mu.Lock()
    defer n.mu.Unlock()
    version := make(map[string]int, len(n.version))
    for key, value := range n.version {
        version[key] = value
    }
    return version
}

func (n *Node) UpdateState(key, value string) {
    n.mu.Lock()
    defer n.mu.Unlock()
    n.state[key] = value
    n.version[key]++
}

func (n *Node) Gossip() {
    n.mu.Lock()
    if time.Since(n.lastGossip) < time.Second {
        n.mu.Unlock()
        return
    }
    n.lastGossip = time.Now()
    if len(n.knownPeers) == 0 {
        n.mu.Unlock()
        return
    }
    peer := n.knownPeers[rand.Intn(len(n.knownPeers))]
    n.mu.Unlock()

    fmt.Printf("\nNode %d gossiping with Node %d\n", n.id, peer.id)

    // Lock both nodes in a consistent order to avoid deadlock when two
    // nodes gossip with each other concurrently.
    first, second := n, peer
    if peer.id < n.id {
        first, second = peer, n
    }
    first.mu.Lock()
    second.mu.Lock()
    defer second.mu.Unlock()
    defer first.mu.Unlock()

    // Exchange states with peer
    for key, value := range peer.state {
        if n.version[key] < peer.version[key] {
            n.state[key] = value
            n.version[key] = peer.version[key]
            fmt.Printf("Node %d updated key %s to value %s (version %d)\n", n.id, key, value, n.version[key])
        }
    }

    // Also let peer learn from our state
    for key, value := range n.state {
        if peer.version[key] < n.version[key] {
            peer.state[key] = value
            peer.version[key] = n.version[key]
            fmt.Printf("Node %d updated key %s to value %s (version %d)\n", peer.id, key, value, peer.version[key])
        }
    }
}

func (n *Node) PrintState() {
    n.mu.Lock()
    defer n.mu.Unlock()
    fmt.Printf("\n=== Node %d State ===\n", n.id)
    if len(n.state) == 0 {
        fmt.Println("State is empty")
        return
    }
    for key, value := range n.state {
        fmt.Printf("Key: %s, Value: %s, Version: %d\n", key, value, n.version[key])
    }
    fmt.Println("==================")
}
