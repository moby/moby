<!--[metadata]>
+++
title = "Installation on Mac OS X"
description = "Instructions for installing Docker on OS X using boot2docker."
keywords = ["Docker, Docker documentation, requirements, boot2docker, VirtualBox, SSH, Linux, OSX, OS X,  Mac"]
[menu.main]
parent = "engine_install"
weight="-90"
+++
<![end-metadata]-->

# Mac OS X

You have two options for installing Docker on Mac:

- [Docker for Mac](#docker-for-mac)
- [Docker Toolbox](#docker-toolbox)

## Docker for Mac

Docker for Mac is our newest offering for the Mac. It runs as a native Mac application and uses <a href="https://github.com/mist64/xhyve/" target="_blank">xhyve</a> to virtualize the Docker Engine environment and Linux kernel-specific features for the Docker daemon.

Go to [Getting Started with Docker for Mac](https://docs.docker.com/docker-for-mac/) for download and install instructions, and to learn all about Docker for Mac.

**Requirements**

- Mac must be a 2010 or newer model, with Intel's hardware support for memory management unit (MMU) virtualization; i.e., Extended Page Tables (EPT)

- OS X 10.10.3 Yosemite or newer

- At least 4GB of RAM

- VirtualBox prior to version 4.3.30 must NOT be installed (it is incompatible with Docker for Mac). Docker for Mac will error out on install in this case. Uninstall the older version of VirtualBox and re-try the install.

## Docker Toolbox

If you have an earlier Mac that doesn't meet the Docker for Mac requirements, <a href="https://www.docker.com/products/docker-toolbox" target="_blank">get Docker Toolbox</a> for the Mac.

See [Docker Toolbox Overview](/toolbox/overview.md) for help on installing Docker with Toolbox.

The Docker Toolbox setup does not run Docker natively in OS X. Instead, it uses `docker-machine` to create and attach to a virtual machine (VM). This machine is a Linux VM that hosts Docker for you on your Mac.

**Requirements**

Your Mac must be running OS X 10.8 "Mountain Lion" or newer to install the Docker Toolbox. Full install instructions are at [Toolbox install instructions for Mac](/toolbox/toolbox_install_mac.md).


## Learning more

* If you are new to Docker, try out the [Getting Started](../getstarted/index.md) tutorial for a hands-on tour, including using Docker commands, running containers, building images, and working with Docker Hub.

* You can find more extensive examples in [Learn by example](../tutorials/index.md) and in the [Docker Engine User Guide](../userguide/index.md).

* If you are interested in using the Kitematic GUI, see the [Kitematic user guide](https://docs.docker.com/kitematic/userguide/).

> **Note**: The Boot2Docker command line was deprecated several releases back in favor of Docker Machine, and now Docker for Mac.
