page_title: About Docker
page_description: Introduction to Docker.
page_keywords: docker, introduction, documentation, about, technology, understanding, Dockerfile

# About Docker

**Develop, Ship and Run Any Application, Anywhere**

[**Docker**](https://www.docker.com) is a platform for developers and sysadmins
to develop, ship, and run applications.  Docker lets you quickly assemble
applications from components and eliminates the friction that can come when
shipping code. Docker lets you get your code tested and deployed into production
as fast as possible.

Docker consists of:

* The Docker Engine - our lightweight and powerful open source container
  virtualization technology combined with a work flow for building
  and containerizing your applications.
* [Docker Hub](https://hub.docker.com) - our SaaS service for
  sharing and managing your application stacks.

## Why Docker?

*Faster delivery of your applications*

* We want your environment to work better. Docker containers,
      and the work flow that comes with them, help your developers,
      sysadmins, QA folks, and release engineers work together to get your code
      into production and make it useful. We've created a standard
      container format that lets developers care about their applications
      inside containers while sysadmins and operators can work on running the
      container in your deployment. This separation of duties streamlines and
      simplifies the management and deployment of code.
* We make it easy to build new containers, enable rapid iteration of
      your applications, and increase the visibility of changes. This
      helps everyone in your organization understand how an application works
      and how it is built.
* Docker containers are lightweight and fast! Containers have
      sub-second launch times, reducing the cycle
      time of development, testing, and deployment.

*Deploy and scale more easily*

* Docker containers run (almost) everywhere. You can deploy
      containers on desktops, physical servers, virtual machines, into
      data centers, and up to public and private clouds.
* Since Docker runs on so many platforms, it's easy to move your
      applications around. You can easily move an application from a
      testing environment into the cloud and back whenever you need.
* Docker's lightweight containers also make scaling up and
      down fast and easy. You can quickly launch more containers when
      needed and then shut them down easily when they're no longer needed.

*Get higher density and run more workloads*

* Docker containers don't need a hypervisor, so you can pack more of
      them onto your hosts. This means you get more value out of every
      server and can potentially reduce what you spend on equipment and
      licenses.

*Faster deployment makes for easier management*

* As Docker speeds up your work flow, it gets easier to make lots
      of small changes instead of huge, big bang updates. Smaller
      changes mean reduced risk and more uptime.

## About this guide

The [Understanding Docker section](introduction/understanding-docker.md) will help you:

 - See how Docker works at a high level
 - Understand the architecture of Docker
 - Discover Docker's features;
 - See how Docker compares to virtual machines
 - See some common use cases.

### Installation Guides

The [installation section](/installation/#installation) will show you how to install
Docker on a variety of platforms.


### Docker User Guide

To learn about Docker in more detail and to answer questions about usage and
implementation, check out the [Docker User Guide](/userguide/).

## Release Notes

**Version 1.2.0**

This version fixes a number of bugs and issues and adds new functions and other
improvements. These include:

*New restart policies*

We added a `--restart flag` to `docker run` to specify a restart policy for your
container. Currently, there are three policies available:

* `no` – Do not restart the container if it dies. (default)
* `on-failure` – Restart the container if it exits with a non-zero exit code.
This can also accept an optional maximum restart count (e.g. `on-failure:5`).
* `always` – Always restart the container no matter what exit code is returned.
This deprecates the `--restart` flag on the Docker daemon.

*New flags for `docker run`: `--cap-add` and `–-cap-drop`*

In previous releases, Docker containers could either be given complete capabilities or
they could all follow a whitelist of allowed capabilities while dropping all others.
Further, using `--privileged` would grant all capabilities inside a container, rather than
applying a whitelist. This was not recommended for production use because it’s really
unsafe; it’s as if you were directly in the host.

This release introduces two new flags for `docker run`, `--cap-add` and `--cap-drop`, that
give you fine-grain control over the specific capabilities you want grant to a particular
container.

*New `-–device` flag for `docker run`*

Previously, you could only use devices inside your containers by bind mounting them (with
`-v`) in a `--privileged` container. With this release, we introduce the `--device flag`
to `docker run` which lets you use a device without requiring a privileged container.

*Writable `/etc/hosts`, `/etc/hostname` and `/etc/resolv.conf`*

You can now edit `/etc/hosts`, `/etc/hostname` and `/etc/resolve.conf` in a running
container. This is useful if you need to install BIND or other services that might
override one of those files.

Note, however, that changes to these files are not saved when running `docker build` and
so will not be preserved in the resulting image. The changes will only “stick” in a
running container.

*Docker proxy in a separate process*

The Docker userland proxy that routes outbound traffic to your containers now has its own
separate process (one process per connection). This greatly reduces the load on the
daemon, which increases stability and efficiency.

*Other improvements & changes*

* When using `docker rm -f`, Docker now kills the container (instead of stopping it)
before removing it . If you intend to stop the container cleanly, you can use `docker
stop`.

* Added support for IPv6 addresses in `--dns`

* Added search capability in private registries

