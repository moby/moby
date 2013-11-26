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

Note that sometimes there is a disparity between the latest version and what's
in the overlay, and between the latest version in the overlay and what's in the
portage tree.  Please be patient, and the latest version should propagate
shortly.

Installation
^^^^^^^^^^^^

The package should properly pull in all the necessary dependencies and prompt
for all necessary kernel options.  The ebuilds for 0.7+ include use flags to
pull in the proper dependencies of the major storage drivers, with the
"device-mapper" use flag being enabled by default, since that is the simplest
installation path.

.. code-block:: bash

   sudo emerge -av app-emulation/docker

If any issues arise from this ebuild or the resulting binary, including and
especially missing kernel configuration flags and/or dependencies, `open an
issue on the docker-overlay repository
<https://github.com/tianon/docker-overlay/issues>`_ or ping tianon directly in
the #docker IRC channel on the freenode network.

Starting Docker
^^^^^^^^^^^^^^^

Ensure that you are running a kernel that includes all the necessary modules
and/or configuration for LXC (and optionally for device-mapper and/or AUFS,
depending on the storage driver you've decided to use).

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
