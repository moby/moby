<!--[metadata]>
+++
title = "info"
description = "The info command description and usage"
keywords = ["display, docker, information"]
[menu.main]
parent = "smn_cli"
+++
<![end-metadata]-->

# info


    Usage: docker info [OPTIONS]

    Display system-wide information

      --help=false        Print usage

For example:

    $ docker -D info
    Containers: 14
    Images: 52
    Server Version: 1.9.0
    Storage Driver: aufs
     Root Dir: /var/lib/docker/aufs
     Backing Filesystem: extfs
     Dirs: 545
     Dirperm1 Supported: true
    Execution Driver: native-0.2
    Logging Driver: json-file
    Plugins:
     Volume: local
     Network: bridge null host
    Kernel Version: 3.19.0-22-generic
    OSType: linux
    Architecture: x86_64
    Operating System: Ubuntu 15.04
    CPUs: 24
    Total Memory: 62.86 GiB
    Name: docker
    ID: I54V:OLXT:HVMM:TPKO:JPHQ:CQCD:JNLC:O3BZ:4ZVJ:43XJ:PFHZ:6N2S
    Debug mode (server): true
     File Descriptors: 59
     Goroutines: 159
     System Time: 2015-09-23T14:04:20.699842089+08:00
     EventsListeners: 0
     Init SHA1:
     Init Path: /usr/bin/docker
     Docker Root Dir: /var/lib/docker
     Http Proxy: http://test:test@localhost:8080
     Https Proxy: https://test:test@localhost:8080
    WARNING: No swap limit support
    Username: svendowideit
    Registry: [https://index.docker.io/v1/]
    Labels:
     storage=ssd

The global `-D` option tells all `docker` commands to output debug information.

When sending issue reports, please use `docker version` and `docker -D info` to
ensure we know how your setup is configured.
