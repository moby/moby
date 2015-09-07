package consul

import (
	"testing"
	"time"

	"github.com/docker/libkv/store"
	"github.com/docker/libkv/testutils"
	"github.com/stretchr/testify/assert"
)

func makeConsulClient(t *testing.T) store.Store {
	client := "localhost:8500"

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

func TestConsulStore(t *testing.T) {
	kv := makeConsulClient(t)
	backup := makeConsulClient(t)

	testutils.RunTestStore(t, kv, backup)
}

func TestGetActiveSession(t *testing.T) {
	kv := makeConsulClient(t)

	consul := kv.(*Consul)

	key := "foo"
	value := []byte("bar")

	// Put the first key with the Ephemeral flag
	err := kv.Put(key, value, &store.WriteOptions{Ephemeral: true})
	assert.NoError(t, err)

	// Session should not be empty
	session, err := consul.getActiveSession(key)
	assert.NoError(t, err)
	assert.NotEqual(t, session, "")

	// Delete the key
	err = kv.Delete(key)
	assert.NoError(t, err)

	// Check the session again, it should return nothing
	session, err = consul.getActiveSession(key)
	assert.NoError(t, err)
	assert.Equal(t, session, "")
}
