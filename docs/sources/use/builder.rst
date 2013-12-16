:title: Build Images (Dockerfile Reference)
:description: Dockerfiles use a simple DSL which allows you to automate the steps you would normally manually take to create an image.
:keywords: builder, docker, Dockerfile, automation, image creation

.. _dockerbuilder:

===================================
Build Images (Dockerfile Reference)
===================================

**Docker can act as a builder** and read instructions from a text
``Dockerfile`` to automate the steps you would otherwise take manually
to create an image. Executing ``docker build`` will run your steps and
commit them along the way, giving you a final image.

.. contents:: Table of Contents

.. _dockerfile_usage:

1. Usage
========

To :ref:`build <cli_build>` an image from a source repository, create
a description file called ``Dockerfile`` at the root of your
repository. This file will describe the steps to assemble the image.

Then call ``docker build`` with the path of your source repository as
argument (for example, ``.``):

    ``sudo docker build .``

The path to the source repository defines where to find the *context*
of the build. The build is run by the Docker daemon, not by the CLI,
so the whole context must be transferred to the daemon. The Docker CLI
reports "Uploading context" when the context is sent to the daemon.

You can specify a repository and tag at which to save the new image if the
build succeeds:

    ``sudo docker build -t shykes/myapp .``

The Docker daemon will run your steps one-by-one, committing the
result if necessary, before finally outputting the ID of your new
image. The Docker daemon will automatically clean up the context you
sent.

When you're done with your build, you're ready to look into
:ref:`image_push`.

.. _dockerfile_format:

2. Format
=========

The Dockerfile format is quite simple:

::

    # Comment
    INSTRUCTION arguments

The Instruction is not case-sensitive, however convention is for them to be
UPPERCASE in order to distinguish them from arguments more easily.

Docker evaluates the instructions in a Dockerfile in order. **The
first instruction must be `FROM`** in order to specify the
:ref:`base_image_def` from which you are building.

Docker will treat lines that *begin* with ``#`` as a comment. A ``#``
marker anywhere else in the line will be treated as an argument. This
allows statements like:

::

    # Comment
    RUN echo 'we are running some # of cool things'

.. _dockerfile_instructions:

3. Instructions
===============

Here is the set of instructions you can use in a ``Dockerfile`` for
building images.

.. _dockerfile_from:

3.1 FROM
--------

    ``FROM <image>``

Or

    ``FROM <image>:<tag>``

The ``FROM`` instruction sets the :ref:`base_image_def` for subsequent
instructions. As such, a valid Dockerfile must have ``FROM`` as its
first instruction. The image can be any valid image -- it is
especially easy to start by **pulling an image** from the
:ref:`using_public_repositories`.

``FROM`` must be the first non-comment instruction in the
``Dockerfile``.

``FROM`` can appear multiple times within a single Dockerfile in order
to create multiple images. Simply make a note of the last image id
output by the commit before each new ``FROM`` command.

If no ``tag`` is given to the ``FROM`` instruction, ``latest`` is
assumed. If the used tag does not exist, an error will be returned.

.. _dockerfile_maintainer:

3.2 MAINTAINER
--------------

    ``MAINTAINER <name>``

The ``MAINTAINER`` instruction allows you to set the *Author* field of
the generated images.

.. _dockerfile_run:

3.3 RUN
-------

    ``RUN <command>``

The ``RUN`` instruction will execute any commands on the current image
and commit the results. The resulting committed image will be used for
the next step in the Dockerfile.

Layering ``RUN`` instructions and generating commits conforms to the
core concepts of Docker where commits are cheap and containers can be
created from any point in an image's history, much like source
control.

Known Issues (RUN)
..................

* :issue:`783` is about file permissions problems that can occur when
  using the AUFS file system. You might notice it during an attempt to
  ``rm`` a file, for example. The issue describes a workaround.
* :issue:`2424` Locale will not be set automatically.

.. _dockerfile_cmd:

3.4 CMD
-------

CMD has three forms:

* ``CMD ["executable","param1","param2"]`` (like an *exec*, preferred form)
* ``CMD ["param1","param2"]`` (as *default parameters to ENTRYPOINT*)
* ``CMD command param1 param2`` (as a *shell*)

There can only be one CMD in a Dockerfile. If you list more than one
CMD then only the last CMD will take effect.

**The main purpose of a CMD is to provide defaults for an executing
container.** These defaults can include an executable, or they can
omit the executable, in which case you must specify an ENTRYPOINT as
well.

When used in the shell or exec formats, the ``CMD`` instruction sets
the command to be executed when running the image.  This is
functionally equivalent to running ``docker commit -run '{"Cmd":
<command>}'`` outside the builder.

If you use the *shell* form of the CMD, then the ``<command>`` will
execute in ``/bin/sh -c``:

.. code-block:: bash

    FROM ubuntu
    CMD echo "This is a test." | wc -

If you want to **run your** ``<command>`` **without a shell** then you
must express the command as a JSON array and give the full path to the
executable. **This array form is the preferred format of CMD.** Any
additional parameters must be individually expressed as strings in the
array:

.. code-block:: bash

    FROM ubuntu
    CMD ["/usr/bin/wc","--help"]

If you would like your container to run the same executable every
time, then you should consider using ``ENTRYPOINT`` in combination
with ``CMD``. See :ref:`dockerfile_entrypoint`.

If the user specifies arguments to ``docker run`` then they will
override the default specified in CMD.

.. note::
    Don't confuse ``RUN`` with ``CMD``. ``RUN`` actually runs a
    command and commits the result; ``CMD`` does not execute anything at
    build time, but specifies the intended command for the image.

.. _dockerfile_expose:

3.5 EXPOSE
----------

    ``EXPOSE <port> [<port>...]``

The ``EXPOSE`` instruction exposes ports for use within links. This is
functionally equivalent to running ``docker commit -run '{"PortSpecs":
["<port>", "<port2>"]}'`` outside the builder. Refer to
:ref:`port_redirection` for detailed information.

.. _dockerfile_env:

3.6 ENV
-------

    ``ENV <key> <value>``

The ``ENV`` instruction sets the environment variable ``<key>`` to the
value ``<value>``. This value will be passed to all future ``RUN``
instructions. This is functionally equivalent to prefixing the command
with ``<key>=<value>``

.. note::
    The environment variables will persist when a container is run
    from the resulting image.

.. _dockerfile_add:

3.7 ADD
-------

    ``ADD <src> <dest>``

The ``ADD`` instruction will copy new files from <src> and add them to
the container's filesystem at path ``<dest>``.

``<src>`` must be the path to a file or directory relative to the
source directory being built (also called the *context* of the build) or
a remote file URL.

``<dest>`` is the path at which the source will be copied in the
destination container.

All new files and directories are created with mode 0755, uid and gid
0.

.. note::
   if you build using STDIN (``docker build - < somefile``), there is no build 
   context, so the Dockerfile can only contain an URL based ADD statement.

The copy obeys the following rules:

* The ``<src>`` path must be inside the *context* of the build; you cannot 
  ``ADD ../something /something``, because the first step of a 
  ``docker build`` is to send the context directory (and subdirectories) to 
  the docker daemon.
* If ``<src>`` is a URL and ``<dest>`` does not end with a trailing slash,
  then a file is downloaded from the URL and copied to ``<dest>``.
* If ``<src>`` is a URL and ``<dest>`` does end with a trailing slash,
  then the filename is inferred from the URL and the file is downloaded to
  ``<dest>/<filename>``. For instance, ``ADD http://example.com/foobar /``
  would create the file ``/foobar``. The URL must have a nontrivial path
  so that an appropriate filename can be discovered in this case
  (``http://example.com`` will not work).
* If ``<src>`` is a directory, the entire directory is copied,
  including filesystem metadata.
* If ``<src>`` is a *local* tar archive in a recognized compression
  format (identity, gzip, bzip2 or xz) then it is unpacked as a
  directory. Resources from *remote* URLs are **not** decompressed.

  When a directory is copied or unpacked, it has the same behavior as
  ``tar -x``: the result is the union of

  1. whatever existed at the destination path and
  2. the contents of the source tree,

  with conflicts resolved in favor of "2." on a file-by-file basis.

* If ``<src>`` is any other kind of file, it is copied individually
  along with its metadata. In this case, if ``<dest>`` ends with a
  trailing slash ``/``, it will be considered a directory and the
  contents of ``<src>`` will be written at ``<dest>/base(<src>)``.
* If ``<dest>`` does not end with a trailing slash, it will be
  considered a regular file and the contents of ``<src>`` will be
  written at ``<dest>``.
* If ``<dest>`` doesn't exist, it is created along with all missing
  directories in its path.

.. _dockerfile_entrypoint:

3.8 ENTRYPOINT
--------------

ENTRYPOINT has two forms:

* ``ENTRYPOINT ["executable", "param1", "param2"]`` (like an *exec*,
  preferred form)
* ``ENTRYPOINT command param1 param2`` (as a *shell*)

There can only be one ``ENTRYPOINT`` in a Dockerfile. If you have more
than one ``ENTRYPOINT``, then only the last one in the Dockerfile will
have an effect.

An ``ENTRYPOINT`` helps you to configure a container that you can run
as an executable. That is, when you specify an ``ENTRYPOINT``, then
the whole container runs as if it was just that executable.

The ``ENTRYPOINT`` instruction adds an entry command that will **not**
be overwritten when arguments are passed to ``docker run``, unlike the
behavior of ``CMD``.  This allows arguments to be passed to the
entrypoint.  i.e. ``docker run <image> -d`` will pass the "-d"
argument to the ENTRYPOINT.

You can specify parameters either in the ENTRYPOINT JSON array (as in
"like an exec" above), or by using a CMD statement. Parameters in the
ENTRYPOINT will not be overridden by the ``docker run`` arguments, but
parameters specified via CMD will be overridden by ``docker run``
arguments.

Like a ``CMD``, you can specify a plain string for the ENTRYPOINT and
it will execute in ``/bin/sh -c``:

.. code-block:: bash

    FROM ubuntu
    ENTRYPOINT wc -l -

For example, that Dockerfile's image will *always* take stdin as input
("-") and print the number of lines ("-l"). If you wanted to make
this optional but default, you could use a CMD:

.. code-block:: bash

    FROM ubuntu
    CMD ["-l", "-"]
    ENTRYPOINT ["/usr/bin/wc"]

.. _dockerfile_volume:

3.9 VOLUME
----------

    ``VOLUME ["/data"]``

The ``VOLUME`` instruction will create a mount point with the specified name and mark it 
as holding externally mounted volumes from native host or other containers. For more information/examples 
and mounting instructions via docker client, refer to :ref:`volume_def` documentation. 

.. _dockerfile_user:

3.10 USER
---------

    ``USER daemon``

The ``USER`` instruction sets the username or UID to use when running
the image.

.. _dockerfile_workdir:

3.11 WORKDIR
------------

    ``WORKDIR /path/to/workdir``

The ``WORKDIR`` instruction sets the working directory in which
the command given by ``CMD`` is executed.

.. _dockerfile_examples:

4. Dockerfile Examples
======================

.. code-block:: bash

    # Nginx
    #
    # VERSION               0.0.1

    FROM      ubuntu
    MAINTAINER Guillaume J. Charmes <guillaume@dotcloud.com>

    # make sure the package repository is up to date
    RUN echo "deb http://archive.ubuntu.com/ubuntu precise main universe" > /etc/apt/sources.list
    RUN apt-get update

    RUN apt-get install -y inotify-tools nginx apache2 openssh-server

.. code-block:: bash

    # Firefox over VNC
    #
    # VERSION               0.3

    FROM ubuntu
    # make sure the package repository is up to date
    RUN echo "deb http://archive.ubuntu.com/ubuntu precise main universe" > /etc/apt/sources.list
    RUN apt-get update

    # Install vnc, xvfb in order to create a 'fake' display and firefox
    RUN apt-get install -y x11vnc xvfb firefox
    RUN mkdir /.vnc
    # Setup a password
    RUN x11vnc -storepasswd 1234 ~/.vnc/passwd
    # Autostart firefox (might not be the best way, but it does the trick)
    RUN bash -c 'echo "firefox" >> /.bashrc'

    EXPOSE 5900
    CMD    ["x11vnc", "-forever", "-usepw", "-create"]

.. code-block:: bash

    # Multiple images example
    #
    # VERSION               0.1

    FROM ubuntu
    RUN echo foo > bar
    # Will output something like ===> 907ad6c2736f

    FROM ubuntu
    RUN echo moo > oink
    # Will output something like ===> 695d7793cbe4

    # You'll now have two images, 907ad6c2736f with /bar, and 695d7793cbe4 with
    # /oink.
