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

2. Run the following command to create a worker node joined to
the existing swarm:

    ```
    docker swarm join <MANAGER-IP>:<PORT>
    ```

    Replace `<MANAGER-IP>` with the address of the manager node and `<PORT>`
    with the port where the manager listens.

    In the tutorial, the following command joins `worker1` to the swarm on `manager1`:

    ```
    $ docker swarm join 192.168.99.100:2377

    This node joined a Swarm as a worker.
    ```

3. Open a terminal and ssh into the machine where you want to run a second
worker node. This tutorial uses the name `worker2`.

4. Run `docker swarm join <MANAGER-IP>:<PORT>` to create a worker node joined to
the existing Swarm.

    Replace `<MANAGER-IP>` with the address of the manager node and `<PORT>`
    with the port where the manager listens.

5. Open a terminal and ssh into the machine where the manager node runs and run
the `docker node ls` command to see the worker nodes:

    ```bash
    ID                           NAME      MEMBERSHIP  STATUS  AVAILABILITY  MANAGER STATUS  LEADER
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

<p style="margin-bottom:300px">&nbsp;</p>
