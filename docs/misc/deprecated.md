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

### Old Command Line Options
**Deprecated In Release: [v1.8.0](/release-notes/#docker-engine-1-8-0)**

**Target For Removal In Release: v1.10**

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

The following single-dash options are deprecated and have no replacement:

    docker run --networking
    docker ps --since-id
    docker ps --before-id
    docker search --trusted
