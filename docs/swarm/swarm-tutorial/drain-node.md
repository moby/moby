<!--[metadata]>
+++
title = "Drain a node"
description = "Drain nodes on the Swarm"
keywords = ["tutorial, cluster management, swarm, service, drain"]
[menu.main]
identifier="swarm-tutorial-drain-node"
parent="swarm-tutorial"
weight=21
+++
<![end-metadata]-->

# Drain a node on the Swarm

In earlier steps of the tutorial, all the nodes have been running with `ACTIVE`
availability. The Swarm manager can assign tasks to any `ACTIVE` node, so all
nodes have been available to receive tasks.

Sometimes, such as planned maintenance times, you need to set a node to `DRAIN`
availabilty. `DRAIN` availabilty  prevents a node from receiving new tasks
from the Swarm manager. It also means the manager stops tasks running on the
node and launches replica tasks on a node with `ACTIVE` availability.

1. If you haven't already, open a terminal and ssh into the machine where you
run your manager node. For example, the tutorial uses a machine named
`manager1`.

2. Verify that all your nodes are actively available.

    ```
    $ docker node ls

    ID               NAME      MEMBERSHIP  STATUS  AVAILABILITY  MANAGER STATUS  LEADER
  1x2bldyhie1cj    worker1   Accepted    Ready   Active
  1y3zuia1z224i    worker2   Accepted    Ready   Active
  2p5bfd34mx4op *  manager1  Accepted    Ready   Active        Reachable       Yes
    ```

2. If you aren't still running the `redis` service from the [rolling
update](rolling-update.md) tutorial, start it now:

    ```bash
    $ docker service create --scale 3 --name redis --update-delay 10s --update-parallelism 1 redis:3.0.6

    69uh57k8o03jtqj9uvmteodbb
    ```

3. Run `docker service tasks redis` to see how the Swarm manager assigned the
tasks to different nodes:

    ```
    $ docker service tasks redis
    ID                         NAME     SERVICE  IMAGE        LAST STATE          DESIRED STATE  NODE
    3wfqsgxecktpwoyj2zjcrcn4r  redis.1  redis    redis:3.0.6  RUNNING 13 minutes  RUNNING        worker2
    8lcm041z3v80w0gdkczbot0gg  redis.2  redis    redis:3.0.6  RUNNING 13 minutes  RUNNING        worker1
    d48skceeph9lkz4nbttig1z4a  redis.3  redis    redis:3.0.6  RUNNING 12 minutes  RUNNING        manager1
    ```

    In this case the Swarm manager distributed one task to each node. You may
    see the tasks distributed differently among the nodes in your environment.

4. Run `docker node update --availability drain NODE-ID` to drain a node that
had a task assigned to it:

    ```bash
    docker node update --availability drain worker1
    worker1
    ```

5. Inspect the node to check its availability:

    ```
    $ docker node inspect --pretty worker1
    ID:			1x2bldyhie1cj
    Hostname:		worker1
    Status:
     State:			READY
     Availability:		DRAIN
    ...snip...
    ```

    The drained node shows `Drain` for `AVAILABILITY`.

6. Run `docker service tasks redis` to see how the Swarm manager updated the
task assignments for the `redis` service:

    ```
    ID                         NAME     SERVICE  IMAGE        LAST STATE          DESIRED STATE  NODE
    3wfqsgxecktpwoyj2zjcrcn4r  redis.1  redis    redis:3.0.6  RUNNING 26 minutes  RUNNING        worker2
    ah7o4u5upostw3up1ns9vbqtc  redis.2  redis    redis:3.0.6  RUNNING 9 minutes   RUNNING        manager1
    d48skceeph9lkz4nbttig1z4a  redis.3  redis    redis:3.0.6  RUNNING 26 minutes  RUNNING        manager1
    ```

    The Swarm manager maintains the desired state by ending the task on a node
    with `Drain` availability and creating a new task on a node with `Active`
    availability.

7. Run  `docker node update --availability active NODE-ID` to return the drained
node to an active state:

    ```bash
    $ docker node update --availability active worker1
    worker1
    ```

8. Inspect the node to see the updated state:

   ```
   $ docker node inspect --pretty worker1
   ID:			1x2bldyhie1cj
   Hostname:		worker1
   Status:
    State:			READY
    Availability:		ACTIVE
  ...snip...
  ```

  When you set the node back to `Active` availability, it can receive new tasks:

  * during a service update to scale up
  * during a rolling update
  * when you set another node to `Drain` availability
  * when a task fails on another active node

## What's next?

The next topic in the tutorial introduces volumes.

<p style="margin-bottom:300px">&nbsp;</p>
