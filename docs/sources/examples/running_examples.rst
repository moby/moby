:title: Running the Examples
:description: An overview on how to run the docker examples
:keywords: docker, examples, how to

.. _running_examples:

Running the Examples
--------------------

All the examples assume your machine is running the docker daemon. To
run the docker daemon in the background, simply type:

   .. code-block:: bash

      sudo docker -d &

Now you can run docker in client mode: by defalt all commands will be
forwarded to the ``docker`` daemon via a protected Unix socket, so you
must run as root.

   .. code-block:: bash

      sudo docker help
