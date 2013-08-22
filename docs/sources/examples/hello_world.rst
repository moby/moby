:title: Hello world example
:description: A simple hello world example with Docker
:keywords: docker, example, hello world

.. _hello_world:

Hello World
===========

.. include:: example_header.inc

This is the most basic example available for using Docker.

Download the base image (named "ubuntu"):

.. code-block:: bash

    # Download an ubuntu image
    sudo docker pull ubuntu

Alternatively to the *ubuntu* image, you can select *busybox*, a bare
minimal Linux system. The images are retrieved from the Docker
repository.


.. code-block:: bash

    #run a simple echo command, that will echo hello world back to the console over standard out.
    sudo docker run ubuntu /bin/echo hello world

**Explanation:**

- **"sudo"** execute the following commands as user *root* 
- **"docker run"** run a command in a new container 
- **"ubuntu"** is the image we want to run the command inside of.
- **"/bin/echo"** is the command we want to run in the container
- **"hello world"** is the input for the echo command



**Video:**

See the example in action

.. raw:: html

    <div style="margin-top:10px;">
      <iframe width="560" height="350" src="http://ascii.io/a/2603/raw" frameborder="0"></iframe>
    </div>


Continue to the :ref:`hello_world_daemon` example.
