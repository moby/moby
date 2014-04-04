page_title: Installation on Arch Linux
page_description: Please note this project is currently under heavy development. It should not be used in production.
page_keywords: arch linux, virtualization, docker, documentation, installation

# Arch Linux

Note

Docker is still under heavy development! We don’t recommend using it in
production yet, but we’re getting closer with each release. Please see
our blog post, ["Getting to Docker
1.0"](http://blog.docker.io/2013/08/getting-to-docker-1-0/)

Note

This is a community contributed installation path. The only ‘official’
installation is using the [*Ubuntu*](../ubuntulinux/#ubuntu-linux)
installation path. This version may be out of date because it depends on
some binaries to be updated and published

Installing on Arch Linux can be handled via the package in community:

-   [docker](https://www.archlinux.org/packages/community/x86_64/docker/)

or the following AUR package:

-   [docker-git](https://aur.archlinux.org/packages/docker-git/)

The docker package will install the latest tagged version of docker. The
docker-git package will build from the current master branch.

## Dependencies

Docker depends on several packages which are specified as dependencies
in the packages. The core dependencies are:

-   bridge-utils
-   device-mapper
-   iproute2
-   lxc
-   sqlite

## Installation

For the normal package a simple

    pacman -S docker

is all that is needed.

For the AUR package execute:

    yaourt -S docker-git

The instructions here assume **yaourt** is installed. See [Arch User
Repository](https://wiki.archlinux.org/index.php/Arch_User_Repository#Installing_packages)
for information on building and installing packages from the AUR if you
have not done so before.

## Starting Docker

There is a systemd service unit created for docker. To start the docker
service:

    sudo systemctl start docker

To start on system boot:

    sudo systemctl enable docker
