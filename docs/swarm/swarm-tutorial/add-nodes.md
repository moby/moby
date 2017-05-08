<!--[metadata]>
+++
title = "Add nodes to the swarm"
description = "Add nodes to the swarm"
keywords = ["tutorial, cluster management, swarm"]
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
    $ docker swarm join \
      --token  SWMTKN-1-49nj1cmql0jkz5s954yi3oex3nedyz0fb0xx14ie39trti4wxv-8vxv8rssmk743ojnwacrr2e7c \
      192.168.99.100:2377

    This node joined a swarm as a worker.
    ```

    If you don't have the command available, you can run the following command
    on a manager node to retrieve the join command for a worker:

    ```bash
    $ docker swarm join-token worker

    To add a worker to this swarm, run the following command:

        docker swarm join \
        --token SWMTKN-1-49nj1cmql0jkz5s954yi3oex3nedyz0fb0xx14ie39trti4wxv-8vxv8rssmk743ojnwacrr2e7c \
        192.168.99.100:2377
    ```

3. Open a terminal and ssh into the machine where you want to run a second
worker node. This tutorial uses the name `worker2`.

4. Run the command produced by the `docker swarm init` output from the
[Create a swarm](create-swarm.md) tutorial step to create a second worker node
joined to the existing swarm:

    ```bash
    $ docker swarm join \
      --token SWMTKN-1-49nj1cmql0jkz5s954yi3oex3nedyz0fb0xx14ie39trti4wxv-8vxv8rssmk743ojnwacrr2e7c \
      192.168.99.100:2377

    This node joined a swarm as a worker.
    ```

5. Open a terminal and ssh into the machine where the manager node runs and run
the `docker node ls` command to see the worker nodes:

    ```bash
    ID                           HOSTNAME  STATUS  AVAILABILITY  MANAGER STATUS
    03g1y59jwfg7cf99w4lt0f662    worker2   Ready   Active
    9j68exjopxe7wfl6yuxml7a7j    worker1   Ready   Active
    dxn1zf6l61qsb1josjja83ngz *  manager1  Ready   Active        Leader
    ```

    The `MANAGER` column identifies the manager nodes in the swarm. The empty
    status in this column for `worker1` and `worker2` identifies them as worker nodes.

    Swarm management commands like `docker node ls` only work on manager nodes.


## What's next?

Now your swarm consists of a manager and two worker nodes. In the next step of
the tutorial, you [deploy a service](deploy-service.md) to the swarm.
