<!--[metadata]>
+++
title = "Add nodes to the swarm"
description = "Add nodes to the swarm"
keywords = ["tutorial, cluster management, swarm"]
advisory = "rc"
[menu.main]
identifier="add-nodes"
parent="swarm-tutorial"
weight=13
+++
<![end-metadata]-->

# Add nodes to the swarm

Once you've [created a swarm](create-swarm.md) with a manager node, you're ready
to add worker nodes.

1. Open a terminal and ssh into the machine where you want to run a worker node.
This tutorial uses the name `worker1`.

2. Run the command produced by the `docker swarm init` output from the
[Create a swarm](create-swarm.md) tutorial step to create a worker node joined to the existing swarm:

    ```bash
    $ docker swarm join --secret 4ao565v9jsuogtq5t8s379ulb \
      --ca-hash sha256:07ce22bd1a7619f2adc0d63bd110479a170e7c4e69df05b67a1aa2705c88ef09 \
      192.168.99.100:2377
    ```

    If you don't have the command available, you can run the following command:

    ```bash
    docker swarm join --secret <SECRET> <MANAGER-IP>:<PORT>
    ```

    Replace `<SECRET>` with the secret that was printed by `docker swarm init`
    in the previous step. Replace `<MANAGER-IP>` with the address of the manager
    node and `<PORT>` with the port where the manager listens.

    The command generated from `docker swarm init` includes the `--ca-hash` to
    securely identify the manager node according to its root CA. For the
    tutorial, it is OK to join without it.

3. Open a terminal and ssh into the machine where you want to run a second
worker node. This tutorial uses the name `worker2`.

4. Run the command produced by the `docker swarm init` output from the
[Create a swarm](create-swarm.md) tutorial step to create a second worker node
joined to the existing swarm:

    ```bash
    $ docker swarm join --secret 4ao565v9jsuogtq5t8s379ulb \
      --ca-hash sha256:07ce22bd1a7619f2adc0d63bd110479a170e7c4e69df05b67a1aa2705c88ef09 \
      192.168.99.100:2377
    ```

5. Open a terminal and ssh into the machine where the manager node runs and run
the `docker node ls` command to see the worker nodes:

    ```bash
    ID                           HOSTNAME  MEMBERSHIP  STATUS  AVAILABILITY  MANAGER STATUS  LEADER
    03g1y59jwfg7cf99w4lt0f662    worker2   Accepted    Ready   Active
    9j68exjopxe7wfl6yuxml7a7j    worker1   Accepted    Ready   Active
    dxn1zf6l61qsb1josjja83ngz *  manager1  Accepted    Ready   Active        Reachable       Yes
    ```

    The `MANAGER` column identifies the manager nodes in the swarm. The empty
    status in this column for `worker1` and `worker2` identifies them as worker nodes.

    Swarm management commands like `docker node ls` only work on manager nodes.


## What's next?

Now your swarm consists of a manager and two worker nodes. In the next step of
the tutorial, you [deploy a service](deploy-service.md) to the swarm.
