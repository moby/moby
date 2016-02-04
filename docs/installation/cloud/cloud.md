<!--[metadata]>
+++
title = "Choose how to install"
description = "Installation instructions for Docker on cloud."
keywords = ["cloud, docker, machine, documentation,  installation"]
[menu.main]
parent = "install_cloud"
weight=-3
+++
<![end-metadata]-->

# Understand cloud install options and choose one

You can install Docker Engine on any cloud platform that runs an operating system (OS) that Docker supports. This includes many flavors and versions of Linux, along with Mac and Windows.

You have two options for installing:

* Manually install on the cloud (create cloud hosts, then install Docker Engine on them)
* Use Docker Machine to provision cloud hosts

## Manually install Docker Engine on a cloud host

To install on a cloud provider:

1. Create an account with the cloud provider, and read cloud provider documentation to understand their process for creating hosts.

2. Decide which OS you want to run on the cloud host.

3. Understand the Docker prerequisites and install process for the chosen OS. See [Install Docker Engine](index.md) for a list of supported systems and links to the install guides.

4. Create a host with a Docker supported OS, and install Docker per the instructions for that OS.

[Example: Manual install on a cloud provider](cloud-ex-aws.md) shows how to create an <a href="https://aws.amazon.com/" target="_blank"> Amazon Web Services (AWS)</a> EC2 instance, and install Docker Engine on it.


## Use Docker Machine to provision cloud hosts

Docker Machine driver plugins are available for several popular cloud platforms, so you can use Machine to provision one or more Dockerized hosts on those platforms.

With Docker Machine, you can use the same interface to create cloud hosts with Docker Engine on them, each configured per the options you specify.

To do this, you use the `docker-machine create` command with the driver for the cloud provider, and provider-specific flags for account verification, security credentials, and other configuration details.

[Example: Use Docker Machine to provision cloud hosts](cloud-ex-machine-ocean.md) walks you through the steps to set up Docker Machine and provision a Dockerized host on [Digital Ocean](https://www.digitalocean.com/).

## Where to go next
* [Example: Manual install on a cloud provider](cloud-ex-aws.md) (AWS EC2)

* [Example: Use Docker Machine to provision cloud hosts](cloud-ex-machine-ocean.md) (Digital Ocean)

* [Using Docker Machine with a cloud provider](https://docs.docker.com/machine/get-started-cloud/)

* <a href="https://docs.docker.com/engine/userguide/" target="_blank"> Docker User Guide </a> (after your install is complete, get started using Docker)
