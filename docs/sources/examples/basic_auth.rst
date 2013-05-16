:title: Set up Basic Auth
:description: An overview on how to set up basic auth in docker
:keywords: docker, examples, how to

.. _basic_auth:

Basic Auth
----------

To run the docker daemon in the background with basic auth, simply add the -auth parameter:

   .. code-block:: bash

      sudo docker -auth mytoken -d &

Now you can run docker in client mode: all commands will be forwarded to the docker daemon, so the client also need the token.

   .. code-block:: bash

      docker -auth mytoken images -a