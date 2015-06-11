package etcd

import (
	"testing"
	"time"

	"github.com/docker/libkv/store"
)

func makeEtcdClient(t *testing.T) store.Store {
	client := "localhost:4001"

	kv, err := InitializeEtcd(
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

func TestEtcdStore(t *testing.T) {
	kv := makeEtcdClient(t)
	backup := makeEtcdClient(t)

	store.TestStore(t, kv, backup)
}
