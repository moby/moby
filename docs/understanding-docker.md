<!--[metadata]>
+++
aliases = [
"/introduction/understanding-docker/",
"/engine/userguide/basics/",
"/engine/quickstart.md"
]
title = "Docker Overview"
description = "Docker explained in depth"
keywords = ["docker, introduction, documentation, about, technology,  understanding"]
[menu.main]
parent = "engine_use"
weight = -90
+++
<![end-metadata]-->

# Docker Overview
Docker is an open platform for developing, shipping, and running applications.
Docker enables you to separate your applications from your infrastructure so
you can deliver software quickly. With Docker, you can manage your infrastructure
in the same ways you manage your applications. By taking advantage of Docker's
methodologies for shipping, testing, and deploying code quickly, you can
significantly reduce the delay between writing code and running it in production.

## What is the Docker platform?

Docker provides the ability to package and run an application in a loosely isolated
environment called a container. The isolation and security allow you to run many
containers simultaneously on a given host. Because of the lightweight nature of
containers, which run without the extra load of a hypervisor, you can run more
containers on a given hardware combination than if you were using virtual machines.

Docker provides tooling and a platform to manage the lifecycle of your containers:

* Encapsulate your applications (and supporting components) into Docker containers
* Distribute and ship those containers to your teams for further development
  and testing
* Deploy those applications to your production environment, whether it is in a
  local data center or the Cloud

## What is Docker Engine?

_Docker Engine_ is a client-server application with these major components:

* A server which is a type of long-running program called a daemon process.

* A REST API which specifies interfaces that programs can use to talk to the
    daemon and instruct it what to do.

* A command line interface (CLI) client.

![Docker Engine Components Flow](article-img/engine-components-flow.png)

The CLI uses the Docker REST API to control or interact with the Docker daemon
through scripting or direct CLI commands. Many other Docker applications use the
underlying API and CLI.

The daemon creates and manages Docker _objects_, such as images, containers,
networks, and data volumes.

> **Note:** Docker is licensed under the open source Apache 2.0 license.

## What can I use Docker for?

*Fast, consistent delivery of your applications*

Docker can streamline the development lifecycle by allowing developers to work in
standardized environments using local containers which provide your applications
and services. You can also integrate Docker into your continuous integration and
continuous deployment (CI/CD) workflow.

Consider the following example scenario. Your developers write code locally and
share their work with their colleagues using Docker containers. They can use
Docker to push their applications into a test environment and execute automated
and manual tests. When developers find problems, they can fix them in the development
environment and redeploy them to the test environment for testing. When testing is
complete, getting the fix to the customer is as simple as pushing the updated image
to the production environment.

*Responsive deployment and scaling*

Docker's container-based platform allows for highly portable workloads. Docker
containers can run on a developer's local host, on physical or virtual machines
in a data center, in the Cloud, or in a mixture of environments.

Docker's portability and lightweight nature also make it easy to dynamically manage
workloads, scaling up or tearing down applications and services as business
needs dictate, in near real time.

*Running more workloads on the same hardware*

Docker is lightweight and fast. It provides a viable, cost-effective alternative
to hypervisor-based virtual machines, allowing you to use more of your compute
capacity to achieve your business goals. This is useful in high density
environments and for small and medium deployments where you need to do more with
fewer resources.

## What is Docker's architecture?
Docker uses a client-server architecture. The Docker *client* talks to the
Docker *daemon*, which does the heavy lifting of building, running, and
distributing your Docker containers. The Docker client and daemon *can*
run on the same system, or you can connect a Docker client to a remote Docker
daemon. The Docker client and daemon communicate via sockets or through a
REST API.

![Docker Architecture Diagram](article-img/architecture.svg)

### The Docker daemon
The Docker daemon runs on a host machine. The user uses the Docker client to
interact with the daemon.

### The Docker client
The Docker client, in the form of the `docker` binary, is the primary user
interface to Docker. It accepts commands and configuration flags from the user and
communicates with a Docker daemon. One client can even communicate with multiple
unrelated daemons.

### Inside Docker
To understand Docker's internals, you need to know about _images_, _registries_,
and _containers_.

#### Docker images

A Docker _image_ is a read-only template with instructions for creating a Docker
container. For example, an image might contain an Ubuntu operating system with
Apache web server and your web application installed. You can build or update
images from scratch or download and use images created by others. An image may be
based on, or may extend, one or more other images. A docker image is described in
text file called a _Dockerfile_, which has a simple, well-defined syntax. For more
details about images, see [How does a Docker image work?](#how-does-a-docker-image-work).

Docker images are the **build** component of Docker.

#### Docker containers
A Docker container is a runnable instance of a Docker image. You can run, start,
stop, move, or delete a container using Docker API or CLI commands. When you run
a container, you can provide configuration metadata such as networking information
or environment variables. Each container is an isolated and secure application
platform, but can be given access to resources running in a different host or
container, as well as persistent storage or databases. For more details about
containers, see [How does a container work?](#how-does-a-container-work).

Docker containers are the **run** component of Docker.

#### Docker registries
A docker registry is a library of images. A registry can be public or private,
and can be on the same server as the Docker daemon or Docker client, or on a
totally separate server. For more details about registries, see
[How does a Docker registry work?](#how-does-a-docker-registry-work)

Docker registries are the **distribution** component of Docker.

#### Docker services
A Docker _service_ allows a _swarm_ of Docker nodes to work together, running a
defined number of instances of a replica task, which is itself a Docker image.
You can specify the number of concurrent replica tasks to run, and the swarm
manager ensures that the load is spread evenly across the worker nodes. To
the consumer, the Docker service appears to be a single application. Docker
Engine supports swarm mode in Docker 1.12 and higher.

Docker services are the **scalability** component of Docker.

### How does a Docker image work?
Docker images are read-only templates from which Docker containers are instantiated.
Each image consists of a series of layers. Docker uses
[union file systems](http://en.wikipedia.org/wiki/UnionFS) to
combine these layers into a single image. Union file systems allow files and
directories of separate file systems, known as branches, to be transparently
overlaid, forming a single coherent file system.

These layers are one of the reasons Docker is so lightweight. When you
change a Docker image, such as when you update an application to a new version,
a new layer is built and replaces only the layer it updates. The other layers
remain intact. To distribute the update, you only need to transfer the updated
layer. Layering speeds up distribution of Docker images. Docker determines which
layers need to be updated at runtime.

An image is defined in a Dockerfile. Every image starts from a base image, such as
`ubuntu`, a base Ubuntu image, or `fedora`, a base Fedora image. You can also use
images of your own as the basis for a new image, for example if you have a base
Apache image you could use this as the base of all your web application images. The
base image is defined using the `FROM` keyword in the dockerfile.

> **Note:** [Docker Hub](https://hub.docker.com) is a public registry and stores
images.

The docker image is built from the base image using a simple, descriptive
set of steps we call *instructions*, which are stored in a `Dockerfile`. Each
instruction creates a new layer in the image. Some examples of Dockerfile
instructions are:

* Specify the base image (`FROM`)
* Specify the maintainer (`MAINTAINER`)
* Run a command (`RUN`)
* Add a file or directory (`ADD`)
* Create an environment variable (`ENV`)
* What process to run when launching a container from this image (`CMD`)

Docker reads this `Dockerfile` when you request a build of
an image, executes the instructions, and returns the image.

### How does a Docker registry work?
A Docker registry stores Docker images. After you build a Docker image, you
can *push* it to a public registry such as [Docker Hub](https://hub.docker.com)
or to a private registry running behind your firewall. You can also search for
existing images and pull them from the registry to a host.

[Docker Hub](http://hub.docker.com) is a public Docker
registry which serves a huge collection of existing images and allows you to
contribute your own. For more information, go to
[Docker Registry](https://docs.docker.com/registry/overview/) and
[Docker Trusted Registry](https://docs.docker.com/docker-trusted-registry/overview/).

[Docker store](http://store.docker.com) allows you to buy and sell Docker images.
For image, you can buy a Docker image containing an application or service from
the software vendor, and use the image to deploy the application into your
testing, staging, and production environments, and upgrade the application by pulling
the new version of the image and redeploying the containers. Docker Store is currently
in private beta.

### How does a container work?
A container uses the host machine's Linux kernel, and consists of any extra files
you add when the image is created, along with metadata associated with the container
at creation or when the container is started. Each container is built from an image.
The image defines the container's contents, which process to run when the container
is launched, and a variety of other configuration details. The Docker image is
read-only. When Docker runs a container from an image, it adds a read-write layer
on top of the image (using a UnionFS as we saw earlier) in which your application
runs.

#### What happens when you run a container?
When you use the `docker run` CLI command or the equivalent API, the Docker Engine
client instructs the Docker daemon to run a container. This example tells the
Docker daemon to run a container using the `ubuntu` Docker image, to remain in
the foreground in interactive mode (`-i`), and to run the `/bin/bash` command.

    $ docker run -i -t ubuntu /bin/bash


When you run this command, Docker Engine does the following:

1. **Pulls the `ubuntu` image:** Docker Engine checks for the presence of the
    `ubuntu` image. If the image already exists locally, Docker Engine uses it for
    the new container. Otherwise, then Docker Engine pulls it from
    [Docker Hub](https://hub.docker.com).

1. **Creates a new container:** Docker uses the image to create a container.

1. **Allocates a filesystem and mounts a read-write _layer_:** The container is
    created in the file system and a read-write layer is added to the image.

1. **Allocates a network / bridge interface:** Creates a network interface that
    allows the Docker container to talk to the local host.

1. **Sets up an IP address:** Finds and attaches an available IP address from a
    pool.

1. **Executes a process that you specify:** Executes the `/bin/bash` executable.

1. **Captures and provides application output:** Connects and logs standard input,
    outputs and errors for you to see how your application is running, because you
    requested interactive mode.

Your container is now running. You can manage and interact with it, use the services
and applications it provides, and eventually stop and remove it.

## The underlying technology
Docker is written in [Go](https://golang.org/) and takes advantage of several
features of the Linux kernel to deliver its functionality.

### Namespaces
Docker uses a technology called `namespaces` to provide the isolated workspace
called the *container*.  When you run a container, Docker creates a set of
*namespaces* for that container.

These namespaces provide a layer of isolation. Each aspect of a container runs
in a separate namespace and its access is limited to that namespace.

Docker Engine uses namespaces such as the following on Linux:

 - **The `pid` namespace:** Process isolation (PID: Process ID).
 - **The `net` namespace:** Managing network interfaces (NET:
 Networking).
 - **The `ipc` namespace:** Managing access to IPC
 resources (IPC: InterProcess Communication).
 - **The `mnt` namespace:** Managing filesystem mount points (MNT: Mount).
 - **The `uts` namespace:** Isolating kernel and version identifiers. (UTS: Unix
Timesharing System).

### Control groups
Docker Engine on Linux also relies on another technology called _control groups_
(`cgroups`). A cgroup limits an application to a specific set of resources.
Control groups allow Docker Engine to share available hardware resources to
containers and optionally enforce limits and constraints. For example,
you can limit the memory available to a specific container.

### Union file systems
Union file systems, or UnionFS, are file systems that operate by creating layers,
making them very lightweight and fast. Docker Engine uses UnionFS to provide
the building blocks for containers. Docker Engine can use multiple UnionFS variants,
including AUFS, btrfs, vfs, and DeviceMapper.

### Container format
Docker Engine combines the namespaces, control groups, and UnionFS into a wrapper
called a container format. The default container format is `libcontainer`. In
the future, Docker may support other container formats by integrating with
technologies such as BSD Jails or Solaris Zones.

## Next steps
- Read about [Installing Docker Engine](installation/index.md#installation).
- Get hands-on experience with the [Get Started With Docker](getstarted/index.md)
    tutorial.
- Check out examples and deep dive topics in the
    [Docker Engine User Guide](userguide/index.md).
