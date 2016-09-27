<!--[metadata]>
+++
title = "Set up for the tutorial"
description = "Getting Started tutorial for Docker Engine swarm mode"
keywords = ["tutorial, cluster management, swarm mode"]
[menu.main]
identifier="tutorial-setup"
parent="swarm-tutorial"
weight=11
+++
<![end-metadata]-->

# Getting started with swarm mode

This tutorial introduces you to the features of Docker Engine Swarm mode. You
may want to familiarize yourself with the [key concepts](../key-concepts.md)
before you begin.

The tutorial guides you through the following activities:

* initializing a cluster of Docker Engines in swarm mode
* adding nodes to the swarm
* deploying application services to the swarm
* managing the swarm once you have everything running

This tutorial uses Docker Engine CLI commands entered on the command line of a
terminal window. You should be able to install Docker on networked machines and
be comfortable running commands in the shell of your choice.

If you are brand new to Docker, see [About Docker Engine](../../index.md).

## Set up

To run this tutorial, you need the following:

* [three networked host machines](#three-networked-host-machines)
* [Docker Engine 1.12 or later installed](#docker-engine-1-12-or-newer)
* [the IP address of the manager machine](#the-ip-address-of-the-manager-machine)
* [open ports between the hosts](#open-ports-between-the-hosts)

### Three networked host machines

The tutorial uses three networked host machines as nodes in the swarm. These can
be virtual machines on your PC, in a data center, or on a cloud service
provider. This tutorial uses the following machine names:

* manager1
* worker1
* worker2

>**Note:** You can follow many of the tutorial steps to test single-node swarm
as well, in which case you need only one host. Multi-node commands will not
work, but you can initialize a swarm, create services, and scale them.

###  Docker Engine 1.12 or newer

This tutorial requires Docker Engine 1.12 or newer on each of the host machines.
Install Docker Engine and verify that the Docker Engine daemon is running on
each of the machines. You can get the latest version of Docker Engine as
follows:

* [install Docker Engine on Linux machines](#install-docker-engine-on-linux-machines)

* [use Docker for Mac or Docker for Windows](#use-docker-for-mac-or-docker-for-windows)

#### Install Docker Engine on Linux machines

If you are using Linux based physical computers or cloud-provided computers as
hosts, simply follow the [Linux install
instructions](../../installation/index.md) for your platform. Spin up the three
machines, and you are ready. You can test both
single-node and multi-node swarm scenarios on Linux machines.

#### Use Docker for Mac or Docker for Windows

Alternatively, install the latest [Docker for Mac](/docker-for-mac/index.md) or
[Docker for Windows](/docker-for-windows/index.md) application on a one
computer. You can test both single-node and multi-node swarm from this computer,
but you will need to use Docker Machine to test the multi-node scenarios.

* You can use Docker for Mac or Windows to test _single-node_ features of swarm
mode, including initializing a swarm with a single node, creating services,
and scaling services. Docker "Moby" on Hyperkit (Mac) or Hyper-V (Windows)
will serve as the single swarm node.

<p />

* Currently, you cannot use Docker for Mac or Windows alone to test a
_multi-node_ swarm. However, you can use the included version of [Docker
Machine](/machine/overview.md) to create the swarm nodes (see [Get started with Docker Machine and a local VM](/machine/get-started.md)), then follow the
tutorial for all multi-node features. For this scenario, you run commands from
a Docker for Mac or Docker for Windows host, but that Docker host itself is
_not_ participating in the swarm (i.e., it will not be `manager1`, `worker1`,
or `worker2` in our example). After you create the nodes, you can run all
swarm commands as shown from the Mac terminal or Windows PowerShell with
Docker for Mac or Docker for Windows running.

### The IP address of the manager machine

The IP address must be assigned to a network interface available to the host
operating system. All nodes in the swarm must be able to access the manager at
the IP address.

Because other nodes contact the manager node on its IP address, you should use a
fixed IP address.

You can run `ifconfig` on Linux or Mac OS X to see a list of the
available network interfaces.

If you are using Docker Machine, you can get the manager IP with either
`docker-machine ls` or `docker-machine ip <MACHINE-NAME>` &#8212; for example,
`docker-machine ip manager1`.

The tutorial uses `manager1` : `192.168.99.100`.

### Open ports between the hosts

The following ports must be available. On some systems, these ports are open by default.

* **TCP port 2377** for cluster management communications
* **TCP** and **UDP port 7946** for communication among nodes
* **TCP** and **UDP port 4789** for overlay network traffic

## What's next?

After you have set up your environment, you are ready to [create a swarm](create-swarm.md).
