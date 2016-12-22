---
title: "Docker Glossary"
description: "Glossary of terms used around Docker"
keywords: "glossary, docker, terms, definitions"
---

<!-- This file is maintained within the docker/docker Github
     repository at https://github.com/docker/docker/. Make all
     pull requests against that repo. If you see this file in
     another repository, consider it read-only there, as it will
     periodically be overwritten by the definitive file. Pull
     requests which include edits to this file in other repositories
     will be rejected.
-->

# Glossary

A list of terms used around the Docker project.

## aufs

aufs (advanced multi layered unification filesystem) is a Linux [filesystem](#filesystem) that
Docker supports as a storage backend. It implements the
[union mount](http://en.wikipedia.org/wiki/Union_mount) for Linux file systems.

## base image

An image that has no parent is a **base image**.

## boot2docker

[boot2docker](http://boot2docker.io/) is a lightweight Linux distribution made
specifically to run Docker containers. The boot2docker management tool for Mac and Windows was deprecated and replaced by [`docker-machine`](#machine) which you can install with the Docker Toolbox.

## btrfs

btrfs (B-tree file system) is a Linux [filesystem](#filesystem) that Docker
supports as a storage backend. It is a [copy-on-write](http://en.wikipedia.org/wiki/Copy-on-write)
filesystem.

## build

build is the process of building Docker images using a [Dockerfile](#dockerfile).
The build uses a Dockerfile and a "context". The context is the set of files in the
directory in which the image is built.

## cgroups

cgroups is a Linux kernel feature that limits, accounts for, and isolates
the resource usage (CPU, memory, disk I/O, network, etc.) of a collection
of processes. Docker relies on cgroups to control and isolate resource limits.

*Also known as : control groups*

## Compose

[Compose](https://github.com/docker/compose) is a tool for defining and
running complex applications with Docker. With compose, you define a
multi-container application in a single file, then spin your
application up in a single command which does everything that needs to
be done to get it running.

*Also known as : docker-compose, fig*

## copy-on-write

Docker uses a
[copy-on-write](https://docs.docker.com/engine/userguide/storagedriver/imagesandcontainers/#/the-copy-on-write-strategy)
technique and a [union file system](#union-file-system) for both images and
containers to optimize resources and speed performance. Multiple copies of an
entity share the same instance and each one makes only specific changes to its
unique layer.

Multiple containers can share access to the same image, and make
container-specific changes on a writable layer which is deleted when
the container is removed. This speeds up container start times and performance.

Images are essentially layers of filesystems typically predicated on a base
image under a writable layer, and built up with layers of differences from the
base image. This minimizes the footprint of the image and enables shared
development.

For more about copy-on-write in the context of Docker, see [Understand images,
containers, and storage
drivers](https://docs.docker.com/engine/userguide/storagedriver/imagesandcontainers/).

## container

A container is a runtime instance of a [docker image](#image).

A Docker container consists of

- A Docker image
- Execution environment
- A standard set of instructions

The concept is borrowed from Shipping Containers, which define a standard to ship
goods globally. Docker defines a standard to ship software.

## data volume

A data volume is a specially-designated directory within one or more containers
that bypasses the Union File System. Data volumes are designed to persist data,
independent of the container's life cycle. Docker therefore never automatically
delete volumes when you remove a container, nor will it "garbage collect"
volumes that are no longer referenced by a container.


## Docker

The term Docker can refer to

- The Docker project as a whole, which is a platform for developers and sysadmins to
develop, ship, and run applications
- The docker daemon process running on the host which manages images and containers


## Docker for Mac

[Docker for Mac](https://docs.docker.com/docker-for-mac/) is an easy-to-install,
lightweight Docker development environment designed specifically for the Mac. A
native Mac application, Docker for Mac uses the macOS Hypervisor framework,
networking, and filesystem. It's the best solution if you want to build, debug,
test, package, and ship Dockerized applications on a Mac. Docker for Mac
supersedes [Docker Toolbox](#toolbox) as state-of-the-art Docker on macOS.


## Docker for Windows

[Docker for Windows](https://docs.docker.com/docker-for-windows/) is an
easy-to-install, lightweight Docker development environment designed
specifically for Windows 10 systems that support Microsoft Hyper-V
(Professional, Enterprise and Education). Docker for Windows uses Hyper-V for
virtualization, and runs as a native Windows app. It works with Windows Server
2016, and gives you the ability to set up and run Windows containers as well as
the standard Linux containers, with an option to switch between the two. Docker
for Windows is the best solution if you want to build, debug, test, package, and
ship Dockerized applications from Windows machines. Docker for Windows
supersedes [Docker Toolbox](#toolbox) as state-of-the-art Docker on Windows.

## Docker Hub

The [Docker Hub](https://hub.docker.com/) is a centralized resource for working with
Docker and its components. It provides the following services:

- Docker image hosting
- User authentication
- Automated image builds and work-flow tools such as build triggers and web hooks
- Integration with GitHub and Bitbucket


## Dockerfile

A Dockerfile is a text document that contains all the commands you would
normally execute manually in order to build a Docker image. Docker can
build images automatically by reading the instructions from a Dockerfile.

## filesystem

A file system is the method an operating system uses to name files
and assign them locations for efficient storage and retrieval.

Examples :

- Linux : ext4, aufs, btrfs, zfs
- Windows : NTFS
- macOS : HFS+

## image

Docker images are the basis of [containers](#container). An Image is an
ordered collection of root filesystem changes and the corresponding
execution parameters for use within a container runtime. An image typically
contains a union of layered filesystems stacked on top of each other. An image
does not have state and it never changes.

## libcontainer

libcontainer provides a native Go implementation for creating containers with
namespaces, cgroups, capabilities, and filesystem access controls. It allows
you to manage the lifecycle of the container performing additional operations
after the container is created.

## libnetwork

libnetwork provides a native Go implementation for creating and managing container
network namespaces and other network resources. It manage the networking lifecycle
of the container performing additional operations after the container is created.

## link

links provide a legacy interface to connect Docker containers running on the
same host to each other without exposing the hosts' network ports. Use the
Docker networks feature instead.

## Machine

[Machine](https://github.com/docker/machine) is a Docker tool which
makes it really easy to create Docker hosts on  your computer, on
cloud providers and inside your own data center. It creates servers,
installs Docker on them, then configures the Docker client to talk to them.

*Also known as : docker-machine*

## node

A [node](https://docs.docker.com/engine/swarm/how-swarm-mode-works/nodes/) is a physical or virtual
machine running an instance of the Docker Engine in swarm mode.

**Manager nodes** perform swarm management and orchestration duties. By default
manager nodes are also worker nodes.

**Worker nodes** execute tasks.

## overlay network driver

Overlay network driver provides out of the box multi-host network connectivity
for docker containers in a cluster.

## overlay storage driver

OverlayFS is a [filesystem](#filesystem) service for Linux which implements a
[union mount](http://en.wikipedia.org/wiki/Union_mount) for other file systems.
It is supported by the Docker daemon as a storage driver.

## registry

A Registry is a hosted service containing [repositories](#repository) of [images](#image)
which responds to the Registry API.

The default registry can be accessed using a browser at [Docker Hub](#docker-hub)
or using the `docker search` command.

## repository

A repository is a set of Docker images. A repository can be shared by pushing it
to a [registry](#registry) server. The different images in the repository can be
labeled using [tags](#tag).

Here is an example of the shared [nginx repository](https://hub.docker.com/_/nginx/)
and its [tags](https://hub.docker.com/r/library/nginx/tags/)


## service

A [service](https://docs.docker.com/engine/swarm/how-swarm-mode-works/services/) is the definition of how
you want to run your application containers in a swarm. At the most basic level
a service  defines which container image to run in the swarm and which commands
to run in the container. For orchestration purposes, the service defines the
"desired state", meaning how many containers to run as tasks and constraints for
deploying the containers.

Frequently a service is a microservice within the context of some larger
application. Examples of services might include an HTTP server, a database, or
any other type of executable program that you wish to run in a distributed
environment.

## service discovery

Swarm mode [service discovery](https://docs.docker.com/engine/swarm/networking/#use-swarm-mode-service-discovery) is a DNS component
internal to the swarm that automatically assigns each service on an overlay
network in the swarm a VIP and DNS entry. Containers on the network share DNS
mappings for the service via gossip so any container on the network can access
the service via its service name.

You don’t need to expose service-specific ports to make the service available to
other services on the same overlay network. The swarm’s internal load balancer
automatically distributes requests to the service VIP among the active tasks.

## swarm

A [swarm](https://docs.docker.com/engine/swarm/) is a cluster of one or more Docker Engines running in [swarm mode](#swarm-mode).

## Docker Swarm

Do not confuse [Docker Swarm](https://github.com/docker/swarm) with the [swarm mode](#swarm-mode) features in Docker Engine.

Docker Swarm is the name of a standalone native clustering tool for Docker.
Docker Swarm pools together several Docker hosts and exposes them as a single
virtual Docker host. It serves the standard Docker API, so any tool that already
works with Docker can now transparently scale up to multiple hosts.

*Also known as : docker-swarm*

## swarm mode

[Swarm mode](https://docs.docker.com/engine/swarm/) refers to cluster management and orchestration
features embedded in Docker Engine. When you initialize a new swarm (cluster) or
join nodes to a swarm, the Docker Engine runs in swarm mode.

## tag

A tag is a label applied to a Docker image in a [repository](#repository).
tags are how various images in a repository are distinguished from each other.

*Note : This label is not related to the key=value labels set for docker daemon*

## task

A [task](https://docs.docker.com/engine/swarm/how-swarm-mode-works/services/#/tasks-and-scheduling) is the
atomic unit of scheduling within a swarm. A task carries a Docker container and
the commands to run inside the container. Manager nodes assign tasks to worker
nodes according to the number of replicas set in the service scale.

The diagram below illustrates the relationship of services to tasks and
containers.

![services diagram](https://docs.docker.com/engine/swarm/images/services-diagram.png)

## Toolbox

[Docker Toolbox](https://docs.docker.com/toolbox/overview/) is a legacy
installer for Mac and Windows users. It uses Oracle VirtualBox for
virtualization.

For Macs running OS X El Capitan 10.11 and newer macOS releases, [Docker for
Mac](https://docs.docker.com/docker-for-mac/) is the better solution.

For Windows 10 systems that support Microsoft Hyper-V (Professional, Enterprise
and Education), [Docker for
Windows](https://docs.docker.com/docker-for-windows/) is the better solution.

## Union file system

Union file systems implement a [union
mount](https://en.wikipedia.org/wiki/Union_mount) and operate by creating
layers. Docker uses union file systems in conjunction with
[copy-on-write](#copy-on-write) techniques to provide the building blocks for
containers, making them very lightweight and fast.

For more on Docker and union file systems, see [Docker and AUFS in
practice](https://docs.docker.com/engine/userguide/storagedriver/aufs-driver/),
[Docker and Btrfs in
practice](https://docs.docker.com/engine/userguide/storagedriver/btrfs-driver/),
and [Docker and OverlayFS in
practice](https://docs.docker.com/engine/userguide/storagedriver/overlayfs-driver/)

Example implementations of union file systems are
[UnionFS](https://en.wikipedia.org/wiki/UnionFS),
[AUFS](https://en.wikipedia.org/wiki/Aufs), and
[Btrfs](https://btrfs.wiki.kernel.org/index.php/Main_Page).

## virtual machine

A virtual machine is a program that emulates a complete computer and imitates dedicated hardware.
It shares physical hardware resources with other users but isolates the operating system. The
end user has the same experience on a Virtual Machine as they would have on dedicated hardware.

Compared to containers, a virtual machine is heavier to run, provides more isolation,
gets its own set of resources and does minimal sharing.

*Also known as : VM*
