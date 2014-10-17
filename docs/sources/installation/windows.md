page_title: Installation on Windows
page_description: Docker installation on Microsoft Windows
page_keywords: Docker, Docker documentation, Windows, requirements, virtualbox, boot2docker

# Windows
> **Note:**
> Docker has been tested on Windows 7.1 and 8; it may also run on older versions.
> Your processor needs to support hardware virtualization.

The Docker Engine uses Linux-specific kernel features, so to run it on Windows
we need to use a lightweight virtual machine (vm).  You use the Windows Docker client to
control the virtualized Docker Engine to build, run, and manage Docker containers.

To make this process easier, we've designed a helper application called
[Boot2Docker](https://github.com/boot2docker/boot2docker) that installs the
virtual machine and runs the Docker daemon.

## Demonstration

<iframe width="640" height="480" src="//www.youtube.com/embed/oSHN8_uiZd4?rel=0" frameborder="0" allowfullscreen></iframe>

## Installation

1. Download the latest release of the [Docker for Windows Installer](https://github.com/boot2docker/windows-installer/releases/latest)
2. Run the installer, which will install VirtualBox, MSYS-git, the boot2docker Linux ISO,
and the Boot2Docker management tool.
   ![](/installation/images/windows-installer.png)
3. Run the `Boot2Docker Start` shell script from your Desktop or Program Files > Boot2Docker for Windows.
   The Start script will ask you to enter an ssh key passphrase - the simplest
   (but least secure) is to just hit [Enter].

   ![](/installation/images/windows-boot2docker-start.png)

   The `Boot2Docker Start` script will connect you to a shell session in the virtual
   machine. If needed, it will initialize a new VM and start it.

## Upgrading

1. Download the latest release of the [Docker for Windows Installer](
   https://github.com/boot2docker/windows-installer/releases/latest)

2. Run the installer, which will update the Boot2Docker management tool.

3. To upgrade your existing virtual machine, open a terminal and run:

        boot2docker stop
        boot2docker download
        boot2docker start

## Running Docker

Boot2Docker will log you in automatically so you can start using Docker right away.

Let's try the `hello-world` example image. Run

    $ docker run hello-world

This should download the very small `hello-world` image and print a `Hello from Docker.` message.


# Further Details

The Boot2Docker management tool provides several commands:

    $ ./boot2docker
    Usage: ./boot2docker [<options>] {help|init|up|ssh|save|down|poweroff|reset|restart|config|status|info|ip|delete|download|version} [<args>]


## Container port redirection

If you are curious, the username for the boot2docker default user is `docker` and the password is `tcuser`.

The latest version of `boot2docker` sets up a host only network adaptor which provides access to the container's ports.

If you run a container with an exposed port:

    docker run --rm -i -t -p 80:80 nginx

Then you should be able to access that nginx server using the IP address reported
to you using:

    boot2docker ip

Typically, it is 192.168.59.103, but it could get changed by Virtualbox's DHCP
implementation.

For further information or to report issues, please see the [Boot2Docker site](http://boot2docker.io)
