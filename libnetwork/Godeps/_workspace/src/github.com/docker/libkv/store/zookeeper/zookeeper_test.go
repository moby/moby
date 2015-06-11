package zookeeper

import (
	"testing"
	"time"

	"github.com/docker/libkv/store"
)

func makeZkClient(t *testing.T) store.Store {
	client := "localhost:2181"

	kv, err := InitializeZookeeper(
		[]string{client},
		&store.Config{
			ConnectionTimeout: 3 * time.Second,
			EphemeralTTL:      2 * time.Second,
		},
	)

	if err != nil {
		t.Fatalf("cannot create store: %v", err)
	}

	return kv
}

func TestZkStore(t *testing.T) {
	kv := makeZkClient(t)
	backup := makeZkClient(t)

	store.TestStore(t, kv, backup)
}
