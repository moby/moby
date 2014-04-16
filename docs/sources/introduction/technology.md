page_title: Understanding the Technology
page_description: Technology of Docker explained in depth
page_keywords: docker, introduction, documentation, about, technology, understanding, Dockerfile

# Understanding the Technology

*What is the architecture of Docker? What is its underlying technology?*

## Introduction

When it comes to understanding Docker and its underlying technology
there is no *magic* involved. Everything is based on tried and tested
features of the *Linux kernel*. Docker either makes use of those
features directly or builds upon them to provide new functionality.

Aside from the technology, one of the major factors that make Docker
great is the way it is built. The project's core is very lightweight and
as much of Docker as possible is designed to be pluggable. Docker is
also built with integration in mind and has a fully featured API that
allows you to access all of the power of Docker from inside your own
applications.

## The Architecture of Docker

Docker is designed for developers and sysadmins. It's built to help you
build applications and services and then deploy them quickly and
efficiently: from development to production.

Let's take a look.

-- Docker is a client-server application.  
-- Both the Docker client and the daemon *can* run on the same system, or;  
-- You can connect a Docker client with a remote Docker daemon.  
-- They communicate via sockets or through a RESTful API.  
-- Users interact with the client to command the daemon, e.g. to create, run, and stop containers.  
-- The daemon, receiving those commands, does the job, e.g. run a container, stop a container.

![Docker Architecture Diagram](/article-img/architecture.svg)

## The components of Docker

Docker's main components are:

 - Docker *daemon*;
 - Docker *client*, and;
 - The Docker Index.

### The Docker daemon

As shown on the diagram above, the Docker daemon runs on a host machine.
The user does not directly interact with the daemon, but instead through
an intermediary: the Docker client.

### Docker client

The Docker client is the primary user interface to Docker. It is tasked
with accepting commands from the user and communicating back and forth
with a Docker daemon to manage the container lifecycle on any host.

### Docker Index, the central Docker registry

The [Docker Index](http://index.docker.io) is the global archive (and
directory) of user supplied Docker container images. It currently hosts
a large – in fact, rapidly growing – number of projects where you
can find almost any popular application or deployment stack readily
available to download and run with a single command.

As a social community project, Docker tries to provide all necessary
tools for everyone to grow with other *Dockers*. By issuing a single
command through the Docker client you can start sharing your own
creations with the rest of the world.

However, knowing that not everything can be shared the Docker Index also
offers private repositories. In order to see the available plans, you
can click [here](https://index.docker.io/plans).

Using the [Docker Registry](https://github.com/dotcloud/docker-registry), it is
also possible to run your own private Docker image registry service on your own
servers.

> **Note:** To learn more about the [*Docker Image Index*](
> http://index.docker.io) (public *and* private), check out the [Registry &
> Index Spec](http://docs.docker.io/en/latest/api/registry_index_spec/).

### Summary

 - **When you install Docker, you get all the components:**  
 The daemon, the client and access to the public image registry: the [Docker Index](http://index.docker.io).
 - **You can run these components together or distributed:**  
 Servers with the Docker daemon running, controlled by the Docker client.
 - **You can benefit form the public registry:**  
 Download and build upon images created by the community.
 - **You can start a private repository for proprietary use.**  
 Sign up for a [plan](https://index.docker.io/plans) or host your own [Docker registry](https://github.com/dotcloud/docker-registry).

## Elements of Docker

The basic elements of Docker are:

 - **Containers, which allow:**  
 The run portion of Docker. Your applications run inside of containers.
 - **Images, which provide:**  
 The build portion of Docker. Your containers are built from images.
 - **The Dockerfile, which automates:**  
 A file that contains simple instructions that build Docker images.

To get practical and learn what they are, and **_how to work_** with
them, continue to [Working with Docker](working-with-docker.md). If you would like to
understand **_how they work_**, stay here and continue reading.

## The underlying technology

The power of Docker comes from the underlying technology it is built
from. A series of operating system features are carefully glued together
to provide Docker's features and provide an easy to use interface to
those features. In this section, we will see the main operating system
features that Docker uses to make easy containerization happen.

### Namespaces

Docker takes advantage of a technology called `namespaces` to provide
an isolated workspace we call a *container*.  When you run a container,
Docker creates a set of *namespaces* for that container.

This provides a layer of isolation: each process runs in its own
namespace and does not have access outside it.

Some of the namespaces Docker uses are:

 - **The `pid` namespace:**  
 Used for process numbering (PID: Process ID)
 - **The `net` namespace:**  
 Used for managing network interfaces (NET: Networking)
 - **The `ipc` namespace:**  
 Used for managing access to IPC resources (IPC: InterProcess Communication)
 - **The `mnt` namespace:**  
 Used for managing mount-points (MNT: Mount)
 - **The `uts` namespace:**  
 Used for isolating kernel / version identifiers. (UTS: Unix Timesharing System)

### Control groups

Docker also makes use of another technology called `cgroups` or control
groups. A key need to run applications in isolation is to have them
contained, not just in terms of related filesystem and/or dependencies,
but also, resources. Control groups allow Docker to fairly
share available hardware resources to containers and if asked, set up to
limits and constraints, for example limiting the memory to a maximum of 128
MBs.

### UnionFS

UnionFS or union filesystems are filesystems that operate by creating
layers, making them very lightweight and fast. Docker uses union
filesystems to provide the building blocks for containers. We'll see
more about this below.

### Containers

Docker combines these components to build a container format we call
`libcontainer`. Docker also supports traditional Linux containers like
[LXC](https://linuxcontainers.org/) which also make use of these
components.

## How does everything work

A lot happens when Docker creates a container.

Let's see how it works!

### How does a container work?

A container consists of an operating system, user added files and
meta-data. Each container is built from an image. That image tells
Docker what the container holds, what process to run when the container
is launched and a variety of other configuration data. The Docker image
is read-only. When Docker runs a container from an image it adds a
read-write layer on top of the image (using the UnionFS technology we
saw earlier) to run inside the container.

### What happens when you run a container?

The Docker client (or the API!) tells the Docker daemon to run a
container. Let's take a look at a simple `Hello world` example.

    $ docker run -i -t ubuntu /bin/bash

Let's break down this command. The Docker client is launched using the
`docker` binary. The bare minimum the Docker client needs to tell the
Docker daemon is:

* What Docker image to build the container from;
* The command you want to run inside the container when it is launched.

So what happens under the covers when we run this command?

Docker begins with:

 - **Pulling the `ubuntu` image:**  
 Docker checks for the presence of the `ubuntu` image and if it doesn't
 exist locally on the host, then Docker downloads it from the [Docker Index](https://index.docker.io)
 - **Creates a new container:**  
 Once Docker has the image it creates a container from it.
 - **Allocates a filesystem and mounts a read-write _layer_:**  
 The container is created in the filesystem and a read-write layer is added to the image.
 - **Allocates a network / bridge interface:**  
 Creates a network interface that allows the Docker container to talk to the local host.
 - **Sets up an IP address:**  
 Intelligently finds and attaches an available IP address from a pool.
 - **Executes _a_ process that you specify:**  
 Runs your application, and;
 - **Captures and provides application output:**  
 Connects and logs standard input, outputs and errors for you to see how your application is running.

### How does a Docker Image work?

We've already seen that Docker images are read-only templates that
Docker containers are launched from. When you launch that container it
creates a read-write layer on top of that image that your application is
run in.

Docker images are built using a simple descriptive set of steps we
call *instructions*. Instructions are stored in a file called a
`Dockerfile`. Each instruction writes a new layer to an image using the
UnionFS technology we saw earlier.

Every image starts from a base image, for example `ubuntu` a base Ubuntu
image or `fedora` a base Fedora image. Docker builds and provides these
base images via the [Docker Index](http://index.docker.io).

### How does a Docker registry work?

The Docker registry is a store for your Docker images. Once you build a
Docker image you can *push* it to the [Docker
Index](http://index.docker.io) or to a private registry you run behind
your firewall.

Using the Docker client, you can search for already published images and
then pull them down to your Docker host to build containers from them
(or even build on these images).

The [Docker Index](http://index.docker.io) provides both public and
private storage for images. Public storage is searchable and can be
downloaded by anyone. Private repositories are excluded from search
results and only you and your users can pull them down and use them to
build containers. You can [sign up for a plan here](https://index.docker.io/plans).

To learn more, check out the [Working With Repositories](
http://docs.docker.io/en/latest/use/workingwithrepository) section of our
[User's Manual](http://docs.docker.io).

## Where to go from here

### Understanding Docker

Visit [Understanding Docker](understanding-docker.md) in our Getting Started manual.

### Get practical and learn how to use Docker straight away

Visit [Working with Docker](working-with-docker.md) in our Getting Started manual.

### Get the product and go hands-on

Visit [Get Docker](get-docker.md) in our Getting Started manual.

### Get the whole story

[https://www.docker.io/the_whole_story/](https://www.docker.io/the_whole_story/)
