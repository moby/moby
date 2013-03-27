:title: Base commands
:description: Common usage and commands
:keywords: Examples, Usage


Base commands
=============


Running an interactive shell
----------------------------

.. code-block:: bash

  # Download a base image
  docker import base

  # Run an interactive shell in the base image,
  # allocate a tty, attach stdin and stdout
  docker run -a -i -t base /bin/bash


Starting a long-running worker process
--------------------------------------

.. code-block:: bash

  # Run docker in daemon mode
  (docker -d || echo "Docker daemon already running") &

  # Start a very useful long-running process
  JOB=$(docker run base /bin/sh -c "while true; do echo Hello world!; sleep 1; done")

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
  JOB=$(docker run -p 4444 base /bin/nc -l -p 4444)

  # Which public port is NATed to my container?
  PORT=$(docker port $JOB 4444)

  # Connect to the public port via the host's public address
  echo hello world | nc $(hostname) $PORT

  # Verify that the network connection worked
  echo "Daemon received: $(docker logs $JOB)"

Continue to the complete `Command Line Interface`_

.. _Command Line Interface: ../commandline/cli.html
