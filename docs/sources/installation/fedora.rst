:title: Installation on Fedora
:description: Docker installation instructions and nuances for Fedora.
:keywords: fedora, virtualization, docker, documentation, installation

.. _fedora:

Fedora
======

.. include:: install_header.inc

Docker is available as an official Fedora package.

Installation
^^^^^^^^^^^^

To install docker on Fedora, simply do:

.. code-block:: bash

   sudo yum install docker-io

For the EPEL branch, lxc needs to be installed from the epel-testing repo.

.. code-bloack:: bash

   sudo yum install --enablerepo=epel-testing lxc

This is taken care of automatically for the other fedora versions.

Starting Docker
^^^^^^^^^^^^^^^

To start docker, do:

.. code-block:: bash

   sudo systemctl start docker

To start docker on system startup, do:

.. code-block:: bash

   sudo systemctl enable docker
