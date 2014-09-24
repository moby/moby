page_title: Installation on Debian
page_description: Instructions for installing Docker on Debian.
page_keywords: Docker, Docker documentation, installation, debian

# Debian

Docker is supported on the following versions of Debian:

 - [*Debian 8.0 Jessie (64-bit)*](#debian-jessie-8-64-bit)
 - [*Debian 7.5 Wheezy (64-bit)*](#debian-wheezy-7-64-bit)

## Debian Jessie 8.0 (64-bit)

Debian 8 comes with a 3.14.0 Linux kernel, and a `docker.io` package which
installs all its prerequisites from Debian's repository.

> **Note**:
> Debian contains a much older KDE3/GNOME2 package called ``docker``, so the
> package and the executable are called ``docker.io``.

### Installation

To install the latest Debian package (may not be the latest Docker release):

    $ sudo apt-get update
    $ sudo apt-get install docker.io

To verify that everything has worked as expected:

    $ sudo docker run -i -t ubuntu /bin/bash

Which should download the `ubuntu` image, and then start `bash` in a container.

> **Note**: 
> If you want to enable memory and swap accounting see
> [this](/installation/ubuntulinux/#memory-and-swap-accounting).

## Debian Wheezy/Stable 7.x (64-bit)

Docker requires Kernel 3.8+, while Wheezy ships with Kernel 3.2 (for more details
on why 3.8 is required, see discussion on
[bug #407](https://github.com/docker/docker/issues/407%20kernel%20versions)).

Fortunately, wheezy-backports currently has [Kernel 3.14
](https://packages.debian.org/search?suite=wheezy-backports&section=all&arch=any&searchon=names&keywords=linux-image-amd64),
which is officially supported by Docker.

### Installation

1. Install Kernel 3.14 from wheezy-backports
 
    Add the following line to your `/etc/apt/sources.list`

    `deb http://http.debian.net/debian wheezy-backports main`

    then install the `linux-image-amd64` package (note the use of
    `-t wheezy-backports`)
 
        $ sudo apt-get update
        $ sudo apt-get install -t wheezy-backports linux-image-amd64

2. Install Docker using the get.docker.com script:
 
    `curl -sSL https://get.docker.com/ | sh`

## Giving non-root access

The `docker` daemon always runs as the `root` user and the `docker`
daemon binds to a Unix socket instead of a TCP port. By default that
Unix socket is owned by the user `root`, and so, by default, you can
access it with `sudo`.

If you (or your Docker installer) create a Unix group called `docker`
and add users to it, then the `docker` daemon will make the ownership of
the Unix socket read/writable by the `docker` group when the daemon
starts. The `docker` daemon must always run as the root user, but if you
run the `docker` client as a user in the `docker` group then you don't
need to add `sudo` to all the client commands. From Docker 0.9.0 you can
use the `-G` flag to specify an alternative group.

> **Warning**: 
> The `docker` group (or the group specified with the `-G` flag) is
> `root`-equivalent; see [*Docker Daemon Attack Surface*](
> /articles/security/#dockersecurity-daemon) details.

**Example:**

    # Add the docker group if it doesn't already exist.
    $ sudo groupadd docker

    # Add the connected user "${USER}" to the docker group.
    # Change the user name to match your preferred user.
    # You may have to logout and log back in again for
    # this to take effect.
    $ sudo gpasswd -a ${USER} docker

    # Restart the Docker daemon.
    $ sudo service docker restart


## What next?

Continue with the [User Guide](/userguide/).
