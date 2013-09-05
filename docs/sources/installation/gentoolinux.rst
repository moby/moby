:title: Installation on Gentoo Linux
:description: Docker installation instructions and nuances for Gentoo Linux.
:keywords: gentoo linux, virtualization, docker, documentation, installation

.. _gentoo_linux:

Gentoo Linux
============

.. include:: install_header.inc

.. include:: install_unofficial.inc

Installing Docker on Gentoo Linux can be accomplished by using the overlay
provided at https://github.com/tianon/docker-overlay.  The most up-to-date
documentation for properly installing the overlay can be found in the overlay
README.  The information here is provided for reference, and may be out of date.

Installation
^^^^^^^^^^^^

Ensure that layman is installed:

.. code-block:: bash

   sudo emerge -av app-portage/layman

Using your favorite editor, add
``https://raw.github.com/tianon/docker-overlay/master/repositories.xml`` to the
``overlays`` section in ``/etc/layman/layman.cfg`` (as per instructions on the
`Gentoo Wiki <http://wiki.gentoo.org/wiki/Layman#Adding_custom_overlays>`_),
then invoke the following:

.. code-block:: bash

   sudo layman -f -a docker

Once that completes, the ``app-emulation/docker`` package will be available
for emerge:

.. code-block:: bash

   sudo emerge -av app-emulation/docker

If you prefer to use the official binaries, or just do not wish to compile
docker, emerge ``app-emulation/docker-bin`` instead.  It is important to
remember that Gentoo is still an unsupported platform, even when using the
official binaries.

The package should already include all the necessary dependencies.  For the
simplest installation experience, use ``sys-kernel/aufs-sources`` directly as
your kernel sources.  If you prefer not to use ``sys-kernel/aufs-sources``, the
portage tree also contains ``sys-fs/aufs3``, which contains the patches
necessary for adding AUFS support to other kernel source packages (and a
``kernel-patch`` use flag to perform the patching automatically).

Between ``app-emulation/lxc`` and ``app-emulation/docker``, all the
necessary kernel configuration flags should be checked for and warned about in
the standard manner.

If any issues arise from this ebuild or the resulting binary, including and
especially missing kernel configuration flags and/or dependencies, `open an
issue <https://github.com/tianon/docker-overlay/issues>`_ on the docker-overlay
repository or ping tianon in the #docker IRC channel.

Starting Docker
^^^^^^^^^^^^^^^

Ensure that you are running a kernel that includes the necessary AUFS support
and includes all the necessary modules and/or configuration for LXC.

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

fork/exec /usr/sbin/lxc-start: operation not permitted
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Unfortunately, Gentoo suffers from `issue #1422
<https://github.com/dotcloud/docker/issues/1422>`_, meaning that after every
fresh start of docker, the first docker run fails due to some tricky terminal
issues, so be sure to run something trivial (such as ``docker run -i -t busybox
echo hi``) before attempting to run anything important.
