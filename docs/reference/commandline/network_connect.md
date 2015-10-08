<!--[metadata]>
+++
title = "network connect"
description = "The network connect command description and usage"
keywords = ["network, connect"]
[menu.main]
parent = "smn_cli"
+++
<![end-metadata]-->

# network connect

    Usage:  docker network connect [OPTIONS] NETWORK CONTAINER

    Connects a container to a network

      --help=false       Print usage

Connects a running container to a network. This enables instant communication with other containers belonging to the same network.

```
  $ docker network create -d overlay multi-host-network
  $ docker run -d --name=container1 busybox top
  $ docker network connect multi-host-network container1
```

the container will be connected to the network that is created and managed by the driver (multi-host overlay driver in the above example) or external network plugins.

Multiple containers can be connected to the same network and the containers in the same network will start to communicate with each other. If the driver/plugin supports multi-host connectivity, then the containers connected to the same multi-host network will be able to communicate seamlessly.
