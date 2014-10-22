page_title: Installation on Mac OS X
page_description: Instructions for installing Docker on OS X using boot2docker.
page_keywords: Docker, Docker documentation, requirements, boot2docker, VirtualBox, SSH, Linux, OSX, OS X, Mac

# Installing Docker on Mac OS X

> **Note:**
> Docker is supported on Mac OS X 10.6 "Snow Leopard" or newer.

Because the Docker Engine uses Linux-specific kernel features, you'll need to use a
lightweight virtual machine (VM) to run it on OS X. You use the OS X Docker client to
control the virtualized Docker Engine to build, run, and manage Docker containers.

To make this process easier, we've built a helper application called
[Boot2Docker](https://github.com/boot2docker/boot2docker) that installs a
virtual machine (using VirtualBox) that's all set up to run the Docker daemon.

## Demonstration

<iframe width="640" height="360" src="//www.youtube.com/embed/wQsrKX4588U?rel=0" frameborder="0" allowfullscreen></iframe>

## Installation

1. Download the latest release of the [Docker for OS X Installer](
   https://github.com/boot2docker/osx-installer/releases/latest) (Look for the
   green Boot2Docker-x.x.x.pkg button near the bottom of the page.)

2. Run the installer by double-clicking the downloaded package, which will install a
VirtualBox VM, Docker itself, and the Boot2Docker management tool.
   ![](/installation/images/osx-installer.png)

3. Locate the `Boot2Docker` app in your `Applications` folder and run it.
   Or, you can initialize Boot2Docker from the command line by running:

	     $ boot2docker init
	     $ boot2docker start
	     $ $(boot2docker shellinit)

A terminal window will open and you'll see the virtual machine starting up. 
Once you have an initialized virtual machine, you can control it with `boot2docker stop`
and `boot2docker start`.

> **Note:**
> If you see a message in the terminal that looks something like this:
>
>    `To connect the Docker client to the Docker daemon, please set: export 
DOCKER_HOST=tcp://192.168.59.103:2375`
> 
you can safely set the environment variable as instructed.

View the
[Boot2Docker ReadMe](https://github.com/boot2docker/boot2docker/blob/master/README.md)
for more information.

## Upgrading

1. Download the latest release of the [Docker for OS X Installer](
   https://github.com/boot2docker/osx-installer/releases/latest)

2. If Boot2Docker is currently running, stop it with `boot2docker stop`. Then, run
the installer package, which will update Docker and the Boot2Docker management tool.

3. To complete the upgrade, you also need to update your existing virtual machine. Open a
terminal window and run:

        $ boot2docker stop
        $ boot2docker download
        $ boot2docker start

This will download an .iso image containing a fresh VM and start it up. Your upgrade is
complete. You can test it by following the directions below.

## Running Docker

From your terminal, you can test that Docker is running with our small `hello-world`
example image:
Start the vm (`boot2docker start`) and then run:

    $ docker run hello-world

This should download the `hello-world` image, which then creates a small
container with an executable that prints a brief `Hello from Docker.` message.

## Container port redirection

The latest version of `boot2docker` sets up a host-only network adaptor which provides
access to the container's ports.

If you run a container with an exposed port,

    $ docker run --rm -i -t -p 80:80 nginx

then you should be able to access that Nginx server using the IP address reported by:

    $ boot2docker ip

Typically, it is 192.168.59.103:2375, but VirtualBox's DHCP implementation might change
this address in the future.

# Further details

If you are curious, the username for the boot2docker default user is `docker` and the
password is `tcuser`.

The Boot2Docker management tool provides several additional commands for working with the
VM and Docker:

    $ ./boot2docker
    Usage: ./boot2docker [<options>]
    {help|init|up|ssh|save|down|poweroff|reset|restart|config|status|info|ip|delete|download|version} [<args>]

Continue with the [User Guide](/userguide/).

For further information or to report issues, please visit the [Boot2Docker site](http://boot2docker.io).
