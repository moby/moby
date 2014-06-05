page_title: Understanding Docker
page_description: Docker explained in depth
page_keywords: docker, introduction, documentation, about, technology, understanding

# Understanding Docker

**What is Docker?**

Docker is a platform for developing, shipping, and running applications.
Docker is designed to deliver your applications faster. With Docker you
can separate your applications from your infrastructure AND treat your
infrastructure like a managed application. We want to help you ship code
faster, test faster, deploy faster and shorten the cycle between writing
code and running code.

Docker does this by combining a lightweight container virtualization
platform with workflow and tooling that helps you manage and deploy your
applications.

At its core Docker provides a way to run almost any application securely
isolated into a container. The isolation and security allows you to run
many containers simultaneously on your host. The lightweight nature of
containers, which run without the extra overload of a hypervisor, means
you can get more out of your hardware.

Surrounding the container virtualization, we provide tooling and a
platform to help you get your applications (and its supporting
components) into Docker containers, to distribute and ship those
containers to your teams to develop and test on them and then to deploy
those applications to your production environment whether it be in a
local data center or the Cloud.

## What can I use Docker for?

* Faster delivery of your applications

Docker is perfect for helping you with the development lifecycle. Docker
can allow your developers to develop on local containers that contain
your applications and services. It can integrate into a continuous
integration and deployment workflow.

Your developers write code locally and share their development stack via
Docker with their colleagues. When they are ready they can push their
code and the stack they are developing on to a test environment and
execute any required tests. From the testing environment you can then
push your Docker images into production and deploy your code.

* Deploy and scale more easily

Docker's container platform allows you to have highly portable
workloads. Docker containers can run on a developer's local host, on
physical or virtual machines in a data center or in the Cloud.

Docker's portability and lightweight nature also makes managing
workloads dynamically easy. You can use Docker to build and scale out
applications and services. Docker's speed means that scaling can be near
real time.

* Get higher density and run more workloads

Docker is lightweight and fast. It provides a viable (and
cost-effective!) alternative to hypervisor-based virtual machines. This
is especially useful in high density environments, for example building
your own Cloud or Platform-as-a-Service. But it is also useful
for small and medium deployments where you want to get more out of the
resources you have.

## What are the major Docker components?

Docker has two major components:

* Docker: the open source container virtualization platform.
* [Docker.io](https://index.docker.io): our Software-as-a-Service
  platform for sharing and managing Docker containers.

**Note:** Docker is licensed with the open source Apache 2.0 license.

## What is the architecture of Docker?

Docker has a client-server architecture. The Docker *client* talks to
the Docker *daemon* which does the heavy lifting of building, running
and distributing your Docker containers. Both the Docker client and the
daemon *can* run on the same system, or you can connect a Docker client
with a remote Docker daemon. The Docker client and service can
communicate via sockets or through a RESTful API.

![Docker Architecture Diagram](/article-img/architecture.svg)

### The Docker daemon

As shown on the diagram above, the Docker daemon runs on a host machine.
The user does not directly interact with the daemon, but instead through
the Docker client.

### The Docker client

The Docker client, in the form of the `docker` binary, is the primary user
interface to Docker. It is tasked with accepting commands from the user
and communicating back and forth with a Docker daemon.

### Inside Docker

Inside Docker there are three concepts we’ll need to understand:

* Docker images.
* Docker registries.
* Docker containers.

#### Docker images

The Docker image is a read-only template, for example an Ubuntu operating system
with Apache and your web application installed. Docker containers are
created from images. You can download Docker images that other people
have created or Docker provides a simple way to build new images or
update existing images. You can consider Docker images to be the **build**
portion of Docker.

#### Docker Registries

Docker registries hold images. These are public (or private!) stores
that you can upload or download images to and from. The public Docker
registry is called [Docker.io](http://index.docker.io). It provides a
huge collection of existing images that you can use. These images can be
images you create yourself or you can make use of images that others
have previously created. You can consider Docker registries the
**distribution** portion of Docker.

#### Docker containers

Docker containers are like a directory. A Docker container holds
everything that is needed for an application to run. Each container is
created from a Docker image. Docker containers can be run, started,
stopped, moved and deleted. Each container is an isolated and secure
application platform. You can consider Docker containers the **run**
portion of Docker.

## So how does Docker work?

We've learned so far that:

1. You can build Docker images that hold your applications.
2. You can create Docker containers from those Docker images to run your
   applications.
3. You can share those Docker images via
   [Docker.io](https://index.docker.io) or your own registry.

Let's look at how these elements combine together to make Docker work.

### How does a Docker Image work?

We've already seen that Docker images are read-only templates that
Docker containers are launched from. Each image consists of a series of
layers. Docker makes use of [union file
systems](http://en.wikipedia.org/wiki/UnionFS) to combine these layers
into a single image. Union file systems allow files and directories of
separate file systems, known as branches, to be transparently overlaid,
forming a single coherent file system.

One of the reasons Docker is so lightweight is because of these layers.
When you change a Docker image, for example update an application to a
new version, this builds a new layer. Hence, rather than replacing the whole
image or entirely rebuilding, as you may do with a virtual machine, only
that layer is added or updated. Now you don't need to distribute a whole new image,
just the update, making distributing Docker images fast and simple.

Every image starts from a base image, for example `ubuntu`, a base Ubuntu
image, or `fedora`, a base Fedora image. You can also use images of your
own as the basis for a new image, for example if you have a base Apache
image you could use this as the base of all your web application images.

> **Note:**  
> Docker usually gets these base images from [Docker.io](https://index.docker.io).

Docker images are then built from these base images using a simple
descriptive set of steps we call *instructions*. Each instruction
creates a new layer in our image. Instructions include steps like:

* Run a command.
* Add a file or directory.
* Create an environment variable.
* What process to run when launching a container from this image.

These instructions are stored in a file called a `Dockerfile`. Docker
reads this `Dockerfile` when you request an image be built, executes the
instructions and returns a final image.

### How does a Docker registry work?

The Docker registry is the store for your Docker images. Once you build
a Docker image you can *push* it to a public registry [Docker.io](
https://index.docker.io) or to your own registry running behind your
firewall.

Using the Docker client, you can search for already published images and
then pull them down to your Docker host to build containers from them.

[Docker.io](https://index.docker.io) provides both public and
private storage for images. Public storage is searchable and can be
downloaded by anyone. Private storage is excluded from search
results and only you and your users can pull them down and use them to
build containers. You can [sign up for a plan
here](https://index.docker.io/plans).

### How does a container work?

A container consists of an operating system, user added files and
meta-data. As we've discovered each container is built from an image. That image tells
Docker what the container holds, what process to run when the container
is launched and a variety of other configuration data. The Docker image
is read-only. When Docker runs a container from an image it adds a
read-write layer on top of the image (using a union file system as we
saw earlier) in which your application is then run.

### What happens when you run a container?

The Docker client using the `docker` binary, or via the API, tells the
Docker daemon to run a container. Let's take a look at what happens
next.

    $ docker run -i -t ubuntu /bin/bash

Let's break down this command. The Docker client is launched using the
`docker` binary with the `run` option telling it to launch a new
container. The bare minimum the Docker client needs to tell the
Docker daemon to run the container is:

* What Docker image to build the container from, here `ubuntu`, a base
  Ubuntu image;
* The command you want to run inside the container when it is launched,
  here `bin/bash` to shell the Bash shell inside the new container.

So what happens under the covers when we run this command?

Docker begins with:

- **Pulling the `ubuntu` image:**  
  Docker checks for the presence of the `ubuntu` image and if it doesn't
  exist locally on the host, then Docker downloads it from
  [Docker.io](https://index.docker.io). If the image already exists then
  Docker uses it for the new container.
- **Creates a new container:**  
  Once Docker has the image it creates a container from it:
    * **Allocates a filesystem and mounts a read-write _layer_:**  
      The container is created in the file system and a read-write layer is
      added to the image.
    * **Allocates a network / bridge interface:**  
      Creates a network interface that allows the Docker container to talk to
      the local host.
    * **Sets up an IP address:**  
      Finds and attaches an available IP address from a pool.
- **Executes a process that you specify:**  
  Runs your application, and;
- **Captures and provides application output:**  
  Connects and logs standard input, outputs and errors for you to see how
  your application is running.

Now you have a running container! From here you can manage your running
container, interact with your application and then when finished stop
and remove your container.

## The underlying technology

Docker is written in Go and makes use of several Linux kernel features to
deliver the features we've seen.

### Namespaces

Docker takes advantage of a technology called `namespaces` to provide an
isolated workspace we call a *container*.  When you run a container,
Docker creates a set of *namespaces* for that container.

This provides a layer of isolation: each aspect of a container runs in
its own namespace and does not have access outside it.

Some of the namespaces that Docker uses are:

 - **The `pid` namespace:**
 Used for process isolation (PID: Process ID).
 - **The `net` namespace:**
 Used for managing network interfaces (NET: Networking).
 - **The `ipc` namespace:**
 Used for managing access to IPC resources (IPC: InterProcess
Communication).
 - **The `mnt` namespace:**
 Used for managing mount-points (MNT: Mount).
 - **The `uts` namespace:**
 Used for isolating kernel and version identifiers. (UTS: Unix Timesharing
System).

### Control groups

Docker also makes use of another technology called `cgroups` or control
groups. A key need to run applications in isolation is to have them only
use the resources you want. This ensures containers are good
multi-tenant citizens on a host. Control groups allow Docker to
share available hardware resources to containers and if required, set up to
limits and constraints, for example limiting the memory available to a
specific container.

### Union file systems

Union file systems or UnionFS are file systems that operate by creating
layers, making them very lightweight and fast. Docker uses union file
systems to provide the building blocks for containers. We learned about
union file systems earlier in this document. Docker can make use of
several union file system variants including: AUFS, btrfs, vfs, and
DeviceMapper.

### Container format

Docker combines these components into a wrapper we call a container
format. The default container format is called `libcontainer`. Docker
also supports traditional Linux containers using
[LXC](https://linuxcontainers.org/). In future Docker may support other
container formats, for example integration with BSD Jails or Solaris
Zones.

## Next steps

### Installing Docker

Visit the [installation](/installation/#installation) section.

### The Docker User Guide

[Learn how to use Docker](/userguide/).


