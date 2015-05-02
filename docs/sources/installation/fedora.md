page_title: Installation on Fedora
page_description: Instructions for installing Docker on Fedora.
page_keywords: Docker, Docker documentation, Fedora, requirements, linux

# Fedora

Docker is supported on the following versions of Fedora:

- [*Fedora 20 (64-bit)*](#fedora-20-installation)
- [*Fedora 21 and later (64-bit)*](#fedora-21-and-later-installation)

Currently the Fedora project will only support Docker when running on kernels
shipped by the distribution. There are kernel changes which will cause issues
if one decides to step outside that box and run non-distribution kernel packages.

## Fedora 21 and later

### Installation

Install the Docker package which will install Docker on our host.

    $ sudo yum -y install docker

To update the Docker package:

    $ sudo yum -y update docker

Please continue with the [Starting the Docker daemon](#starting-the-docker-daemon).

### Uninstallation

To uninstall the Docker package:

    $ sudo yum -y remove docker

The above command will not remove images, containers, volumes, or user created
configuration files on your host. If you wish to delete all images, containers,
and volumes run the following command:

    $ rm -rf /var/lib/docker

You must delete the user created configuration files manually.

## Fedora 20

### Installation

For `Fedora 20`, there is a package name conflict with a system tray application
and its executable, so the Docker RPM package was called `docker-io`.

To proceed with `docker-io` installation on Fedora 20, please remove the `docker`
package first.

    $ sudo yum -y remove docker
    $ sudo yum -y install docker-io

To update the Docker package:

    $ sudo yum -y update docker-io

Please continue with the [Starting the Docker daemon](#starting-the-docker-daemon).

### Uninstallation

To uninstall the Docker package:

    $ sudo yum -y remove docker-io

The above command will not remove images, containers, volumes, or user created
configuration files on your host. If you wish to delete all images, containers,
and volumes run the following command:

    $ rm -rf /var/lib/docker

You must delete the user created configuration files manually.

## Starting the Docker daemon

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
Docker runtime files, or make other customizations, read our Systemd article to
learn how to [customize your Systemd Docker daemon options](/articles/systemd/).

## What next?

Continue with the [User Guide](/userguide/).

