package synapse

import (
	"context"
	"fmt"
	"sync"
)

// MemKVStore is a simple in-memory key-value store with namespace isolation.
// Each plugin gets its own namespace, preventing cross-plugin data leaks.
type MemKVStore struct {
	mu   sync.RWMutex
	data map[string]map[string][]byte // namespace → key → value
}

// NewMemKVStore creates a new in-memory KV store.
func NewMemKVStore() *MemKVStore {
	return &MemKVStore{
		data: make(map[string]map[string][]byte),
	}
}

// Set stores a value under the given namespace and key.
func (s *MemKVStore) Set(_ context.Context, namespace, key string, value []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.data[namespace]; !exists {
		s.data[namespace] = make(map[string][]byte)
	}

	// Store a copy
	cp := make([]byte, len(value))
	copy(cp, value)
	s.data[namespace][key] = cp
	return nil
}

// Get retrieves a value by namespace and key. Returns a copy.
func (s *MemKVStore) Get(_ context.Context, namespace, key string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ns, exists := s.data[namespace]
	if !exists {
		return nil, fmt.Errorf("key %q not found in namespace %q", key, namespace)
	}

	val, exists := ns[key]
	if !exists {
		return nil, fmt.Errorf("key %q not found in namespace %q", key, namespace)
	}

	// Return a copy to prevent mutation
	cp := make([]byte, len(val))
	copy(cp, val)
	return cp, nil
}

// Delete removes a key from a namespace.
func (s *MemKVStore) Delete(_ context.Context, namespace, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if ns, exists := s.data[namespace]; exists {
		delete(ns, key)
	}
	return nil
}
