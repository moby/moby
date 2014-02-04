:title: Installation on FrugalWare
:description: Docker installation on FrugalWare.
:keywords: frugalware linux, virtualization, docker, documentation, installation

.. _frugalware:

FrugalWare
==========

.. include:: install_header.inc

.. include:: install_unofficial.inc

Installing on FrugalWare is handled via the official packages:

* `lxc-docker i686 <http://www.frugalware.org/packages/200141>`_

* `lxc-docker x86_64 <http://www.frugalware.org/packages/200130>`_

The `lxc-docker` package will install the latest tagged version of Docker. 

Dependencies
------------

Docker depends on several packages which are specified as dependencies in
the packages. The core dependencies are:

* systemd
* lvm2
* sqlite3
* libguestfs
* lxc
* iproute2 
* bridge-utils


Installation
------------

A simple
::

    pacman -S lxc-docker
    
is all that is needed.


Starting Docker
---------------

There is a systemd service unit created for Docker.  To start Docker as service:

::

    sudo systemctl start lxc-docker


To start on system boot:

::

    sudo systemctl enable lxc-docker
