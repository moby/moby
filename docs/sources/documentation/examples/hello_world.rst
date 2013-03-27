:title: Hello world example
:description: A simple hello world example with Docker
:keywords: docker, example, hello world

.. _hello_world:

Hello World
===========
This is the most basic example available for using docker

This example assumes you have Docker installed and it will download the busybox image and then use that image to run a simple echo command, that will echo hello world back to the console over standard out.

.. code-block:: bash

    $ docker run busybox /bin/echo hello world

**Explanation:**

- **"docker run"** run a command in a new container 
- **"busybox"** is the image we want to run the command inside of.
- **"/bin/echo"** is the command we want to run in the container
- **"hello world"** is the input for the echo command

**Video:**

See the example in action

.. raw:: html

    <div style="margin-top:10px;">
      <iframe width="560" height="350" src="http://ascii.io/a/2561/raw" frameborder="0"></iframe>
    </div>

Continue to the :ref:`hello_world_daemon` example.