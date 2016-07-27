<!--[metadata]>
+++
title = "Swarm mode key concepts"
description = "Introducing key concepts for Docker Engine swarm mode"
keywords = ["docker, container, cluster, swarm mode"]
[menu.main]
identifier="swarm-mode-concepts"
parent="engine_swarm"
weight="2"
+++
<![end-metadata]-->
# Swarm mode key concepts

This topic introduces some of the concepts unique to the cluster management and
orchestration features of Docker Engine 1.12.

## Swarm

The cluster management and orchestration features embedded in the Docker Engine
are built using **SwarmKit**. Engines participating in a cluster are
running in **swarm mode**. You enable swarm mode for the Engine by either
initializing a swarm or joining an existing swarm.

A **swarm** is a cluster of Docker Engines where you deploy
[services](#Services-and-tasks). The Docker Engine CLI includes the commands for
swarm management, such as adding and removing nodes. The CLI also includes the
commands you need to deploy services to the swarm and manage service
orchestration.

When you run Docker Engine outside of swarm mode, you execute container
commands. When you run the Engine in swarm mode, you orchestrate services.

## Node

A **node** is an instance of the Docker Engine participating in the swarm.

To deploy your application to a swarm, you submit a service definition to a
**manager node**. The manager node dispatches units of work called
[tasks](#Services-and-tasks) to worker nodes.

Manager nodes also perform the orchestration and cluster management functions
required to maintain the desired state of the swarm. Manager nodes elect a single leader to conduct orchestration tasks.

**Worker nodes** receive and execute tasks dispatched from manager nodes. By
default manager nodes are also worker nodes, but you can configure managers to
be manager-only nodes. The agent notifies the manager node of the current
state of its assigned tasks so the manager can maintain the desired state.

## Services and tasks

A **service** is the definition of the tasks to execute on the worker nodes. It
is the central structure of the swarm system and the primary root of user
interaction with the swarm.

When you create a service, you specify which container image to use and which
commands to execute inside running containers.

In the **replicated services** model, the swarm manager distributes a specific
number of replica tasks among the nodes based upon the scale you set in the
desired state.

For **global services**, the swarm runs one task for the service on every
available node in the cluster.

A **task** carries a Docker container and the commands to run inside the
container. It is the atomic scheduling unit of swarm. Manager nodes assign tasks
to worker nodes according to the number of replicas set in the service scale.
Once a task is assigned to a node, it cannot move to another node. It can only
run on the assigned node or fail.

## Load balancing

The swarm manager uses **ingress load balancing** to expose the services you
want to make available externally to the swarm. The swarm manager can
automatically assign the service a **PublishedPort** or you can configure a
PublishedPort for the service in the 30000-32767 range.

External components, such as cloud load balancers, can access the service on the
PublishedPort of any node in the cluster whether or not the node is currently
running the task for the service.  All nodes in the swarm cluster route ingress
connections to a running task instance.

Swarm mode has an internal DNS component that automatically assigns each service
in the swarm a DNS entry. The swarm manager uses **internal load balancing** to
distribute requests among services within the cluster based upon the DNS name of
the service.

## What's next?
* Read the [swarm mode overview](index.md).
* Get started with the [swarm mode tutorial](swarm-tutorial/index.md).
