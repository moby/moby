<!--[metadata]>
+++
title = "Deploy a service"
description = "Deploy a service to the swarm"
keywords = ["tutorial, cluster management, swarm mode"]
[menu.main]
identifier="deploy-application"
parent="swarm-tutorial"
weight=16
+++
<![end-metadata]-->

# Deploy a service to the swarm

After you [create a swarm](create-swarm.md), you can deploy a service to the
swarm. For this tutorial, you also [added worker nodes](add-nodes.md), but that
is not a requirement to deploy a service.

1. Open a terminal and ssh into the machine where you run your manager node. For
example, the tutorial uses a machine named `manager1`.

2. Run the the following command:

    ```bash
    $ docker service create --replicas 1 --name helloworld alpine ping docker.com

    9uk4639qpg7npwf3fn2aasksr
    ```

    * The `docker service create` command creates the service.
    * The `--name` flag names the service `helloworld`.
    * The `--replicas` flag specifies the desired state of 1 running instance.
    * The arguments `alpine ping docker.com` define the service as an Alpine
    Linux container that executes the command `ping docker.com`.

3. Run `docker service ls` to see the list of running services:

    ```
    $ docker service ls

    ID            NAME        SCALE  IMAGE   COMMAND
    9uk4639qpg7n  helloworld  1/1    alpine  ping docker.com
    ```

## What's next?

Now you've deployed a service to the swarm, you're ready to [inspect the service](inspect-service.md).
