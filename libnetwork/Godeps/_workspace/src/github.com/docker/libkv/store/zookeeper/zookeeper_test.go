package zookeeper

import (
	"testing"
	"time"

	"github.com/docker/libkv/store"
	"github.com/docker/libkv/testutils"
)

func makeZkClient(t *testing.T) store.Store {
	client := "localhost:2181"

	kv, err := New(
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

	testutils.RunTestStore(t, kv, backup)
}
