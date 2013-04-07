:title: Running the Examples
:description: An overview on how to run the docker examples
:keywords: docker, examples, how to

.. _running_examples:

Running The Examples
--------------------

There are two ways to run docker, daemon and standalone mode. 

When you run the docker command it will first check to see if there is already a docker daemon running in the background it can connect too, and if so, it will use that daemon to run all of the commands. 

If there is no daemon then docker will run in standalone mode. 

Docker needs to be run from a privileged account (root). Depending on which mode you are using, will determine how you need to execute docker.

1. The most common way is to run a docker daemon as root in the background, and then connect to it from the docker client from any account.

    .. code-block:: bash

        # starting docker daemon in the background
        $ sudo docker -d &
    
        # now you can run docker commands from any account.
        $ docker <command>

2. Standalone: You need to run every command as root, or using sudo

    .. code-block:: bash

        $ sudo docker <command>
