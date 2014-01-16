:title: Installing Docker on a Mac
:description: Installing Docker on a Mac
:keywords: Docker, Docker documentation, virtualbox, git, ssh

.. _install_using_vagrant:

Installing Docker on a Mac
==========================

This guide explains how to install a full Docker setup on your Mac,
using Virtualbox and Vagrant.

Install Vagrant and Virtualbox
------------------------------

.. include:: install_header.inc

.. include:: install_unofficial.inc

#. Install virtualbox from https://www.virtualbox.org/ (or use your
   package manager)
#. Install vagrant from http://www.vagrantup.com/ (or use your package
   manager)
#. Install git if you had not installed it before, check if it is
   installed by running ``git`` in a terminal window


Spin it up
----------

1. Fetch the docker sources (this includes the ``Vagrantfile`` for
   machine setup).

   .. code-block:: bash

      git clone https://github.com/dotcloud/docker.git

2. Change directory to docker

   .. code-block:: bash

      cd docker

3. Run vagrant from the sources directory

   .. code-block:: bash

      vagrant up

   Vagrant will:

   * Download the 'official' Precise64 base ubuntu virtual machine image from vagrantup.com
   * Boot this image in virtualbox
   * Follow official :ref:`ubuntu_linux` installation path

   You now have a Ubuntu Virtual Machine running with docker pre-installed.

Connect
-------

To access the VM and use Docker, Run ``vagrant ssh`` from the same directory as where you ran
``vagrant up``. Vagrant will connect you to the correct VM.

.. code-block:: bash

   vagrant ssh


Upgrades
--------

Since your local VM is based on Ubuntu, you can upgrade docker by logging in to the
VM and calling ``apt-get``:


.. code-block:: bash

   # Log into the VM
   vagrant ssh

   # update your sources list
   sudo apt-get update

   # install the latest
   sudo apt-get install lxc-docker


Run
---

Now you are in the VM, run docker

.. code-block:: bash

   sudo docker


Continue with the :ref:`hello_world` example.
