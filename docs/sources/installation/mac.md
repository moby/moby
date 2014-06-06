page_title: Installation on Mac OS X
page_description: Instructions for installing Docker on OS X using boot2docker.
page_keywords: Docker, Docker documentation, requirements, boot2docker, VirtualBox, SSH, Linux, OSX, OS X, Mac

# Installing Docker on Mac OS X

> **Note**:
> Docker is still under heavy development! We don't recommend using it in
> production yet, but we're getting closer with each release. Please see
> our blog post, [Getting to Docker 1.0](
> http://blog.docker.io/2013/08/getting-to-docker-1-0/)

> **Note:**
> Docker is supported on Mac OS X 10.6 "Snow Leopard" or newer.

The Docker Engine uses Linux-specific kernel features, so we run it on OS X
using a lightweight virtual machine.  You can use the OS X Docker client to
control the virtualized engine to build, run and manage Docker containers.

To make this process easier we designed a helper application called
[boot2docker](https://github.com/boot2docker/boot2docker) to install the
virtual machine and run the Docker daemon.

## Installation

1. Download the latest release of the [Docker for OSX Installer](
   https://github.com/boot2docker/osx-installer/releases)
2. Run the installer, which will install VirtualBox and the Boot2Docker management
   tool.
   ![](/installation/images/osx-installer.png)
3. Open a terminal and run:

```
	boot2docker init
	boot2docker start
	export DOCKER_HOST=tcp://localhost:2375
```

`boot2docker init` will ask you to enter an ssh key passphrase - the simplest
(but least secure) is to just hit [Enter]. This passphrase is used by the
`boot2docker ssh` command.

Once you have an initialized virtual machine, you can `boot2docker stop`
and `boot2docker start` it.

## Upgrading

To upgrade:

1. Download the latest release of the [Docker for OSX Installer](
   https://github.com/boot2docker/osx-installer/releases)
2. Run the installer, which will update VirtualBox and the Boot2Docker management
   tool.
3. To upgrade your existing virtual machine, open a terminal and run:

```
	boot2docker stop
	boot2docker download
	boot2docker start
```

## Running Docker

From your terminal, you can try the “hello world” example. Run:

    $ docker run ubuntu echo hello world

This will download the `ubuntu` image and print `hello world`.

## Container port redirection

The latest version of `boot2docker` sets up two network adapters: one using NAT
to allow the VM to download images and files from the Internet, and one host only
network adapter to which the container's ports will be exposed on.

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

# Further details

The Boot2Docker management tool provides some commands:

```
$ ./boot2docker
Usage: ./boot2docker [<options>]
{help|init|up|ssh|save|down|poweroff|reset|restart|config|status|info|delete|download|version}
[<args>]
```

Continue with the [User Guide](/userguide/).

For further information or to report issues, please see the [Boot2Docker site](http://boot2docker.io).
