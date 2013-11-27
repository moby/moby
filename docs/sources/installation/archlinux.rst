:title: Installation on Arch Linux
:description: Docker installation on Arch Linux.
:keywords: arch linux, virtualization, docker, documentation, installation

.. _arch_linux:

Arch Linux
==========

.. include:: install_header.inc

.. include:: install_unofficial.inc

Installing on Arch Linux is not officially supported but can be handled via 
one of the following AUR packages:

* `lxc-docker <https://aur.archlinux.org/packages/lxc-docker/>`_
* `lxc-docker-git <https://aur.archlinux.org/packages/lxc-docker-git/>`_
* `lxc-docker-nightly <https://aur.archlinux.org/packages/lxc-docker-nightly/>`_

The lxc-docker package will install the latest tagged version of docker. 
The lxc-docker-git package will build from the current master branch.
The lxc-docker-nightly package will install the latest build.

Dependencies
------------

Docker depends on several packages which are specified as dependencies in
the AUR packages. The core dependencies are:

* bridge-utils
* device-mapper
* iproute2
* lxc


Installation
------------

The instructions here assume **yaourt** is installed.  See 
`Arch User Repository <https://wiki.archlinux.org/index.php/Arch_User_Repository#Installing_packages>`_
for information on building and installing packages from the AUR if you have not
done so before.

::

    yaourt -S lxc-docker


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
