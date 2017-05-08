<!--[metadata]>
+++
title = "Installation on Windows"
description = "Docker installation on Microsoft Windows"
keywords = ["Docker, Docker documentation, Windows, requirements, virtualbox,  boot2docker"]
[menu.main]
parent = "engine_install"
weight="-80"
+++
<![end-metadata]-->

# Windows

You have two options for installing Docker on Windows:

- [Docker for Windows](#docker-for-windows)
- [Docker Toolbox](#docker-toolbox)

## Docker for Windows

Docker for Windows is our newest offering for PCs. It runs as a native Windows application and uses Hyper-V to virtualize the Docker Engine environment and Linux kernel-specific features for the Docker daemon.

Go to [Getting Started with Docker for Windows](https://docs.docker.com/docker-for-windows/) for download and install instructions, and to learn all about Docker for Windows.

**Requirements**

* 64bit Windows 10 Pro, Enterprise and Education (1511 November update, Build 10586 or later). In the future we will support more versions of Windows 10.

* The Hyper-V package must be enabled. The Docker for Windows installer will enable it for you, if needed. (This requires a reboot).

## Docker Toolbox

If you have an earlier Windows system that doesn't meet the Docker for Windows requirements, <a href="https://www.docker.com/products/docker-toolbox" target="_blank">get Docker Toolbox</a>.

See [Docker Toolbox Overview](/toolbox/overview.md) for help on installing Docker with Toolbox.

The Docker Toolbox setup does not run Docker natively on Windows. Instead, it uses `docker-machine` to create and attach to a virtual machine (VM). This machine is a Linux VM that hosts Docker for you on your Windows system.

**Requirements**

To run Docker, your machine must have a 64-bit operating system running Windows 7 or higher. Additionally, you must make sure that virtualization is enabled on your machine. For details, see the [Toolbox install instructions for Windows](/toolbox/toolbox_install_windows.md).

## Learning more

* If you are new to Docker, try out the [Getting Started](../getstarted/index.md) tutorial for a hands-on tour, including using Docker commands, running containers, building images, and working with Docker Hub.

* You can find more extensive examples in [Learn by example](../tutorials/index.md) and in the [Docker Engine User Guide](../userguide/index.md).

* If you are interested in using the Kitematic GUI, see the [Kitematic user guide](https://docs.docker.com/kitematic/userguide/).

> **Note**: The Boot2Docker command line was deprecated several releases > back in favor of Docker Machine, and now Docker for Windows.
