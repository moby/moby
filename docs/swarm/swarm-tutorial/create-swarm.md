<!--[metadata]>
+++
title = "Create a Swarm"
description = "Initialize the Swarm"
keywords = ["tutorial, cluster management, swarm"]
[menu.main]
identifier="initialize-swarm"
parent="swarm-tutorial"
weight=12
advisory = "rc"
+++
<![end-metadata]-->

# Create a Swarm

After you complete the [tutorial setup](index.md) steps, you're ready
to create a Swarm. Make sure the Docker Engine daemon is started on the host
machines.

1. Open a terminal and ssh into the machine where you want to run your manager
node. For example, the tutorial uses a machine named `manager1`.

2. Run `docker swarm init --listen-addr MANAGER-IP:PORT` to create a new Swarm.

    In the tutorial, the following command creates a Swarm on the `manager1` machine:

    ```
    $ docker swarm init --listen-addr 192.168.99.100:2377

    Swarm initialized: current node (09fm6su6c24qn) is now a manager.
    ```

    The `--listen-addr` flag configures the manager node to listen on port
    `2377`. The other nodes in the Swarm must be able to access the manager at
    the IP address.

3. Run `docker info` to view the current state of the Swarm:

     ```
     $ docker info

     Containers: 2
      Running: 0
      Paused: 0
      Stopped: 2
     ...snip...
     Swarm:
      NodeID: 09fm6su6c24qn
      IsManager: YES
      Managers: 1
      Nodes: 1
     ...snip...
     ```

4. Run the `docker node ls` command to view information about nodes:

    ```
    $ docker node ls

    ID              NAME      MEMBERSHIP  STATUS  AVAILABILITY  MANAGER STATUS  LEADER
09fm6su6c24q *  manager1  Accepted    Ready   Active        Reachable       Yes

    ```

     The `*` next to the node id, indicates that you're currently connected on
     this node.

     Docker Swarm automatically names the node for the machine host name. The
     tutorial covers other columns in later steps.

## What's next?

In the next section of the tutorial, we'll [add two more nodes](add-nodes.md) to
the cluster.


<p style="margin-bottom:300px">&nbsp;</p>
