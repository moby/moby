:title: Basic Commands
:description: Common usage and commands
:keywords: Examples, Usage, basic commands, docker, documentation, examples


The Basics
==========

Starting Docker
---------------

If you have used one of the quick install paths', Docker may have been installed with upstart, Ubuntu's
system for starting processes at boot time. You should be able to run ``docker help`` and get output.

If you get ``docker: command not found`` or something like ``/var/lib/docker/repositories: permission denied``
you will need to specify the path to it and manually start it.

.. code-block:: bash

    # Run docker in daemon mode
    sudo <path to>/docker -d &


Running an interactive shell
----------------------------

.. code-block:: bash

  # Download an ubuntu image
  docker pull ubuntu

  # Run an interactive shell in the ubuntu image,
  # allocate a tty, attach stdin and stdout
  docker run -i -t ubuntu /bin/bash

Bind Docker to another host/port or a unix socket
-------------------------------------------------

With -H it is possible to make the Docker daemon to listen on a specific ip and port. By default, it will listen on 127.0.0.1:4243 to allow only local connections but you can set it to 0.0.0.0:4243 or a specific host ip to give access to everybody.

Similarly, the Docker client can use -H to connect to a custom port.

-H accepts host and port assignment in the following format: tcp://[host][:port] or unix://path
For example:

* tcp://host -> tcp connection on host:4243
* tcp://host:port -> tcp connection on host:port
* tcp://:port -> tcp connection on 127.0.0.1:port
* unix://path/to/socket -> unix socket located at path/to/socket

.. code-block:: bash

   # Run docker in daemon mode
   sudo <path to>/docker -H 0.0.0.0:5555 -d &
   # Download an ubuntu image
   docker -H :5555 pull ubuntu

You can use multiple -H, for example, if you want to listen
on both tcp and a unix socket

.. code-block:: bash

   # Run docker in daemon mode
   sudo <path to>/docker -H tcp://127.0.0.1:4243 -H unix:///var/run/docker.sock -d &
   # Download an ubuntu image
   docker pull ubuntu
   # OR
   docker -H unix:///var/run/docker.sock pull ubuntu

Starting a long-running worker process
--------------------------------------

.. code-block:: bash

  # Start a very useful long-running process
  JOB=$(docker run -d ubuntu /bin/sh -c "while true; do echo Hello world; sleep 1; done")

  # Collect the output of the job so far
  docker logs $JOB

  # Kill the job
  docker kill $JOB


Listing all running containers
------------------------------

.. code-block:: bash

  docker ps

Expose a service on a TCP port
------------------------------

.. code-block:: bash

  # Expose port 4444 of this container, and tell netcat to listen on it
  JOB=$(docker run -d -p 4444 ubuntu /bin/nc -l -p 4444)

  # Which public port is NATed to my container?
  PORT=$(docker port $JOB 4444)

  # Connect to the public port via the host's public address
  # Please note that because of how routing works connecting to localhost or 127.0.0.1 $PORT will not work.
  # Replace *eth0* according to your local interface name.
  IP=$(ip -o -4 addr list eth0 | perl -n -e 'if (m{inet\s([\d\.]+)\/\d+\s}xms) { print $1 }')
  echo hello world | nc $IP $PORT

  # Verify that the network connection worked
  echo "Daemon received: $(docker logs $JOB)"


Committing (saving) a container state
-------------------------------------

Save your containers state to a container image, so the state can be re-used.

When you commit your container only the differences between the image the container was created from
and the current state of the container will be stored (as a diff). See which images you already have
using ``docker images``

.. code-block:: bash

    # Commit your container to a new named image
    docker commit <container_id> <some_name>

    # List your containers
    docker images

You now have a image state from which you can create new instances.



Read more about :ref:`working_with_the_repository` or continue to the complete :ref:`cli`

