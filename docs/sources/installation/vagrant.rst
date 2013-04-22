
.. _install_using_vagrant:

Install using Vagrant
=====================

  Please note this is a community contributed installation path. The only 'official' installation is using the
  :ref:`ubuntu_linux` installation path. This version may sometimes be out of date.

**requirements**
This guide will setup a new virtual machine with docker installed on your computer. This works on most operating
systems, including MacOX, Windows, Linux, FreeBSD and others. If you can install these and have at least 400Mb RAM
to spare you should be good.


Install Vagrant and Virtualbox
------------------------------

1. Install virtualbox from https://www.virtualbox.org/ (or use your package manager)
2. Install vagrant from http://www.vagrantup.com/ (or use your package manager)
3. Install git if you had not installed it before, check if it is installed by running
   ``git`` in a terminal window


Spin up your machine
--------------------

1. Fetch the docker sources (this includes the instructions for machine setup).

.. code-block:: bash

   git clone https://github.com/dotcloud/docker.git

2. Run vagrant from the sources directory

.. code-block:: bash

    vagrant up

Vagrant will:

* Download the 'official' Precise64 base ubuntu virtual machine image from vagrantup.com
* Boot this image in virtualbox
* Add the `Docker PPA sources <https://launchpad.net/~dotcloud/+archive/lxc-docker>`_ to /etc/apt/sources.lst
* Update your sources
* Install lxc-docker

You now have a Ubuntu Virtual Machine running with docker pre-installed.

To access the VM and use Docker, Run ``vagrant ssh`` from the same directory as where you ran
``vagrant up``. Vagrant will connect you to the correct VM.

.. code-block:: bash

    vagrant ssh

Now you are in the VM, run docker

.. code-block:: bash

    docker

Continue with the :ref:`hello_world` example.
