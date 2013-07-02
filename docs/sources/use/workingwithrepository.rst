:title: Working With Repositories
:description: Repositories allow users to share images.
:keywords: repo, repositiores, usage, pull image, push image, image, documentation

.. _working_with_the_repository:

Working with Repositories
=========================


Top-level repositories and user repositories
--------------------------------------------

Generally, there are two types of repositories: Top-level repositories
which are controlled by the people behind Docker, and user
repositories.

* Top-level repositories can easily be recognized by not having a ``/`` (slash) in their name. These repositories can  generally be trusted.
* User repositories always come in the form of ``<username>/<repo_name>``. This is what your published images will look like.
* User images are not checked, it is therefore up to you whether or not you trust the creator of this image.


Find public images available on the index
-----------------------------------------

Seach by name, namespace or description

.. code-block:: bash

    docker search <value>


Download them simply by their name

.. code-block:: bash

    docker pull <value>


Very similarly you can search for and browse the index online on https://index.docker.io


Connecting to the repository
----------------------------

You can create a user on the central docker repository online, or by running

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


Changing the server to connect to
----------------------------------

When you are running your own index and/or registry, You can change the server the docker client will connect to.

Variable
^^^^^^^^

.. code-block:: sh

    DOCKER_INDEX_URL

Setting this environment variable on the docker server will change the URL docker index.
This address is used in commands such as ``docker login``, ``docker push`` and ``docker pull``.
The docker daemon doesn't need to be restarted for this parameter to take effect.

Example
^^^^^^^

.. code-block:: sh

    docker -d &
    export DOCKER_INDEX_URL="https://index.docker.io"

