page_title: Installation on CentOS
page_description: Instructions for installing Docker on CentOS
page_keywords: Docker, Docker documentation, requirements, linux, centos, epel, docker.io, docker-io

# CentOS

Docker is supported on the following versions of CentOS:

- [*CentOS 7 (64-bit)*](#installing-docker-centos-7)
- [*CentOS 6.5 (64-bit)*](#installing-docker-centos-6.5) or later

These instructions are likely work for other binary compatible EL6/EL7 distributions
such as Scientific Linux, but they haven't been tested.

Please note that due to the current Docker limitations, Docker is able to
run only on the **64 bit** architecture.

## Kernel support

Currently the CentOS project will only support Docker when running on kernels
shipped by the distribution. There are kernel changes which will cause issues
if one decides to step outside that box and run non-distribution kernel packages.

To run Docker on [CentOS-6.5](http://www.centos.org) or later, you will need
kernel version 2.6.32-431 or higher as this has specific kernel fixes to allow
Docker to run.

## Installing Docker - CentOS-7
Docker is included by default in the CentOS-Extras repository. To install
run the following command:

    $ sudo yum install docker

Please continue with the [Starting the Docker daemon](#starting-the-docker-daemon).

### FirewallD

CentOS-7 introduced firewalld, which is a wrapper around iptables and can
conflict with Docker.

When `firewalld` is started or restarted it will remove the `DOCKER` chain
from iptables, preventing Docker from working properly.

When using Systemd, `firewalld` is started before Docker, but if you
start or restart `firewalld` after Docker, you will have to restart the Docker daemon.

## Installing Docker - CentOS-6.5

For CentOS-6.5, the Docker package is part of [Extra Packages
for Enterprise Linux (EPEL)](https://fedoraproject.org/wiki/EPEL) repository,
a community effort to create and maintain additional packages for the RHEL distribution.

Firstly, you need to ensure you have the EPEL repository enabled. Please
follow the [EPEL installation instructions](
https://fedoraproject.org/wiki/EPEL#How_can_I_use_these_extra_packages.3F).

For CentOS-6, there is a package name conflict with a system tray application
and its executable, so the Docker RPM package was called `docker-io`.

To proceed with `docker-io` installation on CentOS-6, you may need to remove the
`docker` package first.

    $ sudo yum -y remove docker

Next, let's install the `docker-io` package which will install Docker on our host.

    $ sudo yum install docker-io

Please continue with the [Starting the Docker daemon](#starting-the-docker-daemon).

## Manual installation of latest Docker release

While using a package is the recommended way of installing Docker,
the above package might not be the current release version. If you need the latest
version, [you can install the binary directly](
https://docs.docker.com/installation/binaries/).

When installing the binary without a package, you may want
to integrate Docker with Systemd. For this, install the two unit files
(service and socket) from [the GitHub
repository](https://github.com/docker/docker/tree/master/contrib/init/systemd)
to `/etc/systemd/system`.

Please continue with the [Starting the Docker daemon](#starting-the-docker-daemon).

## Starting the Docker daemon

Once Docker is installed, you will need to start the docker daemon.

    $ sudo service docker start

If we want Docker to start at boot, we should also:

    $ sudo chkconfig docker on

Now let's verify that Docker is working. First we'll need to get the latest
`centos` image.

    $ sudo docker pull centos

Next we'll make sure that we can see the image by running:

    $ sudo docker images centos

This should generate some output similar to:

    $ sudo docker images centos
    REPOSITORY      TAG             IMAGE ID          CREATED             VIRTUAL SIZE
    centos          latest          0b443ba03958      2 hours ago         297.6 MB

Run a simple bash shell to test the image:

    $ sudo docker run -i -t centos /bin/bash

If everything is working properly, you'll get a simple bash prompt. Type
`exit` to continue.

## Custom daemon options

If you need to add an HTTP Proxy, set a different directory or partition for the
Docker runtime files, or make other customizations, read our Systemd article to
learn how to [customize your Systemd Docker daemon options](/articles/systemd/).

## Dockerfiles
The CentOS Project provides a number of sample Dockerfiles which you may use
either as templates or to familiarize yourself with docker. These templates
are available on GitHub at [https://github.com/CentOS/CentOS-Dockerfiles](
https://github.com/CentOS/CentOS-Dockerfiles)

**Done!** You can either continue with the [Docker User
Guide](/userguide/) or explore and build on the images yourself.

## Issues?

If you have any issues - please report them directly in the
[CentOS bug tracker](http://bugs.centos.org).
