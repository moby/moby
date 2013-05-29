:title: Basic Commands
:description: Common usage and commands
:keywords: Examples, Usage, basic commands, docker, documentation, examples


The basics
=============

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

  # Download a base image
  docker pull base

  # Run an interactive shell in the base image,
  # allocate a tty, attach stdin and stdout
  docker run -i -t base /bin/bash

Bind Docker to another host/port
--------------------------------

If you want Docker to listen to another port and bind to another ip
use -host and -port on both deamon and client

.. code-block:: bash

   # Run docker in daemon mode
   sudo <path to>/docker -H 0.0.0.0:5555 &
   # Download a base image
   docker -H :5555 pull base


Starting a long-running worker process
--------------------------------------

.. code-block:: bash

  # Start a very useful long-running process
  JOB=$(docker run -d base /bin/sh -c "while true; do echo Hello world; sleep 1; done")

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
  JOB=$(docker run -d -p 4444 base /bin/nc -l -p 4444)

  # Which public port is NATed to my container?
  PORT=$(docker port $JOB 4444)

  # Connect to the public port via the host's public address
  # Please note that because of how routing works connecting to localhost or 127.0.0.1 $PORT will not work.
  IP=$(ifconfig eth0 | perl -n -e 'if (m/inet addr:([\d\.]+)/g) { print $1 }')
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

