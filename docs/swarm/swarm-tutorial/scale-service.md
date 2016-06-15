<!--[metadata]>
+++
title = "Scale the service"
description = "Scale the service running in the Swarm"
keywords = ["tutorial, cluster management, swarm, scale"]
[menu.main]
identifier="swarm-tutorial-scale-service"
parent="swarm-tutorial"
weight=18
advisory = "rc"
+++
<![end-metadata]-->

# Scale the service in the Swarm

Once you have [deployed a service](deploy-service.md) to a Swarm, you are ready
to use the Docker CLI to scale the number of service tasks in
the Swarm.

1. If you haven't already, open a terminal and ssh into the machine where you
run your manager node. For example, the tutorial uses a machine named
`manager1`.

2. Run the following command to change the desired state of the
service runing in the Swarm:

    ```bash
    $ docker service update --replicas <NUMBER-OF-TASKS> <SERVICE-ID>
    ```

    The `--replicas` flag indicates the number of tasks you want in the new
    desired state. For example:

    ```bash
    $ docker service update --replicas 5 helloworld
    helloworld
    ```

3. Run `docker service tasks <SERVICE-ID>` to see the updated task list:

    ```
    $ docker service tasks helloworld

    ID                         NAME          SERVICE     IMAGE   DESIRED STATE  LAST STATE          NODE
    1n6wif51j0w840udalgw6hphg  helloworld.1  helloworld  alpine  RUNNING        RUNNING 2 minutes   manager1
    dfhsosk00wxfb7j0cazp3fmhy  helloworld.2  helloworld  alpine  RUNNING        RUNNING 15 seconds  worker2
    6cbedbeywo076zn54fnwc667a  helloworld.3  helloworld  alpine  RUNNING        RUNNING 15 seconds  worker1
    7w80cafrry7asls96lm2tmwkz  helloworld.4  helloworld  alpine  RUNNING        RUNNING 10 seconds  worker1
    bn67kh76crn6du22ve2enqg5j  helloworld.5  helloworld  alpine  RUNNING        RUNNING 10 seconds  manager1
    ```

    You can see that Swarm has created 4 new tasks to scale to a total of 5
    running instances of Alpine Linux. The tasks are distributed between the
    three nodes of the Swarm. Two are running on `manager1`.

4. Run `docker ps` to see the containers running on the node where you're
connected. The following example shows the tasks running on `manager1`:

    ```
    $ docker ps

    CONTAINER ID        IMAGE               COMMAND             CREATED             STATUS              PORTS               NAMES
    910669d5e188        alpine:latest       "ping docker.com"   10 seconds ago      Up 10 seconds                           helloworld.5.bn67kh76crn6du22ve2enqg5j
    a0b6c02868ca        alpine:latest       "ping docker.com"   2 minutes  ago      Up 2 minutes                            helloworld.1.1n6wif51j0w840udalgw6hphg
    ```

    If you want to see the containers running on other nodes, you can ssh into
    those nodes and run the `docker ps` command.

## What's next?

At this point in the tutorial, you're finished with the `helloworld` service.
The next step shows how to [delete the service](delete-service.md).

<p style="margin-bottom:300px">&nbsp;</p>
