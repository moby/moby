package boltdb

import (
	"os"
	"testing"
	"time"

	"github.com/docker/libkv"
	"github.com/docker/libkv/store"
	"github.com/docker/libkv/testutils"
	"github.com/stretchr/testify/assert"
)

func makeBoltDBClient(t *testing.T) store.Store {
	kv, err := New([]string{"/tmp/not_exist_dir/__boltdbtest"}, &store.Config{Bucket: "boltDBTest"})

	if err != nil {
		t.Fatalf("cannot create store: %v", err)
	}

	return kv
}

func TestRegister(t *testing.T) {
	Register()

	kv, err := libkv.NewStore(
		store.BOLTDB,
		[]string{"/tmp/not_exist_dir/__boltdbtest"},
		&store.Config{Bucket: "boltDBTest"},
	)
	assert.NoError(t, err)
	assert.NotNil(t, kv)

	if _, ok := kv.(*BoltDB); !ok {
		t.Fatal("Error registering and initializing boltDB")
	}

	_ = os.Remove("/tmp/not_exist_dir/__boltdbtest")
}

// TestMultiplePersistConnection tests the second connection to a
// BoltDB fails when one is already open with PersistConnection flag
func TestMultiplePersistConnection(t *testing.T) {
	kv, err := libkv.NewStore(
		store.BOLTDB,
		[]string{"/tmp/not_exist_dir/__boltdbtest"},
		&store.Config{
			Bucket:            "boltDBTest",
			ConnectionTimeout: 1 * time.Second,
			PersistConnection: true},
	)
	assert.NoError(t, err)
	assert.NotNil(t, kv)

	if _, ok := kv.(*BoltDB); !ok {
		t.Fatal("Error registering and initializing boltDB")
	}

	// Must fail if multiple boltdb requests are made with a valid timeout
	kv, err = libkv.NewStore(
		store.BOLTDB,
		[]string{"/tmp/not_exist_dir/__boltdbtest"},
		&store.Config{
			Bucket:            "boltDBTest",
			ConnectionTimeout: 1 * time.Second,
			PersistConnection: true},
	)
	assert.Error(t, err)

	_ = os.Remove("/tmp/not_exist_dir/__boltdbtest")
}

// TestConcurrentConnection tests simultaenous get/put using
// two handles.
func TestConcurrentConnection(t *testing.T) {
	var err error
	kv1, err1 := libkv.NewStore(
		store.BOLTDB,
		[]string{"/tmp/__boltdbtest"},
		&store.Config{
			Bucket:            "boltDBTest",
			ConnectionTimeout: 1 * time.Second},
	)
	assert.NoError(t, err1)
	assert.NotNil(t, kv1)

	kv2, err2 := libkv.NewStore(
		store.BOLTDB,
		[]string{"/tmp/__boltdbtest"},
		&store.Config{Bucket: "boltDBTest",
			ConnectionTimeout: 1 * time.Second},
	)
	assert.NoError(t, err2)
	assert.NotNil(t, kv2)

	key1 := "TestKV1"
	value1 := []byte("TestVal1")
	err = kv1.Put(key1, value1, nil)
	assert.NoError(t, err)

	key2 := "TestKV2"
	value2 := []byte("TestVal2")
	err = kv2.Put(key2, value2, nil)
	assert.NoError(t, err)

	pair1, err1 := kv1.Get(key1)
	assert.NoError(t, err)
	if assert.NotNil(t, pair1) {
		assert.NotNil(t, pair1.Value)
	}
	assert.Equal(t, pair1.Value, value1)

	pair2, err2 := kv2.Get(key2)
	assert.NoError(t, err)
	if assert.NotNil(t, pair2) {
		assert.NotNil(t, pair2.Value)
	}
	assert.Equal(t, pair2.Value, value2)

	// AtomicPut using kv1 and kv2 should succeed
	_, _, err = kv1.AtomicPut(key1, []byte("TestnewVal1"), pair1, nil)
	assert.NoError(t, err)

	_, _, err = kv2.AtomicPut(key2, []byte("TestnewVal2"), pair2, nil)
	assert.NoError(t, err)

	testutils.RunTestCommon(t, kv1)
	testutils.RunTestCommon(t, kv2)

	kv1.Close()
	kv2.Close()

	_ = os.Remove("/tmp/__boltdbtest")
}

func TestBoldDBStore(t *testing.T) {
	kv := makeBoltDBClient(t)

	testutils.RunTestCommon(t, kv)
	testutils.RunTestAtomic(t, kv)

	_ = os.Remove("/tmp/not_exist_dir/__boltdbtest")
}
