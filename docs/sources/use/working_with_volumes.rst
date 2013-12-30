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
features for persistent or shared data:

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

Using data volumes is as simple as adding a ``-v`` parameter to the ``docker run`` 
command. The ``-v`` parameter can be used more than once in order to 
create more volumes within the new container. To create a new container with 
two new volumes::

  $ docker run -v /var/volume1 -v /var/volume2 busybox true

This command will create the new container with two new volumes that 
exits instantly (``true`` is pretty much the smallest, simplest program 
that you can run). Once created you can mount its volumes in any other 
container using the ``-volumes-from`` option; irrespecive of whether the
container is running or not. 

Or, you can use the VOLUME instruction in a Dockerfile to add one or more new
volumes to any container created from that image::

  # BUILD-USING:        docker build -t data .
  # RUN-USING:          docker run -name DATA data 
  FROM          busybox
  VOLUME        ["/var/volume1", "/var/volume2"]
  CMD           ["/usr/bin/true"]

Creating and mounting a Data Volume Container
---------------------------------------------

If you have some persistent data that you want to share between containers, 
or want to use from non-persistent containers, its best to create a named
Data Volume Container, and then to mount the data from it.

Create a named container with volumes to share (``/var/volume1`` and ``/var/volume2``)::

  $ docker run -v /var/volume1 -v /var/volume2 -name DATA busybox true

Then mount those data volumes into your application containers::

  $ docker run -t -i -rm -volumes-from DATA -name client1 ubuntu bash

You can use multiple ``-volumes-from`` parameters to bring together multiple 
data volumes from multiple containers. 

Interestingly, you can mount the volumes that came from the ``DATA`` container in 
yet another container via the ``client1`` middleman container::

  $ docker run -t -i -rm -volumes-from client1 ubuntu -name client2 bash

This allows you to abstract the actual data source from users of that data, 
similar to :ref:`ambassador_pattern_linking <ambassador_pattern_linking>`.

If you remove containers that mount volumes, including the initial DATA container, 
or the middleman, the volumes will not be deleted until there are no containers still
referencing those volumes. This allows you to upgrade, or effectivly migrate data volumes
between containers.

Mount a Host Directory as a Container Volume:
---------------------------------------------

::

  -v=[]: Create a bind mount with: [host-dir]:[container-dir]:[rw|ro].
  If "host-dir" is missing, then docker creates a new volume.

This is not available from a Dockerfile as it makes the built image less portable
or shareable. [host-dir] volumes are 100% host dependent and will break on any 
other machine.

For example::

  sudo docker run -v /var/logs:/var/host_logs:ro ubuntu bash

The command above mounts the host directory ``/var/logs`` into the
container with read only permissions as ``/var/host_logs``.

.. versionadded:: v0.5.0

Known Issues
............

* :issue:`2702`: "lxc-start: Permission denied - failed to mount"
  could indicate a permissions problem with AppArmor. Please see the
  issue for a workaround.
* :issue:`2528`:  the busybox container is used to make the resulting container as small and
  simple as possible - whenever you need to interact with the data in the volume
  you mount it into another container.
