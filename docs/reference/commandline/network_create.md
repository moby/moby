<!--[metadata]>
+++
title = "network create"
description = "The network create command description and usage"
keywords = ["network, create"]
[menu.main]
parent = "smn_cli"
+++
<![end-metadata]-->

# network create

    Usage:  docker network create [OPTIONS] NETWORK-NAME

    Creates a new network with a name specified by the user

      -d, --driver=      Driver to manage the Network
      --help=false       Print usage

Creates a new network that containers can connect to. If the driver supports multi-host networking, the created network will be made available across all the hosts in the cluster. Daemon will do its best to identify network name conflicts. But its the users responsibility to make sure network name is unique across the cluster. You create a network and then configure the container to use it, for example:

```
  $ docker network create -d overlay multi-host-network
  $ docker run -itd --net=multi-host-network busybox
```

the container will be connected to the network that is created and managed by the driver (multi-host overlay driver in the above example) or external network plugins.

Multiple containers can be connected to the same network and the containers in the same network will start to communicate with each other. If the driver/plugin supports multi-host connectivity, then the containers connected to the same multi-host network will be able to communicate seamlessly.

*Note*: UX needs enhancement to accept network options to be passed to the drivers
