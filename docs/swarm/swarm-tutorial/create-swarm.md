<!--[metadata]>
+++
title = "Create a swarm"
description = "Initialize the swarm"
keywords = ["tutorial, cluster management, swarm mode"]
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

    ```bash
    docker swarm init --advertise-addr <MANAGER-IP>
    ```

    >**Note:** If you are using Docker for Mac or Docker for Windows to test
single-node swarm, simply run `docker swarm init` with no arguments. There is no
need to specify ` --advertise-addr` in this case. To learn more, see the topic
on how to [Use Docker for Mac or Docker for
Windows](index.md#use-docker-for-mac-or-docker-for-windows) with Swarm.

    In the tutorial, the following command creates a swarm on the `manager1`
    machine:

    ```bash
    $ docker swarm init --advertise-addr 192.168.99.100
    Swarm initialized: current node (dxn1zf6l61qsb1josjja83ngz) is now a manager.

    To add a worker to this swarm, run the following command:

        docker swarm join \
        --token SWMTKN-1-49nj1cmql0jkz5s954yi3oex3nedyz0fb0xx14ie39trti4wxv-8vxv8rssmk743ojnwacrr2e7c \
        192.168.99.100:2377

    To add a manager to this swarm, run 'docker swarm join-token manager' and follow the instructions.
    ```

    The `--advertise-addr` flag configures the manager node to publish its
    address as `192.168.99.100`. The other nodes in the swarm must be able
    to access the manager at the IP address.

    The output includes the commands to join new nodes to the swarm. Nodes will
    join as managers or workers depending on the value for the `--token`
    flag.

2. Run `docker info` to view the current state of the swarm:

    ```bash
    $ docker info

    Containers: 2
    Running: 0
    Paused: 0
    Stopped: 2
      ...snip...
    Swarm: active
      NodeID: dxn1zf6l61qsb1josjja83ngz
      Is Manager: true
      Managers: 1
      Nodes: 1
      ...snip...
    ```

3. Run the `docker node ls` command to view information about nodes:

    ```bash
    $ docker node ls

    ID                           HOSTNAME  STATUS  AVAILABILITY  MANAGER STATUS
    dxn1zf6l61qsb1josjja83ngz *  manager1  Ready   Active        Leader

    ```

    The `*` next to the node id indicates that you're currently connected on
    this node.

    Docker Engine swarm mode automatically names the node for the machine host
    name. The tutorial covers other columns in later steps.

## What's next?

In the next section of the tutorial, we'll [add two more nodes](add-nodes.md) to
the cluster.
