:title: Create a Base Image
:description: How to create base images
:keywords: Examples, Usage, base image, docker, documentation, examples

.. _base_image_creation:

Create a Base Image
===================

So you want to create your own :ref:`base_image_def`? Great!

The specific process will depend heavily on the Linux distribution you
want to package. We have some examples below, and you are encouraged
to submit pull requests to contribute new ones.

Getting Started
...............

In general, you'll want to start with a working machine that is
running the distribution you'd like to package as a base image, though
that is not required for some tools like Debian's `Debootstrap
<https://wiki.debian.org/Debootstrap>`_, which you can also use to
build Ubuntu images.

It can be as simple as this to create an Ubuntu base image::

  $ sudo debootstrap raring raring > /dev/null
  $ sudo tar -C raring -c . | sudo docker import - raring
  a29c15f1bf7a
  $ sudo docker run raring cat /etc/lsb-release
  DISTRIB_ID=Ubuntu
  DISTRIB_RELEASE=13.04
  DISTRIB_CODENAME=raring
  DISTRIB_DESCRIPTION="Ubuntu 13.04"

There are more example scripts for creating base images in the
Docker Github Repo:

* `BusyBox <https://github.com/dotcloud/docker/blob/master/contrib/mkimage-busybox.sh>`_
* `CentOS / Scientific Linux CERN (SLC)
  <https://github.com/dotcloud/docker/blob/master/contrib/mkimage-rinse.sh>`_
* `Debian / Ubuntu
  <https://github.com/dotcloud/docker/blob/master/contrib/mkimage-debootstrap.sh>`_
