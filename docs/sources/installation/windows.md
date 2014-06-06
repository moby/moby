page_title: Installation on Windows
page_description: Docker installation on Microsoft Windows
page_keywords: Docker, Docker documentation, Windows, requirements, virtualbox, boot2docker

# Windows

> **Note**:
> Docker is still under heavy development! We don't recommend using it in
> production yet, but we're getting closer with each release. Please see
> our blog post, [Getting to Docker 1.0](
> http://blog.docker.io/2013/08/getting-to-docker-1-0/)

Docker Engine runs on Windows using a lightweight virtual machine. There
is no native Windows Docker client yet, so everything is done inside the virtual
machine.

To make this process easier we designed a helper application called
[boot2docker](https://github.com/boot2docker/boot2docker) to install the
virtual machine and run the Docker daemon.


## Installation

1. Download the latest release of the [Docker for Windows Installer](https://github.com/boot2docker/windows-installer/releases)
2. Run the installer, which will install VirtualBox, MSYS-git, the boot2docker Linux ISO and the
   Boot2Docker management tool.
   ![](/installation/images/windows-installer.png)
3. Run the `Boot2Docker Start` shell script from your Desktop or Program Files > Docker.
   The Start script will ask you to enter an ssh key passphrase - the simplest
   (but least secure) is to just hit [Enter].
   ![](/installation/images/windows-boot2docker-start.png)

The `Boot2Docker Start` script will connect you to a shell session in the virtual
Machine. If needed, it will initialise a new VM and start it.

## Upgrading

To upgrade:

1. Download the latest release of the [Docker for Windows Installer](
   https://github.com/boot2docker/windows-installer/releases)
2. Run the installer, which will update the Boot2Docker management tool.
3. To upgrade your existing virtual machine, open a terminal and run:
    
```
        boot2docker stop
        boot2docker download
        boot2docker start
```


## Running Docker

Boot2Docker will log you in automatically so you can start using Docker
right away.

Let's try the “hello world” example. Run

    $ docker run busybox echo hello world

This will download the small busybox image and print hello world.

# Further Details

The Boot2Docker management tool provides some commands:

```
$ ./boot2docker
Usage: ./boot2docker [<options>] {help|init|up|ssh|save|down|poweroff|reset|restart|config|status|info|delete|download|version} [<args>]
```

## Container port redirection 

The latest version of `boot2docker` sets up two network adaptors: one using NAT
to allow the VM to download images and files from the Internet, and one host only
network adaptor to which the container's ports will be exposed on.

If you run a container with an exposed port:

```
   docker run --rm -i -t -p 80:80 apache
```

Then you should be able to access that Apache server using the IP address reported
to you using:

```
   boot2docker ssh ip addr show dev eth1
```

Typically, it is 192.168.59.103, but at this point it can change.

If you want to share container ports with other computers on your LAN, you will
need to set up [NAT adaptor based port forwarding](
https://github.com/boot2docker/boot2docker/blob/master/doc/WORKAROUNDS.md)



For further information or to report issues, please see the [Boot2Docker site](http://boot2docker.io)
