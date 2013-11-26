:title: Requirements and Installation on Ubuntu Linux
:description: Please note this project is currently under heavy development. It should not be used in production.
:keywords: Docker, Docker documentation, requirements, virtualbox, vagrant, git, ssh, putty, cygwin, linux

.. _ubuntu_linux:

Ubuntu
======

.. warning::

   These instructions have changed for 0.6. If you are upgrading from
   an earlier version, you will need to follow them again.

.. include:: install_header.inc

Docker is supported on the following versions of Ubuntu:

- :ref:`ubuntu_precise`
- :ref:`ubuntu_raring`

Please read :ref:`ufw`, if you plan to use `UFW (Uncomplicated
Firewall) <https://help.ubuntu.com/community/UFW>`_

.. _ubuntu_precise:

Ubuntu Precise 12.04 (LTS) (64-bit)
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

This installation path should work at all times.


Dependencies
------------

**Linux kernel 3.8**

Due to a bug in LXC, docker works best on the 3.8 kernel. Precise
comes with a 3.2 kernel, so we need to upgrade it. The kernel you'll
install when following these steps comes with AUFS built in. We also
include the generic headers to enable packages that depend on them,
like ZFS and the VirtualBox guest additions. If you didn't install the
headers for your "precise" kernel, then you can skip these headers for
the "raring" kernel. But it is safer to include them if you're not
sure.


.. code-block:: bash

   # install the backported kernel
   sudo apt-get update
   sudo apt-get install linux-image-generic-lts-raring linux-headers-generic-lts-raring

   # reboot
   sudo reboot


Installation
------------

.. warning::

   These instructions have changed for 0.6. If you are upgrading from
   an earlier version, you will need to follow them again.

Docker is available as a Debian package, which makes installation easy.

First add the Docker repository key to your local keychain. You can use the
``apt-key`` command to check the fingerprint matches: ``36A1 D786 9245 C895 0F96
6E92 D857 6A8B A88D 21E9``

.. code-block:: bash

   sudo sh -c "wget -qO- https://get.docker.io/gpg | apt-key add -"

Add the Docker repository to your apt sources list, update and install the
``lxc-docker`` package. 

*You may receive a warning that the package isn't trusted. Answer yes to
continue installation.*

.. code-block:: bash

   sudo sh -c "echo deb http://get.docker.io/ubuntu docker main\
   > /etc/apt/sources.list.d/docker.list"
   sudo apt-get update
   sudo apt-get install lxc-docker

Now verify that the installation has worked by downloading the ``ubuntu`` image
and launching a container.

.. code-block:: bash

   sudo docker run -i -t ubuntu /bin/bash

Type ``exit`` to exit

**Done!**, now continue with the :ref:`hello_world` example.

.. _ubuntu_raring:

Ubuntu Raring 13.04 (64 bit)
^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Dependencies
------------

**Optional AUFS filesystem support**

Ubuntu Raring already comes with the 3.8 kernel, so we don't need to install it. However, not all systems
have AUFS filesystem support enabled. AUFS support is optional as of version 0.7, but it's still available as
a driver and we recommend using it if you can.

To make sure AUFS is installed, run the following commands:

.. code-block:: bash

   sudo apt-get update
   sudo apt-get install linux-image-extra-`uname -r`


Installation
------------

Docker is available as a Debian package, which makes installation easy.

.. warning::

    Please note that these instructions have changed for 0.6. If you are upgrading from an earlier version, you will need
    to follow them again.

First add the Docker repository key to your local keychain. You can use the
``apt-key`` command to check the fingerprint matches: ``36A1 D786 9245 C895 0F96
6E92 D857 6A8B A88D 21E9``

.. code-block:: bash

   sudo sh -c "wget -qO- https://get.docker.io/gpg | apt-key add -"

Add the Docker repository to your apt sources list, update and install the
``lxc-docker`` package.

.. code-block:: bash

   sudo sh -c "echo deb http://get.docker.io/ubuntu docker main\
   > /etc/apt/sources.list.d/docker.list"
   sudo apt-get update
   sudo apt-get install lxc-docker

Now verify that the installation has worked by downloading the ``ubuntu`` image
and launching a container.

.. code-block:: bash

   sudo docker run -i -t ubuntu /bin/bash

Type ``exit`` to exit

**Done!**, now continue with the :ref:`hello_world` example.


.. _ufw:

Docker and UFW
^^^^^^^^^^^^^^

Docker uses a bridge to manage container networking. By default, UFW drops all
`forwarding` traffic. As a result will you need to enable UFW forwarding:

.. code-block:: bash

   sudo nano /etc/default/ufw
   ----
   # Change:
   # DEFAULT_FORWARD_POLICY="DROP"
   # to
   DEFAULT_FORWARD_POLICY="ACCEPT"

Then reload UFW:

.. code-block:: bash

   sudo ufw reload


UFW's default set of rules denies all `incoming` traffic. If you want to be
able to reach your containers from another host then you should allow
incoming connections on the Docker port (default 4243):

.. code-block:: bash

   sudo ufw allow 4243/tcp

