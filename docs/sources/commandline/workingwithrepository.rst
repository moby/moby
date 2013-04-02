.. _working_with_the_repository:

Working with the repository
============================

Connecting to the repository
----------------------------

You create a user on the central docker repository by running

.. code-block:: bash

    docker login


If your username does not exist it will prompt you to also enter a password and your e-mail address. It will then
automatically log you in.


Committing a container to a named image
---------------------------------------

In order to commit to the repository it is required to have committed your container to an image with your namespace.

.. code-block:: bash

    # for example docker commit $CONTAINER_ID dhrp/kickassapp
    docker commit <container_id> <your username>/<some_name>


Pushing a container to the repository
-----------------------------------------

In order to push an image to the repository you need to have committed your container to a named image (see above)

Now you can commit this image to the repository

.. code-block:: bash

    # for example docker push dhrp/kickassapp
    docker push <image-name>

