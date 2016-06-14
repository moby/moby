<!--[metadata]>
+++
title = "Apply rolling updates"
description = "Apply rolling updates to a service on the Swarm"
keywords = ["tutorial, cluster management, swarm, service, rolling-update"]
[menu.main]
identifier="swarm-tutorial-rolling-update"
parent="swarm-tutorial"
weight=20
advisory = "rc"
+++
<![end-metadata]-->

# Apply rolling updates to a service

In a previous step of the tutorial, you [scaled](scale-service.md) the number of
instances of a service. In this part of the tutorial, you deploy a new Redis
service and upgrade the service using rolling updates.

1. If you haven't already, open a terminal and ssh into the machine where you
run your manager node. For example, the tutorial uses a machine named
`manager1`.

2. Deploy Redis 3.0.6 to all nodes in the Swarm and configure
the swarm to update one node every 10 seconds:

    ```bash
    $ docker service create --scale 3 --name redis --update-delay 10s --update-parallelism 1 redis:3.0.6

    8m228injfrhdym2zvzhl9k3l0
    ```

    You configure the rolling update policy at service deployment time.

    The `--update-parallelism` flag configures the number of service tasks
    to update simultaneously.

    The `--update-delay` flag configures the time delay between updates to
    a service task or sets of tasks. You can describe the time `T` in the number
    of seconds `Ts`, minutes `Tm`, or hours `Th`. So `10m` indicates a 10 minute
    delay.

3. Inspect the `redis` service:
    ```
    $ docker service inspect redis --pretty

    ID:		75kcmhuf8mif4a07738wttmgl
    Name:		redis
    Mode:		REPLICATED
     Scale:	3
    Placement:
     Strategy:	SPREAD
    UpateConfig:
     Parallelism:	1
     Delay:		10s
    ContainerSpec:
     Image:		redis:3.0.6
    ```

4. Now you can update the container image for `redis`. Swarm applies the update
to nodes according to the `UpdateConfig` policy:

    ```bash
    $ docker service update --image redis:3.0.7 redis
    redis
    ```

5. Run `docker service inspect --pretty redis` to see the new image in the
desired state:

    ```
    docker service inspect --pretty redis

    ID:		1yrcci9v8zj6cokua2eishlob
    Name:		redis
    Mode:		REPLICATED
     Scale:		3
    Placement:
     Strategy:	SPREAD
    UpdateConfig:
     Parallelism:	1
     Delay:		10s
   ContainerSpec:
   Image:		redis:3.0.7
   ```

6. Run `docker service tasks TASK-ID` to watch the rolling update:

    ```
    $ docker service tasks redis

    ID                         NAME     SERVICE  IMAGE        DESIRED STATE  LAST STATE          NODE
    5409nu4crb0smamziqwuug67u  redis.1  redis    redis:3.0.7  RUNNING        RUNNING 21 seconds  worker2
    b8ezq58zugcg1trk8k7jrq9ym  redis.2  redis    redis:3.0.7  RUNNING        RUNNING 1 seconds   worker1
    cgdcbipxnzx0y841vysiafb64  redis.3  redis    redis:3.0.7  RUNNING        RUNNING 11 seconds  worker1
    ```

    Before Swarm updates all of the tasks, you can see that some are running
    `redis:3.0.6` while others are running `redis:3.0.7`. The output above shows
    the state once the rolling updates are done. You can see that each instances
    entered the `RUNNING` state in 10 second increments.

Next, learn about how to [drain a node](drain-node.md) in the Swarm.

<p style="margin-bottom:300px">&nbsp;</p>
