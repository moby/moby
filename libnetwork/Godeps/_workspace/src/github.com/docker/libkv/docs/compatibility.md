#Cross-Backend Compatibility

The value of `libkv` is not to duplicate the code for programs that should support multiple distributed K/V stores like the classic `Consul`/`etcd`/`zookeeper` trio.

This document provides with general guidelines for users willing to support those backends with the same code using `libkv`.

Please note that most of those workarounds are going to disappear in the future with `etcd` APIv3.

##Etcd directory/key distinction

`etcd` with APIv2 makes the distinction between keys and directories. The result with `libkv` is that when using the etcd driver:

- You cannot store values on directories
- You cannot invoke `WatchTree` (watching on child values), on a regular key

This is fundamentaly different than `Consul` and `zookeeper` which are more permissive and allow the same set of operations on keys and directories (called a Node for zookeeper).

Apiv3 is in the work for `etcd`, which removes this key/directory distinction, but until then you should follow these workarounds to make your `libkv` code work across backends.

###Put

`etcd` cannot put values on directories, so this puts a major restriction compared to `Consul` and `zookeeper`.

If you want to support all those three backends, you should make sure to only put data on **leaves**.

For example:

```go
_ := kv.Put("path/to/key/bis", []byte("foo"), nil)
_ := kv.Put("path/to/key", []byte("bar"), nil)
```

Will work on `Consul` and `zookeeper` but fail for `etcd`. This is because the first `Put` in the case of `etcd` will recursively create the directory hierarchy and `path/to/key` is now considered as a directory. Thus, values should always be stored on leaves if the support for the three backends is planned.

###WatchTree

When initializing the `WatchTree`, the natural way to do so is through the following code:

```go
key := "path/to/key"
if !kv.Exists(key) {
    err := kv.Put(key, []byte("data"), nil)
}
events, err := kv.WatchTree(key, nil)
```

The code above will not work across backends and etcd will fail on the `WatchTree` call. What happens exactly:

- `Consul` will create a regular `key` because it has no distinction between directories and keys. This is not an issue as we can invoke `WatchTree` on regular keys.
- `zookeeper` is going to create a `node` that can either be a directory or a key during the lifetime of a program but it does not matter as a directory can hold values and be watchable like a regular key.
- `etcd` is going to create a regular `key`. We cannot invoke `WatchTree` on regular keys using etcd.

To be cross-compatible between those three backends for `WatchTree`, we need to enforce a parameter that is only interpreted with `etcd` and which tells the client to create a `directory` instead of a key.

```go
key := "path/to/key"
if !kv.Exists(key) {
    // We enforce IsDir = true to make sure etcd creates a directory
    err := kv.Put(key, []byte("data"), &store.WriteOptions{IsDir:true})
}
events, err := kv.WatchTree(key, nil)
```

The code above will work for the three backends but make sure to not try to store any value at that path as the call to `Put` will fail for `etcd` (you can only put at `path/to/key/foo`, `path/to/key/bar` for example).

##Etcd distributed locking

There is `Lock` mechanisms baked in the `coreos/etcd/client` for now. Instead, `libkv` has its own implementation of a `Lock` on top of `etcd`.

The general workflow for the `Lock` is as follows:

- Call Lock concurrently on a `key` between threads/programs
- Only one will create that key, others are going to fail because the key has already been created
- The thread locking the key can get the right index to set the value of the key using Compare And Swap and effectively Lock and hold the key
- Other threads are given a wrong index to fail the Compare and Swap and block until the key has been released by the thread holding the Lock
- Lock seekers are setting up a Watch listening on that key and events happening on the key
- When the thread/program stops holding the lock, it deletes the key triggering a `delete` event that will notify all the other threads. In case the program crashes, the key has a TTL attached that will send an `expire` event when this TTL expires.
- Once everyone is notified, back to the first step. First come, first served with the Lock.

The whole Lock process is highly dependent on the `delete`/`expire` events of `etcd`. So don't expect the key to be still there once the Lock is released.

For example if the whole logic is to `Lock` a key and expect the value to still be there after it has been unlocked, it is not going to be cross-backend compatible with `Consul` and `zookeeper`. On the other end the `etcd` Lock can still be used to do Leader Election for example and still be cross-compatible with other backends.