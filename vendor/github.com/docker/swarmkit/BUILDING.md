## Build the development environment

To build Swarmkit, you must set up a Go development environment.
[How to Write Go Code](https://golang.org/doc/code.html) contains full instructions.
When setup correctly, you should have a GOROOT and GOPATH set in the environment.

After you set up the Go development environment, use `go get` to check out
`swarmkit`:

    go get -d github.com/docker/swarmkit

This command installs the source repository into the `GOPATH`.

It is not mandatory to use `go get` to checkout the SwarmKit project. However,
for these instructions to work, you need to check out the project to the
correct subdirectory of the `GOPATH`: `$GOPATH/src/github.com/docker/swarmkit`.

### Repeatable Builds

For the full development experience, one should `cd` into
`$GOPATH/src/github.com/docker/swarmkit`. From there, the regular `go`
commands, such as `go test`, should work per package (please see
[Developing](#developing) if they don't work).

Docker provides a `Makefile` as a convenience to support repeatable builds.
`make setup` installs tools onto the `GOPATH` for use with developing
`SwarmKit`:

    make setup

Once these commands are available in the `GOPATH`, run `make` to get a full
build:

    $ make
    🐳 fmt
    🐳 bin/swarmd
    🐳 bin/swarmctl
    🐳 bin/swarm-bench
    🐳 bin/protoc-gen-gogoswarm
    🐳 binaries
    🐳 vet
    🐳 lint
    🐳 build
    github.com/docker/swarmkit
    github.com/docker/swarmkit/vendor/github.com/davecgh/go-spew/spew
    github.com/docker/swarmkit/vendor/github.com/pmezard/go-difflib/difflib
    github.com/docker/swarmkit/cmd/protoc-gen-gogoswarm
    github.com/docker/swarmkit/cmd/swarm-bench
    github.com/docker/swarmkit/cmd/swarmctl
    github.com/docker/swarmkit/vendor/github.com/stretchr/testify/assert
    github.com/docker/swarmkit/ca/testutils
    github.com/docker/swarmkit/cmd/swarmd
    github.com/docker/swarmkit/vendor/github.com/pivotal-golang/clock/fakeclock
    github.com/docker/swarmkit/vendor/github.com/stretchr/testify/require
    github.com/docker/swarmkit/manager/state/raft/testutils
    github.com/docker/swarmkit/manager/testcluster
    github.com/docker/swarmkit/protobuf/plugin/deepcopy/test
    github.com/docker/swarmkit/protobuf/plugin/raftproxy/test
    🐳 test
    ?       github.com/docker/swarmkit      [no test files]
    ?       github.com/docker/swarmkit      [no test files]
    ok      github.com/docker/swarmkit/agent        2.264s
    ok      github.com/docker/swarmkit/agent/exec   1.055s
    ok      github.com/docker/swarmkit/agent/exec/container 1.094s
    ?       github.com/docker/swarmkit/api  [no test files]
    ?       github.com/docker/swarmkit/api/duration [no test files]
    ?       github.com/docker/swarmkit/api/timestamp        [no test files]
    ok      github.com/docker/swarmkit/ca   15.634s
    ...
    ok      github.com/docker/swarmkit/protobuf/plugin/raftproxy/test       1.084s
    ok      github.com/docker/swarmkit/protobuf/ptypes      1.025s
    ?       github.com/docker/swarmkit/version      [no test files]

The above provides a repeatable build using the contents of the vendored
`./vendor` directory. This includes formatting, vetting, linting, building,
and testing. The binaries created will be available in `./bin`.

Several `make` targets are provided for common tasks. Please see the `Makefile`
for details.

### Update vendored dependencies

To update dependency you need just change `vendor.conf` file and run `vndr` tool:
```
go get github.com/LK4D4/vndr
vndr
```

It's possible to update only one dependency:
```
vndr github.com/coreos/etcd v3.0.9
```

but it should be in sync with `vendor.conf`.

Also, you can update dependency from fork for testing:
```
vndr github.com/coreos/etcd <revision> https://github.com/LK4D4/etcd.git
```

### Regenerating protobuf bindings

This requires that you have [Protobuf 3.x or
higher](https://developers.google.com/protocol-buffers/docs/downloads). Once
that is installed the bindings can be regenerated with:

```
make setup
make generate
```

NB: As of version 3.0.0-7 the Debian `protobuf-compiler` package lacks
a dependency on `libprotobuf-dev` which contains some standard proto
definitions, be sure to install both packages. This is [Debian bug
#842158](https://bugs.debian.org/842158).
