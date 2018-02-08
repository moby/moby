### Important: This repository is in an early development phase

[![asciicinema example](https://asciinema.org/a/gPEIEo1NzmDTUu2bEPsUboqmU.png)](https://asciinema.org/a/gPEIEo1NzmDTUu2bEPsUboqmU)


## BuildKit

<!-- godoc is mainly for LLB stuff -->
[![GoDoc](https://godoc.org/github.com/moby/buildkit?status.svg)](https://godoc.org/github.com/moby/buildkit/client/llb)
[![Build Status](https://travis-ci.org/moby/buildkit.svg?branch=master)](https://travis-ci.org/moby/buildkit)
[![Go Report Card](https://goreportcard.com/badge/github.com/moby/buildkit)](https://goreportcard.com/report/github.com/moby/buildkit)


BuildKit is a toolkit for converting source code to build artifacts in an efficient, expressive and repeatable manner.

Key features:
- Automatic garbage collection
- Extendable frontend formats
- Concurrent dependency resolution
- Efficient instruction caching
- Build cache import/export
- Nested build job invocations
- Distributable workers
- Multiple output formats
- Pluggable architecture


Read the proposal from https://github.com/moby/moby/issues/32925

#### Quick start

BuildKit daemon can be built in two different versions: one that uses [containerd](https://github.com/containerd/containerd) for execution and distribution, and a standalone version that doesn't have other dependencies apart from [runc](https://github.com/opencontainers/runc). We are open for adding more backends. `buildd` is a CLI utility for serving the gRPC API. 

```bash
# buildd daemon (choose one)
go build -o buildd-containerd -tags containerd ./cmd/buildd
go build -o buildd-standalone -tags standalone ./cmd/buildd

# buildctl utility
go build -o buildctl ./cmd/buildctl
```

You can also use `make binaries` that prepares all binaries into the `bin/` directory.

`examples/buildkit*` directory contains scripts that define how to build different configurations of BuildKit and its dependencies using the `client` package. Running one of these script generates a protobuf definition of a build graph. Note that the script itself does not execute any steps of the build.

You can use `buildctl debug dump-llb` to see what data is in this definition. Add `--dot` to generate dot layout.

```bash
go run examples/buildkit0/buildkit.go | buildctl debug dump-llb | jq .
```

To start building use `buildctl build` command. The example script accepts `--target` flag to choose between `containerd` and `standalone` configurations. In standalone mode BuildKit binaries are built together with `runc`. In containerd mode, the `containerd` binary is built as well from the upstream repo.

```bash
go run examples/buildkit0/buildkit.go | buildctl build
```

`buildctl build` will show interactive progress bar by default while the build job is running. It will also show you the path to the trace file that contains all information about the timing of the individual steps and logs.

Different versions of the example scripts show different ways of describing the build definition for this project to show the capabilities of the library. New versions have been added when new features have become available.

- `./examples/buildkit0` - uses only exec operations, defines a full stage per component.
- `./examples/buildkit1` - cloning git repositories has been separated for extra concurrency.
- `./examples/buildkit2` - uses git sources directly instead of running `git clone`, allowing better performance and much safer caching.
- `./examples/buildkit3` - allows using local source files for separate components eg. `./buildkit3 --runc=local | buildctl build --local runc-src=some/local/path`  
- `./examples/dockerfile2llb` - can be used to convert a Dockerfile to LLB for debugging purposes
- `./examples/gobuild` - shows how to use nested invocation to generate LLB for Go package internal dependencies


#### Examples

##### Starting the buildd daemon:

```
buildd-standalone --debug --root /var/lib/buildkit
```

##### Building a Dockerfile:

```
buildctl build --frontend=dockerfile.v0 --local context=. --local dockerfile=.
```

`context` and `dockerfile` should point to local directories for build context and Dockerfile location.


##### Exporting resulting image to containerd

Containerd version of buildd needs to be used

```
buildctl build ... --exporter=image --exporter-opt name=docker.io/username/image
ctr --namespace=buildkit images ls
```

##### Exporting build result back to client

```
buildctl build ... --exporter=local --exporter-opt output=path/to/output-dir
```

#### View build cache

```
buildctl du -v
```

#### Supported runc version

During development buildkit is tested with the version of runc that is being used by the containerd repository. Please refer to [runc.md](https://github.com/containerd/containerd/blob/d1e11f17ec7b325f89608dd46c128300b8727d50/RUNC.md) for more information.


#### Contributing

Running tests:

```bash
make test
```

Updating vendored dependencies:

```bash
# update vendor.conf
make vendor
```

Validating your updates before submission:

```bash
make validate-all
```
