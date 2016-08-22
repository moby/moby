<!--[metadata]>
+++
title = "Swarm mode overview"
description = "Docker Engine swarm mode overview"
keywords = ["docker, container, cluster, swarm"]
[menu.main]
identifier="swarm_overview"
parent="engine_swarm"
weight="1"
+++
<![end-metadata]-->
# Swarm mode overview

To use Docker Engine in swarm mode, install the Docker Engine `v1.12.0` or
later from the [Docker releases GitHub
repository](https://github.com/docker/docker/releases). Alternatively, install
the latest Docker for Mac or Docker for Windows Beta.

Docker Engine 1.12 includes swarm mode for natively managing a cluster of
Docker Engines called a *swarm*. Use the Docker CLI to create a swarm, deploy
application services to a swarm, and manage swarm behavior.


If you’re using a Docker version prior to `v1.12.0`, see [Docker
Swarm](https://docs.docker.com/swarm).

## Feature highlights

* **Cluster management integrated with Docker Engine:** Use the Docker Engine
CLI to create a swarm of Docker Engines where you can deploy application
services. You don't need additional orchestration software to create or manage
a swarm.

* **Decentralized design:** Instead of handling differentiation between node
roles at deployment time, the Docker Engine handles any specialization at
runtime. You can deploy both kinds of nodes, managers and workers, using the
Docker Engine. This means you can build an entire swarm from a single disk
image.

* **Declarative service model:** Docker Engine uses a declarative approach to
let you define the desired state of the various services in your application
stack. For example, you might describe an application comprised of a web front
end service with message queueing services and a database backend.

* **Scaling:** For each service, you can declare the number of tasks you want to
run. When you scale up or down, the swarm manager automatically adapts by
adding or removing tasks to maintain the desired state.

* **Desired state reconciliation:** The swarm manager node constantly monitors
the cluster state and reconciles any differences between the actual state your
expressed desired state. For example, if you set up a service to run 10
replicas of a container, and a worker machine hosting two of those replicas
crashes, the manager will create two new replicas to replace the replicas that
crashed. The swarm manager assigns the new replicas to workers that are
running and available.

* **Multi-host networking:** You can specify an overlay network for your
services. The swarm manager automatically assigns addresses to the containers
on the overlay network when it initializes or updates the application.

* **Service discovery:** Swarm manager nodes assign each service in the swarm a
unique DNS name and load balances running containers. You can query every
container running in the swarm through a DNS server embedded in the swarm.

* **Load balancing:** You can expose the ports for services to an
external load balancer. Internally, the swarm lets you specify how to distribute
service containers between nodes.

* **Secure by default:** Each node in the swarm enforces TLS mutual
authentication and encryption to secure communications between itself and all
other nodes. You have the option to use self-signed root certificates or
certificates from a custom root CA.

* **Rolling updates:** At rollout time you can apply service updates to nodes
incrementally. The swarm manager lets you control the delay between service
deployment to different sets of nodes. If anything goes wrong, you can
roll-back a task to a previous version of the service.

## What's next?
* Learn swarm mode [key concepts](key-concepts.md).
* Get started with the [swarm mode tutorial](swarm-tutorial/index.md).
* Explore swarm mode CLI commands:
    * [swarm init](../reference/commandline/swarm_init.md)
    * [swarm join](../reference/commandline/swarm_join.md)
    * [service create](../reference/commandline/service_create.md)
    * [service inspect](../reference/commandline/service_inspect.md)
    * [service ls](../reference/commandline/service_ls.md)
    * [service rm](../reference/commandline/service_rm.md)
    * [service scale](../reference/commandline/service_scale.md)
    * [service ps](../reference/commandline/service_ps.md)
    * [service update](../reference/commandline/service_update.md)
