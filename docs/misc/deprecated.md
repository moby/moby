<!--[metadata]>
+++
title = "Docker Deprecated Features"
description = "Deprecated Features."
keywords = ["docker, documentation, about, technology, deprecate"]
[menu.main]
parent = "mn_use_docker"
+++
<![end-metadata]-->

# Deprecated Features

The following list of features are deprecated.

### LXC built-in exec driver
**Deprecated In Release: v1.8**

**Target For Removal In Release: v1.10**

The built-in LXC execution driver is deprecated for an external implementation.
The lxc-conf flag and API fields will also be removed.

### Old Command Line Options
**Deprecated In Release: [v1.8.0](/release-notes/#docker-engine-1-8-0)**

**Target For Removal In Release: v1.10**

The flags `-d` and `--daemon` are deprecated in favor of the `daemon` subcommand:

    docker daemon -H ...

The following single-dash (`-opt`) variant of certain command line options
are deprecated and replaced with double-dash options (`--opt`):

    docker attach -nostdin
    docker attach -sig-proxy
    docker build -no-cache
    docker build -rm
    docker commit -author
    docker commit -run
    docker events -since
    docker history -notrunc
    docker images -notrunc
    docker inspect -format
    docker ps -beforeId
    docker ps -notrunc
    docker ps -sinceId
    docker rm -link
    docker run -cidfile
    docker run -cpuset
    docker run -dns
    docker run -entrypoint
    docker run -expose
    docker run -link
    docker run -lxc-conf
    docker run -n
    docker run -privileged
    docker run -volumes-from
    docker search -notrunc
    docker search -stars
    docker search -t
    docker search -trusted
    docker tag -force

The following double-dash options are deprecated and have no replacement:

    docker run --networking
    docker ps --since-id
    docker ps --before-id
    docker search --trusted

### Interacting with V1 registries

Version 1.8.3 adds a flag (`--disable-legacy-registry=false`) which prevents the docker daemon from `pull`, `push`, and `login` operations against v1 registries.  Though disabled by default, this signals the intent to deprecate the v1 protocol.
