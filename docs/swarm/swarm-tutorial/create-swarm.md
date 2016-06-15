<!--[metadata]>
+++
title = "Create a swarm"
description = "Initialize the swarm"
keywords = ["tutorial, cluster management, swarm mode"]
advisory = "rc"
[menu.main]
identifier="initialize-swarm"
parent="swarm-tutorial"
weight=12
+++
<![end-metadata]-->

# Create a swarm

After you complete the [tutorial setup](index.md) steps, you're ready
to create a swarm. Make sure the Docker Engine daemon is started on the host
machines.

1. Open a terminal and ssh into the machine where you want to run your manager
node. For example, the tutorial uses a machine named `manager1`.

2. Run the following command to create a new swarm:

    ```
    docker swarm init --listen-addr <MANAGER-IP>:<PORT>
    ```

    In the tutorial, the following command creates a swarm on the `manager1` machine:

    ```
    $ docker swarm init --listen-addr 192.168.99.100:2377

    Swarm initialized: current node (dxn1zf6l61qsb1josjja83ngz) is now a manager.
    ```

    The `--listen-addr` flag configures the manager node to listen on port
    `2377`. The other nodes in the swarm must be able to access the manager at
    the IP address.

3. Run `docker info` to view the current state of the swarm:

     ```
     $ docker info

     Containers: 2
      Running: 0
      Paused: 0
      Stopped: 2
     ...snip...
     Swarm: active
      NodeID: dxn1zf6l61qsb1josjja83ngz
      IsManager: Yes
      Managers: 1
      Nodes: 1
      CACertHash: sha256:b7986d3baeff2f5664dfe350eec32e2383539ec1a802ba541c4eb829056b5f61
     ...snip...
     ```

4. Run the `docker node ls` command to view information about nodes:

    ```
    $ docker node ls

    ID                           NAME      MEMBERSHIP  STATUS  AVAILABILITY  MANAGER STATUS  LEADER
    dxn1zf6l61qsb1josjja83ngz *  manager1  Accepted    Ready   Active        Reachable       Yes

    ```

     The `*` next to the node id, indicates that you're currently connected on
     this node.

     Docker Engine swarm mode automatically names the node for the machine host
     name. The tutorial covers other columns in later steps.

## What's next?

In the next section of the tutorial, we'll [add two more nodes](add-nodes.md) to
the cluster.


<p style="margin-bottom:300px">&nbsp;</p>
