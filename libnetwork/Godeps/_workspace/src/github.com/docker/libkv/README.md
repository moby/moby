# libkv

[![GoDoc](https://godoc.org/github.com/docker/libkv?status.png)](https://godoc.org/github.com/docker/libkv)
[![Build Status](https://travis-ci.org/docker/libkv.svg?branch=master)](https://travis-ci.org/docker/libkv)
[![Coverage Status](https://coveralls.io/repos/docker/libkv/badge.svg)](https://coveralls.io/r/docker/libkv)

`libkv` provides a `Go` native library to store metadata.

The goal of `libkv` is to abstract common store operations for multiple Key/Value backends and offer the same experience no matter which one of the backend you want to use.

For example, you can use it to store your metadata or for service discovery to register machines and endpoints inside your cluster.

You can also easily implement a generic *Leader Election* on top of it (see the [swarm/leadership](https://github.com/docker/swarm/tree/master/leadership) package).

As of now, `libkv` offers support for `Consul`, `Etcd` and `Zookeeper`.

## Example of usage

### Create a new store and use Put/Get

```go
package main

import (
	"fmt"
	"time"
	
	"github.com/docker/libkv"
	"github.com/docker/libkv/store"
	log "github.com/Sirupsen/logrus"
)

func main() {
	client := "localhost:8500"

	// Initialize a new store with consul
	kv, err = libkv.NewStore(
		store.CONSUL, // or "consul"
		[]string{client},
		&store.Config{
			ConnectionTimeout: 10*time.Second,
		},
	)
	if err != nil {
		log.Fatal("Cannot create store consul")
	}

	key := "foo"
	err = kv.Put(key, []byte("bar"), nil)
	if err != nil {
		log.Error("Error trying to put value at key `", key, "`")
	}

	pair, err := kv.Get(key)
	if err != nil {
		log.Error("Error trying accessing value at key `", key, "`")
	}

	log.Info("value: ", string(pair.Value))
}
```

You can find other usage examples for `libkv` under the `docker/swarm` or `docker/libnetwork` repositories.

## Details

You should expect the same experience for basic operations like `Get`/`Put`, etc.

However calls like `WatchTree` may return different events (or number of events) depending on the backend (for now, `Etcd` and `Consul` will likely return more events than `Zookeeper` that you should triage properly).

## Create a new storage backend

A new **storage backend** should include those calls:

```go
type Store interface {
	Put(key string, value []byte, options *WriteOptions) error
	Get(key string) (*KVPair, error)
	Delete(key string) error
	Exists(key string) (bool, error)
	Watch(key string, stopCh <-chan struct{}) (<-chan *KVPair, error)
	WatchTree(directory string, stopCh <-chan struct{}) (<-chan []*KVPair, error)
	NewLock(key string, options *LockOptions) (Locker, error)
	List(directory string) ([]*KVPair, error)
	DeleteTree(directory string) error
	AtomicPut(key string, value []byte, previous *KVPair, options *WriteOptions) (bool, *KVPair, error)
	AtomicDelete(key string, previous *KVPair) (bool, error)
	Close()
}
```

You can get inspiration from existing backends to create a new one. This interface could be subject to changes to improve the experience of using the library and contributing to a new backend.

##Roadmap

- Make the API nicer to use (using `options`)
- Provide more options (`consistency` for example)
- Improve performance (remove extras `Get`/`List` operations)
- Add more exhaustive tests
- New backends?

##Contributing

Want to hack on libkv? [Docker's contributions guidelines](https://github.com/docker/docker/blob/master/CONTRIBUTING.md) apply.

##Copyright and license

Code and documentation copyright 2015 Docker, inc. Code released under the Apache 2.0 license. Docs released under Creative commons.
