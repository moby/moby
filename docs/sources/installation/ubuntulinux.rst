.. _ubuntu_linux:

Ubuntu Linux
============

  **Please note this project is currently under heavy development. It should not be used in production.**


Docker is now available as a Ubuntu PPA (Personal Package Archive),
`hosted on launchpad  <https://launchpad.net/~dotcloud/+archive/lxc-docker>`_
which makes installing Docker on Ubuntu very easy.

**The Requirements**

* Ubuntu 12.04 (LTS) or Ubuntu 12.10
* **64-bit Operating system**


Add the custom package sources to your apt sources list. Copy and paste both the following lines at once.

.. code-block:: bash

   sudo sh -c "echo 'deb http://ppa.launchpad.net/dotcloud/lxc-docker/ubuntu precise main' >> /etc/apt/sources.list"


Update your sources. You will see a warning that GPG signatures cannot be verified.

.. code-block:: bash

   sudo apt-get update


Now install it, you will see another warning that the package cannot be authenticated. Confirm install.

.. code-block:: bash

   sudo apt-get install lxc-docker


Verify it worked

.. code-block:: bash

   docker


**Done!**, now continue with the :ref:`hello_world` example.
