FrugalWare[¶](#frugalware "Permalink to this headline")
=======================================================

Note

Docker is still under heavy development! We don’t recommend using it in
production yet, but we’re getting closer with each release. Please see
our blog post, [“Getting to Docker
1.0”](http://blog.docker.io/2013/08/getting-to-docker-1-0/)

Note

This is a community contributed installation path. The only ‘official’
installation is using the [*Ubuntu*](../ubuntulinux/#ubuntu-linux)
installation path. This version may be out of date because it depends on
some binaries to be updated and published

Installing on FrugalWare is handled via the official packages:

-   [lxc-docker i686](http://www.frugalware.org/packages/200141)
-   [lxc-docker x86\_64](http://www.frugalware.org/packages/200130)

The lxc-docker package will install the latest tagged version of Docker.

Dependencies[¶](#dependencies "Permalink to this headline")
-----------------------------------------------------------

Docker depends on several packages which are specified as dependencies
in the packages. The core dependencies are:

-   systemd
-   lvm2
-   sqlite3
-   libguestfs
-   lxc
-   iproute2
-   bridge-utils

Installation[¶](#installation "Permalink to this headline")
-----------------------------------------------------------

A simple

    pacman -S lxc-docker

is all that is needed.

Starting Docker[¶](#starting-docker "Permalink to this headline")
-----------------------------------------------------------------

There is a systemd service unit created for Docker. To start Docker as
service:

    sudo systemctl start lxc-docker

To start on system boot:

    sudo systemctl enable lxc-docker
