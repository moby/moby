<!--[metadata]>
+++
title = "Installation on Arch Linux"
description = "Installation instructions for Docker on ArchLinux."
keywords = ["arch linux, virtualization, docker, documentation,  installation"]
[menu.main]
parent = "smn_linux"
+++
<![end-metadata]-->

# Arch Linux

Installing on Arch Linux can be handled via the package in community:

 - [docker](https://www.archlinux.org/packages/community/x86_64/docker/)

or the following AUR package:

 - [docker-git](https://aur.archlinux.org/packages/docker-git/)

The docker package will install the latest tagged version of docker. The
docker-git package will build from the current master branch.

## Dependencies

Docker depends on several packages which are specified as dependencies
in the packages. The core dependencies are:

 - bridge-utils
 - device-mapper
 - iproute2
 - sqlite

## Installation

For the normal package a simple

    $ sudo pacman -S docker

is all that is needed.

For the AUR package execute:

    $ yaourt -S docker-git

The instructions here assume **yaourt** is installed. See [Arch User
Repository](https://wiki.archlinux.org/index.php/Arch_User_Repository#Installing_packages)
for information on building and installing packages from the AUR if you
have not done so before.

## Starting Docker

There is a systemd service unit created for docker. To start the docker
service:

    $ sudo systemctl start docker

To start on system boot:

    $ sudo systemctl enable docker

## Custom daemon options

If you need to add an HTTP Proxy, set a different directory or partition for the
Docker runtime files, or make other customizations, read our systemd article to
learn how to [customize your systemd Docker daemon options](../articles/systemd.md).

## Running Docker with a manually-defined network

If you manually configure your network using `systemd-network` version 220 or
higher, containers you start with Docker may be unable to access your network.
Beginning with version 220, the forwarding setting for a given network
(`net.ipv4.conf.<interface>.forwarding`) defaults to *off*. This setting
prevents IP forwarding. It also conflicts with Docker which enables the
`net.ipv4.conf.all.forwarding` setting within a container.

To work around this, edit the `<interface>.network` file in
`/etc/systemd/network/` on your Docker host add the following block:

```
[Network]
...
IPForward=kernel
...
```

This configuration allows IP forwarding from the container as expected.
## Uninstallation

To uninstall the Docker package:

    $ sudo pacman -R docker

To uninstall the Docker package and dependencies that are no longer needed:

    $ sudo pacman -Rns docker

The above commands will not remove images, containers, volumes, or user created
configuration files on your host. If you wish to delete all images, containers,
and volumes run the following command:

    $ rm -rf /var/lib/docker

You must delete the user created configuration files manually.
