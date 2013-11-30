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

.. _cli_daemon:

``daemon``
----------

::

    Usage of docker:
      -D=false: Enable debug mode
      -H=[unix:///var/run/docker.sock]: Multiple tcp://host:port or unix://path/to/socket to bind in daemon mode, single connection otherwise
      -api-enable-cors=false: Enable CORS headers in the remote API
      -b="": Attach containers to a pre-existing network bridge; use 'none' to disable container networking
      -d=false: Enable daemon mode
      -dns="": Force docker to use specific DNS servers
      -g="/var/lib/docker": Path to use as the root of the docker runtime
      -icc=true: Enable inter-container communication
      -ip="0.0.0.0": Default IP address to use when binding container ports
      -iptables=true: Disable docker's addition of iptables rules
      -p="/var/run/docker.pid": Path to use for daemon PID file
      -r=true: Restart previously running containers
      -s="": Force the docker runtime to use a specific storage driver
      -v=false: Print version information and quit

The docker daemon is the persistent process that manages containers.  Docker uses the same binary for both the 
daemon and client.  To run the daemon you provide the ``-d`` flag.

To force docker to use devicemapper as the storage driver, use ``docker -d -s devicemapper``

To set the dns server for all docker containers, use ``docker -d -dns 8.8.8.8``

To run the daemon with debug output, use ``docker -d -D``

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
the Docker client when it quits.  When you detach from the container's 
process the exit code will be retuned to the client.

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
      -t="": Repository name (and optionally a tag) to be applied 
             to the resulting image in case of success.
      -q=false: Suppress verbose build output.
      -no-cache: Do not use the cache when building the image.
      -rm: Remove intermediate containers after a successful build

The files at PATH or URL are called the "context" of the build. The
build process may refer to any of the files in the context, for
example when using an :ref:`ADD <dockerfile_add>` instruction.  When a
single ``Dockerfile`` is given as URL, then no context is set.  When a
git repository is set as URL, then the repository is used as the
context

.. _cli_build_examples:

.. seealso:: :ref:`dockerbuilder`.

Examples:
~~~~~~~~~

.. code-block:: bash

    sudo docker build .
    Uploading context 10240 bytes
    Step 1 : FROM busybox
    Pulling repository busybox
     ---> e9aa60c60128MB/2.284 MB (100%) endpoint: https://cdn-registry-1.docker.io/v1/
    Step 2 : RUN ls -lh /
     ---> Running in 9c9e81692ae9
    total 24
    drwxr-xr-x    2 root     root        4.0K Mar 12  2013 bin
    drwxr-xr-x    5 root     root        4.0K Oct 19 00:19 dev
    drwxr-xr-x    2 root     root        4.0K Oct 19 00:19 etc
    drwxr-xr-x    2 root     root        4.0K Nov 15 23:34 lib
    lrwxrwxrwx    1 root     root           3 Mar 12  2013 lib64 -> lib
    dr-xr-xr-x  116 root     root           0 Nov 15 23:34 proc
    lrwxrwxrwx    1 root     root           3 Mar 12  2013 sbin -> bin
    dr-xr-xr-x   13 root     root           0 Nov 15 23:34 sys
    drwxr-xr-x    2 root     root        4.0K Mar 12  2013 tmp
    drwxr-xr-x    2 root     root        4.0K Nov 15 23:34 usr
     ---> b35f4035db3f
    Step 3 : CMD echo Hello World
     ---> Running in 02071fceb21b
     ---> f52f38b7823e
    Successfully built f52f38b7823e

This example specifies that the PATH is ``.``, and so all the files in
the local directory get tar'd and sent to the Docker daemon.  The PATH
specifies where to find the files for the "context" of the build on
the Docker daemon. Remember that the daemon could be running on a
remote machine and that no parsing of the Dockerfile happens at the
client side (where you're running ``docker build``). That means that
*all* the files at PATH get sent, not just the ones listed to
:ref:`ADD <dockerfile_add>` in the ``Dockerfile``.

The transfer of context from the local machine to the Docker daemon is
what the ``docker`` client means when you see the "Uploading context"
message.


.. code-block:: bash

   sudo docker build -t vieux/apache:2.0 .

This will build like the previous example, but it will then tag the
resulting image. The repository name will be ``vieux/apache`` and the
tag will be ``2.0``


.. code-block:: bash

    sudo docker build - < Dockerfile

This will read a ``Dockerfile`` from *stdin* without context. Due to
the lack of a context, no contents of any local directory will be sent
to the ``docker`` daemon.  Since there is no context, a Dockerfile
``ADD`` only works if it refers to a remote URL.

.. code-block:: bash

    sudo docker build github.com/creack/docker-firefox

This will clone the Github repository and use the cloned repository as
context. The ``Dockerfile`` at the root of the repository is used as
``Dockerfile``.  Note that you can specify an arbitrary git repository
by using the ``git://`` schema.


.. _cli_commit:

``commit``
----------

::

    Usage: docker commit [OPTIONS] CONTAINER [REPOSITORY[:TAG]]

    Create a new image from a container's changes

      -m="": Commit message
      -author="": Author (eg. "John Hannibal Smith <hannibal@a-team.com>"
      -run="": Configuration to be applied when the image is launched with `docker run`.
               (ex: -run='{"Cmd": ["cat", "/world"], "PortSpecs": ["22"]}')

Simple commit of an existing container
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

.. code-block:: bash

	$ docker ps
	ID                  IMAGE               COMMAND             CREATED             STATUS              PORTS
	c3f279d17e0a        ubuntu:12.04        /bin/bash           7 days ago          Up 25 hours                             
	197387f1b436        ubuntu:12.04        /bin/bash           7 days ago          Up 25 hours                             
	$ docker commit c3f279d17e0a  SvenDowideit/testimage:version3
	f5283438590d
	$ docker images | head
	REPOSITORY                        TAG                 ID                  CREATED             SIZE
	SvenDowideit/testimage            version3            f5283438590d        16 seconds ago      204.2 MB (virtual 335.7 MB)
	

Full -run example
.................

(multiline is ok within a single quote ``'``)

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

    Usage: docker cp CONTAINER:PATH HOSTPATH

    Copy files/folders from the containers filesystem to the host
    path.  Paths are relative to the root of the filesystem.
    
.. code-block:: bash

    $ sudo docker cp 7bb0e258aefe:/etc/debian_version .
    $ sudo docker cp blue_frog:/etc/hosts .

.. _cli_diff:

``diff``
--------

::

    Usage: docker diff CONTAINER
 
    List the changed files and directories in a container's filesystem

There are 3 events that are listed in the 'diff':

1. ```A``` - Add
2. ```D``` - Delete
3. ```C``` - Change

for example:

.. code-block:: bash

	$ sudo docker diff 7bb0e258aefe

	C /dev
	A /dev/kmsg
	C /etc
	A /etc/mtab
	A /go
	A /go/src
	A /go/src/github.com
	A /go/src/github.com/dotcloud
	A /go/src/github.com/dotcloud/docker
	A /go/src/github.com/dotcloud/docker/.git
	....

.. _cli_events:

``events``
----------

::

    Usage: docker events

    Get real time events from the server
    
    -since="": Show previously created events and then stream.
               (either seconds since epoch, or date string as below)

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

Show events in the past from a specified time
.............................................

.. code-block:: bash

    $ sudo docker events -since 1378216169
    [2013-09-03 15:49:29 +0200 CEST] 4386fb97867d: (from 12de384bfb10) die
    [2013-09-03 15:49:29 +0200 CEST] 4386fb97867d: (from 12de384bfb10) stop

    $ sudo docker events -since '2013-09-03'
    [2013-09-03 15:49:26 +0200 CEST] 4386fb97867d: (from 12de384bfb10) start
    [2013-09-03 15:49:29 +0200 CEST] 4386fb97867d: (from 12de384bfb10) die
    [2013-09-03 15:49:29 +0200 CEST] 4386fb97867d: (from 12de384bfb10) stop

    $ sudo docker events -since '2013-09-03 15:49:29 +0200 CEST'
    [2013-09-03 15:49:29 +0200 CEST] 4386fb97867d: (from 12de384bfb10) die
    [2013-09-03 15:49:29 +0200 CEST] 4386fb97867d: (from 12de384bfb10) stop

.. _cli_export:

``export``
----------

::

    Usage: docker export CONTAINER

    Export the contents of a filesystem as a tar archive to STDOUT
    
for example:

.. code-block:: bash

    $ sudo docker export red_panda > latest.tar

.. _cli_history:

``history``
-----------

::

    Usage: docker history [OPTIONS] IMAGE

    Show the history of an image

      -notrunc=false: Don't truncate output
      -q=false: only show numeric IDs

To see how the docker:latest image was built:

.. code-block:: bash

	$ docker history docker
	ID                  CREATED             CREATED BY
	docker:latest       19 hours ago        /bin/sh -c #(nop) ADD . in /go/src/github.com/dotcloud/docker
	cf5f2467662d        2 weeks ago         /bin/sh -c #(nop) ENTRYPOINT ["hack/dind"]
	3538fbe372bf        2 weeks ago         /bin/sh -c #(nop) WORKDIR /go/src/github.com/dotcloud/docker
	7450f65072e5        2 weeks ago         /bin/sh -c #(nop) VOLUME /var/lib/docker
	b79d62b97328        2 weeks ago         /bin/sh -c apt-get install -y -q lxc
	36714852a550        2 weeks ago         /bin/sh -c apt-get install -y -q iptables
	8c4c706df1d6        2 weeks ago         /bin/sh -c /bin/echo -e '[default]\naccess_key=$AWS_ACCESS_KEY\nsecret_key=$AWS_SECRET_KEYn' > /.s3cfg
	b89989433c48        2 weeks ago         /bin/sh -c pip install python-magic
	a23e640d85b5        2 weeks ago         /bin/sh -c pip install s3cmd
	41f54fec7e79        2 weeks ago         /bin/sh -c apt-get install -y -q python-pip
	d9bc04add907        2 weeks ago         /bin/sh -c apt-get install -y -q reprepro dpkg-sig
	e74f4760fa70        2 weeks ago         /bin/sh -c gem install --no-rdoc --no-ri fpm
	1e43224726eb        2 weeks ago         /bin/sh -c apt-get install -y -q ruby1.9.3 rubygems libffi-dev
	460953ae9d7f        2 weeks ago         /bin/sh -c #(nop) ENV GOPATH=/go:/go/src/github.com/dotcloud/docker/vendor
	8b63eb1d666b        2 weeks ago         /bin/sh -c #(nop) ENV PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/goroot/bin
	3087f3bcedf2        2 weeks ago         /bin/sh -c #(nop) ENV GOROOT=/goroot
	635840d198e5        2 weeks ago         /bin/sh -c cd /goroot/src && ./make.bash
	439f4a0592ba        2 weeks ago         /bin/sh -c curl -s https://go.googlecode.com/files/go1.1.2.src.tar.gz | tar -v -C / -xz && mv /go /goroot
	13967ed36e93        2 weeks ago         /bin/sh -c #(nop) ENV CGO_ENABLED=0
	bf7424458437        2 weeks ago         /bin/sh -c apt-get install -y -q build-essential
	a89ec997c3bf        2 weeks ago         /bin/sh -c apt-get install -y -q mercurial
	b9f165c6e749        2 weeks ago         /bin/sh -c apt-get install -y -q git
	17a64374afa7        2 weeks ago         /bin/sh -c apt-get install -y -q curl
	d5e85dc5b1d8        2 weeks ago         /bin/sh -c apt-get update
	13e642467c11        2 weeks ago         /bin/sh -c echo 'deb http://archive.ubuntu.com/ubuntu precise main universe' > /etc/apt/sources.list
	ae6dde92a94e        2 weeks ago         /bin/sh -c #(nop) MAINTAINER Solomon Hykes <solomon@dotcloud.com>
	ubuntu:12.04        6 months ago 

.. _cli_images:

``images``
----------

::

    Usage: docker images [OPTIONS] [NAME]

    List images

      -a=false: show all images (by default filter out the intermediate images used to build)
      -notrunc=false: Don't truncate output
      -q=false: only show numeric IDs
      -tree=false: output graph in tree format
      -viz=false: output graph in graphviz format
      
Listing the most recently created images
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

.. code-block:: bash

	$ sudo docker images | head
	REPOSITORY                    TAG                 IMAGE ID            CREATED             SIZE
	<none>                        <none>              77af4d6b9913        19 hours ago        30.53 MB (virtual 1.089 GB)
	committest                    latest              b6fa739cedf5        19 hours ago        30.53 MB (virtual 1.089 GB)
	<none>                        <none>              78a85c484f71        19 hours ago        30.53 MB (virtual 1.089 GB)
	docker                        latest              30557a29d5ab        20 hours ago        30.53 MB (virtual 1.089 GB)
	<none>                        <none>              0124422dd9f9        20 hours ago        30.53 MB (virtual 1.089 GB)
	<none>                        <none>              18ad6fad3402        22 hours ago        23.68 MB (virtual 1.082 GB)
	<none>                        <none>              f9f1e26352f0        23 hours ago        30.46 MB (virtual 1.089 GB)
	tryout                        latest              2629d1fa0b81        23 hours ago        16.4 kB (virtual 131.5 MB)
	<none>                        <none>              5ed6274db6ce        24 hours ago        30.44 MB (virtual 1.089 GB)

Listing the full length image IDs
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

.. code-block:: bash

	$ sudo docker images -notrunc | head
	REPOSITORY                    TAG                 IMAGE ID                                                           CREATED             SIZE
	<none>                        <none>              77af4d6b9913e693e8d0b4b294fa62ade6054e6b2f1ffb617ac955dd63fb0182   19 hours ago        30.53 MB (virtual 1.089 GB)
	committest                    latest              b6fa739cedf5ea12a620a439402b6004d057da800f91c7524b5086a5e4749c9f   19 hours ago        30.53 MB (virtual 1.089 GB)
	<none>                        <none>              78a85c484f71509adeaace20e72e941f6bdd2b25b4c75da8693efd9f61a37921   19 hours ago        30.53 MB (virtual 1.089 GB)
	docker                        latest              30557a29d5abc51e5f1d5b472e79b7e296f595abcf19fe6b9199dbbc809c6ff4   20 hours ago        30.53 MB (virtual 1.089 GB)
	<none>                        <none>              0124422dd9f9cf7ef15c0617cda3931ee68346455441d66ab8bdc5b05e9fdce5   20 hours ago        30.53 MB (virtual 1.089 GB)
	<none>                        <none>              18ad6fad340262ac2a636efd98a6d1f0ea775ae3d45240d3418466495a19a81b   22 hours ago        23.68 MB (virtual 1.082 GB)
	<none>                        <none>              f9f1e26352f0a3ba6a0ff68167559f64f3e21ff7ada60366e2d44a04befd1d3a   23 hours ago        30.46 MB (virtual 1.089 GB)
	tryout                        latest              2629d1fa0b81b222fca63371ca16cbf6a0772d07759ff80e8d1369b926940074   23 hours ago        16.4 kB (virtual 131.5 MB)
	<none>                        <none>              5ed6274db6ceb2397844896966ea239290555e74ef307030ebb01ff91b1914df   24 hours ago        30.44 MB (virtual 1.089 GB)

Displaying images visually
~~~~~~~~~~~~~~~~~~~~~~~~~~

.. code-block:: bash

    $ sudo docker images -viz | dot -Tpng -o docker.png

.. image:: docker_images.gif
   :alt: Example inheritance graph of Docker images.


Displaying image hierarchy
~~~~~~~~~~~~~~~~~~~~~~~~~~

.. code-block:: bash

    $ sudo docker images -tree

    |─8dbd9e392a96 Size: 131.5 MB (virtual 131.5 MB) Tags: ubuntu:12.04,ubuntu:latest,ubuntu:precise
    └─27cf78414709 Size: 180.1 MB (virtual 180.1 MB)
      └─b750fe79269d Size: 24.65 kB (virtual 180.1 MB) Tags: ubuntu:12.10,ubuntu:quantal
        |─f98de3b610d5 Size: 12.29 kB (virtual 180.1 MB)
        | └─7da80deb7dbf Size: 16.38 kB (virtual 180.1 MB)
        |   └─65ed2fee0a34 Size: 20.66 kB (virtual 180.2 MB)
        |     └─a2b9ea53dddc Size: 819.7 MB (virtual 999.8 MB)
        |       └─a29b932eaba8 Size: 28.67 kB (virtual 999.9 MB)
        |         └─e270a44f124d Size: 12.29 kB (virtual 999.9 MB) Tags: progrium/buildstep:latest
        └─17e74ac162d8 Size: 53.93 kB (virtual 180.2 MB)
          └─339a3f56b760 Size: 24.65 kB (virtual 180.2 MB)
            └─904fcc40e34d Size: 96.7 MB (virtual 276.9 MB)
              └─b1b0235328dd Size: 363.3 MB (virtual 640.2 MB)
                └─7cb05d1acb3b Size: 20.48 kB (virtual 640.2 MB)
                  └─47bf6f34832d Size: 20.48 kB (virtual 640.2 MB)
                    └─f165104e82ed Size: 12.29 kB (virtual 640.2 MB)
                      └─d9cf85a47b7e Size: 1.911 MB (virtual 642.2 MB)
                        └─3ee562df86ca Size: 17.07 kB (virtual 642.2 MB)
                          └─b05fc2d00e4a Size: 24.96 kB (virtual 642.2 MB)
                            └─c96a99614930 Size: 12.29 kB (virtual 642.2 MB)
                              └─a6a357a48c49 Size: 12.29 kB (virtual 642.2 MB) Tags: ndj/mongodb:latest

.. _cli_import:

``import``
----------

::

    Usage: docker import URL|- [REPOSITORY[:TAG]]

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

This will create a new untagged image.

``$ sudo docker import http://example.com/exampleimage.tgz``

Import from a local file
........................

Import to docker via pipe and standard in

``$ cat exampleimage.tgz | sudo docker import - exampleimagelocal:new``

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

.. code-block:: bash

	$ sudo docker info
	Containers: 292
	Images: 194
	Debug mode (server): false
	Debug mode (client): false
	Fds: 22
	Goroutines: 67
	LXC Version: 0.9.0
	EventsListeners: 115
	Kernel Version: 3.8.0-33-generic
	WARNING: No swap limit support


.. _cli_insert:

``insert``
----------

::

    Usage: docker insert IMAGE URL PATH

    Insert a file from URL in the IMAGE at PATH

Use the specified IMAGE as the parent for a new image which adds a
:ref:`layer <layer_def>` containing the new file. ``insert`` does not modify 
the original image, and the new image has the contents of the parent image, 
plus the new file.


Examples
~~~~~~~~

Insert file from github
.......................

.. code-block:: bash

    $ sudo docker insert 8283e18b24bc https://raw.github.com/metalivedev/django/master/postinstall /tmp/postinstall.sh
    06fd35556d7b

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

Known Issues (kill)
~~~~~~~~~~~~~~~~~~~

* :issue:`197` indicates that ``docker kill`` may leave directories
  behind and make it difficult to remove the container.

.. _cli_load:

``load``
--------

::

    Usage: docker load < repository.tar

    Loads a tarred repository from the standard input stream.
    Restores both images and tags.

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

Known Issues (rm)
~~~~~~~~~~~~~~~~~

* :issue:`197` indicates that ``docker kill`` may leave directories
  behind and make it difficult to remove the container.


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

.. code-block:: bash

    $ docker rm `docker ps -a -q`


This command will delete all stopped containers. The command ``docker ps -a -q`` will return all
existing container IDs and pass them to the ``rm`` command which will delete them. Any running
containers will not be deleted.

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
      -volumes-from="": Mount all volumes from the given container(s)
      -entrypoint="": Overwrite the default entrypoint set by the image
      -w="": Working directory inside the container
      -lxc-conf=[]: Add custom lxc options -lxc-conf="lxc.cgroup.cpuset.cpus = 0,1"
      -sig-proxy=true: Proxify all received signal to the process (even in non-tty mode)
      -expose=[]: Expose a port from the container without publishing it to your host
      -link="": Add link to another container (name:alias)
      -name="": Assign the specified name to the container. If no name is specific docker will generate a random name
      -P=false: Publish all exposed ports to the host interfaces

Known Issues (run -volumes-from)
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

* :issue:`2702`: "lxc-start: Permission denied - failed to mount"
  could indicate a permissions problem with AppArmor. Please see the
  issue for a workaround.

Examples:
~~~~~~~~~

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

    docker run -p 127.0.0.1:80:8080 ubuntu bash

This binds port ``8080`` of the container to port ``80`` on 127.0.0.1 of the
host machine. :ref:`port_redirection` explains in detail how to manipulate ports
in Docker.

.. code-block:: bash

    docker run -expose 80 ubuntu bash

This exposes port ``80`` of the container for use within a link without
publishing the port to the host system's interfaces. :ref:`port_redirection`
explains in detail how to manipulate ports in Docker.

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

.. code-block:: bash

   docker run -volumes-from 777f7dc92da7,ba8c0c54f0f2:ro -i -t ubuntu pwd

The ``-volumes-from`` flag mounts all the defined volumes from the
refrence containers. Containers can be specified by a comma seperated
list or by repetitions of the ``-volumes-from`` argument. The container
id may be optionally suffixed with ``:ro`` or ``:rw`` to mount the volumes in
read-only or read-write mode, respectively. By default, the volumes are mounted
in the same mode (rw or ro) as the reference container.

.. _cli_save:

``save``
---------

::

    Usage: docker save image > repository.tar

    Streams a tarred repository to the standard output stream.
    Contains all parent layers, and all tags + versions.

.. _cli_search:

``search``
----------

::

    Usage: docker search TERM

    Search the docker index for images

     -notrunc=false: Don't truncate output
     -stars=0: Only displays with at least xxx stars
     -trusted=false: Only show trusted builds

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

    Usage: docker tag [OPTIONS] IMAGE REPOSITORY[:TAG]

    Tag an image into a repository

      -f=false: Force

.. _cli_top:

``top``
-------

::

    Usage: docker top CONTAINER [ps OPTIONS]

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
