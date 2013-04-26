.. _ubuntu_linux:

Ubuntu Linux
============

  **Please note this project is currently under heavy development. It should not be used in production.**


Right now, the officially supported distributions are:

- Ubuntu 12.04 (precise LTS) (64-bit)
- Ubuntu 12.10 (quantal) (64-bit)

Dependencies
------------

The linux-image-extra package is only needed on standard Ubuntu EC2 AMIs in order to install the aufs kernel module.

.. code-block:: bash

   sudo apt-get install linux-image-extra-`uname -r`


Installation
------------

Docker is available as a Ubuntu PPA (Personal Package Archive),
`hosted on launchpad  <https://launchpad.net/~dotcloud/+archive/lxc-docker>`_
which makes installing Docker on Ubuntu very easy.


Add the PPA.

.. code-block:: bash

   sudo add-apt-repository ppa:dotcloud/lxc-docker


Update your sources.

.. code-block:: bash

   sudo apt-get update


Now install it.

.. code-block:: bash

   sudo apt-get install lxc-docker


Verify it worked

.. code-block:: bash

   docker


**Done!**, now continue with the :ref:`hello_world` example.
