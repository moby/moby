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

func TestTimeout(t *testing.T) {
	kv, err := libkv.NewStore(
		store.BOLTDB,
		[]string{"/tmp/not_exist_dir/__boltdbtest"},
		&store.Config{Bucket: "boltDBTest", ConnectionTimeout: 1 * time.Second},
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
		&store.Config{Bucket: "boltDBTest", ConnectionTimeout: 1 * time.Second},
	)
	assert.Error(t, err)

	_ = os.Remove("/tmp/not_exist_dir/__boltdbtest")
}

func TestBoldDBStore(t *testing.T) {
	kv := makeBoltDBClient(t)

	testutils.RunTestCommon(t, kv)
	testutils.RunTestAtomic(t, kv)

	_ = os.Remove("/tmp/not_exist_dir/__boltdbtest")
}
