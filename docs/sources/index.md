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

To learn about Docker in more detail and to answer questions about usage and implementation, check out the [Docker User Guide](/userguide/).

## Release Notes

<b>Version 1.1.0</b>

### New Features

*`.dockerignore` support*

You can now add a `.dockerignore` file next to your `Dockerfile` and Docker will ignore files and directories specified in that file when sending the build context to the daemon. 
Example: https://github.com/docker/docker/blob/master/.dockerignore

*Pause containers during commit*

Doing a commit on a running container was not recommended because you could end up with files in an inconsistent state (for example, if they were being written during the commit). Containers are now paused when a commit is made to them.
You can disable this feature by doing a `docker commit --pause=false <container_id>`

*Tailing logs*

You can now tail the logs of a container. For example, you can get the last ten lines of a log by using `docker logs --tail 10 <container_id>`. You can also follow the logs of a container without having to read the whole log file with `docker logs --tail 0 -f <container_id>`.

*Allow a tar file as context for docker build*

You can now pass a tar archive to `docker build` as context. This can be used to automate docker builds, for example: `cat context.tar | docker build -` or `docker run builder_image | docker build -`

*Bind mounting your whole filesystem in a container*

`/` is now allowed as source of `--volumes`. This means you can bind-mount your whole system in a container if you need to. For example: `docker run -v /:/my_host ubuntu:ro ls /my_host`. However, it is now forbidden to mount to /.


### Other Improvements & Changes

* Port allocation has been improved. In the previous release, Docker could prevent you from starting a container with previously allocated ports which seemed to be in use when in fact they were not. This has been fixed.

* A bug in `docker save` was introduced in the last release. The `docker save` command could produce images with invalid metadata. The command now produces images with correct metadata.

* Running `docker inspect` in a container now returns which containers it is linked to.

* Parsing of the `docker commit` flag has improved validation, to better prevent you from committing an image with a name such as  `-m`. Image names with dashes in them potentially conflict with command line flags.

* The API now has Improved status codes for  `start` and `stop`. Trying to start a running container will now return a 304 error.

* Performance has been improved overall. Starting the daemon is faster than in previous releases. The daemon’s performance has also been improved when it is working with large numbers of images and containers.

* Fixed an issue with white-spaces and multi-lines in Dockerfiles. 


