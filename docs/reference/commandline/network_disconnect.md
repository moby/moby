<!--[metadata]>
+++
title = "network disconnect"
description = "The network disconnect command description and usage"
keywords = ["network, disconnect"]
[menu.main]
parent = "smn_cli"
+++
<![end-metadata]-->

# network disconnect

    Usage:  docker network disconnect [OPTIONS] NETWORK CONTAINER

    Disconnects a container from a network

      --help=false       Print usage

Disconnects a running container from a  network.

```
  $ docker network create -d overlay multi-host-network
  $ docker run -d --net=multi-host-network --name=container1 busybox top
  $ docker network disconnect multi-host-network container1
```

the container will be disconnected from the network.
