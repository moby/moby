:title: Installation on Arch Linux
:description: Docker installation on Arch Linux.
:keywords: arch linux, virtualization, docker, documentation, installation

.. _arch_linux:

Arch Linux
==========

.. include:: install_header.inc

.. include:: install_unofficial.inc

Installing on Arch Linux can be handled via the package in community:

* `docker <https://www.archlinux.org/packages/community/x86_64/docker/>`_

or the following AUR package:

* `docker-git <https://aur.archlinux.org/packages/docker-git/>`_

The docker package will install the latest tagged version of docker. 
The docker-git package will build from the current master branch.

Dependencies
------------

Docker depends on several packages which are specified as dependencies in
the packages. The core dependencies are:

* bridge-utils
* device-mapper
* iproute2
* lxc
* sqlite


Installation
------------

For the normal package a simple
::

    pacman -S docker
    
is all that is needed.

For the AUR package execute:
::

    yaourt -S docker-git
    
The instructions here assume **yaourt** is installed.  See 
`Arch User Repository <https://wiki.archlinux.org/index.php/Arch_User_Repository#Installing_packages>`_
for information on building and installing packages from the AUR if you have not
done so before.


Starting Docker
---------------

There is a systemd service unit created for docker.  To start the docker service:

::

    sudo systemctl start docker


To start on system boot:

::

    sudo systemctl enable docker
    
Network Configuration
---------------------

IPv4 packet forwarding is disabled by default on Arch, so internet access from inside
the container may not work.

To enable the forwarding, run as root on the host system:

::

    sysctl net.ipv4.ip_forward=1
    
And, to make it persistent across reboots, enable it on the host's **/etc/sysctl.d/docker.conf**:

::

    net.ipv4.ip_forward=1
