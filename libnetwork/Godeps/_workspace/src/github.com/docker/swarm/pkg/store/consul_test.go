package store

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func makeConsulClient(t *testing.T) Store {
	client := "localhost:8500"

	kv, err := NewStore(
		CONSUL,
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

func TestConsulStore(t *testing.T) {
	kv := makeConsulClient(t)

	testStore(t, kv)
}

func TestCreateEphemeralSession(t *testing.T) {
	kv := makeConsulClient(t)

	consul := kv.(*Consul)

	err := consul.createEphemeralSession()
	assert.NoError(t, err)
	assert.NotEqual(t, consul.ephemeralSession, "")
}

func TestCheckActiveSession(t *testing.T) {
	kv := makeConsulClient(t)

	consul := kv.(*Consul)

	key := "foo"
	value := []byte("bar")

	// Put the first key with the Ephemeral flag
	err := kv.Put(key, value, &WriteOptions{Ephemeral: true})
	assert.NoError(t, err)

	// Session should not be empty
	session, err := consul.checkActiveSession(key)
	assert.NoError(t, err)
	assert.NotEqual(t, session, "")

	// Delete the key
	err = kv.Delete(key)
	assert.NoError(t, err)

	// Check the session again, it should return nothing
	session, err = consul.checkActiveSession(key)
	assert.NoError(t, err)
	assert.Equal(t, session, "")
}
