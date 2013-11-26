:title: Requirements and Installation on Fedora
:description: Please note this project is currently under heavy development. It should not be used in production.
:keywords: Docker, Docker documentation, requirements, virtualbox, vagrant, git, ssh, putty, cygwin, linux

.. _fedora:

Fedora
======

.. include:: install_header.inc

Right now, the officially supported distribution are:

- :ref:`fedora_core19`

Docker has the following dependencies

* Linux kernel 3.8 or later 
* Device Mapper

.. _fedora_core19:

Fedora Core 19 (64-bit)
^^^^^^^^^^^^^^^^^^^^^^^

This installation path should work at all times.

Dependencies
------------

**Linux kernel 3.8**

Due to a bug in LXC, docker works best on the 3.8 kernel or later. Fedora Core
19 ships with 3.10 so you shouldn't need to update anything. You can confirm 
the running kernel version with the ``uname`` command.

.. code-block:: bash

    uname -a
    Linux fedora-19.example.com 3.10.4-300.fc19.x86_64 #1 SMP Tue Jul 30 11:29:05 UTC 2013 x86_64 x86_64 x86_64 GNU/Linux

**Device Mapper**

Fedora Core 19 and later should also ship with Device Mapper that provides
Docker's filesystem layering.  You can confirm Device Mapper is install by
checking for the ``sys/class/misc/device-mapper`` file.

.. code-block:: bash

    ls -l /sys/class/misc/device-mapper
    lrwxrwxrwx. 1 root root 0 Nov 26 03:00 /sys/class/misc/device-mapper -> ../../devices/virtual/misc/device-mapper

If the file is present then Device Mapper is available. If the file is not
present you can try to install Device Mapper.

.. code-block:: bash

    sudo yum -y install device-mapper

Installation
------------

Firstly, let's make sure our Fedora host is up-to-date.

.. code-block:: bash

    sudo yum -y upgrade

Next let's install the ``docker-io`` package which will install Docker on our host.

.. code-block:: bash

   sudo yum -y install docker-io

Now let's verify that it worked

.. code-block:: bash

   sudo docker run -i -t ubuntu /bin/bash

**Done!**, now continue with the :ref:`hello_world` example.

