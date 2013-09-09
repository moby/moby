:title: Hello world daemon example
:description: A simple hello world daemon example with Docker
:keywords: docker, example, hello world, daemon

.. _hello_world_daemon:

Hello World Daemon
==================

.. include:: example_header.inc

The most boring daemon ever written.

This example assumes you have Docker installed and the Ubuntu
image already imported with ``docker pull ubuntu``.  We will use the Ubuntu
image to run a simple hello world daemon that will just print hello
world to standard out every second. It will continue to do this until
we stop it.

**Steps:**

.. code-block:: bash

    CONTAINER_ID=$(sudo docker run -d ubuntu /bin/sh -c "while true; do echo hello world; sleep 1; done")

We are going to run a simple hello world daemon in a new container
made from the *ubuntu* image.

- **"docker run -d "** run a command in a new container. We pass "-d"
  so it runs as a daemon.
- **"ubuntu"** is the image we want to run the command inside of.
- **"/bin/sh -c"** is the command we want to run in the container
- **"while true; do echo hello world; sleep 1; done"** is the mini
  script we want to run, that will just print hello world once a
  second until we stop it.
- **$CONTAINER_ID** the output of the run command will return a
  container id, we can use in future commands to see what is going on
  with this process.

.. code-block:: bash

    sudo docker logs $CONTAINER_ID

Check the logs make sure it is working correctly.

- **"docker logs**" This will return the logs for a container
- **$CONTAINER_ID** The Id of the container we want the logs for.

.. code-block:: bash

    sudo docker attach $CONTAINER_ID

Attach to the container to see the results in realtime.

- **"docker attach**" This will allow us to attach to a background
  process to see what is going on.
- **$CONTAINER_ID** The Id of the container we want to attach too.

Exit from the container attachment by pressing Control-C.

.. code-block:: bash

    sudo docker ps

Check the process list to make sure it is running.

- **"docker ps"** this shows all running process managed by docker

.. code-block:: bash

    sudo docker stop $CONTAINER_ID

Stop the container, since we don't need it anymore.

- **"docker stop"** This stops a container
- **$CONTAINER_ID** The Id of the container we want to stop.

.. code-block:: bash

    sudo docker ps

Make sure it is really stopped.


**Video:**

See the example in action

.. raw:: html

    <div style="margin-top:10px;">
      <iframe width="560" height="350" src="http://ascii.io/a/2562/raw" frameborder="0"></iframe>
    </div>

Continue to the :ref:`python_web_app` example.
