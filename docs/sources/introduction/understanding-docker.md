page_title: Understanding Docker
page_description: Docker explained in depth
page_keywords: docker, introduction, documentation, about, technology, understanding, Dockerfile

# Understanding Docker

*What is Docker? What makes it great?*

Building development lifecycles, pipelines and deployment tooling is
hard. It's not easy to create portable applications and services.
There's often high friction getting code from your development
environment to production. It's also hard to ensure those applications
and services are consistent, up-to-date and managed.

Docker is designed to solve these problem for both developers and
sysadmins. It is a lightweight framework (with a powerful API) that
provides a lifecycle for building and deploying applications into
containers.

Docker provides a way to run almost any application securely isolated
into a container. The isolation and security allows you to run many
containers simultaneously on your host. The lightweight nature of
containers, which run without the extra overload of a hypervisor, means
you can get more out of your hardware.

**Note:** Docker itself is *shipped* with the Apache 2.0 license and it
is completely open-source — *the pun? very much intended*.

### What are the Docker basics I need to know?

Docker has three major components:

* Docker containers.
* Docker images.
* Docker registries.

#### Docker containers

Docker containers are like a directory. A Docker container holds
everything that is needed for an application to run. Each container is
created from a Docker image. Docker containers can be run, started,
stopped, moved and deleted. Each container is an isolated and secure
application platform. You can consider Docker containers the *run*
portion of the Docker framework.

#### Docker images

The Docker image is a template, for example an Ubuntu
operating system with Apache and your web application installed. Docker
containers are launched from images. Docker provides a simple way to
build new images or update existing images. You can consider Docker
images to be the *build* portion of the Docker framework.

#### Docker Registries

Docker registries hold images. These are public (or private!) stores
that you can upload or download images to and from. These images can be
images you create yourself or you can make use of images that others
have previously created. Docker registries allow you to build simple and
powerful development and deployment work flows. You can consider Docker
registries the *share* portion of the Docker framework.

### How does Docker work?

Docker is a client-server framework. The Docker *client* commands the Docker
*daemon*, which in turn creates, builds and manages containers.

The Docker daemon takes advantage of some neat Linux kernel and
operating system features, like `namespaces` and `cgroups`, to build
isolated container. Docker provides a simple abstraction layer to these
technologies.

> **Note:** If you would like to learn more about the underlying technology,
> why not jump to [Understanding the Technology](technology.md) where we talk about them? You can
> always come back here to continue learning about features of Docker and what
> makes it different.

## Features of Docker

In order to get a good grasp of the capabilities of Docker you should
read the [User's Manual](http://docs.docker.io). Let's look at a summary
of Docker's features to give you an idea of how Docker might be useful
to you.

### User centric and simple to use

*Docker is made for humans.*

It's easy to get started and easy to build and deploy applications with
Docker: or as we say "*dockerise*" them! As much of Docker as possible
uses plain English for commands and tries to be as lightweight and
transparent as possible. We want to get out of the way so you can build
and deploy your applications.

### Docker is Portable

*Dockerise And Go!*

Docker containers are highly portable. Docker provides a standard
container format to hold your applications:

* You take care of your applications inside the container, and;
* Docker takes care of managing the container.

Any machine, be it bare-metal or virtualized, can run any Docker
container. The sole requirement is to have Docker installed.

**This translates to:**

 - Reliability;
 - Freeing your applications out of the dependency-hell;
 - A natural guarantee that things will work, anywhere.

### Lightweight

*No more resources waste.*

Containers are lightweight, in fact, they are extremely lightweight.
Unlike traditional virtual machines, which have the overhead of a
hypervisor, Docker relies on operating system level features to provide
isolation and security. A Docker container does not need anything more
than what your application needs to run.

This translates to:

 - Ability to deploy a large number of applications on a single system;
 - Lightning fast start up times and reduced overhead.

### Docker can run anything

*An amazing host! (again, pun intended.)*

Docker isn't prescriptive about what applications or services you can run
inside containers. We provide use cases and examples for running web
services, databases, applications - just about anything you can imagine
can run in a Docker container.

**This translates to:**

 - Ability to run a wide range of applications;
 - Ability to deploy reliably without repeating yourself.

### Plays well with others

*A wonderful guest.*

Today, it is possible to install and use Docker almost anywhere. Even on
non-Linux systems such as Windows or Mac OS X thanks to a project called
[Boot2Docker](http://boot2docker.io).

**This translates to running Docker (and Docker containers!) _anywhere_:**

 - **Linux:**  
 Ubuntu, CentOS / RHEL, Fedora, Gentoo, openSUSE and more.
 - **Infrastructure-as-a-Service:**  
 Amazon AWS, Google GCE, Rackspace Cloud and probably, your favorite IaaS.
 - **Microsoft Windows**  
 - **OS X**  

### Docker is Responsible

*A tool that you can trust.*

Docker does not just bring you a set of tools to isolate and run
applications. It also allows you to specify constraints and controls on
those resources.

**This translates to:**

 - Fine tuning available resources for each application;
 - Allocating memory or CPU intelligently to make most of your environment;

Without dealing with complicated commands or third party applications.

### Docker is Social

*Docker knows that No One Is an Island.*

Docker allows you to share the images you've built with the world. And
lots of people have already shared their own images.

To facilitate this sharing Docker comes with a public registry and index
called the [Docker Index](http://index.docker.io). If you don't want
your images to be public you can also use private images on the Index or
even run your own registry behind your firewall.

**This translates to:**

 - No more wasting time building everything from scratch;
 - Easily and quickly save your application stack;
 - Share and benefit from the depth of the Docker community.

## Docker versus Virtual Machines

> I suppose it is tempting, if the *only* tool you have is a hammer, to
> treat *everything* as if it were a nail.
> — **_Abraham Maslow_**

**Docker containers are:**

 - Easy on the resources;
 - Extremely light to deal with;
 - Do not come with substantial overhead;
 - Very easy to work with;
 - Agnostic;
 - Can work *on* virtual machines;
 - Secure and isolated;
 - *Artful*, *social*, *fun*, and;
 - Powerful sand-boxes.

**Docker containers are not:**

 - Hardware or OS emulators;
 - Resource heavy;
 - Platform, software or language dependent.

## Docker Use Cases

Docker is a framework. As a result it's flexible and powerful enough to
be used in a lot of different use cases.

### For developers

 - **Developed with developers in mind:**  
 Build, test and ship applications with nothing but Docker and lean
 containers.
 - **Re-usable building blocks to create more:**  
 Docker images are easily updated building blocks.
 - **Automatically build-able:**  
 It has never been this easy to build - *anything*.
 - **Easy to integrate:**  
 A powerful, fully featured API allows you to integrate Docker into your tooling.

### For sysadmins

 - **Efficient (and DevOps friendly!) lifecycle:**  
 Operations and developments are consistent, repeatable and reliable.
 - **Balanced environments:**  
 Processes between development, testing and production are leveled.
 - **Improvements on speed and integration:**  
 Containers are almost nothing more than isolated, secure processes.
 - **Lowered costs of infrastructure:**  
 Containers are lightweight and heavy on resources compared to virtual machines.
 - **Portable configurations:**  
 Issues and overheads with dealing with configurations and systems are eliminated.

### For everyone

 - **Increased security without performance loss:**  
 Replacing VMs with containers provide security without additional
 hardware (or software).
 - **Portable:**  
 You can easily move applications and workloads from different operating
 systems and platforms.

## Where to go from here

### Learn about Parts of Docker and the underlying technology

Visit [Understanding the Technology](technology.md) in our Getting Started manual.

### Get practical and learn how to use Docker straight away

Visit [Working with Docker](working-with-docker.md) in our Getting Started manual.

### Get the product and go hands-on

Visit [Get Docker](get-docker.md) in our Getting Started manual.

### Get the whole story

[https://www.docker.io/the_whole_story/](https://www.docker.io/the_whole_story/)
