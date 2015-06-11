package store

import (
	"testing"
	"time"
)

func makeEtcdClient(t *testing.T) Store {
	client := "localhost:4001"

	kv, err := NewStore(
		ETCD,
		[]string{client},
		&Config{
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

	testStore(t, kv)
}
