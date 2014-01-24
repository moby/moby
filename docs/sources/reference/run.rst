:title: Docker Run Reference 
:description: Configure containers at runtime
:keywords: docker, run, configure, runtime

.. _run_docker:

====================
Docker Run Reference
====================

**Docker runs processes in isolated containers**.  When an operator
executes ``docker run``, she starts a process with its own file
system, its own networking, and its own isolated process tree. The
:ref:`image_def` which starts the process may define defaults related
to the binary to run, the networking to expose, and more, but ``docker
run`` gives final control to the operator who starts the container
from the image. That's the main reason :ref:`cli_run` has more options
than any other ``docker`` command.

Every one of the :ref:`example_list` shows running containers, and so
here we try to give more in-depth guidance.

.. contents:: Table of Contents

.. _run_running:

General Form
============

As you've seen in the :ref:`example_list`, the basic `run` command
takes this form::

  docker run [OPTIONS] IMAGE[:TAG] [COMMAND] [ARG...]

To learn how to interpret the types of ``[OPTIONS]``, see
:ref:`cli_options`.

The list of ``[OPTIONS]`` breaks down into two groups: 

* options that define the runtime behavior or environment, and 
* options that override image defaults. 

Since image defaults usually get set in :ref:`Dockerfiles
<dockerbuilder>` (though they could also be set at :ref:`cli_commit`
time too), we will group the runtime options here by their related
Dockerfile commands so that it is easier to see how to override image
defaults and set new behavior.

We'll start, though, with the options that are unique to ``docker
run``, the options which define the runtime behavior or the container
environment.

.. note:: The runtime operator always has final control over the
   behavior of a Docker container.

Detached or Foreground
======================

When starting a Docker container, you must first decide if you want to
run the container in the background in a "detached" mode or in the
default foreground mode::

   -d=false: Detached mode: Run container in the background, print new container id

Detached (-d)
.............

In detached mode (``-d=true`` or just ``-d``), all IO should be done
through network connections or shared volumes because the container is
no longer listening to the commandline where you executed ``docker
run``. You can reattach to a detached container with ``docker``
:ref:`cli_attach`. If you choose to run a container in the detached
mode, then you cannot use the ``-rm`` option.

Foreground
..........

In foreground mode (the default when ``-d`` is not specified),
``docker run`` can start the process in the container and attach the
console to the process's standard input, output, and standard
error. It can even pretend to be a TTY (this is what most commandline
executables expect) and pass along signals. All of that is
configurable::

   -a=[]          : Attach to stdin, stdout and/or stderr
   -t=false       : Allocate a pseudo-tty
   -sig-proxy=true: Proxify all received signal to the process (even in non-tty mode)
   -i=false       : Keep stdin open even if not attached

If you do not specify ``-a`` then Docker will `attach everything
(stdin,stdout,stderr)
<https://github.com/dotcloud/docker/blob/master/commands.go#L1797>`_. You
can specify which of the three standard streams (stdin, stdout,
stderr) you'd like to connect between your  instead, as in::

   docker run -a stdin -a stdout -i -t ubuntu /bin/bash

For interactive processes (like a shell) you will typically want a tty
as well as persistent standard in, so you'll use ``-i -t`` together in
most interactive cases.

Clean Up (-rm)
--------------

By default a container's file system persists even after the container
exits. This makes debugging a lot easier (since you can inspect the
final state) and you retain all your data by default. But if you are
running short-term **foreground** processes, these container file
systems can really pile up. If instead you'd like Docker to
**automatically clean up the container and remove the file system when
the container exits**, you can add the ``-rm`` flag::

   -rm=false: Automatically remove the container when it exits (incompatible with -d)

Name (-name)
============

The operator can identify a container in three ways:

* UUID long identifier ("f78375b1c487e03c9438c729345e54db9d20cfa2ac1fc3494b6eb60872e74778")
* UUID short identifier ("f78375b1c487")
* name ("evil_ptolemy")

The UUID identifiers come from the Docker daemon, and if you do not
assign a name to the container with ``-name`` then the daemon will
also generate a random string name too. The name can become a handy
way to add meaning to a container since you can use this name when
defining :ref:`links <working_with_links_names>` (or any other place
you need to identify a container). This works for both background and
foreground Docker containers.

PID Equivalent
==============

And finally, to help with automation, you can have Docker write the
container id out to a file of your choosing. This is similar to how
some programs might write out their process ID to a file (you've seen
them as .pid files)::

      -cidfile="": Write the container ID to the file

Overriding Dockerfile Image Defaults
====================================

When a developer builds an image from a :ref:`Dockerfile
<dockerbuilder>` or when she commits it, the developer can set a
number of default parameters that take effect when the image starts up
as a container.

Four of the Dockerfile commands cannot be overridden at runtime:
``FROM, MAINTAINER, RUN``, and ``ADD``. Everything else has a
corresponding override in ``docker run``. We'll go through what the
developer might have set in each Dockerfile instruction and how the
operator can override that setting.


CMD
...

Remember the optional ``COMMAND`` in the Docker commandline::

  docker run [OPTIONS] IMAGE[:TAG] [COMMAND] [ARG...]

This command is optional because the person who created the ``IMAGE``
may have already provided a default ``COMMAND`` using the Dockerfile
``CMD``. As the operator (the person running a container from the
image), you can override that ``CMD`` just by specifying a new
``COMMAND``.

If the image also specifies an ``ENTRYPOINT`` then the ``CMD`` or
``COMMAND`` get appended as arguments to the ``ENTRYPOINT``.


ENTRYPOINT
..........

::

   -entrypoint="": Overwrite the default entrypoint set by the image

The ENTRYPOINT of an image is similar to a COMMAND because it
specifies what executable to run when the container starts, but it is
(purposely) more difficult to override. The ENTRYPOINT gives a
container its default nature or behavior, so that when you set an
ENTRYPOINT you can run the container *as if it were that binary*,
complete with default options, and you can pass in more options via
the COMMAND. But, sometimes an operator may want to run something else
inside the container, so you can override the default ENTRYPOINT at
runtime by using a string to specify the new ENTRYPOINT. Here is an
example of how to run a shell in a container that has been set up to
automatically run something else (like ``/usr/bin/redis-server``)::

  docker run -i -t -entrypoint /bin/bash example/redis

or two examples of how to pass more parameters to that ENTRYPOINT::

  docker run -i -t -entrypoint /bin/bash example/redis -c ls -l
  docker run -i -t -entrypoint /usr/bin/redis-cli example/redis --help


EXPOSE (``run`` Networking Options)
...................................

The *Dockerfile* doesn't give much control over networking, only
providing the EXPOSE instruction to give a hint to the operator about
what incoming ports might provide services. At runtime, however,
Docker provides a number of ``run`` options related to networking::

   -n=true   : Enable networking for this container
   -dns=[]   : Set custom dns servers for the container
   -expose=[]: Expose a port from the container 
               without publishing it to your host
   -P=false  : Publish all exposed ports to the host interfaces
   -p=[]     : Publish a container's port to the host (format: 
               ip:hostPort:containerPort | ip::containerPort | 
               hostPort:containerPort) 
               (use 'docker port' to see the actual mapping)
   -link=""  : Add link to another container (name:alias)

By default, all containers have networking enabled and they can make
any outgoing connections. The operator can completely disable
networking with ``run -n`` which disables all incoming and outgoing
networking. In cases like this, you would perform IO through files or
stdin/stdout only.

Your container will use the same DNS servers as the host by default,
but you can override this with ``-dns``.

As mentioned previously, ``EXPOSE`` (and ``-expose``) make a port
available **in** a container for incoming connections. The port number
on the inside of the container (where the service listens) does not
need to be the same number as the port exposed on the outside of the
container (where clients connect), so inside the container you might
have an HTTP service listening on port 80 (and so you ``EXPOSE 80`` in
the Dockerfile), but outside the container the port might be 42800.

To help a new client container reach the server container's internal
port operator ``-expose'd`` by the operator or ``EXPOSE'd`` by the
developer, the operator has three choices: start the server container
with ``-P`` or ``-p,`` or start the client container with ``-link``.

If the operator uses ``-P`` or ``-p`` then Docker will make the
exposed port accessible on the host and the ports will be available to
any client that can reach the host. To find the map between the host
ports and the exposed ports, use ``docker port``)

If the operator uses ``-link`` when starting the new client container,
then the client container can access the exposed port via a private
networking interface. Docker will set some environment variables in
the client container to help indicate which interface and port to use.

ENV (Environment Variables)
...........................

The operator can **set any environment variable** in the container by
using one or more ``-e``, even overriding those already defined by the
developer with a Dockefile ``ENV``::

   $ docker run -e "deep=purple" -rm ubuntu /bin/bash -c export
   declare -x HOME="/"
   declare -x HOSTNAME="85bc26a0e200"
   declare -x OLDPWD
   declare -x PATH="/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
   declare -x PWD="/"
   declare -x SHLVL="1"
   declare -x container="lxc"
   declare -x deep="purple"

Similarly the operator can set the **hostname** with ``-h``.

``-link name:alias`` also sets environment variables, using the
*alias* string to define environment variables within the container
that give the IP and PORT information for connecting to the service
container. Let's imagine we have a container running Redis::

   # Start the service container, named redis-name
   $ docker run -d -name redis-name dockerfiles/redis
   4241164edf6f5aca5b0e9e4c9eccd899b0b8080c64c0cd26efe02166c73208f3

   # The redis-name container exposed port 6379
   $ docker ps  
   CONTAINER ID        IMAGE                      COMMAND                CREATED             STATUS              PORTS               NAMES
   4241164edf6f        dockerfiles/redis:latest   /redis-stable/src/re   5 seconds ago       Up 4 seconds        6379/tcp            redis-name  

   # Note that there are no public ports exposed since we didn't use -p or -P
   $ docker port 4241164edf6f 6379
   2014/01/25 00:55:38 Error: No public port '6379' published for 4241164edf6f


Yet we can get information about the redis container's exposed ports with ``-link``. Choose an alias that will form a valid environment variable!

::

   $ docker run -rm -link redis-name:redis_alias -entrypoint /bin/bash dockerfiles/redis -c export
   declare -x HOME="/"
   declare -x HOSTNAME="acda7f7b1cdc"
   declare -x OLDPWD
   declare -x PATH="/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
   declare -x PWD="/"
   declare -x REDIS_ALIAS_NAME="/distracted_wright/redis"
   declare -x REDIS_ALIAS_PORT="tcp://172.17.0.32:6379"
   declare -x REDIS_ALIAS_PORT_6379_TCP="tcp://172.17.0.32:6379"
   declare -x REDIS_ALIAS_PORT_6379_TCP_ADDR="172.17.0.32"
   declare -x REDIS_ALIAS_PORT_6379_TCP_PORT="6379"
   declare -x REDIS_ALIAS_PORT_6379_TCP_PROTO="tcp"
   declare -x SHLVL="1"
   declare -x container="lxc"

And we can use that information to connect from another container as a client::

   $ docker run -i -t -rm -link redis-name:redis_alias -entrypoint /bin/bash dockerfiles/redis -c '/redis-stable/src/redis-cli -h $REDIS_ALIAS_PORT_6379_TCP_ADDR -p $REDIS_ALIAS_PORT_6379_TCP_PORT'
   172.17.0.32:6379>

VOLUME (Shared Filesystems)
...........................

::

   -v=[]: Create a bind mount with: [host-dir]:[container-dir]:[rw|ro]. 
          If "container-dir" is missing, then docker creates a new volume.
   -volumes-from="": Mount all volumes from the given container(s)

The volumes commands are complex enough to have their own
documentation in section :ref:`volume_def`. A developer can define one
or more VOLUMEs associated with an image, but only the operator can
give access from one container to another (or from a container to a
volume mounted on the host).

USER
....

::

   -u="": Username or UID

WORKDIR
.......

::

   -w="": Working directory inside the container

Performance
===========

The operator can also adjust the performance parameters of the container::

   -c=0 : CPU shares (relative weight)
   -m="": Memory limit (format: <number><optional unit>, where unit = b, k, m or g)

   -lxc-conf=[]: Add custom lxc options -lxc-conf="lxc.cgroup.cpuset.cpus = 0,1"
   -privileged=false: Give extended privileges to this container

