package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/shehio/anti-entropy/src/anti_entropy/merkle"
	"github.com/shehio/anti-entropy/src/anti_entropy/node"
)

var n *node.Node
var merkleMu sync.Mutex
var merkleTree *merkle.MerkleTree
var httpClient = &http.Client{Timeout: time.Second * 10}

func main() {
	nodeIDStr := os.Getenv("NODE_ID")
	if nodeIDStr == "" {
		log.Fatal("NODE_ID environment variable is required")
	}
	nodeID, err := strconv.ParseUint(nodeIDStr, 10, 64)
	if err != nil {
		log.Fatalf("Invalid NODE_ID: %v", err)
	}

	n = node.NewNode(nodeID)

	var peers []string
	peerNodes := os.Getenv("PEER_NODES")
	if peerNodes != "" {
		peers = strings.Split(peerNodes, ",")
		for _, peer := range peers {
			fmt.Printf("Node %d knows about peer: %s\n", nodeID, peer)
		}
	}

	// Initialize Merkle tree with empty state
	merkleTree = merkle.NewMerkleTree([][]byte{})

	// Periodic anti-entropy: sync with a random peer every 5 seconds.
	go func() {
		for {
			time.Sleep(time.Second * 5)
			if len(peers) > 0 {
				syncWithPeer(peers[rand.Intn(len(peers))])
			}
		}
	}()

	http.HandleFunc("/state", handleState)
	http.HandleFunc("/gossip", handleGossip)
	http.HandleFunc("/merkle/root", handleMerkleRoot)
	http.HandleFunc("/merkle/verify", handleMerkleVerify)
	http.HandleFunc("/sync", handleSync)

	port := 8080
	fmt.Printf("Node %d starting on port %d...\n", nodeID, port)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil); err != nil {
		log.Fatal(err)
	}
}

func handleState(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		state := n.GetState()
		json.NewEncoder(w).Encode(state)
	case http.MethodPost:
		var req struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		n.UpdateState(req.Key, req.Value)
		// Update Merkle tree with new state
		updateMerkleTree()
		w.WriteHeader(http.StatusOK)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleGossip(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get peer nodes from environment variable
	peerNodes := os.Getenv("PEER_NODES")
	if peerNodes != "" {
		peers := strings.Split(peerNodes, ",")
		for _, peer := range peers {
			// Send sync request to each peer
			go syncWithPeer(peer)
		}
	}

	n.Gossip()
	w.WriteHeader(http.StatusOK)
}

func handleMerkleRoot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	merkleMu.Lock()
	rootHash := merkleTree.GetRootHash()
	merkleMu.Unlock()
	json.NewEncoder(w).Encode(map[string]string{"root_hash": rootHash})
}

func handleMerkleVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Data []byte `json:"data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	merkleMu.Lock()
	tree := merkleTree
	merkleMu.Unlock()
	isValid := tree.Verify(req.Data)
	json.NewEncoder(w).Encode(map[string]bool{"valid": isValid})
}

func handleSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		State   map[string]string `json:"state"`
		Version map[string]int    `json:"version"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Update state based on received data
	for key, value := range req.State {
		if n.GetVersion()[key] < req.Version[key] {
			n.UpdateState(key, value)
		}
	}

	updateMerkleTree()

	response := struct {
		State   map[string]string `json:"state"`
		Version map[string]int    `json:"version"`
	}{
		State:   n.GetState(),
		Version: n.GetVersion(),
	}

	json.NewEncoder(w).Encode(response)
}

func syncWithPeer(peer string) {
	state := n.GetState()
	version := n.GetVersion()

	reqBody := struct {
		State   map[string]string `json:"state"`
		Version map[string]int    `json:"version"`
	}{
		State:   state,
		Version: version,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		log.Printf("Error marshaling request: %v", err)
		return
	}

	resp, err := httpClient.Post(fmt.Sprintf("http://%s/sync", peer), "application/json", strings.NewReader(string(jsonData)))
	if err != nil {
		log.Printf("Error syncing with peer %s: %v", peer, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Error syncing with peer %s: status code %d", peer, resp.StatusCode)
		return
	}

	var response struct {
		State   map[string]string `json:"state"`
		Version map[string]int    `json:"version"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		log.Printf("Error decoding response from peer %s: %v", peer, err)
		return
	}

	// Update our state based on response
	for key, value := range response.State {
		if n.GetVersion()[key] < response.Version[key] {
			n.UpdateState(key, value)
		}
	}

	updateMerkleTree()
}

func updateMerkleTree() {
	type entry struct {
		key   string
		value string
	}
	
	entries := make([]entry, 0, len(n.GetState()))
	for key, value := range n.GetState() {
		entries = append(entries, entry{key, value})
	}
	
	// Sort entries by key
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].key < entries[j].key
	})
	
	var data [][]byte
	for _, e := range entries {
		entry := fmt.Sprintf("%s:%s", e.key, e.value)
		data = append(data, []byte(entry))
	}
	merkleMu.Lock()
	merkleTree = merkle.NewMerkleTree(data)
	merkleMu.Unlock()
} 