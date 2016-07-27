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

If you’re brand new to Docker, see [About Docker Engine](../../index.md).

## Set up

To run this tutorial, you need the following:

* [three networked host machines](#three-networked-host-machines)
* [Docker Engine 1.12 or later installed](#docker-engine-1-12-or-later)
* [the IP address of the manager machine](#the-ip-address-of-the-manager-machine)
* [open ports between the hosts](#open-ports-between-the-hosts)

### Three networked host machines

The tutorial uses three networked host machines as nodes in the swarm. These can
be virtual machines on your PC, in a data center, or on a cloud service
provider. This tutorial uses the following machine names:

* manager1
* worker1
* worker2

###  Docker Engine 1.12 or later

You must install Docker Engine on each one of the host machines. To use swarm
mode, install the Docker Engine `v1.12.0-rc1` or later from the [Docker releases
GitHub repository](https://github.com/docker/docker/releases). Alternatively,
install the latest Docker for Mac or Docker for Windows Beta.

>**Advisory**: Some multi-node features may not work for Docker for Mac Beta and
Docker for Windows Beta. We're working on the multi-node features for GA.

Verify that the Docker Engine daemon is running on each of the machines.

<!-- See the following options to install:

* [Install Docker Engine](../../installation/index.md).

* [Example: Manual install on cloud provider](../../installation/cloud/cloud-ex-aws.md).
-->

### The IP address of the manager machine

The IP address must be assigned to an a network interface available to the host
operating system. All nodes in the swarm must be able to access the manager at the IP address.

Because other nodes contact the manager node on its IP address, you should use a
fixed IP address.

>**Tip**: You can run `ifconfig` on Linux or Mac OS X to see a list of the
available network interfaces.

The tutorial uses `manager1` : `192.168.99.100`.

### Open ports between the hosts

* **TCP port 2377** for cluster management communications
* **TCP** and **UDP port 7946** for communication among nodes
* **TCP** and **UDP port 4789** for overlay network traffic

## What's next?

After you have set up your environment, you're ready to [create a swarm](create-swarm.md).
