package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/shehio/anti-entropy/src/anti_entropy/merkle"
	"github.com/shehio/anti-entropy/src/anti_entropy/node"
)

func TestMerkleRootConcurrentAccess(t *testing.T) {
	n = node.NewNode(1)
	merkleTree = merkle.NewMerkleTree([][]byte{})

	var wg sync.WaitGroup
	const iterations = 100

	// Concurrent writes via updateMerkleTree
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			n.UpdateState(fmt.Sprintf("key%d", i), fmt.Sprintf("value%d", i))
			updateMerkleTree()
		}
	}()

	// Concurrent reads via handleMerkleRoot
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			req := httptest.NewRequest(http.MethodGet, "/merkle/root", nil)
			w := httptest.NewRecorder()
			handleMerkleRoot(w, req)
			if w.Code != http.StatusOK {
				t.Errorf("handleMerkleRoot returned status %d", w.Code)
			}
		}
	}()

	wg.Wait()
}

func TestSyncWithPeerConcurrentAccess(t *testing.T) {
	n = node.NewNode(1)
	merkleTree = merkle.NewMerkleTree([][]byte{})

	// Mock peer server that always returns a different root to trigger full sync
	peer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/merkle/root":
			json.NewEncoder(w).Encode(map[string]string{"root_hash": "different"})
		case "/sync":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"state":   map[string]string{"peerKey": "peerValue"},
				"version": map[string]int{"peerKey": 1},
			})
		}
	}))
	defer peer.Close()

	peerAddr := peer.Listener.Addr().String()

	var wg sync.WaitGroup
	const iterations = 100

	// Concurrent state updates + merkle tree rebuilds
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			n.UpdateState(fmt.Sprintf("key%d", i), fmt.Sprintf("value%d", i))
			updateMerkleTree()
		}
	}()

	// Concurrent syncs reading merkleTree for root comparison
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			syncWithPeer(peerAddr)
		}
	}()

	// Concurrent reads via handleMerkleRoot
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			req := httptest.NewRequest(http.MethodGet, "/merkle/root", nil)
			w := httptest.NewRecorder()
			handleMerkleRoot(w, req)
			if w.Code != http.StatusOK {
				t.Errorf("handleMerkleRoot returned status %d", w.Code)
			}
		}
	}()

	wg.Wait()
}

func TestMerkleRootNilTree(t *testing.T) {
	n = node.NewNode(1)
	merkleTreeMu.Lock()
	merkleTree = nil
	merkleTreeMu.Unlock()

	req := httptest.NewRequest(http.MethodGet, "/merkle/root", nil)
	w := httptest.NewRecorder()
	handleMerkleRoot(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", w.Code)
	}
}

func TestMerkleVerifyNilTree(t *testing.T) {
	n = node.NewNode(1)
	merkleTreeMu.Lock()
	merkleTree = nil
	merkleTreeMu.Unlock()

	req := httptest.NewRequest(http.MethodPost, "/merkle/verify", nil)
	w := httptest.NewRecorder()
	handleMerkleVerify(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", w.Code)
	}
}
