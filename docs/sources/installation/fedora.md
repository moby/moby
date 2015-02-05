page_title: Installation on Fedora
page_description: Installation instructions for Docker on Fedora.
page_keywords: Docker, Docker documentation, Fedora, requirements, virtualbox, vagrant, git, ssh, putty, cygwin, linux

# Fedora

Docker is supported only on Fedora 20 and later,
on the **64 bit** architecture.

## Installation

For `Fedora 20`, the `docker-io` package provides Docker.

If you have the (unrelated) `docker` package installed already, it will
conflict with `docker-io`. To proceed with `docker-io` installation on
Fedora 20, please remove `docker` first.

    $ sudo yum -y remove docker
    $ sudo yum -y install docker-io

For `Fedora 21 and later`, there are no package conflicts as the system
tray application and its executable have been renamed `wmdocker`.

Install the `docker` package which will install Docker on our host.

    $ sudo yum -y install docker

To update the `docker` package:

    $ sudo yum -y update docker

Now that it's installed, let's start the Docker daemon.

    $ sudo systemctl start docker

If we want Docker to start at boot, we should also:

    $ sudo systemctl enable docker

Now let's verify that Docker is working.

    $ sudo docker run -i -t fedora /bin/bash

> Note: If you get a `Cannot start container` error mentioning SELinux
> or permission denied, you may need to update the SELinux policies.
> This can be done using `sudo yum upgrade selinux-policy` and then rebooting.

## Granting rights to users to use Docker

The `docker` command line tool contacts the `docker` daemon process via a
socket file `/var/run/docker.sock` owned by `root:root`. Though it's
[recommended](https://lists.projectatomic.io/projectatomic-archives/atomic-devel/2015-January/msg00034.html)
to use `sudo` for docker commands, if users wish to avoid it, an administrator can
create a `docker` group, have it own `/var/run/docker.sock`, and add users to this group.

    $ sudo groupadd docker
    $ sudo chown root:docker /var/run/docker.sock
    $ sudo usermod -a -G docker $USERNAME

## Custom daemon options

If you need to add an HTTP Proxy, set a different directory or partition for the
Docker runtime files, or make other customizations, read our systemd article to
learn how to [customize your systemd Docker daemon options](/articles/systemd/).

## What next?

Continue with the [User Guide](/userguide/).

