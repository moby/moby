package c8dutil

import (
	"sync"

	"github.com/opencontainers/go-digest"
)

type memoryLabelStore struct {
	mu     sync.Mutex
	labels map[digest.Digest]map[string]string
}

func NewMemoryLabelStore() *memoryLabelStore {
	return &memoryLabelStore{
		labels: make(map[digest.Digest]map[string]string),
	}
}

// Get returns all the labels for the given digest
func (s *memoryLabelStore) Get(dgst digest.Digest) (map[string]string, error) {
	s.mu.Lock()
	labels := s.labels[dgst]
	s.mu.Unlock()
	return labels, nil
}

// Set sets all the labels for a given digest
func (s *memoryLabelStore) Set(dgst digest.Digest, labels map[string]string) error {
	s.mu.Lock()
	if s.labels == nil {
		s.labels = make(map[digest.Digest]map[string]string)
	}
	s.labels[dgst] = labels
	s.mu.Unlock()
	return nil
}

// Update replaces the given labels for a digest,
// a key with an empty value removes a label.
func (s *memoryLabelStore) Update(dgst digest.Digest, update map[string]string) (map[string]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	labels, ok := s.labels[dgst]
	if !ok {
		labels = map[string]string{}
	}
	for k, v := range update {
		labels[k] = v
	}
	if s.labels == nil {
		s.labels = map[digest.Digest]map[string]string{}
	}
	s.labels[dgst] = labels

	return labels, nil
}
