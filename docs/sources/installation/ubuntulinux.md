page_title: Installation on Ubuntu
page_description: Instructions for installing Docker on Ubuntu.
page_keywords: Docker, Docker documentation, requirements, virtualbox, installation, ubuntu

# Ubuntu

Docker is supported on the following versions of Ubuntu:

 - [*Ubuntu Trusty 14.04 (LTS) (64-bit)*](#ubuntu-trusty-1404-lts-64-bit)
 - [*Ubuntu Precise 12.04 (LTS) (64-bit)*](#ubuntu-precise-1204-lts-64-bit)
 - [*Ubuntu Raring 13.04 and Saucy 13.10 (64
   bit)*](#ubuntu-raring-1304-and-saucy-1310-64-bit)

Please read [*Docker and UFW*](#docker-and-ufw), if you plan to use [UFW
(Uncomplicated Firewall)](https://help.ubuntu.com/community/UFW)

## Ubuntu Trusty 14.04 (LTS) (64-bit)

Ubuntu Trusty comes with a 3.13.0 Linux kernel, and a `docker.io` package which
installs all its prerequisites from Ubuntu's repository.

> **Note**:
> Ubuntu (and Debian) contain a much older KDE3/GNOME2 package called ``docker``, so the
> package and the executable are called ``docker.io``.

### Installation

To install the latest Ubuntu package (may not be the latest Docker release):

    $ sudo apt-get update
    $ sudo apt-get install docker.io
    $ sudo ln -sf /usr/bin/docker.io /usr/local/bin/docker
    $ sudo sed -i '$acomplete -F _docker docker' /etc/bash_completion.d/docker.io

To verify that everything has worked as expected:

    $ sudo docker run -i -t ubuntu /bin/bash

Which should download the `ubuntu` image, and then start `bash` in a container.


## Ubuntu Precise 12.04 (LTS) (64-bit)

This installation path should work at all times.

### Dependencies

**Linux kernel 3.8**

Due to a bug in LXC, Docker works best on the 3.8 kernel. Precise comes
with a 3.2 kernel, so we need to upgrade it. The kernel you'll install
when following these steps comes with AUFS built in. We also include the
generic headers to enable packages that depend on them, like ZFS and the
VirtualBox guest additions. If you didn't install the headers for your
"precise" kernel, then you can skip these headers for the "raring"
kernel. But it is safer to include them if you're not sure.

    # install the backported kernel
    $ sudo apt-get update
    $ sudo apt-get install linux-image-generic-lts-raring linux-headers-generic-lts-raring

    # reboot
    $ sudo reboot

### Installation

> **Warning**: 
> These instructions have changed for 0.6. If you are upgrading from an
> earlier version, you will need to follow them again.

Docker is available as a Debian package, which makes installation easy.
**See the** [*Mirrors*](#mirrors) **section below if you are not
in the United States.** Other sources of the Debian packages may be
faster for you to install.

First, check that your APT system can deal with `https`
URLs: the file `/usr/lib/apt/methods/https`
should exist. If it doesn't, you need to install the package
`apt-transport-https`.

    [ -e /usr/lib/apt/methods/https ] || {
      apt-get update
      apt-get install apt-transport-https
    }

Then, add the Docker repository key to your local keychain.

    $ sudo apt-key adv --keyserver hkp://keyserver.ubuntu.com:80 --recv-keys 36A1D7869245C8950F966E92D8576A8BA88D21E9

Add the Docker repository to your apt sources list, update and install
the `lxc-docker` package.

*You may receive a warning that the package isn't trusted. Answer yes to
continue installation.*

    $ sudo sh -c "echo deb https://get.docker.io/ubuntu docker main\
    > /etc/apt/sources.list.d/docker.list"
    $ sudo apt-get update
    $ sudo apt-get install lxc-docker

> **Note**:
> 
> There is also a simple `curl` script available to help with this process.
> 
>     $ curl -s https://get.docker.io/ubuntu/ | sudo sh

Now verify that the installation has worked by downloading the
`ubuntu` image and launching a container.

    $ sudo docker run -i -t ubuntu /bin/bash

Type `exit` to exit

**Done!**, continue with the [User Guide](/userguide/).

## Ubuntu Raring 13.04 and Saucy 13.10 (64 bit)

These instructions cover both Ubuntu Raring 13.04 and Saucy 13.10.

### Dependencies

**Optional AUFS filesystem support**

Ubuntu Raring already comes with the 3.8 kernel, so we don't need to
install it. However, not all systems have AUFS filesystem support
enabled. AUFS support is optional as of version 0.7, but it's still
available as a driver and we recommend using it if you can.

To make sure AUFS is installed, run the following commands:

    $ sudo apt-get update
    $ sudo apt-get install linux-image-extra-`uname -r`

### Installation

Docker is available as a Debian package, which makes installation easy.

> **Warning**: 
> Please note that these instructions have changed for 0.6. If you are
> upgrading from an earlier version, you will need to follow them again.

First add the Docker repository key to your local keychain.

    $ sudo apt-key adv --keyserver hkp://keyserver.ubuntu.com:80 --recv-keys 36A1D7869245C8950F966E92D8576A8BA88D21E9

Add the Docker repository to your apt sources list, update and install
the `lxc-docker` package.

    $ sudo sh -c "echo deb http://get.docker.io/ubuntu docker main\
    > /etc/apt/sources.list.d/docker.list"
    $ sudo apt-get update
    $ sudo apt-get install lxc-docker

Now verify that the installation has worked by downloading the
`ubuntu` image and launching a container.

    $ sudo docker run -i -t ubuntu /bin/bash

Type `exit` to exit

**Done!**, now continue with the [User Guide](/userguide/).

### Giving non-root access

The `docker` daemon always runs as the `root` user, and since Docker
version 0.5.2, the `docker` daemon binds to a Unix socket instead of a
TCP port. By default that Unix socket is owned by the user `root`, and
so, by default, you can access it with `sudo`.

Starting in version 0.5.3, if you (or your Docker installer) create a
Unix group called `docker` and add users to it, then the `docker` daemon
will make the ownership of the Unix socket read/writable by the `docker`
group when the daemon starts. The `docker` daemon must always run as the
`root` user, but if you run the `docker` client as a user in the
`docker` group then you don't need to add `sudo` to all the client
commands.  From Docker 0.9.0 you can use the `-G` flag to specify an
alternative group.

> **Warning**: 
> The `docker` group (or the group specified with the `-G` flag) is
> `root`-equivalent; see [*Docker Daemon Attack Surface*](
> /articles/security/#dockersecurity-daemon) details.

**Example:**

    # Add the docker group if it doesn't already exist.
    $ sudo groupadd docker

    # Add the connected user "${USER}" to the docker group.
    # Change the user name to match your preferred user.
    # You may have to logout and log back in again for
    # this to take effect.
    $ sudo gpasswd -a ${USER} docker

    # Restart the Docker daemon.
    # If you are in Ubuntu 14.04, use docker.io instead of docker
    $ sudo service docker restart

### Upgrade

To install the latest version of docker, use the standard
`apt-get` method:

    # update your sources list
    $ sudo apt-get update

    # install the latest
    $ sudo apt-get install lxc-docker

## Memory and Swap Accounting

If you want to enable memory and swap accounting, you must add the
following command-line parameters to your kernel:

    $ cgroup_enable=memory swapaccount=1

On systems using GRUB (which is the default for Ubuntu), you can add
those parameters by editing `/etc/default/grub` and
extending `GRUB_CMDLINE_LINUX`. Look for the
following line:

    $ GRUB_CMDLINE_LINUX=""

And replace it by the following one:

    $ GRUB_CMDLINE_LINUX="cgroup_enable=memory swapaccount=1"

Then run `sudo update-grub`, and reboot.

These parameters will help you get rid of the following warnings:

    WARNING: Your kernel does not support cgroup swap limit.
    WARNING: Your kernel does not support swap limit capabilities. Limitation discarded.

## Troubleshooting

On Linux Mint, the `cgroup-lite` package is not
installed by default. Before Docker will work correctly, you will need
to install this via:

    $ sudo apt-get update && sudo apt-get install cgroup-lite

## Docker and UFW

Docker uses a bridge to manage container networking. By default, UFW
drops all forwarding traffic. As a result you will need to enable UFW
forwarding:

    $ sudo nano /etc/default/ufw

    # Change:
    # DEFAULT_FORWARD_POLICY="DROP"
    # to
    $ DEFAULT_FORWARD_POLICY="ACCEPT"

Then reload UFW:

    $ sudo ufw reload

UFW's default set of rules denies all incoming traffic. If you want to
be able to reach your containers from another host then you should allow
incoming connections on the Docker port (default 2375):

    $ sudo ufw allow 2375/tcp

## Docker and local DNS server warnings

Systems which are running Ubuntu or an Ubuntu derivative on the desktop
will use 127.0.0.1 as the default nameserver in /etc/resolv.conf.
NetworkManager sets up dnsmasq to use the real DNS servers of the
connection and sets up nameserver 127.0.0.1 in /etc/resolv.conf.

When starting containers on these desktop machines, users will see a
warning:

    WARNING: Local (127.0.0.1) DNS resolver found in resolv.conf and containers can't use it. Using default external servers : [8.8.8.8 8.8.4.4]

This warning is shown because the containers can't use the local DNS
nameserver and Docker will default to using an external nameserver.

This can be worked around by specifying a DNS server to be used by the
Docker daemon for the containers:

    $ sudo nano /etc/default/docker
    ---
    # Add:
    $ docker_OPTS="--dns 8.8.8.8"
    # 8.8.8.8 could be replaced with a local DNS server, such as 192.168.1.1
    # multiple DNS servers can be specified: --dns 8.8.8.8 --dns 192.168.1.1

The Docker daemon has to be restarted:

    $ sudo restart docker

> **Warning**: 
> If you're doing this on a laptop which connects to various networks,
> make sure to choose a public DNS server.

An alternative solution involves disabling dnsmasq in NetworkManager by
following these steps:

    $ sudo nano /etc/NetworkManager/NetworkManager.conf
    ----
    # Change:
    dns=dnsmasq
    # to
    #dns=dnsmasq

NetworkManager and Docker need to be restarted afterwards:

    $ sudo restart network-manager
    $ sudo restart docker

> **Warning**: This might make DNS resolution slower on some networks.

## Mirrors

You should `ping get.docker.io` and compare the
latency to the following mirrors, and pick whichever one is best for
you.

### Yandex

[Yandex](http://yandex.ru/) in Russia is mirroring the Docker Debian
packages, updating every 6 hours.
Substitute `http://mirror.yandex.ru/mirrors/docker/` for
`http://get.docker.io/ubuntu` in the instructions above.
For example:

    $ sudo sh -c "echo deb http://mirror.yandex.ru/mirrors/docker/ docker main\
    > /etc/apt/sources.list.d/docker.list"
    $ sudo apt-get update
    $ sudo apt-get install lxc-docker
