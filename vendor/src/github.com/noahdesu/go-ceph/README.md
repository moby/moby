# go-ceph - Go bindings for Ceph APIs (RBD, RADOS, CephFS)

[![Build Status](https://travis-ci.org/noahdesu/go-ceph.svg)](https://travis-ci.org/noahdesu/go-ceph) [![Godoc](http://img.shields.io/badge/godoc-reference-blue.svg?style=flat)](https://godoc.org/github.com/noahdesu/go-ceph) [![license](http://img.shields.io/badge/license-MIT-red.svg?style=flat)](https://raw.githubusercontent.com/noahdesu/go-ceph/master/LICENSE)


This project uses Semantic Versioning (http://semver.org/).

## Installation

    go get github.com/noahdesu/go-ceph

The native RADOS library and development headers are expected to be installed.

## Documentation

Detailed documentation is available at
<http://godoc.org/github.com/noahdesu/go-ceph>.

### Connecting to a cluster

Connect to a Ceph cluster using a configuration file located in the default
search paths.

```go
conn, _ := rados.NewConn()
conn.ReadDefaultConfigFile()
conn.Connect()
```

A connection can be shutdown by calling the `Shutdown` method on the
connection object (e.g. `conn.Shutdown()`). There are also other methods for
configuring the connection. Specific configuration options can be set:

```go
conn.SetConfigOption("log_file", "/dev/null")
```

and command line options can also be used using the `ParseCmdLineArgs` method.

```go
args := []string{ "--mon-host", "1.1.1.1" }
err := conn.ParseCmdLineArgs(args)
```

For other configuration options see the full documentation.

### Object I/O

Object in RADOS can be written to and read from with through an interface very
similar to a standard file I/O interface:

```go
// open a pool handle
ioctx, err := conn.OpenIOContext("mypool")

// write some data
bytes_in := []byte("input data")
err = ioctx.Write("obj", bytes_in, 0)

// read the data back out
bytes_out := make([]byte, len(bytes_in))
n_out, err := ioctx.Read("obj", bytes_out, 0)

if bytes_in != bytes_out {
    fmt.Println("Output is not input!")
}
```

### Pool maintenance

The list of pools in a cluster can be retreived using the `ListPools` method
on the connection object. On a new cluster the following code snippet:

```go
pools, _ := conn.ListPools()
fmt.Println(pools)
```

will produce the output `[data metadata rbd]`, along with any other pools that
might exist in your cluster. Pools can also be created and destroyed. The
following creates a new, empty pool with default settings.

```go
conn.MakePool("new_pool")
```

Deleting a pool is also easy. Call `DeletePool(name string)` on a connection object to
delete a pool with the given name. The following will delete the pool named
`new_pool` and remove all of the pool's data.

```go
conn.DeletePool("new_pool")
```

## Contributing

Contributions are welcome & greatly appreciated, every little bit helps. Make code changes via Github pull requests:

- Fork the repo and create a topic branch for every feature/fix. Avoid
  making changes directly on master branch.
- All incoming features should be accompanied with tests.
- Make sure that you run `go fmt` before submitting a change
  set. Alternatively the Makefile has a flag for this, so you can call
  `make fmt` as well.
- The integration tests can be run in a docker container, for this run:

```
make test-docker
```

