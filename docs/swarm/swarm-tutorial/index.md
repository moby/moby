<!--[metadata]>
+++
title = "Set up for the tutorial"
description = "Getting Started tutorial for Docker Swarm"
keywords = ["tutorial, cluster management, swarm"]
advisory = "rc"
[menu.main]
identifier="tutorial-setup"
parent="swarm-tutorial"
weight=11
+++
<![end-metadata]-->

# Getting Started with Docker Swarm

This tutorial introduces you to the key features of Docker Swarm. It guides you
through the following activities:

* initializing a cluster of Docker Engines called a Swarm
* adding nodes to the Swarm
* deploying application services to the Swarm
* managing the Swarm once you have everything running

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

The tutorial uses three networked host machines as nodes in the Swarm. These can
be virtual machines on your PC, in a data center, or on a cloud service
provider. This tutorial uses the following machine names:

* manager1
* worker1
* worker2

###  Docker Engine 1.12 or later

You must install Docker Engine on each one of the host machines. To use this
version of Swarm, install the Docker Engine `v1.12.0-rc1` or later from the
[Docker releases GitHub repository](https://github.com/docker/docker/releases).
Alternatively, install the latest Docker for Mac or Docker for Windows Beta.

Verify that the Docker Engine daemon is running on each of the machines.

<!-- See the following options to install:

* [Install Docker Engine](../../installation/index.md).

* [Example: Manual install on cloud provider](../../installation/cloud/cloud-ex-aws.md).
-->

### The IP address of the manager machine

The IP address must be assigned to an a network interface available to the host
operating system. All nodes in the Swarm must be able to access the manager at the IP address.

>**Tip**: You can run `ifconfig` on Linux or Mac OSX to see a list of the
available network interfaces.

The tutorial uses `manager1` : `192.168.99.100`.

### Open ports between the hosts

* **TCP port 2377** for cluster management communications
* **TCP** and **UDP port 7946** for communication among nodes
* **TCP** and **UDP port 4789** for overlay network traffic

>**Tip**: Docker recommends that every node in the cluster be on the same layer
3 (IP) subnet with all traffic permitted between nodes.

## What's next?

After you have set up your environment, you're ready to [create a Swarm](create-swarm.md).

<p style="margin-bottom:300px">&nbsp;</p>
