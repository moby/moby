page_title: Installation on Mac OS X
page_description: Instructions for installing Docker on OS X using boot2docker.
page_keywords: Docker, Docker documentation, requirements, boot2docker, VirtualBox, SSH, Linux, OSX, OS X, Mac

# Installing Docker on Mac OS X

> **Note:**
> Docker is supported on Mac OS X 10.6 "Snow Leopard" or newer.

The Docker Engine uses Linux-specific kernel features, so to run it on OS X
we need to use a lightweight virtual machine (vm).  You use the OS X Docker client to
control the virtualized Docker Engine to build, run, and manage Docker containers.

To make this process easier, we've designed a helper application called
[Boot2Docker](https://github.com/boot2docker/boot2docker) that installs the
virtual machine and runs the Docker daemon.

## Demonstration

<iframe width="640" height="360" src="//www.youtube.com/embed/wQsrKX4588U?rel=0" frameborder="0" allowfullscreen></iframe>

## Installation

1. Download the latest release of the [Docker for OS X Installer](
   https://github.com/boot2docker/osx-installer/releases)

2. Run the installer, which will install VirtualBox and the Boot2Docker management
   tool.
   ![](/installation/images/osx-installer.png)

3. Run the `Boot2Docker` app in the `Applications` folder:
   ![](/installation/images/osx-Boot2Docker-Start-app.png)

   Or, to initialize Boot2Docker manually, open a terminal and run:

	     $ boot2docker init
	     $ boot2docker start
	     $ export DOCKER_HOST=tcp://$(boot2docker ip 2>/dev/null):2375

Once you have an initialized virtual machine, you can control it with `boot2docker stop`
and `boot2docker start`.

## Upgrading

1. Download the latest release of the [Docker for OS X Installer](
   https://github.com/boot2docker/osx-installer/releases)

2. Run the installer, which will update VirtualBox and the Boot2Docker management
   tool.

3. To upgrade your existing virtual machine, open a terminal and run:

        $ boot2docker stop
        $ boot2docker download
        $ boot2docker start

## Running Docker

From your terminal, you can test that Docker is running with a “hello world” example.
Start the vm and then run:

    $ docker run ubuntu echo hello world

This should download the `ubuntu` image and print `hello world`.

## Container port redirection

The latest version of `boot2docker` sets up a host only network adaptor which provides
access to the container's ports.

If you run a container with an exposed port,

    $ docker run --rm -i -t -p 80:80 nginx

then you should be able to access that Nginx server using the IP address reported by:

    $ boot2docker ip

Typically, it is 192.168.59.103, but it could get changed by Virtualbox's DHCP
implementation.

# Further details

If you are curious, the username for the boot2docker default user is `docker` and the password is `tcuser`.

The Boot2Docker management tool provides several commands:

    $ ./boot2docker
    Usage: ./boot2docker [<options>]
    {help|init|up|ssh|save|down|poweroff|reset|restart|config|status|info|ip|delete|download|version} [<args>]

Continue with the [User Guide](/userguide/).

For further information or to report issues, please visit the [Boot2Docker site](http://boot2docker.io).
