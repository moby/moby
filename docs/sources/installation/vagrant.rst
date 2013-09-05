:title: Using Vagrant (Mac, Linux)
:description: This guide will setup a new virtualbox virtual machine with docker installed on your computer.
:keywords: Docker, Docker documentation, virtualbox, vagrant, git, ssh, putty, cygwin

.. _install_using_vagrant:

Using Vagrant (Mac, Linux)
==========================

This guide will setup a new virtualbox virtual machine with docker
installed on your computer. This works on most operating systems,
including MacOX, Windows, Linux, FreeBSD and others. If you can
install these and have at least 400Mb RAM to spare you should be good.

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
   * Add the `Docker PPA sources <https://launchpad.net/~dotcloud/+archive/lxc-docker>`_ to /etc/apt/sources.lst
   * Update your sources
   * Install lxc-docker

   You now have a Ubuntu Virtual Machine running with docker pre-installed.

Connect
-------

To access the VM and use Docker, Run ``vagrant ssh`` from the same directory as where you ran
``vagrant up``. Vagrant will connect you to the correct VM.

.. code-block:: bash

   vagrant ssh

Run
-----

Now you are in the VM, run docker

.. code-block:: bash

   sudo docker


Continue with the :ref:`hello_world` example.
