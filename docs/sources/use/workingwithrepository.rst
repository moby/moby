:title: Working With Repositories
:description: Generally, there are two types of repositories: Top-level repositories which are controlled by the people behind Docker, and user repositories.
:keywords: repo, repositiores, usage, pull image, push image, image, documentation

.. _working_with_the_repository:

Working with the repository
============================


Top-level repositories and user repositories
--------------------------------------------

Generally, there are two types of repositories: Top-level repositories which are controlled by the people behind
Docker, and user repositories.

* Top-level repositories can easily be recognized by not having a / (slash) in their name. These repositories can
  generally be trusted.
* User repositories always come in the form of <username>/<repo_name>. This is what your published images will look like.
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

