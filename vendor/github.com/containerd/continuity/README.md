# continuity

[![GoDoc](https://godoc.org/github.com/containerd/continuity?status.svg)](https://godoc.org/github.com/containerd/continuity)
[![Build Status](https://travis-ci.org/containerd/continuity.svg?branch=master)](https://travis-ci.org/containerd/continuity)

A transport-agnostic, filesystem metadata manifest system

This project is a staging area for experiments in providing transport agnostic
metadata storage.

Please see https://github.com/opencontainers/specs/issues/11 for more details.

## Manifest Format

A continuity manifest encodes filesystem metadata in Protocol Buffers.
Please refer to [proto/manifest.proto](proto/manifest.proto).

## Usage

Build:

```console
$ make
```

Create a manifest (of this repo itself):

```console
$ ./bin/continuity build . > /tmp/a.pb
```

Dump a manifest:

```console
$ ./bin/continuity ls /tmp/a.pb
...
-rw-rw-r--      270 B   /.gitignore
-rw-rw-r--      88 B    /.mailmap
-rw-rw-r--      187 B   /.travis.yml
-rw-rw-r--      359 B   /AUTHORS
-rw-rw-r--      11 kB   /LICENSE
-rw-rw-r--      1.5 kB  /Makefile
...
-rw-rw-r--      986 B   /testutil_test.go
drwxrwxr-x      0 B     /version
-rw-rw-r--      478 B   /version/version.go
```

Verify a manifest:

```console
$ ./bin/continuity verify . /tmp/a.pb
```

Break the directory and restore using the manifest:
```console
$ chmod 777 Makefile
$ ./bin/continuity verify . /tmp/a.pb
2017/06/23 08:00:34 error verifying manifest: resource "/Makefile" has incorrect mode: -rwxrwxrwx != -rw-rw-r--
$ ./bin/continuity apply . /tmp/a.pb
$ stat -c %a Makefile
664
$ ./bin/continuity verify . /tmp/a.pb
```


## Contribution Guide
### Building Proto Package

If you change the proto file you will need to rebuild the generated Go with `go generate`.

```console
$ go generate ./proto
```
