<!--[metadata]>
+++
title = "version"
description = "The version command description and usage"
keywords = ["version, architecture, api"]
[menu.main]
parent = "smn_cli"
weight=1
+++
<![end-metadata]-->

# version

    Usage: docker version

    Show the Docker version information.

Show the Docker version, API version, Git commit, Go version and
OS/architecture of both Docker client and daemon. Example use:

    $ docker version
    Client version: 1.5.0
    Client API version: 1.17
    Go version (client): go1.4.1
    Git commit (client): a8a31ef
    OS/Arch (client): darwin/amd64
    Server version: 1.5.0
    Server API version: 1.17
    Go version (server): go1.4.1
    Git commit (server): a8a31ef
    OS/Arch (server): linux/amd64