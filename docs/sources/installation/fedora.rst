:title: Installation on Fedora and EPEL
:description: Docker installation instructions and nuances for Fedora.
:keywords: gentoo linux, virtualization, docker, documentation, installation

.. _fedora:

Gentoo
======

.. include:: install_header.inc

Docker is available as an official Fedora package for Fedora 19, 20 and EPEL6.

Installation
^^^^^^^^^^^^

To install docker on Fedora, simply do:

.. code-block:: bash

   sudo yum install docker-io


Starting Docker
^^^^^^^^^^^^^^^

To start docker, do:

.. code-block:: bash

   sudo systemctl start docker

To start docker on system startup, do:

.. code-block:: bash

   sudo systemctl enable docker
