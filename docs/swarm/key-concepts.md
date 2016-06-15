<!--[metadata]>
+++
title = "Swarm key concepts"
description = "Introducing key concepts for Docker Swarm"
keywords = ["docker, container, cluster, swarm"]
[menu.main]
identifier="swarm-concepts"
parent="engine_swarm"
weight="2"
advisory = "rc"
+++
<![end-metadata]-->
# Docker Swarm key concepts

Building upon the core features of Docker Engine, Docker Swarm enables you to
create a Swarm of Docker Engines and orchestrate services to run in the Swarm.
This topic describes key concepts to help you begin using Docker Swarm.

## Swarm

**Docker Swarm** is the name for the cluster management and orchestration
features embedded in the Docker Engine. Engines that are participating in a
cluster are running in **Swarm mode**.

A **Swarm** is a cluster of Docker Engines where you deploy a set of application
services. When you deploy an application to a Swarm, you specify the desired
state of the services, such as which services to run and how many instances of
those services. The Swarm takes care of all orchestration duties required to
keep the services running in the desired state.

## Node

A **node** is an active instance of the Docker Engine in the Swarm.

When you deploy your application to a Swarm, **manager nodes** accept the
service definition that describes the Swarm's desired state. Manager nodes also
perform the orchestration and cluster management functions required to maintain
the desired state of the Swarm. For example, when a manager node receives notice
to deploy a web server, it dispatches the service tasks to worker nodes.

By default the Docker Engine starts one manager node for a Swarm, but as you
scale you can add more managers to make the cluster more fault-tolerant. If you
require high availability Swarm management, Docker recommends three or five
Managers in your cluster.

Because Swarm manager nodes share data using Raft, there must be an odd number
of managers. The Swarm cluster can continue functioning in the face of up to
`N/2` failures where `N` is the number of manager nodes.  More than five
managers is likely to degrade cluster performance and is not recommended.

**Worker nodes** receive and execute tasks dispatched from manager nodes. By
default manager nodes are also worker nodes, but you can configure managers to
be manager-only nodes.

## Services and tasks

A **service** is the definition of how to run the various tasks that make up
your application. For example, you may create a service that deploys a Redis
image in your Swarm.

A **task** is the atomic scheduling unit of Swarm. For example a task may be to
schedule a Redis container to run on a worker node.


## Service types

For **replicated services**, Swarm deploys a specific number of replica tasks
based upon the scale you set in the desired state.

For **global services**, Swarm runs one task for the service on every available
node in the cluster.

## Load balancing

Swarm uses **ingress load balancing** to expose the services you want to make
available externally to the Swarm. Swarm can automatically assign the service a
**PublishedPort** or you can configure a PublishedPort for the service in the
30000-32767 range. External components, such as cloud load balancers, can access
the service on the PublishedPort of any node in the cluster, even if the node is
not currently running the service.

Swarm has an internal DNS component that automatically assigns each service in
the Swarm DNS entry. Swarm uses **internal load balancing** distribute requests
among services within the cluster based upon the services' DNS name.

<p style="margin-bottom:300px">&nbsp;</p>
