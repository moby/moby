page_title: Installation on CRUX Linux
page_description: Docker installation on CRUX Linux.
page_keywords: crux linux, virtualization, Docker, documentation, installation

# CRUX Linux

Installing on CRUX Linux can be handled via the ports from [James
Mills](http://prologic.shortcircuit.net.au/) and are included in the
official [contrib](http://crux.nu/portdb/?a=repo&q=contrib) ports:

- docker
- docker-bin

The `docker` port will install the latest tagged
version of Docker. The `docker-bin` port will
install the latest tagged version of Docker from upstream built binaries.

## Installation

Assuming you have contrib enabled, update your ports tree and install docker (*as root*):

    # prt-get depinst docker

You can install `docker-bin` instead if you wish to avoid compilation time.


## Kernel Requirements

To have a working **CRUX+Docker** Host you must ensure your Kernel has
the necessary modules enabled for LXC containers to function correctly
and Docker Daemon to work properly.

Please read the `README`:

    $ prt-get readme docker

The `docker` and `docker-bin` ports install the `contrib/check-config.sh`
script provided by the Docker contributors for checking your kernel
configuration as a suitable Docker Host.

    $ /usr/share/docker/check-config.sh

## Starting Docker

There is a rc script created for Docker. To start the Docker service (*as root*):

    # /etc/rc.d/docker start

To start on system boot:

 - Edit `/etc/rc.conf`
 - Put `docker` into the `SERVICES=(...)` array after `net`.

## Issues

If you have any issues please file a bug with the
[CRUX Bug Tracker](http://crux.nu/bugs/).

## Support

For support contact the [CRUX Mailing List](http://crux.nu/Main/MailingLists)
or join CRUX's [IRC Channels](http://crux.nu/Main/IrcChannels). on the
[FreeNode](http://freenode.net/) IRC Network.
