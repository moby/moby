:title: Running an SSH service
:description: Installing and running an sshd service
:keywords: docker, example, package installation, networking

.. _running_ssh_service:

SSH Daemon Service
==================

.. include:: example_header.inc

The following Dockerfile sets up an sshd service in a container that you can use
to connect to and inspect other container's volumes, or to get quick access to a
test container.

.. literalinclude:: running_ssh_service.Dockerfile

Build the image using:

.. code-block:: bash

    $ sudo docker build -t eg_sshd .

Then run it. You can then use ``docker port`` to find out what host port the container's
port 22 is mapped to:

.. code-block:: bash

    $ sudo docker run -d -P --name test_sshd eg_sshd
    $ sudo docker port test_sshd 22
    0.0.0.0:49154

And now you can ssh to port ``49154`` on the Docker daemon's host IP address 
(``ip address`` or ``ifconfig`` can tell you that):

.. code-block:: bash

    $ ssh root@192.168.1.2 -p 49154
    # The password is ``screencast``.
    $$

Finally, clean up after your test by stopping and removing the container, and
then removing the image.

.. code-block:: bash

    $ sudo docker stop test_sshd
    $ sudo docker rm test_sshd
    $ sudo docker rmi eg_sshd
