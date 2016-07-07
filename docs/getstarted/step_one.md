<!--[metadata]>
+++
aliases = [
"/mac/step_one/",
"/windows/step_one/",
"/linux/step_one/",
]
title = "Install Docker and run hello-world"
description = "Getting started with Docker"
keywords = ["beginner, getting started, Docker, install"]
[menu.main]
identifier = "getstart_all_install"
parent = "tutorial_getstart_menu"
weight = 1
+++
<![end-metadata]-->

# Install Docker

- [Step 1: Get Docker](#step-1-get-docker)
- [Step 2: Install Docker](#step-2-install-docker)
- [Step 3: Verify your installation](#step-3-verify-your-installation)

## Step 1: Get Docker

### Docker for Mac

Docker for Mac is our newest offering for the Mac. It runs as a native Mac application and uses <a href="https://github.com/mist64/xhyve/" target="_blank">xhyve</a> to virtualize the Docker Engine environment and Linux kernel-specific features for the Docker daemon.

<a class="button" href="https://download.docker.com/mac/beta/Docker.dmg">Get Docker for Mac</a>

**Requirements**

- Mac must be a 2010 or newer model, with Intel's hardware support for memory management unit (MMU) virtualization; i.e., Extended Page Tables (EPT)

- OS X 10.10.3 Yosemite or newer

- At least 4GB of RAM

- VirtualBox prior to version 4.3.30 must NOT be installed (it is incompatible with Docker for Mac). Docker for Mac will error out on install in this case. Uninstall the older version of VirtualBox and re-try the install.

#### Docker Toolbox for the Mac

If you have an earlier Mac that doesn't meet the Docker for Mac prerequisites, <a href="https://www.docker.com/products/docker-toolbox" target="_blank">get Docker Toolbox</a> for the Mac.

See [Docker Toolbox Overview](/toolbox/overview.md) for help on installing Docker with Toolbox.

### Docker for Windows

Docker for Windows is our newest offering for PCs. It runs as a native Windows application and uses Hyper-V to virtualize the Docker Engine environment and Linux kernel-specific features for the Docker daemon.

<a class="button" href="https://download.docker.com/win/beta/InstallDocker.msi">Get Docker for Windows</a>

**Requirements**

* 64bit Windows 10 Pro, Enterprise and Education (1511 November update, Build 10586 or later). In the future we will support more versions of Windows 10.

* The Hyper-V package must be enabled. The Docker for Windows installer will enable it for you, if needed. (This requires a reboot).

#### Docker Toolbox for Windows

If you have an earlier Windows system that doesn't meet the Docker for Windows prerequisites, <a href="https://www.docker.com/products/docker-toolbox" target="_blank">get Docker Toolbox</a>.

See [Docker Toolbox Overview](/toolbox/overview.md) for help on installing Docker with Toolbox.

### Docker for Linux
Docker Engine runs navitvely on Linux distributions.

For full instructions on getting Docker for various Linux distributions, see [Install Docker Engine](/engine/installation/index.md).

## Step 2: Install Docker

- **Docker for Mac** - Install instructions are at [Getting Started with Docker for Mac](https://docs.docker.com/docker-for-mac/).

- **Docker for Windows** - Install instructions are at [Getting Started with Docker for Windows](https://docs.docker.com/docker-for-windows/).

- **Docker Toolbox** - Install instructions are at [Docker Toolbox Overview](/toolbox/overview.md).

- **Docker on Linux** - For a simple example of installing Docker on Ubuntu Linux so that you can work through this tutorial, see [Installing Docker on Ubuntu Linux (Example)](linux_install_help.md). Full install instructions for all flavors of Linux we support are at [Install Docker Engine](/engine/installation/index.md).

## Step 3: Verify your installation

1. Open a command-line terminal, and run some Docker commands to verify that Docker is working as expected.

    Some good commands to try are `docker version` to check that you have the latest release installed and `docker ps` to see if you have any running containers. (Probably not, since you just started.)

2. Type the `docker run hello-world` command and press RETURN.

    The command does some work for you, if everything runs well, the command's
    output looks like this:

        $ docker run hello-world
        Unable to find image 'hello-world:latest' locally
        latest: Pulling from library/hello-world
        535020c3e8ad: Pull complete
        af340544ed62: Pull complete
        Digest: sha256:a68868bfe696c00866942e8f5ca39e3e31b79c1e50feaee4ce5e28df2f051d5c
        Status: Downloaded newer image for hello-world:latest

        Hello from Docker.
        This message shows that your installation appears to be working correctly.

        To generate this message, Docker took the following steps:
        1. The Docker Engine CLI client contacted the Docker Engine daemon.
        2. The Docker Engine daemon pulled the "hello-world" image from the Docker Hub.
        3. The Docker Engine daemon created a new container from that image which runs the
           executable that produces the output you are currently reading.
        4. The Docker Engine daemon streamed that output to the Docker Engine CLI client, which sent it
           to your terminal.

        To try something more ambitious, you can run an Ubuntu container with:
        $ docker run -it ubuntu bash

        Share images, automate workflows, and more with a free Docker Hub account:
        https://hub.docker.com

        For more examples and ideas, visit:
        https://docs.docker.com/userguide/

3. Run `docker ps -a` to show all containers on the system.

        $ docker ps -a

        CONTAINER ID        IMAGE               COMMAND             CREATED             STATUS                      PORTS               NAMES
        592376ff3eb8        hello-world         "/hello"            25 seconds ago      Exited (0) 24 seconds ago                       prickly_wozniak

    You should see your `hello-world` container listed in the output for the `docker ps -a` command.

    The command `docker ps` shows only currently running containers. Since `hello-world` already ran and exited, it wouldn't show up with a `docker ps`.

## Looking for troubleshooting help?

Typically, the above steps work out-of-the-box, but some scenarios can cause problems. If your `docker run hello-world` didn't work and resulted in errors, check out [Troubleshooting](/toolbox/faqs/troubleshoot.md) for quick fixes to common problems.

## Where to go next

At this point, you have successfully installed the Docker software. Leave the
Docker Quickstart Terminal window open. Now, go to the next page to [read a very
short introduction Docker images and containers](step_two.md).


&nbsp;
