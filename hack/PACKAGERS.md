Dear packager.

If you are looking to make docker available on your favorite software distribution,
this document is for you. It summarizes the requirements for building and running
docker.

## Getting started

We really want to help you package Docker successfully. Before anything, a good first step
is to introduce yourself on the [docker-dev mailing list](https://groups.google.com/forum/?fromgroups#!forum/docker-dev)
, explain what you''re trying to achieve, and tell us how we can help. Don''t worry, we don''t bite!
There might even be someone already working on packaging for the same distro!

You can also join the IRC channel - #docker and #docker-dev on Freenode are both active and friendly.

## Package name

If possible, your package should be called "docker". If that name is already taken, a second
choice is "lxc-docker".

## Official build vs distro build

The Docker project maintains its own build and release toolchain. It is pretty neat and entirely
based on Docker (surprise!). This toolchain is the canonical way to build Docker, and the only
method supported by the development team. We encourage you to give it a try, and if the circumstances
allow you to use it, we recommend that you do.

You might not be able to use the official build toolchain - usually because your distribution has a
toolchain and packaging policy of its own. We get it! Your house, your rules. The rest of this document
should give you the information you need to package Docker your way, without denaturing it in
the process.

## System build dependencies

To build docker, you will need the following system dependencies

* An amd64 machine
* A recent version of git and mercurial
* Go version 1.2 or later (see notes below regarding using Go 1.1.2 and dynbinary)
* SQLite version 3.7.9 or later
* A clean checkout of the source must be added to a valid Go [workspace](http://golang.org/doc/code.html#Workspaces)
under the path *src/github.com/dotcloud/docker*.

## Go dependencies

All Go dependencies are vendored under ./vendor. They are used by the official build,
so the source of truth for the current version is whatever is in ./vendor.

To use the vendored dependencies, simply make sure the path to ./vendor is included in $GOPATH.

If you would rather package these dependencies yourself, take a look at ./hack/vendor.sh for an
easy-to-parse list of the exact version for each.

NOTE: if you''re not able to package the exact version (to the exact commit) of a given dependency,
please get in touch so we can remediate! Who knows what discrepancies can be caused by even the
slightest deviation. We promise to do our best to make everybody happy.

## Disabling CGO

Make sure to disable CGO on your system, and then recompile the standard library on the build
machine:

```bash
export CGO_ENABLED=0
cd /tmp && echo 'package main' > t.go && go test -a -i -v
```

## Building Docker

To build the docker binary, run the following command with the source checkout as the
working directory:

```bash
./hack/make.sh binary
```

This will create a static binary under *./bundles/$VERSION/binary/docker-$VERSION*, where
*$VERSION* is the contents of the file *./VERSION*.

You are encouraged to use ./hack/make.sh without modification. If you must absolutely write
your own script (are you really, really sure you need to? make.sh is really not that complicated),
then please take care the respect the following:

* In *./hack/make.sh*: $LDFLAGS, $BUILDFLAGS, $VERSION and $GITCOMMIT
* In *./hack/make/binary*: the exact build command to run

You may be tempted to tweak these settings. In particular, being a rigorous maintainer, you may want
to disable static linking. Please don''t! Docker *needs* to be statically linked to function properly.
You would do the users of your distro a disservice and "void the docker warranty" by changing the flags.

A good comparison is Busybox: all distros package it as a statically linked binary, because it just
makes sense. Docker is the same way.

If you *must* have a non-static Docker binary, or require Go 1.1.2 (since Go 1.2 is still freshly released
at the time of this writing), please use:

```bash
./hack/make.sh dynbinary
```

This will create *./bundles/$VERSION/dynbinary/docker-$VERSION* and *./bundles/$VERSION/binary/dockerinit-$VERSION*.
The first of these would usually be installed at */usr/bin/docker*, while the second must be installed
at */usr/libexec/docker/dockerinit*.

## Testing Docker

Before releasing your binary, make sure to run the tests! Run the following command with the source
checkout as the working directory:

```bash
./hack/make.sh test
```

The test suite includes both live integration tests and unit tests, so you will need all runtime
dependencies to be installed (see below).

The test suite will also download a small test container, so you will need internet connectivity.

## Runtime dependencies

To run properly, docker needs the following software to be installed at runtime:

* GNU Tar version 1.26 or later
* iproute2 version 3.5 or later (build after 2012-05-21), and specifically the "ip" utility
* iptables version 1.4 or later
* The LXC utility scripts (http://lxc.sourceforge.net) version 0.8 or later
* Git version 1.7 or later
* XZ Utils 4.9 or later

## Kernel dependencies

Docker in daemon mode has specific kernel requirements. For details, see
http://docs.docker.io/en/latest/installation/kernel/

Note that Docker also has a client mode, which can run on virtually any linux kernel (it even builds
on OSX!).

## Init script

Docker expects to run as a daemon at machine startup. Your package will need to include a script
for your distro''s process supervisor of choice.

Docker should be run as root, with the following arguments:

```bash
docker -d
```
