<!--[metadata]>
+++
title = "Swarm overview"
description = "Docker Swarm overview"
keywords = ["docker, container, cluster, swarm"]
[menu.main]
identifier="swarm_overview"
parent="engine_swarm"
weight="1"
advisory = "rc"
+++
<![end-metadata]-->
# Docker Swarm overview

To use this version of Swarm, install the Docker Engine `v1.12.0-rc1` or later
from the [Docker releases GitHub
repository](https://github.com/docker/docker/releases). Alternatively, install
the latest Docker for Mac or Docker for Windows Beta.

Docker Engine 1.12 includes Docker Swarm for natively managing a cluster of
Docker Engines called a Swarm. Use the Docker CLI to create a Swarm, deploy
application services to the Swarm, and manage the Swarm behavior.


If you’re using a Docker version prior to `v1.12.0-rc1`, see [Docker
Swarm](https://docs.docker.com/swarm).

## Feature highlights

* **Cluster management integrated with Docker Engine:** Use the Docker Engine
CLI to create a Swarm of Docker Engines where you can deploy application
services. You don't need additional orchestration software to create or manage
a Swarm.

* **Decentralized design:** Instead of handling differentiation between node
roles at deployment time, Swarm handles any specialization at runtime. You can
deploy both kinds of nodes, managers and workers, using the Docker Engine.
This means you can build an entire Swarm from a single disk image.

* **Declarative service model:** Swarm uses a declarative syntax to let you
define the desired state of the various services in your application stack.
For example, you might describe an application comprised of a web front end
service with message queueing services and a database backend.

* **Desired state reconciliation:** Swarm constantly monitors the cluster state
and reconciles any differences between the actual state your expressed desired
state.

* **Multi-host networking:** You can specify an overlay network for your
application. Swarm automatically assigns addresses to the containers on the
overlay network when it initializes or updates the application.

* **Service discovery:** Swarm assigns each service a unique DNS name and load
balances running containers. Each Swarm has an internal DNS server that can
query every container in the cluster using DNS.

* **Load balancing:** Using Swarm, you can expose the ports for services to an
external load balancer. Internally, Swarm lets you specify how to distribute
service containers between nodes.

* **Secure by default:** Each node in the Swarm enforces TLS mutual
authentication and encryption to secure communications between itself and all
other nodes. You have the option to use self-signed root certificates or
certificates from a custom root CA.

* **Scaling:** For each service, you can declare the number of instances you
want to run. When you scale up or down, Swarm automatically adapts by adding
or removing instances of the service to maintain the desired state.

* **Rolling updates:** At rollout time you can apply service updates to nodes
incrementally. Swarm lets you control the delay between service deployment to
different sets of nodes. If anything goes wrong, you can roll-back an instance
of a service.

## What's next?
* Learn Swarm [key concepts](key-concepts.md).
* Get started with the [Swarm tutorial](swarm-tutorial/index.md).

<p style="margin-bottom:300px">&nbsp;</p>
