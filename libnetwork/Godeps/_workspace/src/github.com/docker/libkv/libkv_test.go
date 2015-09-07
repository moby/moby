package libkv

import (
	"testing"
	"time"

	"github.com/docker/libkv/store"
	"github.com/docker/libkv/store/consul"
	"github.com/docker/libkv/store/etcd"
	"github.com/docker/libkv/store/zookeeper"
	"github.com/stretchr/testify/assert"
)

func TestNewStoreConsul(t *testing.T) {
	client := "localhost:8500"

	kv, err := NewStore(
		store.CONSUL,
		[]string{client},
		&store.Config{
			ConnectionTimeout: 10 * time.Second,
		},
	)
	assert.NoError(t, err)
	assert.NotNil(t, kv)

	if _, ok := kv.(*consul.Consul); !ok {
		t.Fatal("Error while initializing store consul")
	}
}

func TestNewStoreEtcd(t *testing.T) {
	client := "localhost:4001"

	kv, err := NewStore(
		store.ETCD,
		[]string{client},
		&store.Config{
			ConnectionTimeout: 10 * time.Second,
		},
	)
	assert.NoError(t, err)
	assert.NotNil(t, kv)

	if _, ok := kv.(*etcd.Etcd); !ok {
		t.Fatal("Error while initializing store etcd")
	}
}

func TestNewStoreZookeeper(t *testing.T) {
	client := "localhost:2181"

	kv, err := NewStore(
		store.ZK,
		[]string{client},
		&store.Config{
			ConnectionTimeout: 10 * time.Second,
		},
	)
	assert.NoError(t, err)
	assert.NotNil(t, kv)

	if _, ok := kv.(*zookeeper.Zookeeper); !ok {
		t.Fatal("Error while initializing store zookeeper")
	}
}

func TestNewStoreUnsupported(t *testing.T) {
	client := "localhost:9999"

	kv, err := NewStore(
		"unsupported",
		[]string{client},
		&store.Config{
			ConnectionTimeout: 10 * time.Second,
		},
	)
	assert.Error(t, err)
	assert.Nil(t, kv)
}
