page_title: Installation from Binaries
page_description: Instructions for installing Docker as a binary. Mostly meant for hackers who want to try out Docker on a variety of environments.
page_keywords: binaries, installation, docker, documentation, linux

# Binaries

**This instruction set is meant for hackers who want to try out Docker
on a variety of environments.**

Before following these directions, you should really check if a packaged
version of Docker is already available for your distribution. We have
packages for many distributions, and more keep showing up all the time!

## Check runtime dependencies

To run properly, docker needs the following software to be installed at
runtime:

 - iptables version 1.4 or later
 - Git version 1.7 or later
 - procps (or similar provider of a "ps" executable)
 - XZ Utils 4.9 or later
 - a [properly mounted](
   https://github.com/tianon/cgroupfs-mount/blob/master/cgroupfs-mount)
   cgroupfs hierarchy (having a single, all-encompassing "cgroup" mount
   point [is](https://github.com/docker/docker/issues/2683)
   [not](https://github.com/docker/docker/issues/3485)
   [sufficient](https://github.com/docker/docker/issues/4568))

## Check kernel dependencies

Docker in daemon mode has specific kernel requirements. For details,
check your distribution in [*Installation*](../#installation-list).

A 3.10 Linux kernel is the minimum requirement for Docker.
Kernels older than 3.10 lack some of the features required to run Docker
containers. These older versions are known to have bugs which cause data loss
and frequently panic under certain conditions.

The latest minor version (3.x.y) of the 3.10 (or a newer maintained version)
Linux kernel is recommended. Keeping the kernel up to date with the latest
minor version will ensure critical kernel bugs get fixed.

> **Warning**:
> Installing custom kernels and kernel packages is probably not
> supported by your Linux distribution's vendor. Please make sure to
> ask your vendor about Docker support first before attempting to
> install custom kernels on your distribution.

> **Warning**:
> Installing a newer kernel might not be enough for some distributions
> which provide packages which are too old or incompatible with
> newer kernels.

Note that Docker also has a client mode, which can run on virtually any
Linux kernel (it even builds on OS X!).

## Enable AppArmor and SELinux when possible

Please use AppArmor or SELinux if your Linux distribution supports
either of the two. This helps improve security and blocks certain
types of exploits. Your distribution's documentation should provide
detailed steps on how to enable the recommended security mechanism.

Some Linux distributions enable AppArmor or SELinux by default and
they run a kernel which doesn't meet the minimum requirements (3.10
or newer). Updating the kernel to 3.10 or newer on such a system
might not be enough to start Docker and run containers.
Incompatibilities between the version of AppArmor/SELinux user
space utilities provided by the system and the kernel could prevent
Docker from running, from starting containers or, cause containers to
exhibit unexpected behaviour.

> **Warning**:
> If either of the security mechanisms is enabled, it should not be
> disabled to make Docker or its containers run. This will reduce
> security in that environment, lose support from the distribution's
> vendor for the system, and might break regulations and security
> policies in heavily regulated environments.

## Get the docker binary:

    $ wget https://get.docker.com/builds/Linux/x86_64/docker-latest -O docker
    $ chmod +x docker

> **Note**:
> If you have trouble downloading the binary, you can also get the smaller
> compressed release file:
> [https://get.docker.com/builds/Linux/x86_64/docker-latest.tgz](
> https://get.docker.com/builds/Linux/x86_64/docker-latest.tgz)

## Run the docker daemon

    # start the docker in daemon mode from the directory you unpacked
    $ sudo ./docker -d &

## Giving non-root access

The `docker` daemon always runs as the root user, and the `docker`
daemon binds to a Unix socket instead of a TCP port. By default that
Unix socket is owned by the user *root*, and so, by default, you can
access it with `sudo`.

If you (or your Docker installer) create a Unix group called *docker*
and add users to it, then the `docker` daemon will make the ownership of
the Unix socket read/writable by the *docker* group when the daemon
starts. The `docker` daemon must always run as the root user, but if you
run the `docker` client as a user in the *docker* group then you don't
need to add `sudo` to all the client commands.

> **Warning**: 
> The *docker* group (or the group specified with `-G`) is root-equivalent;
> see [*Docker Daemon Attack Surface*](
> /articles/security/#docker-daemon-attack-surface) details.

## Upgrades

To upgrade your manual installation of Docker, first kill the docker
daemon:

    $ killall docker

Then follow the regular installation steps.

## Run your first container!

    # check your docker version
    $ sudo ./docker version

    # run a container and open an interactive shell in the container
    $ sudo ./docker run -i -t ubuntu /bin/bash

Continue with the [User Guide](/userguide/).
