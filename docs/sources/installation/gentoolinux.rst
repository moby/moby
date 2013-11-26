:title: Installation on Gentoo Linux
:description: Docker installation instructions and nuances for Gentoo Linux.
:keywords: gentoo linux, virtualization, docker, documentation, installation

.. _gentoo_linux:

Gentoo
======

.. include:: install_header.inc

.. include:: install_unofficial.inc

Installing Docker on Gentoo Linux can be accomplished using one of two methods.
The first and best way if you're looking for a stable experience is to use the
official `app-emulation/docker` package directly in the portage tree.

If you're looking for a ``-bin`` ebuild, a live ebuild, or bleeding edge
ebuild changes/fixes, the second installation method is to use the overlay
provided at https://github.com/tianon/docker-overlay which can be added using
``app-portage/layman``. The most accurate and up-to-date documentation for
properly installing and using the overlay can be found in `the overlay README
<https://github.com/tianon/docker-overlay/blob/master/README.md#using-this-overlay>`_.

Installation
^^^^^^^^^^^^

The package should properly pull in all the necessary dependencies and prompt
for all necessary kernel options.  For the most straightforward installation
experience, use ``sys-kernel/aufs-sources`` as your kernel sources.  If you
prefer not to use ``sys-kernel/aufs-sources``, the portage tree also contains
``sys-fs/aufs3``, which includes the patches necessary for adding AUFS support
to other kernel source packages such as ``sys-kernel/gentoo-sources`` (and a
``kernel-patch`` USE flag to perform the patching to ``/usr/src/linux``
automatically).

.. code-block:: bash

   sudo emerge -av app-emulation/docker

If any issues arise from this ebuild or the resulting binary, including and
especially missing kernel configuration flags and/or dependencies, `open an
issue on the docker-overlay repository
<https://github.com/tianon/docker-overlay/issues>`_ or ping tianon directly in
the #docker IRC channel on the freenode network.

Starting Docker
^^^^^^^^^^^^^^^

Ensure that you are running a kernel that includes the necessary AUFS
patches/support and includes all the necessary modules and/or configuration for
LXC.

OpenRC
------

To start the docker daemon:

.. code-block:: bash

   sudo /etc/init.d/docker start

To start on system boot:

.. code-block:: bash

   sudo rc-update add docker default

systemd
-------

To start the docker daemon:

.. code-block:: bash

   sudo systemctl start docker.service

To start on system boot:

.. code-block:: bash

   sudo systemctl enable docker.service

Network Configuration
^^^^^^^^^^^^^^^^^^^^^

IPv4 packet forwarding is disabled by default, so internet access from inside
the container will not work unless ``net.ipv4.ip_forward`` is enabled:

.. code-block:: bash

   sudo sysctl -w net.ipv4.ip_forward=1

Or, to enable it more permanently:

.. code-block:: bash

   echo net.ipv4.ip_forward = 1 | sudo tee /etc/sysctl.d/docker.conf
