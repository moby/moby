
Mac OS X and other linux
========================

  Please note this is a community contributed installation path. The only 'official' installation is using the :ref:`ubuntu_linux` installation path. This version
  may be out of date because it depends on some binaries to be updated and published


Requirements
------------

We currently rely on some Ubuntu-linux specific packages, this will change in the future, but for now we provide a
streamlined path to install Virtualbox with a Ubuntu 12.10 image using Vagrant.

1. Install virtualbox from https://www.virtualbox.org/ (or use your package manager)
2. Install vagrant from http://www.vagrantup.com/ (or use your package manager)
3. Install git if you had not installed it before, check if it is installed by running
   ``git`` in a terminal window

We recommend having at least about 2Gb of free disk space and 2Gb RAM (or more).

Installation
------------

1. Fetch the docker sources

.. code-block:: bash

   git clone https://github.com/dotcloud/docker.git

2. Run vagrant from the sources directory

.. code-block:: bash

    vagrant up

Vagrant will:

* Download the Quantal64 base ubuntu virtual machine image from get.docker.io/
* Boot this image in virtualbox

Then it will use Puppet to perform an initial setup in this machine:

* Download & untar the most recent docker binary tarball to vagrant homedir.
* Debootstrap to /var/lib/docker/images/ubuntu.
* Install & run dockerd as service.
* Put docker in /usr/local/bin.
* Put latest Go toolchain in /usr/local/go.

You now have a Ubuntu Virtual Machine running with docker pre-installed.

To access the VM and use Docker, Run ``vagrant ssh`` from the same directory as where you ran
``vagrant up``. Vagrant will make sure to connect you to the correct VM.

.. code-block:: bash

    vagrant ssh

Now you are in the VM, run docker

.. code-block:: bash

    docker


Continue with the :ref:`hello_world` example.
