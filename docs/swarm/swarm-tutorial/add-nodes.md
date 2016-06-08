<!--[metadata]>
+++
title = "Add nodes to the Swarm"
description = "Add nodes to the Swarm"
keywords = ["tutorial, cluster management, swarm"]
[menu.main]
identifier="add-nodes"
parent="swarm-tutorial"
weight=13
advisory = "rc"
+++
<![end-metadata]-->

# Add nodes to the Swarm

Once you've [created a Swarm](create-swarm.md) with a manager node, you're ready
to add worker nodes.

1. Open a terminal and ssh into the machine where you want to run a worker node.
This tutorial uses the name `worker1`.

2. Run `docker swarm join MANAGER-IP:PORT` to create a worker node joined to the
existing Swarm. Replace MANAGER-IP address of the manager node and the port
where the manager listens.

    In the tutorial, the following command joins `worker1` to the Swarm on `manager1`:

    ```
    $ docker swarm join 192.168.99.100:2377

    This node joined a Swarm as a worker.
    ```

3. Open a terminal and ssh into the machine where you want to run a second
worker node. This tutorial uses the name `worker2`.

4. Run `docker swarm join MANAGER-IP:PORT` to create a worker node joined to
the existing Swarm. Replace MANAGER-IP address of the manager node and the port
where the manager listens.

5. Open a terminal and ssh into the machine where the manager node runs and run
the `docker node ls` command to see the worker nodes:

    ```bash
    $ docker node ls

    ID              NAME      MEMBERSHIP  STATUS  AVAILABILITY  MANAGER STATUS  LEADER
09fm6su6c24q *  manager1  Accepted    Ready   Active        Reachable       Yes
32ljq6xijzb9    worker1   Accepted    Ready   Active
38fsncz6fal9    worker2   Accepted    Ready   Active
    ```

    The `MANAGER` column identifies the manager nodes in the Swarm. The empty
    status in this column for `worker1` and `worker2` identifies them as worker nodes.

    Swarm management commands like `docker node ls` only work on manager nodes.


## What's next?

Now your Swarm consists of a manager and two worker nodes. In the next step of
the tutorial, you [deploy a service](deploy-service.md) to the Swarm.

<p style="margin-bottom:300px">&nbsp;</p>
