page_title: Understanding the Technology
page_description: Technology of Docker explained in depth
page_keywords: docker, introduction, documentation, about, technology, understanding, dockerfile

# Understanding the Technology

*What are Docker's forming parts? What's the underlying technology?*

## Introduction

When it comes to understanding Docker and the underlying technology, it 
will be a relief to see that there is no *magic* involved. Everything 
is based on research and many years of consistent development on the 
*Linux Kernel*. All the tools and parts either make use of them directly, 
or, build upon them to provide the functionality *intelligently* (e.g. IP
address allocation of containers).

Aside from the technology, one of the major factors that make Docker great
is the way it's shaped. The project comes with less than a handful of 
easy-to-work-with and free tools that handle and provide all of the features
we mentioned on [Page 1](understanding-docker.md). The forming architecture
is designed in a way to support various needs and to allow distributed and
remote set ups.

## Architecture of Docker

Planning and designing *anything* to create *something* complete is a 
challenge which developers and system operators experience in first place, 
everyday.

Docker, being a product with always the end user in mind, has been 
shaped to sustain this process, from development to production.

For this purpose, a distributed set of tools have been created for 
Docker to work.

Let's take a look.

    Architecture of Docker:
    
    - Both the client and the daemon *can* run on the same system;
    - Although the client *can* exist and work *remotely*:
    e.g. your computer, with the daemon on your VPS.
    - They communicate over a chosen protocol:
    e.g. sockets.
    - User interacts with the client to command the daemon;
    e.g. run, stop, create, save, load etc.
    - The daemon, receiving those commands, does the job.
    e.g. pull an image, form the container and run.
    - The daemon has access to the Docker and private image indexes.
    e.g. you can push committed images or pull new ones.
    
                                              _________________
                                              |     Host(s)     |
                  The Client Sends Commands   |_________________|
                  -------------------------   |                 |
      [docker] <= pull, push, run, load .. => | [docker daemon] |
       client                                 |                 |
                                              | - container 1   |
                                              | - container 2   |
                                              | - ..            |
                                              |_______~~________|
                                                      ||
                                            [The Docker Image Index]
                                                    
    P.S. Do not be put off with this scary looking representation.
         It's just our ASCII drawing skills. ;-)
         Tools are uncomplicated and actually, pretty straightforward.
                                                    
## Forming parts of Docker

Forming parts of Docker consist of the main toolset composed of two 
applications that have an unlimited access to the public image index
(i.e., the [Docker Index](https://index.docker.io)).

**The Docker toolset:**

 - Ddocker *daemon* application;
 - Ddocker *client* application, and;
 - The central registry: Docker Index.

### Docker daemon

As shown on the diagram above, the Docker daemon runs on the host 
machine(s). The word "host" here refers to any computer where containers 
are operational. The user does not directly interact with the daemon, but
instead through an intermediary: the Docker client.

### Docker client

The Docker client is an application which can run either on the same 
computer as the Docker daemon, or elsewhere. It is tasked with accepting 
commands from the user (i.e. *you*) and communicating back and forth with 
a Docker daemon to manage container lifecycle on any host.

### Docker Index, the central Docker registry

The Docker image index is the global archive (and directory) of user supplied
Docker container images. It currently hosts a very large – in fact, rapidly
growing – number of projects where you can find almost any popular application
or deployment stack readily available to download and run with a single command
(e.g. `docker pull` `[repository]/[application]`).

As a social community project, Docker tries to provide all necessary tools for
everyone to grow with other *Dockers*. By issuing a single command through the
Docker client (i.e. `docker push [name]`), you can start sharing your own
creations with the rest of the world.

However, knowing that not everything can be shared (e.g. proprietary code), 
Docker Index also offers the possibility to host private repositories. In order
to see the available plans, you can click [here](https://index.docker.io/plans).

Using the [Docker Registry](https://github.com/dotcloud/docker-registry), it is
also possible to run your own private Docker Image registry service on your own
servers. 

> **Note:** To learn more about the [*Docker Image Index*](
> http://index.docker.io) (public *and* private), check out the [Registry & 
> Index Spec](http://docs.docker.io/en/latest/api/registry_index_spec/).

### Summary

 - **When you install Docker, you get all the forming parts:**  
 i.e. the daemon, the client and access to the public image index.
 - **Alternatively, you can spread them across a collection of machines:**  
 e.g. Servers with the Docker daemon running, controlled by the Docker client.
 - **You can benefit form the public registry:**  
 e.g. `docker pull [user or repository]/[image name][:tag]`
 - **You can start a private one for proprietary use.**
 i.e. Sign up for a [plan](https://index.docker.io/plans) or host your own
[Docker registry](https://github.com/dotcloud/docker-registry).

## Elements of Docker

Elements of Docker can be considered anything that the above mentioned tools
(or parts) use. This includes the following:

 - **Containers, which allow:**  
 Security, portability and resources management through LXC.
 - **Images, which provide:**  
 The base for applications inside the containers to run, and;
 - **The Dockerfile, which automates:**  
 Container and container image build-process.
 
To get practical and learn what they are, and **_how to work_** with 
them, continue to [Page 3](working-with-docker.md). If you would like to 
understand **_how they work_**, stay here and continue reading.

## The underlying technology

The power of Docker comes from the underlying technology. Albeit light, 
the features offered by the operating-system are resourcefully glued by 
Docker to extract all the complexities for the user. When you take a 
deep look, it is easy to see how the dots connect. In this section, we 
will see the main Linux kernel features (e.g. [LXC](http://linuxcontainers.org/),
union file system etc.) that Docker uses to make easy containerisation happen.

### Namespaces

When you run a container, Docker creates a set of *namespaces* for container's
use and for it to function.

This allows the first layer of isolation: a process (i.e. an application)
does not have the outside namespace access, hence rendering it are isolated.

Furthermore, containers do not get to have privileged access to the host
networking interfaces (i.e. ports, sockets). Docker intelligently 
discovers which IP block is available and issues an address for the
container to use, as you command. You can also link containers through
[*links*](http://docs.docker.io/en/latest/use/working_with_links_name)
which permits different containers (e.g. Nginx and Unicorn) to communicate
between each other through regular and familiar forms of networking.

Some namespaces are:

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

### Control groups (i.e, cgroups)

A key need to run applications in isolation is to have them contained,
not just in terms of related filesystem and/or dependencies, but also,
resources. 

Control groups allow the functionality to fairly share available hardware
resources to containers and if asked, set up to limits and constraints
(e.g. limiting the memory to a maximum of 128 MBs).

### UnionFS

UnionFS is the term coined to explain a certain filesystem service on
Unix systems whereby overlaying separate files and directories on different
file systems becomes possible. This allows the way Docker images work 
(see the relevant section below for more details).

### Linux Containers (LXC)

Linux Containers and its underlying features are a combination of 
technologies that have been under development for a good 5 years (circa.
2008). It offers an interface (a method to communicate) to work 
efficiently - and collectively - with the Linux kernel features developed
for application containment.

## How does everything work

A lot happens when you `docker run` a container.

Let's discover!

### How does a container work?

A container consists of operating-system images, user added files and
meta-data that hold some additional information such as the process
(i.e. application) to start and the arguments to pass when you command
it to `run`. Since images are always read-only, a read-write layer gets
added on top through the union file system as well.

This, together, provides a location for all sorts of dependencies to reside.
They are operated by executing the process, providing it the container itself
as the base of its system and by controlling it through Linux kernel features
for the purposes of isolation, security and more.

### What happens when you run a container?

The Docker daemon accepts varying instructions to run containers.
Depending on the commands passed, it can skip certain steps. To have
an overall idea, let's take a look at a simple `Hello world` example.

Imagine running the following command: `docker run -i -t ubuntu /bin/bash`

Docker begins with:

 - **Pulling the `ubuntu:latest` image:**  
 i.e. downloading it from the [Docker Index](https://index.docker.io)
 - **Creates a new container:**  
 i.e. starts container creation procedure
 - **Allocates a filesystem and mounts a read-write _layer_:** 
 i.e. makes use of the filesystem drivers and desired technology
 - **Allocates a network / bridge interface:**  
 i.e. `docker0`
 - **Sets up an IP address:**  
 i.e. intelligently find and attach an available IP address
 - **Executes _a_ process that you specify:**  
 i.e. runs your application, and;
 - **Captures and provides application output:**  
 i.e. for you to see the application feedback.

> **Tip:** If you do not specify a tag, Docker fetches the image tagged
> ":latest" from the given repository, i.e., `repo_name/image_name` defaults
> to `repo_name/image_name:latest`. Therefore, `ubuntu` is actually
> `ubuntu:latest`. The Ubuntu repository contains several others, including
> `12.04`, `13.10`, `lucid`, `precise` and more.

### How does a Docker Image work?

When you start building your container (e.g. install an application on
`ubuntu:12.04` and commit the *difference*), technically, this does not
change the parent Ubuntu image but adds a new logical *layer* that forms
yet a new one.

The term layer is used actively when referring to Docker, because all
commands executed on an image during the build process forms a new layer on top
of the parent, without modifying or changing what's underneath (e.g. the
*base image*). 

The lowest-level, read-only images (i.e. Ubuntu) are referred to as
*base-images*. They are supplied by Docker and can be trusted.

Therefore, technically, an image is a read-only layer that you can build upon. 

Through filesystems, drivers and by analysing which technology is available,
Docker creates, manages and works with images depending on the system.

### How do the client and the daemon work?

The two main parts of Docker are the *daemon* and the *client*.

The daemon sits on the host computer where you want to have containers
to run and reside.

It receives the commands and directives from the Docker client on a
set interface and performs the actions from creating a container to
shipping the underlying image to the central public index.

The `client`, on the other hand, can work anywhere as long as it is able
to communicate with the Docker daemon. You can use the client like
any other shell application and it will pass the commands on to the *daemon*
for execution.

This translates to *daemon* and *client* either working together on the
same system or remotely over distance (i.e., through a network).

### How does the image registry index work?

The Docker Index works by users submitting their committed images. A committed
image is a read-only snapshot taken from a container after the build process
that can be used as a base to run new containers or to build new images for new
containers.

Using the Docker client, you can search-and-find (`docker search [name]` and 
`docker pull [name]`) some already published images to start running containers
or push one of your own images to share publically.

If you have a private repository which you'd like to share amongst those you
designate, you can also use the central registry index, the Docker Index by
[signing up for a plan](https://index.docker.io/plans).

To learn more, check out the [Working With Repositories](
http://docs.docker.io/en/latest/use/workingwithrepository) section of our
[User's Manual](http://docs.docker.io).

## Where to go from here

### Understand Docker

Visit [Page 1](understanding-docker.md) of our Getting Started manual.

### Get practical and learn how to use Docker straight away

Visit [Page 3](working-with-docker.md) of our Getting Started manual.

### Get the product and go hands-on

Visit [Page 4](get-docker.md) of our Getting Started manual.

### Get the whole story

[https://www.docker.io/the_whole_story/](https://www.docker.io/the_whole_story/)