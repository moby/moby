:title: Share Directories via Volumes
:description: How to create and share volumes
:keywords: Examples, Usage, volume, docker, documentation, examples

.. _volume_def:

Share Directories via Volumes
=============================

.. versionadded:: v0.3.0
   Data volumes have been available since version 1 of the
   :doc:`../api/docker_remote_api`

A *data volume* is a specially-designated directory within one or more
containers that bypasses the :ref:`ufs_def` to provide several useful
features for persistant or shared data:

* **Data volumes can be shared and reused between containers.** This
  is the feature that makes data volumes so powerful. You can use it
  for anything from hot database upgrades to custom backup or
  replication tools. See the example below.
* **Changes to a data volume are made directly**, without the overhead
  of a copy-on-write mechanism. This is good for very large files.
* **Changes to a data volume will not be included at the next commit**
  because they are not recorded as regular filesystem changes in the
  top layer of the :ref:`ufs_def`

Each container can have zero or more data volumes.

Getting Started
...............

Using data volumes is as simple as adding a new flag: ``-v``. The
parameter ``-v`` can be used more than once in order to create more
volumes within the new container. The example below shows the
instruction to create a container with two new volumes::

  docker run -v /var/volume1 -v /var/volume2 shykes/couchdb

For a Dockerfile, the VOLUME instruction will add one or more new
volumes to any container created from the image::

  VOLUME ["/var/volume1", "/var/volume2"]


Mount Volumes from an Existing Container:
-----------------------------------------

The command below creates a new container which is runnning as daemon
``-d`` and with one volume ``/var/lib/couchdb``::

  COUCH1=$(sudo docker run -d -v /var/lib/couchdb shykes/couchdb:2013-05-03)

From the container id of that previous container ``$COUCH1`` it's
possible to create new container sharing the same volume using the
parameter ``-volumes-from container_id``::

  COUCH2=$(sudo docker run -d -volumes-from $COUCH1 shykes/couchdb:2013-05-03)

Now, the second container has the all the information from the first volume.


Mount a Host Directory as a Container Volume:
---------------------------------------------

::

  -v=[]: Create a bind mount with: [host-dir]:[container-dir]:[rw|ro].
  If "host-dir" is missing, then docker creates a new volume.

This is not available for a Dockerfile due the portability and sharing
purpose of it. The [host-dir] volumes is something 100% host dependent
and will break on any other machine.

For example::

  sudo docker run -v /var/logs:/var/host_logs:ro shykes/couchdb:2013-05-03

The command above mounts the host directory ``/var/logs`` into the
container with read only permissions as ``/var/host_logs``.

.. versionadded:: v0.5.0

Known Issues
............

* :issue:`2702`: "lxc-start: Permission denied - failed to mount"
  could indicate a permissions problem with AppArmor. Please see the
  issue for a workaround.
