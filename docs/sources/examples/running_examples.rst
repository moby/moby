:title: Running the Examples
:description: An overview on how to run the docker examples
:keywords: docker, examples, how to

.. _running_examples:

Running The Examples
--------------------

There are two ways to run docker, daemon mode and standalone mode.

When you run the docker command it will first check if there is a docker daemon running in the background it can connect to.

* If it exists it will use that daemon to run all of the commands.
* If it does not exist docker will run in standalone mode (docker will exit after each command).

Docker needs to be run from a privileged account (root).

1. The most common (and recommended) way is to run a docker daemon as root in the background, and then connect to it from the docker client from any account.

   .. code-block:: bash

      # starting docker daemon in the background
      sudo docker -d &

      # now you can run docker commands from any account.
      docker <command>

2. Standalone: You need to run every command as root, or using sudo

   .. code-block:: bash

       sudo docker <command>
