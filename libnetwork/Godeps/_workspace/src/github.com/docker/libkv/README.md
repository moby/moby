# libkv

[![GoDoc](https://godoc.org/github.com/docker/libkv?status.png)](https://godoc.org/github.com/docker/libkv)
[![Build Status](https://travis-ci.org/docker/libkv.svg?branch=master)](https://travis-ci.org/docker/libkv)
[![Coverage Status](https://coveralls.io/repos/docker/libkv/badge.svg)](https://coveralls.io/r/docker/libkv)

`libkv` provides a `Go` native library to store metadata.

The goal of `libkv` is to abstract common store operations for multiple Key/Value backends and offer the same experience no matter which one of the backend you want to use.

For example, you can use it to store your metadata or for service discovery to register machines and endpoints inside your cluster.

You can also easily implement a generic *Leader Election* on top of it (see the [swarm/leadership](https://github.com/docker/swarm/tree/master/leadership) package).

As of now, `libkv` offers support for `Consul`, `Etcd`, `Zookeeper` and `BoltDB`.

## Example of usage

### Create a new store and use Put/Get

```go
package main

import (
	"fmt"
	"time"

	"github.com/docker/libkv"
	"github.com/docker/libkv/store"
	"github.com/docker/libkv/store/consul"
	log "github.com/Sirupsen/logrus"
)

func init() {
	// Register consul store to libkv
	consul.Register()
}

func main() {
	client := "localhost:8500"

	// Initialize a new store with consul
	kv, err := libkv.NewStore(
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

## TLS

The etcd backend supports etcd servers that require TLS Client Authentication.  Zookeeper and Consul support are planned.  This feature is somewhat experimental and the store.ClientTLSConfig struct may change to accommodate the additional backends.

## Warning

There are a few consistency issues with *etcd*, on the notion of *directory* and *key*. If you want to use the three KV backends in an interchangeable way, you should only put data on leaves (see [Issue 20](https://github.com/docker/libkv/issues/20) for more details). This will be fixed when *etcd* API v3 will be made available (API v3 drops the *directory/key* distinction). An official release for *libkv* with a tag is likely to come after this issue being marked as **solved**.

Other than that, you should expect the same experience for basic operations like `Get`/`Put`, etc.

Calls like `WatchTree` may return different events (or number of events) depending on the backend (for now, `Etcd` and `Consul` will likely return more events than `Zookeeper` that you should triage properly). Although you should be able to use it successfully to watch on events in an interchangeable way (see the **swarm/leadership** or **swarm/discovery** packages in **docker/swarm**).

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

Copyright © 2014-2015 Docker, Inc. All rights reserved, except as follows. Code is released under the Apache 2.0 license. Documentation is licensed to end users under the Creative Commons Attribution 4.0 International License under the terms and conditions set forth in the file "LICENSE.docs". You may obtain a duplicate copy of the same license, titled CC-BY-SA-4.0, at http://creativecommons.org/licenses/by/4.0/.
