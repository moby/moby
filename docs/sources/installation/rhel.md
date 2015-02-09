page_title: Installation on Red Hat Enterprise Linux
page_description: Instructions for installing Docker on Red Hat Enterprise Linux.
page_keywords: Docker, Docker documentation, requirements, linux, rhel

# Red Hat Enterprise Linux

Docker is supported on the following versions of RHEL:

- [*Red Hat Enterprise Linux 7 (64-bit)*](#red-hat-enterprise-linux-7-installation)
- [*Red Hat Enterprise Linux 6.5 (64-bit)*](#red-hat-enterprise-linux-6.5-installation) or later

## Kernel support

RHEL will only support Docker via the *extras* channel or EPEL package when
running on kernels shipped by the distribution. There are kernel changes which
will cause issues if one decides to step outside that box and run
non-distribution kernel packages.

## Red Hat Enterprise Linux 7 Installation

**Red Hat Enterprise Linux 7 (64 bit)** has [shipped with
Docker](https://access.redhat.com/site/products/red-hat-enterprise-linux/docker-and-containers).
An overview and some guidance can be found in the [Release
Notes](https://access.redhat.com/site/documentation/en-US/Red_Hat_Enterprise_Linux/7/html/7.0_Release_Notes/chap-Red_Hat_Enterprise_Linux-7.0_Release_Notes-Linux_Containers_with_Docker_Format.html).

Docker is located in the *extras* channel. To install Docker:

1. Enable the *extras* channel:

        $ sudo subscription-manager repos --enable=rhel-7-server-extras-rpms

2. Install Docker:

        $ sudo yum install docker 

Additional installation, configuration, and usage information,
including a [Get Started with Docker Containers in Red Hat
Enterprise Linux 7](https://access.redhat.com/site/articles/881893)
guide, can be found by Red Hat customers on the [Red Hat Customer
Portal](https://access.redhat.com/).

Please continue with the [Starting the Docker daemon](#starting-the-docker-daemon).

## Red Hat Enterprise Linux 6.5 Installation

You will need **64 bit** [RHEL
6.5](https://access.redhat.com/site/articles/3078#RHEL6) or later, with
a RHEL 6 kernel version 2.6.32-431 or higher as this has specific kernel
fixes to allow Docker to work.

Docker is available for **RHEL6.5** on EPEL. Please note that
this package is part of [Extra Packages for Enterprise Linux
(EPEL)](https://fedoraproject.org/wiki/EPEL), a community effort to
create and maintain additional packages for the RHEL distribution.

### Kernel support

RHEL will only support Docker via the *extras* channel or EPEL package when
running on kernels shipped by the distribution. There are things like namespace
changes which will cause issues if one decides to step outside that box and run
non-distro kernel packages.

> **Warning**:
> Please keep your system up to date using `yum update` and rebooting
> your system. Keeping your system updated ensures critical security
>  vulnerabilities and severe bugs (such as those found in kernel 2.6.32)
> are fixed.

## Installation

Firstly, you need to install the EPEL repository. Please follow the
[EPEL installation
instructions](https://fedoraproject.org/wiki/EPEL#How_can_I_use_these_extra_packages.3F).

There is a package name conflict with a system tray application
and its executable, so the Docker RPM package was called `docker-io`.

To proceed with `docker-io` installation, you may need to remove the
`docker` package first.

    $ sudo yum -y remove docker

Next, let's install the `docker-io` package which will install Docker on our host.

    $ sudo yum install docker-io

To update the `docker-io` package

    $ sudo yum -y update docker-io

Please continue with the [Starting the Docker daemon](#starting-the-docker-daemon).

## Starting the Docker daemon

Now that it's installed, let's start the Docker daemon.

    $ sudo service docker start

If we want Docker to start at boot, we should also:

    $ sudo chkconfig docker on

Now let's verify that Docker is working.

    $ sudo docker run -i -t fedora /bin/bash

> Note: If you get a `Cannot start container` error mentioning SELinux
> or permission denied, you may need to update the SELinux policies.
> This can be done using `sudo yum upgrade selinux-policy` and then rebooting.

**Done!**

Continue with the [User Guide](/userguide/).

## Custom daemon options

If you need to add an HTTP Proxy, set a different directory or partition for the
Docker runtime files, or make other customizations, read our Systemd article to
learn how to [customize your Systemd Docker daemon options](/articles/systemd/).


## Issues?

If you have any issues - please report them directly in the
[Red Hat Bugzilla for docker-io component](
https://bugzilla.redhat.com/enter_bug.cgi?product=Fedora%20EPEL&component=docker-io).
