:title: Building Blocks
:description: An introduction to docker and standard containers?
:keywords: containers, lxc, concepts, explanation


Building blocks
===============

.. _images:

Images
------
An original container image. These are stored on disk and are comparable with what you normally expect from a stopped virtual machine image. Images are stored (and retrieved from) repository

Images are stored on your local file system under /var/lib/docker/graph


.. _containers:

Containers
----------
A container is a local version of an image. It can be running or stopped, The equivalent would be a virtual machine instance.

Containers are stored on your local file system under /var/lib/docker/containers

