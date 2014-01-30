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
- :ref:`ubuntu_raring_saucy`

Please read :ref:`ufw`, if you plan to use `UFW (Uncomplicated
Firewall) <https://help.ubuntu.com/community/UFW>`_

.. _ubuntu_precise:

Ubuntu Precise 12.04 (LTS) (64-bit)
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

This installation path should work at all times.


Dependencies
------------

**Linux kernel 3.8**

Due to a bug in LXC, Docker works best on the 3.8 kernel. Precise
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

Docker is available as a Debian package, which makes installation
easy. **See the :ref:`installmirrors` section below if you are not in
the United States.** Other sources of the Debian packages may be
faster for you to install.

First add the Docker repository key to your local keychain.

.. code-block:: bash

   sudo apt-key adv --keyserver keyserver.ubuntu.com --recv-keys 36A1D7869245C8950F966E92D8576A8BA88D21E9

Add the Docker repository to your apt sources list, update and install the
``lxc-docker`` package.

*You may receive a warning that the package isn't trusted. Answer yes to
continue installation.*

.. code-block:: bash

   sudo sh -c "echo deb http://get.docker.io/ubuntu docker main\
   > /etc/apt/sources.list.d/docker.list"
   sudo apt-get update
   sudo apt-get install lxc-docker

.. note::

    There is also a simple ``curl`` script available to help with this process.

    .. code-block:: bash

        curl -s https://get.docker.io/ubuntu/ | sudo sh

Now verify that the installation has worked by downloading the ``ubuntu`` image
and launching a container.

.. code-block:: bash

   sudo docker run -i -t ubuntu /bin/bash

Type ``exit`` to exit

**Done!**, now continue with the :ref:`hello_world` example.

.. _ubuntu_raring_saucy:

Ubuntu Raring 13.04 and Saucy 13.10 (64 bit)
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

These instructions cover both Ubuntu Raring 13.04 and Saucy 13.10.

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

First add the Docker repository key to your local keychain.

.. code-block:: bash

   sudo apt-key adv --keyserver keyserver.ubuntu.com --recv-keys 36A1D7869245C8950F966E92D8576A8BA88D21E9

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


Giving non-root access
----------------------

The ``docker`` daemon always runs as the root user, and since Docker version
0.5.2, the ``docker`` daemon binds to a Unix socket instead of a TCP port. By
default that Unix socket is owned by the user *root*, and so, by default, you
can access it with ``sudo``.

Starting in version 0.5.3, if you (or your Docker installer) create a
Unix group called *docker* and add users to it, then the ``docker``
daemon will make the ownership of the Unix socket read/writable by the
*docker* group when the daemon starts. The ``docker`` daemon must
always run as the root user, but if you run the ``docker`` client as a user in
the *docker* group then you don't need to add ``sudo`` to all the
client commands.  

.. warning:: The *docker* group is root-equivalent.

**Example:**

.. code-block:: bash

  # Add the docker group if it doesn't already exist.
  sudo groupadd docker

  # Add the connected user "${USER}" to the docker group.
  # Change the user name to match your preferred user.
  # You may have to logout and log back in again for
  # this to take effect.
  sudo gpasswd -a ${USER} docker

  # Restart the Docker daemon.
  sudo service docker restart


Upgrade
--------

To install the latest version of docker, use the standard ``apt-get`` method:


.. code-block:: bash

   # update your sources list
   sudo apt-get update

   # install the latest
   sudo apt-get install lxc-docker

Troubleshooting
^^^^^^^^^^^^^^^

On Linux Mint, the ``cgroups-lite`` package is not installed by default.
Before Docker will work correctly, you will need to install this via:

.. code-block:: bash

    sudo apt-get update && sudo apt-get install cgroups-lite

.. _ufw:

Docker and UFW
^^^^^^^^^^^^^^

Docker uses a bridge to manage container networking. By default, UFW drops all
`forwarding` traffic. As a result you will need to enable UFW forwarding:

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

.. _installmirrors:

Mirrors
^^^^^^^

You should ``ping get.docker.io`` and compare the latency to the
following mirrors, and pick whichever one is best for you.

Yandex
------

`Yandex <http://yandex.ru/>`_ in Russia is mirroring the Docker Debian
packages, updating every 6 hours. Substitute
``http://mirror.yandex.ru/mirrors/docker/`` for
``http://get.docker.io/ubuntu`` in the instructions above. For example:

.. code-block:: bash

   sudo sh -c "echo deb http://mirror.yandex.ru/mirrors/docker/ docker main\
   > /etc/apt/sources.list.d/docker.list"
   sudo apt-get update
   sudo apt-get install lxc-docker
