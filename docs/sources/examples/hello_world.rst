:title: Hello world example
:description: A simple hello world example with Docker
:keywords: docker, example, hello world

.. _hello_world:

Hello World
===========

.. include:: example_header.inc

This is the most basic example available for using Docker.

Download the base container

.. code-block:: bash

    # Download an ubuntu image
    docker pull ubuntu

The *base* image is a minimal *ubuntu* based container, alternatively you can select *busybox*, a bare
minimal linux system. The images are retrieved from the docker repository.


.. code-block:: bash

    #run a simple echo command, that will echo hello world back to the console over standard out.
    docker run base /bin/echo hello world

**Explanation:**

- **"docker run"** run a command in a new container 
- **"base"** is the image we want to run the command inside of.
- **"/bin/echo"** is the command we want to run in the container
- **"hello world"** is the input for the echo command



**Video:**

See the example in action

.. raw:: html

    <div style="margin-top:10px;">
      <iframe width="560" height="350" src="http://ascii.io/a/2603/raw" frameborder="0"></iframe>
    </div>


Continue to the :ref:`hello_world_daemon` example.
