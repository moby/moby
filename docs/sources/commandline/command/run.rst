:title: Run Command
:description: Run a command in a new container
:keywords: run, container, docker, documentation 

===========================================
``run`` -- Run a command in a new container
===========================================

::

    Usage: docker run [OPTIONS] IMAGE[:TAG] [COMMAND] [ARG...]

    Run a command in a new container

      -a=map[]: Attach to stdin, stdout or stderr.
      -c=0: CPU shares (relative weight)
      -cidfile="": Write the container ID to the file
      -d=false: Detached mode: Run container in the background, print new container id
      -e=[]: Set environment variables
      -h="": Container host name
      -i=false: Keep stdin open even if not attached
      -privileged=false: Give extended privileges to this container
      -m=0: Memory limit (in bytes)
      -n=true: Enable networking for this container
      -p=[]: Map a network port to the container
      -t=false: Allocate a pseudo-tty
      -u="": Username or UID
      -d=[]: Set custom dns servers for the container
      -v=[]: Create a bind mount with: [host-dir]:[container-dir]:[rw|ro]. If "host-dir" is missing, then docker creates a new volume.
      -volumes-from="": Mount all volumes from the given container.
      -entrypoint="": Overwrite the default entrypoint set by the image.


Examples
--------

.. code-block:: bash

    docker run -cidfile /tmp/docker_test.cid ubuntu echo "test"

| This will create a container and print "test" to the console. The cidfile flag makes docker attempt to create a new file and write the container ID to it. If the file exists already, docker will return an error. Docker will close this file when docker run exits.

.. code-block:: bash

   docker run mount -t tmpfs none /var/spool/squid

| This will *not* work, because by default, most potentially dangerous kernel capabilities are dropped; including  ``cap_sys_admin`` (which is required to mount filesystems). However, the ``-privileged`` flag will allow it to run:

.. code-block:: bash

   docker run -privileged mount -t tmpfs none /var/spool/squid

| The ``-privileged`` flag gives *all* capabilities to the container, and it also lifts all the limitations enforced by the ``device`` cgroup controller. In other words, the container can then do almost everything that the host can do. This flag exists to allow special use-cases, like running Docker within Docker.
