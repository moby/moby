<!--[metadata]>
+++
title = "Apply rolling updates"
description = "Apply rolling updates to a service on the Swarm"
keywords = ["tutorial, cluster management, swarm, service, rolling-update"]
advisory = "rc"
[menu.main]
identifier="swarm-tutorial-rolling-update"
parent="swarm-tutorial"
weight=20
+++
<![end-metadata]-->

# Apply rolling updates to a service

In a previous step of the tutorial, you [scaled](scale-service.md) the number of
instances of a service. In this part of the tutorial, you deploy a service based
on the Redis 3.0.6 container image. Then you upgrade the service to use the
Redis 3.0.7 container image using rolling updates.

1. If you haven't already, open a terminal and ssh into the machine where you
run your manager node. For example, the tutorial uses a machine named
`manager1`.

2. Deploy Redis 3.0.6 to the swarm and configure the swarm to update one node
every 10 seconds:

    ```bash
    $ docker service create --replicas 3 --name redis --update-delay 10s --update-parallelism 1 redis:3.0.6

    0u6a4s31ybk7yw2wyvtikmu50
    ```

    You configure the rolling update policy at service deployment time.

    The `--update-parallelism` flag configures the number of service tasks
    to update simultaneously.

    The `--update-delay` flag configures the time delay between updates to a
    service task or sets of tasks. You can describe the time `T` as a
    combination of the number of seconds `Ts`, minutes `Tm`, or hours `Th`. So
    `10m30s` indicates a 10 minute 30 second delay.

3. Inspect the `redis` service:

    ```bash
    $ docker service inspect redis --pretty

    ID:		0u6a4s31ybk7yw2wyvtikmu50
    Name:		redis
    Mode:		REPLICATED
     Replicas:		3
    Placement:
     Strategy:	SPREAD
    UpdateConfig:
     Parallelism:	1
     Delay:		10s
    ContainerSpec:
     Image:		redis:3.0.6
    ```

4. Now you can update the container image for `redis`. The swarm  manager
applies the update to nodes according to the `UpdateConfig` policy:

    ```bash
    $ docker service update --image redis:3.0.7 redis
    redis
    ```

5. Run `docker service inspect --pretty redis` to see the new image in the
desired state:

    ```bash
    docker service inspect --pretty redis

    ID:		0u6a4s31ybk7yw2wyvtikmu50
    Name:		redis
    Mode:		REPLICATED
     Replicas:		3
    Placement:
     Strategy:	SPREAD
    UpdateConfig:
     Parallelism:	1
     Delay:		10s
    ContainerSpec:
     Image:		redis:3.0.7
   ```

6. Run `docker service tasks <TASK-ID>` to watch the rolling update:

    ```bash
    $ docker service tasks redis

    ID                         NAME     SERVICE  IMAGE        LAST STATE              DESIRED STATE  NODE
    dos1zffgeofhagnve8w864fco  redis.1  redis    redis:3.0.7  Running 37 seconds      Running        worker1
    9l3i4j85517skba5o7tn5m8g0  redis.2  redis    redis:3.0.7  Running About a minute  Running        worker2
    egiuiqpzrdbxks3wxgn8qib1g  redis.3  redis    redis:3.0.7  Running 48 seconds      Running        worker1
    ```

    Before Swarm updates all of the tasks, you can see that some are running
    `redis:3.0.6` while others are running `redis:3.0.7`. The output above shows
    the state once the rolling updates are done. You can see that each instances
    entered the `RUNNING` state in approximately 10 second increments.

Next, learn about how to [drain a node](drain-node.md) in the Swarm.
