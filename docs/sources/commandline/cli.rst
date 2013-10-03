:title: Command Line Interface
:description: Docker's CLI command description and usage
:keywords: Docker, Docker documentation, CLI, command line

.. _cli:

Command Line Help
-----------------

To list available commands, either run ``docker`` with no parameters or execute
``docker help``::

  $ sudo docker
    Usage: docker [OPTIONS] COMMAND [arg...]
      -H=[unix:///var/run/docker.sock]: tcp://host:port to bind/connect to or unix://path/to/socket to use

    A self-sufficient runtime for linux containers.

    ...

.. _cli_attach:

``attach``
----------

::

    Usage: docker attach CONTAINER

    Attach to a running container.

      -nostdin=false: Do not attach stdin
      -sig-proxy=true: Proxify all received signal to the process (even in non-tty mode)

You can detach from the container again (and leave it running) with
``CTRL-c`` (for a quiet exit) or ``CTRL-\`` to get a stacktrace of
the Docker client when it quits.

To stop a container, use ``docker stop``

To kill the container, use ``docker kill``

.. _cli_attach_examples:
 
Examples:
~~~~~~~~~

.. code-block:: bash

     $ ID=$(sudo docker run -d ubuntu /usr/bin/top -b)
     $ sudo docker attach $ID
     top - 02:05:52 up  3:05,  0 users,  load average: 0.01, 0.02, 0.05
     Tasks:   1 total,   1 running,   0 sleeping,   0 stopped,   0 zombie
     Cpu(s):  0.1%us,  0.2%sy,  0.0%ni, 99.7%id,  0.0%wa,  0.0%hi,  0.0%si,  0.0%st
     Mem:    373572k total,   355560k used,    18012k free,    27872k buffers
     Swap:   786428k total,        0k used,   786428k free,   221740k cached

     PID USER      PR  NI  VIRT  RES  SHR S %CPU %MEM    TIME+  COMMAND            
      1 root      20   0 17200 1116  912 R    0  0.3   0:00.03 top                

      top - 02:05:55 up  3:05,  0 users,  load average: 0.01, 0.02, 0.05
      Tasks:   1 total,   1 running,   0 sleeping,   0 stopped,   0 zombie
      Cpu(s):  0.0%us,  0.2%sy,  0.0%ni, 99.8%id,  0.0%wa,  0.0%hi,  0.0%si,  0.0%st
      Mem:    373572k total,   355244k used,    18328k free,    27872k buffers
      Swap:   786428k total,        0k used,   786428k free,   221776k cached

        PID USER      PR  NI  VIRT  RES  SHR S %CPU %MEM    TIME+  COMMAND            
	    1 root      20   0 17208 1144  932 R    0  0.3   0:00.03 top                


      top - 02:05:58 up  3:06,  0 users,  load average: 0.01, 0.02, 0.05
      Tasks:   1 total,   1 running,   0 sleeping,   0 stopped,   0 zombie
      Cpu(s):  0.2%us,  0.3%sy,  0.0%ni, 99.5%id,  0.0%wa,  0.0%hi,  0.0%si,  0.0%st
      Mem:    373572k total,   355780k used,    17792k free,    27880k buffers
      Swap:   786428k total,        0k used,   786428k free,   221776k cached

      PID USER      PR  NI  VIRT  RES  SHR S %CPU %MEM    TIME+  COMMAND            
           1 root      20   0 17208 1144  932 R    0  0.3   0:00.03 top                
     ^C$ 
     $ sudo docker stop $ID

.. _cli_build:

``build``
---------

::

    Usage: docker build [OPTIONS] PATH | URL | -
    Build a new container image from the source code at PATH
      -t="": Repository name (and optionally a tag) to be applied to the resulting image in case of success.
      -q=false: Suppress verbose build output.
      -no-cache: Do not use the cache when building the image.
      -rm: Remove intermediate containers after a successful build
    When a single Dockerfile is given as URL, then no context is set. When a git repository is set as URL, the repository is used as context

.. _cli_build_examples:

Examples:
~~~~~~~~~

.. code-block:: bash

    sudo docker build .

This will read the ``Dockerfile`` from the current directory. It will
also send any other files and directories found in the current
directory to the ``docker`` daemon.

The contents of this directory would be used by ``ADD`` commands found
within the ``Dockerfile``.  This will send a lot of data to the
``docker`` daemon if the current directory contains a lot of data.  If
the absolute path is provided instead of ``.`` then only the files and
directories required by the ADD commands from the ``Dockerfile`` will be
added to the context and transferred to the ``docker`` daemon.

.. code-block:: bash

   sudo docker build -t vieux/apache:2.0 .

This will build like the previous example, but it will then tag the
resulting image. The repository name will be ``vieux/apache`` and the
tag will be ``2.0``


.. code-block:: bash

    sudo docker build - < Dockerfile

This will read a ``Dockerfile`` from *stdin* without context. Due to
the lack of a context, no contents of any local directory will be sent
to the ``docker`` daemon.  ``ADD`` doesn't work when running in this
mode because the absence of the context provides no source files to
copy to the container.


.. code-block:: bash

    sudo docker build github.com/creack/docker-firefox

This will clone the Github repository and use it as context. The
``Dockerfile`` at the root of the repository is used as
``Dockerfile``.  Note that you can specify an arbitrary git repository
by using the ``git://`` schema.


.. _cli_commit:

``commit``
----------

::

    Usage: docker commit [OPTIONS] CONTAINER [REPOSITORY [TAG]]

    Create a new image from a container's changes

      -m="": Commit message
      -author="": Author (eg. "John Hannibal Smith <hannibal@a-team.com>"
      -run="": Configuration to be applied when the image is launched with `docker run`. 
               (ex: '{"Cmd": ["cat", "/world"], "PortSpecs": ["22"]}')

Full -run example (multiline is ok within a single quote ``'``)

::

  $ sudo docker commit -run='
  {
      "Entrypoint" : null,
      "Privileged" : false,
      "User" : "",
      "VolumesFrom" : "",
      "Cmd" : ["cat", "-e", "/etc/resolv.conf"],
      "Dns" : ["8.8.8.8", "8.8.4.4"],
      "MemorySwap" : 0,
      "AttachStdin" : false,
      "AttachStderr" : false,
      "CpuShares" : 0,
      "OpenStdin" : false,
      "Volumes" : null,
      "Hostname" : "122612f45831",
      "PortSpecs" : ["22", "80", "443"],
      "Image" : "b750fe79269d2ec9a3c593ef05b4332b1d1a02a62b4accb2c21d589ff2f5f2dc",
      "Tty" : false,
      "Env" : [
         "HOME=/",
         "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
      ],
      "StdinOnce" : false,
      "Domainname" : "",
      "WorkingDir" : "/",
      "NetworkDisabled" : false,
      "Memory" : 0,
      "AttachStdout" : false
  }' $CONTAINER_ID

.. _cli_cp:

``cp``
------

::

    Usage: docker cp CONTAINER:RESOURCE HOSTPATH

    Copy files/folders from the containers filesystem to the host
    path.  Paths are relative to the root of the filesystem.

.. _cli_diff:

``diff``
--------

::

    Usage: docker diff CONTAINER [OPTIONS]

    Inspect changes on a container's filesystem

.. _cli_events:

``events``
----------

::

    Usage: docker events

    Get real time events from the server

.. _cli_events_example:

Examples
~~~~~~~~

You'll need two shells for this example.

Shell 1: Listening for events
.............................

.. code-block:: bash
    
    $ sudo docker events

Shell 2: Start and Stop a Container
...................................

.. code-block:: bash

    $ sudo docker start 4386fb97867d
    $ sudo docker stop 4386fb97867d

Shell 1: (Again .. now showing events)
......................................

.. code-block:: bash

    [2013-09-03 15:49:26 +0200 CEST] 4386fb97867d: (from 12de384bfb10) start
    [2013-09-03 15:49:29 +0200 CEST] 4386fb97867d: (from 12de384bfb10) die
    [2013-09-03 15:49:29 +0200 CEST] 4386fb97867d: (from 12de384bfb10) stop


.. _cli_export:

``export``
----------

::

    Usage: docker export CONTAINER

    Export the contents of a filesystem as a tar archive

.. _cli_history:

``history``
-----------

::

    Usage: docker history [OPTIONS] IMAGE

    Show the history of an image

      -notrunc=false: Don't truncate output
      -q=false: only show numeric IDs

.. _cli_images:

``images``
----------

::

    Usage: docker images [OPTIONS] [NAME]

    List images

      -a=false: show all images
      -q=false: only show numeric IDs
      -viz=false: output in graphviz format

Displaying images visually
~~~~~~~~~~~~~~~~~~~~~~~~~~

::

    sudo docker images -viz | dot -Tpng -o docker.png

.. image:: docker_images.gif
   :alt: Example inheritance graph of Docker images.

.. _cli_import:

``import``
----------

::

    Usage: docker import URL|- [REPOSITORY [TAG]]

    Create a new filesystem image from the contents of a tarball

At this time, the URL must start with ``http`` and point to a single
file archive (.tar, .tar.gz, .tgz, .bzip, .tar.xz, .txz) containing a
root filesystem. If you would like to import from a local directory or
archive, you can use the ``-`` parameter to take the data from
standard in.

Examples
~~~~~~~~

Import from a remote location
.............................

``$ sudo docker import http://example.com/exampleimage.tgz exampleimagerepo``

Import from a local file
........................

Import to docker via pipe and standard in

``$ cat exampleimage.tgz | sudo docker import - exampleimagelocal``

Import from a local directory
.............................

``$ sudo tar -c . | docker import - exampleimagedir``

Note the ``sudo`` in this example -- you must preserve the ownership
of the files (especially root ownership) during the archiving with
tar. If you are not root (or sudo) when you tar, then the ownerships
might not get preserved.

.. _cli_info:

``info``
--------

::

    Usage: docker info

    Display system-wide information.

.. _cli_insert:

``insert``
----------

::

    Usage: docker insert IMAGE URL PATH

    Insert a file from URL in the IMAGE at PATH

Examples
~~~~~~~~

Insert file from github
.......................

.. code-block:: bash

    $ sudo docker insert 8283e18b24bc https://raw.github.com/metalivedev/django/master/postinstall /tmp/postinstall.sh

.. _cli_inspect:

``inspect``
-----------

::

    Usage: docker inspect [OPTIONS] CONTAINER

    Return low-level information on a container

.. _cli_kill:

``kill``
--------

::

    Usage: docker kill CONTAINER [CONTAINER...]

    Kill a running container (Send SIGKILL)
    
The main process inside the container will be sent SIGKILL.

.. _cli_login:

``login``
---------

::

    Usage: docker login [OPTIONS] [SERVER]

    Register or Login to the docker registry server

    -e="": email
    -p="": password
    -u="": username

    If you want to login to a private registry you can
    specify this by adding the server name.

    example:
    docker login localhost:8080


.. _cli_logs:

``logs``
--------

::

    Usage: docker logs [OPTIONS] CONTAINER

    Fetch the logs of a container


.. _cli_port:

``port``
--------

::

    Usage: docker port [OPTIONS] CONTAINER PRIVATE_PORT

    Lookup the public-facing port which is NAT-ed to PRIVATE_PORT


.. _cli_ps:

``ps``
------

::

    Usage: docker ps [OPTIONS]

    List containers

      -a=false: Show all containers. Only running containers are shown by default.
      -notrunc=false: Don't truncate output
      -q=false: Only display numeric IDs

.. _cli_pull:

``pull``
--------

::

    Usage: docker pull NAME

    Pull an image or a repository from the registry


.. _cli_push:

``push``
--------

::

    Usage: docker push NAME

    Push an image or a repository to the registry


.. _cli_restart:

``restart``
-----------

::

    Usage: docker restart [OPTIONS] NAME

    Restart a running container

.. _cli_rm:

``rm``
------

::

    Usage: docker rm [OPTIONS] CONTAINER

    Remove one or more containers
        -link="": Remove the link instead of the actual container
 

Examples:
~~~~~~~~~

.. code-block:: bash

    $ docker rm /redis
    /redis


This will remove the container referenced under the link ``/redis``.


.. code-block:: bash

    $ docker rm -link /webapp/redis
    /webapp/redis


This will remove the underlying link between ``/webapp`` and the ``/redis`` containers removing all
network communication.

.. _cli_rmi:

``rmi``
-------

::

    Usage: docker rmi IMAGE [IMAGE...]

    Remove one or more images

.. _cli_run:

``run``
-------

::

    Usage: docker run [OPTIONS] IMAGE[:TAG] [COMMAND] [ARG...]

    Run a command in a new container

      -a=map[]: Attach to stdin, stdout or stderr
      -c=0: CPU shares (relative weight)
      -cidfile="": Write the container ID to the file
      -d=false: Detached mode: Run container in the background, print new container id
      -e=[]: Set environment variables
      -h="": Container host name
      -i=false: Keep stdin open even if not attached
      -privileged=false: Give extended privileges to this container
      -m="": Memory limit (format: <number><optional unit>, where unit = b, k, m or g)
      -n=true: Enable networking for this container
      -p=[]: Map a network port to the container
      -rm=false: Automatically remove the container when it exits (incompatible with -d)
      -t=false: Allocate a pseudo-tty
      -u="": Username or UID
      -dns=[]: Set custom dns servers for the container
      -v=[]: Create a bind mount with: [host-dir]:[container-dir]:[rw|ro]. If "container-dir" is missing, then docker creates a new volume.
      -volumes-from="": Mount all volumes from the given container
      -entrypoint="": Overwrite the default entrypoint set by the image
      -w="": Working directory inside the container
      -lxc-conf=[]: Add custom lxc options -lxc-conf="lxc.cgroup.cpuset.cpus = 0,1"
      -sig-proxy=true: Proxify all received signal to the process (even in non-tty mode)
      -expose=[]: Expose a port from the container without publishing it to your host
      -link="": Add link to another container (name:alias)
      -name="": Assign the specified name to the container. If no name is specific docker will generate a random name

Examples
--------

.. code-block:: bash

    sudo docker run -cidfile /tmp/docker_test.cid ubuntu echo "test"

This will create a container and print "test" to the console. The
``cidfile`` flag makes docker attempt to create a new file and write the
container ID to it. If the file exists already, docker will return an
error. Docker will close this file when docker run exits.

.. code-block:: bash

   docker run mount -t tmpfs none /var/spool/squid

This will *not* work, because by default, most potentially dangerous
kernel capabilities are dropped; including ``cap_sys_admin`` (which is
required to mount filesystems). However, the ``-privileged`` flag will
allow it to run:

.. code-block:: bash

   docker run -privileged mount -t tmpfs none /var/spool/squid

The ``-privileged`` flag gives *all* capabilities to the container,
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

.. code-block:: bash

    docker run -p 127.0.0.0::80 ubuntu bash

The ``-p`` flag now allows you to bind a port to a specific
interface of the host machine.  In this example port ``80`` of the 
container will have a dynamically allocated port bound to 127.0.0.1 
of the host.

.. code-block:: bash

    docker run -p 127.0.0.1:80:80 ubuntu bash

This will bind port ``80`` of the container to port ``80`` on 127.0.0.1 of your
host machine.

.. code-block:: bash

    docker run -expose 80 ubuntu bash

This will expose port ``80`` of the container for use within a link
without publishing the port to the host system's interfaces.  

.. code-block:: bash

    docker run -name console -t -i ubuntu bash

This will create and run a new container with the container name 
being ``console``.

.. code-block:: bash

    docker run -link /redis:redis -name console ubuntu bash

The ``-link`` flag will link the container named ``/redis`` into the 
newly created container with the alias ``redis``.  The new container
can access the network and environment of the redis container via
environment variables.  The ``-name`` flag will assign the name ``console`` 
to the newly created container.

.. _cli_search:

``search``
----------

::

    Usage: docker search TERM

    Searches for the TERM parameter on the Docker index and prints out
    a list of repositories that match.

.. _cli_start:

``start``
---------

::

    Usage: docker start [OPTIONS] NAME

    Start a stopped container

      -a=false: Attach container's stdout/stderr and forward all signals to the process
      -i=false: Attach container's stdin

.. _cli_stop:

``stop``
--------

::

    Usage: docker stop [OPTIONS] CONTAINER [CONTAINER...]

    Stop a running container (Send SIGTERM, and then SIGKILL after grace period)

      -t=10: Number of seconds to wait for the container to stop before killing it.
      
The main process inside the container will receive SIGTERM, and after a grace period, SIGKILL

.. _cli_tag:

``tag``
-------

::

    Usage: docker tag [OPTIONS] IMAGE REPOSITORY [TAG]

    Tag an image into a repository

      -f=false: Force

.. _cli_top:

``top``
-------

::

    Usage: docker top CONTAINER

    Lookup the running processes of a container

.. _cli_version:

``version``
-----------

Show the version of the docker client, daemon, and latest released version.


.. _cli_wait:

``wait``
--------

::

    Usage: docker wait [OPTIONS] NAME

    Block until a container stops, then print its exit code.



