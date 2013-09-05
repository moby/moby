:title: Run Command
:description: Run a command in a new container
:keywords: run, container, docker, documentation 

===========================================
``run`` -- Run a command in a new container
===========================================

::

   Usage: docker run [OPTIONS] IMAGE [COMMAND] [ARG...]

   Run a command in a new container

   OPTIONS:
    -a, --attach=value
                       Attach to stdin, stdout or stderr.
        --cidfile=value
                       Write the container ID to the file
    -c, --cpu=value    CPU shares (relative weight)
    -d, --detached     Detached mode: Run container in the background, print new
		       container id
        --dns=value    Set custom dns servers
        --entrypoint=value
                       Overwrite the default entrypoint of the image
    -e, --env=value    Set environment variables
        --help         Display this help
    -h, --host=value   Container host name
    -i, --interactive  Keep stdin open even if not attached
        --lxc-conf=value
                       Add custom lxc options -lxc-conf="lxc.cgroup.cpuset.cpus =
                       0,1"
    -m, --memory=value
                       Memory limit (in bytes)
    -n, --networking   Enable networking for this container
    -p, --port=value   Expose a container's port to the host (use 'docker port' to
                       see the actual mapping)
        --privileged   Give extended privileges to this container
    -t, --tty          Allocate a pseudo-tty
    -u, --user=value   Username or UID
    -v, --volume=value
                       Bind mount a volume (e.g. from the host: -v
                       /host:/container, from docker: -v /container)
        --volumes-from=value
                       Mount volumes from the specified container
    -w, --workdir=value
                       Working directory inside the container

Examples
--------

.. code-block:: bash

    sudo docker run --cidfile /tmp/docker_test.cid ubuntu echo "test"

This will create a container and print "test" to the console. The
``cidfile`` flag makes docker attempt to create a new file and write the
container ID to it. If the file exists already, docker will return an
error. Docker will close this file when docker run exits.

.. code-block:: bash

   docker run mount -t tmpfs none /var/spool/squid

This will *not* work, because by default, most potentially dangerous
kernel capabilities are dropped; including ``cap_sys_admin`` (which is
required to mount filesystems). However, the ``--privileged`` flag will
allow it to run:

.. code-block:: bash

   docker run --privileged mount -t tmpfs none /var/spool/squid

The ``--privileged`` flag gives *all* capabilities to the container,
and it also lifts all the limitations enforced by the ``device``
cgroup controller. In other words, the container can then do almost
everything that the host can do. This flag exists to allow special
use-cases, like running Docker within Docker.

.. code-block:: bash

   docker  run -w /path/to/dir/ -i -t  ubuntu pwd

The ``-w`` lets the command being executed inside directory given, 
here /path/to/dir/. If the path does not exists it is created inside the 
container.

.. code-block:: bash

   docker  run  -v `pwd`:`pwd` -w `pwd` -i -t  ubuntu pwd

The ``-v`` flag mounts the current working directory into the container. 
The ``-w`` lets the command being executed inside the current 
working directory, by changing into the directory to the value
returned by ``pwd``. So this combination executes the command
using the container, but inside the current working directory.


