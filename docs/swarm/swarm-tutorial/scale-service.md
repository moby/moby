<!--[metadata]>
+++
title = "Scale the service"
description = "Scale the service running in the swarm"
keywords = ["tutorial, cluster management, swarm mode, scale"]
advisory = "rc"
[menu.main]
identifier="swarm-tutorial-scale-service"
parent="swarm-tutorial"
weight=18
+++
<![end-metadata]-->

# Scale the service in the swarm

Once you have [deployed a service](deploy-service.md) to a swarm, you are ready
to use the Docker CLI to scale the number of service tasks in
the swarm.

1. If you haven't already, open a terminal and ssh into the machine where you
run your manager node. For example, the tutorial uses a machine named
`manager1`.

2. Run the following command to change the desired state of the
service running in the swarm:

    ```bash
    $ docker service scale <SERVICE-ID>=<NUMBER-OF-TASKS>
    ```

    For example:

    ```bash
    $ docker service scale helloworld=5

    helloworld scaled to 5
    ```

3. Run `docker service tasks <SERVICE-ID>` to see the updated task list:

    ```
    $ docker service tasks helloworld

    ID                         NAME          SERVICE     IMAGE   LAST STATE          DESIRED STATE  NODE
    8p1vev3fq5zm0mi8g0as41w35  helloworld.1  helloworld  alpine  Running 7 minutes   Running        worker2
    c7a7tcdq5s0uk3qr88mf8xco6  helloworld.2  helloworld  alpine  Running 24 seconds  Running        worker1
    6crl09vdcalvtfehfh69ogfb1  helloworld.3  helloworld  alpine  Running 24 seconds  Running        worker1
    auky6trawmdlcne8ad8phb0f1  helloworld.4  helloworld  alpine  Running 24 seconds  Accepted       manager1
    ba19kca06l18zujfwxyc5lkyn  helloworld.5  helloworld  alpine  Running 24 seconds  Running        worker2
    ```

    You can see that swarm has created 4 new tasks to scale to a total of 5
    running instances of Alpine Linux. The tasks are distributed between the
    three nodes of the swarm. One is running on `manager1`.

4. Run `docker ps` to see the containers running on the node where you're
connected. The following example shows the tasks running on `manager1`:

    ```
    $ docker ps

    CONTAINER ID        IMAGE               COMMAND             CREATED             STATUS              PORTS               NAMES
    528d68040f95        alpine:latest       "ping docker.com"   About a minute ago   Up About a minute                       helloworld.4.auky6trawmdlcne8ad8phb0f1
    ```

    If you want to see the containers running on other nodes, you can ssh into
    those nodes and run the `docker ps` command.

## What's next?

At this point in the tutorial, you're finished with the `helloworld` service.
The next step shows how to [delete the service](delete-service.md).

<p style="margin-bottom:300px">&nbsp;</p>
