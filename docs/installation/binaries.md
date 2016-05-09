<!--[metadata]>
+++
title = "Installation from binaries"
description = "Instructions for installing Docker as a binary. Mostly meant for hackers who want to try out Docker on a variety of environments."
keywords = ["binaries, installation, docker, documentation,  linux"]
[menu.main]
parent = "engine_install"
weight = 110
+++
<![end-metadata]-->

# Installation from binaries

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
check your distribution in [*Installation*](index.md#on-linux).

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

## Get the Docker Engine binaries

You can download either the latest release binaries or a specific version. To get
the list of stable release version numbers from GitHub, view the `docker/docker`
[releases page](https://github.com/docker/docker/releases). You can get the MD5
and SHA256 hashes by appending .md5 and .sha256 to the URLs respectively


### Get the Linux binaries

To download the latest version for Linux, use the
following URLs:

    https://get.docker.com/builds/Linux/i386/docker-latest.tgz

    https://get.docker.com/builds/Linux/x86_64/docker-latest.tgz

To download a specific version for Linux, use the
following URL patterns:

    https://get.docker.com/builds/Linux/i386/docker-<version>.tgz

    https://get.docker.com/builds/Linux/x86_64/docker-<version>.tgz

For example:

    https://get.docker.com/builds/Linux/i386/docker-1.11.0.tgz

    https://get.docker.com/builds/Linux/x86_64/docker-1.11.0.tgz

> **Note** These instructions are for Docker Engine 1.11 and up. Engine 1.10 and
> under consists of a single binary, and instructions for those versions are
> different. To install version 1.10 or below, follow the instructions in the 
> <a href="/v1.10/engine/installation/binaries/" target="_blank">1.10 documentation</a>.


#### Install the Linux binaries

After downloading, you extract the archive, which puts the binaries in a
directory named `docker` in your current location.

```bash
$ tar -xvzf docker-latest.tgz

docker/
docker/docker-containerd-ctr
docker/docker
docker/docker-containerd
docker/docker-runc
docker/docker-containerd-shim
```

Engine requires these binaries to be installed in your host's `$PATH`.
For example, to install the binaries in `/usr/bin`:

```bash
$ mv docker/* /usr/bin/
```

> **Note**: If you already have Engine installed on your host, make sure you
> stop Engine before installing (`killall docker`), and install the binaries
> in the same location. You can find the location of the current installation
> with `dirname $(which docker)`.

#### Run the Engine daemon on Linux

You can manually start the Engine in daemon mode using:

```bash
$ sudo docker daemon &
```

The GitHub repository provides samples of init-scripts you can use to control
the daemon through a process manager, such as upstart or systemd. You can find
these scripts in the <a href="https://github.com/docker/docker/tree/master/contrib/init">
contrib directory</a>.

For additional information about running the Engine in daemon mode, refer to
the [daemon command](../reference/commandline/dockerd.md) in the Engine command
line reference.

### Get the Mac OS X binary

The Mac OS X binary is only a client. You cannot use it to run the `docker`
daemon. To download the latest version for Mac OS X, use the following URLs:

    https://get.docker.com/builds/Darwin/x86_64/docker-latest.tgz

To download a specific version for Mac OS X, use the
following URL pattern:

    https://get.docker.com/builds/Darwin/x86_64/docker-<version>.tgz

For example:

    https://get.docker.com/builds/Darwin/x86_64/docker-1.11.0.tgz

You can extract the downloaded archive either by double-clicking the downloaded
`.tgz` or on the command line, using `tar -xvzf docker-1.11.0.tgz`. The client
binary can be executed from any location on your filesystem.


### Get the Windows binary

You can only download the Windows binary for version `1.9.1` onwards.
Moreover, the 32-bit (`i386`) binary is only a client, you cannot use it to
run the `docker` daemon. The 64-bit binary (`x86_64`) is both a client and
daemon.

To download the latest version for Windows, use the following URLs:

    https://get.docker.com/builds/Windows/i386/docker-latest.zip

    https://get.docker.com/builds/Windows/x86_64/docker-latest.zip

To download a specific version for Windows, use the following URL pattern:

    https://get.docker.com/builds/Windows/i386/docker-<version>.zip

    https://get.docker.com/builds/Windows/x86_64/docker-<version>.zip

For example:

    https://get.docker.com/builds/Windows/i386/docker-1.11.0.zip

    https://get.docker.com/builds/Windows/x86_64/docker-1.11.0.zip


> **Note** These instructions are for Engine 1.11 and up. Instructions for older
> versions are slightly different. To install version 1.10 or below, follow the
> instructions in the <a href="/v1.10/engine/installation/binaries/" target="_blank">1.10 documentation</a>.

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
> see [*Docker Daemon Attack Surface*](../security/security.md#docker-daemon-attack-surface) details.

## Upgrade Docker Engine

To upgrade your manual installation of Docker Engine on Linux, first kill the docker
daemon:

    $ killall docker

Then follow the [regular installation steps](#get-the-linux-binaries).

## Next steps

Continue with the [User Guide](../userguide/index.md).
