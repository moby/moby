:title: Learn Basic Commands
:description: Common usage and commands
:keywords: Examples, Usage, basic commands, docker, documentation, examples


Learn Basic Commands
====================

Starting Docker
---------------

If you have used one of the quick install paths', Docker may have been
installed with upstart, Ubuntu's system for starting processes at boot
time. You should be able to run ``sudo docker help`` and get output.

If you get ``docker: command not found`` or something like
``/var/lib/docker/repositories: permission denied`` you will need to
specify the path to it and manually start it.

.. code-block:: bash

    # Run docker in daemon mode
    sudo <path to>/docker -d &

Download a pre-built image
--------------------------

.. code-block:: bash

  # Download an ubuntu image
  sudo docker pull ubuntu

This will find the ``ubuntu`` image by name in the :ref:`Central Index 
<searching_central_index>` and download it from the top-level Central 
Repository to a local image cache.

.. NOTE:: When the image has successfully downloaded, you will see a
   12 character hash ``539c0211cd76: Download complete`` which is the
   short form of the image ID. These short image IDs are the first 12
   characters of the full image ID - which can be found using ``docker
   inspect`` or ``docker images -notrunc=true``

Running an interactive shell
----------------------------

.. code-block:: bash

  # Run an interactive shell in the ubuntu image,
  # allocate a tty, attach stdin and stdout
  # To detach the tty without exiting the shell,
  # use the escape sequence Ctrl-p + Ctrl-q
  sudo docker run -i -t ubuntu /bin/bash

.. _dockergroup:

sudo and the docker Group
-------------------------

The ``docker`` daemon always runs as root, and since ``docker``
version 0.5.2, ``docker`` binds to a Unix socket instead of a TCP
port. By default that Unix socket is owned by the user *root*, and so,
by default, you can access it with ``sudo``.

Starting in version 0.5.3, if you (or your Docker installer) create a
Unix group called *docker* and add users to it, then the ``docker``
daemon will make the ownership of the Unix socket read/writable by the
*docker* group when the daemon starts. The ``docker`` daemon must
always run as root, but if you run the ``docker`` client as a user in
the *docker* group then you don't need to add ``sudo`` to all the
client commands.  Warning: the *docker* group is root-equivalent.

**Example:**

.. code-block:: bash

  # Add the docker group if it doesn't already exist.
  sudo groupadd docker

  # Add the connected user "${USERNAME}" to the docker group.
  # Change the user name to match your preferred user.
  # You may have to logout and log back in again for
  # this to take effect.
  sudo gpasswd -a ${USERNAME} docker

  # Restart the docker daemon.
  sudo service docker restart

.. _bind_docker:

Bind Docker to another host/port or a Unix socket
-------------------------------------------------

.. warning:: Changing the default ``docker`` daemon binding to a TCP
   port or Unix *docker* user group will increase your security risks
   by allowing non-root users to potentially gain *root* access on the
   host (`e.g. #1369
   <https://github.com/dotcloud/docker/issues/1369>`_). Make sure you
   control access to ``docker``.

With -H it is possible to make the Docker daemon to listen on a
specific ip and port. By default, it will listen on
``unix:///var/run/docker.sock`` to allow only local connections by the
*root* user.  You *could* set it to 0.0.0.0:4243 or a specific host ip to
give access to everybody, but that is **not recommended** because then
it is trivial for someone to gain root access to the host where the
daemon is running.

Similarly, the Docker client can use ``-H`` to connect to a custom port.

``-H`` accepts host and port assignment in the following format:
``tcp://[host][:port]`` or ``unix://path``

For example:

* ``tcp://host:4243`` -> tcp connection on host:4243
* ``unix://path/to/socket`` -> unix socket located at ``path/to/socket``

.. code-block:: bash

   # Run docker in daemon mode
   sudo <path to>/docker -H 0.0.0.0:5555 -d &
   # Download an ubuntu image
   sudo docker -H :5555 pull ubuntu

You can use multiple ``-H``, for example, if you want to listen on
both TCP and a Unix socket

.. code-block:: bash

   # Run docker in daemon mode
   sudo <path to>/docker -H tcp://127.0.0.1:4243 -H unix:///var/run/docker.sock -d &
   # Download an ubuntu image, use default Unix socket
   sudo docker pull ubuntu
   # OR use the TCP port
   sudo docker -H tcp://127.0.0.1:4243 pull ubuntu

Starting a long-running worker process
--------------------------------------

.. code-block:: bash

  # Start a very useful long-running process
  JOB=$(sudo docker run -d ubuntu /bin/sh -c "while true; do echo Hello world; sleep 1; done")

  # Collect the output of the job so far
  sudo docker logs $JOB

  # Kill the job
  sudo docker kill $JOB


Listing all running containers
------------------------------

.. code-block:: bash

  sudo docker ps

Bind a service on a TCP port
------------------------------

.. code-block:: bash

  # Bind port 4444 of this container, and tell netcat to listen on it
  JOB=$(sudo docker run -d -p 4444 ubuntu:12.10 /bin/nc -l 4444)

  # Which public port is NATed to my container?
  PORT=$(sudo docker port $JOB 4444 | awk -F: '{ print $2 }')

  # Connect to the public port
  echo hello world | nc 127.0.0.1 $PORT

  # Verify that the network connection worked
  echo "Daemon received: $(sudo docker logs $JOB)"


Committing (saving) a container state
-------------------------------------

Save your containers state to a container image, so the state can be re-used.

When you commit your container only the differences between the image
the container was created from and the current state of the container
will be stored (as a diff). See which images you already have using
``sudo docker images``

.. code-block:: bash

    # Commit your container to a new named image
    sudo docker commit <container_id> <some_name>

    # List your containers
    sudo docker images

You now have a image state from which you can create new instances.



Read more about :ref:`working_with_the_repository` or continue to the
complete :ref:`cli`
