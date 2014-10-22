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

The [installation section](/installation/#installation) will show you how to
install Docker on a variety of platforms.


### Docker User Guide

To learn about Docker in more detail and to answer questions about usage and
implementation, check out the [Docker User Guide](/userguide/).

## Release Notes

**Version 1.3.0**

This version fixes a number of bugs and issues and adds new functions and other
improvements. The [GitHub 1.3 milestone](https://github.com/docker/docker/issues?q=milestone%3A1.3.0+) has
more detailed information. Major additions and changes include:

*New command: `docker exec`*

The new `docker exec` command lets you run a process in an existing, active
container. The command has APIs for both the daemon and the client. With
`docker exec`, you'll be able to do things like add or remove devices from
running containers, debug running containers, and run commands that are not
part of the container's static specification. Details in the [command line
reference](/reference/commandline/cli/#exec).

*New command: `docker create`*

Traditionally, the `docker run` command has been used to both create a
container and spawn a process to run it. The new `docker create` command breaks
this apart, letting you set up a container without actually starting it. This
provides more control over management of the container lifecycle, giving you the
ability to configure things like volumes or port mappings before the container
is started. For example, in a rapid-response scaling situation, you could use
`create` to prepare and stage ten containers in anticipation of heavy loads.
Details in the [command line reference](/reference/commandline/cli/#create).

*Tech preview of new provenance features*

This release offers a sneak peek at new image signing capabilities that are
currently under development. Soon, these capabilities will allow any image
author to sign their images to certify they have not been tampered with. For
this release, Official images are now signed by Docker, Inc. Not only does this
demonstrate the new functionality, we hope it will improve your confidence in
the security of Official images. Look for the blue ribbons denoting signed
images on the [Docker Hub](https://hub.docker.com/).
The Docker Engine has been updated to automatically verify that a given
Official Repo has a current, valid signature. When pulling a signed image,
you'll see a message stating `the image you are pulling has been verified`. If
no valid signature is detected, Docker Engine will fall back to pulling a
regular, unsigned image.

*Other improvements & changes*

* We've added a new security options flag to the `docker run` command,
`--security-opt`, that lets you set SELinux and AppArmor labels and profiles.
This means you'll  no longer have to use `docker run --privileged` on kernels
that support SE Linux or AppArmor. For more information, see the
[command line reference](/reference/commandline/cli/#run).

* A new flag, `--add-host`, has been added to `docker run` that lets you add
lines to `/etc/hosts`. This allows you to specify different name
resolution for the container than it would get via DNS. For more information,
see the [command line reference](/reference/commandline/cli/#run).

* You can now set a `DOCKER_TLS_VERIFY` environment variable to secure
connections by default (rather than having to pass the `--tlsverify` flag on
every call). For more information, see the [https guide](/articles/https).

* Three security issues have been addressed in this release: [CVE-2014-5280,
CVE-2014-5270, and CVE-2014-5282](https://groups.google.com/forum/#!msg/docker-announce/aQoVmQlcE0A/smPuBNYf8VwJ).
