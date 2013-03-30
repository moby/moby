

Working with the repository
============================

Connecting to the repository
----------------------------

You create a user on the central docker repository by running

::

    docker login


If your username does not exist it will prompt you to also enter a password and your e-mail address. It will then
automatically log you in.


Committing a container to a named image
---------------------------------------

Committing containers to named images is not only usefull when committing to the repository. But in order to commit to
the repository it is required to have an image with your namespace.

The state of a container can be saved at any time by running

::

    docker commit <container_id>

However, it is probably more useful to commit it to a specific name

::

    docker commit <container_id> <your username>/some_name


Committing a container to the repository
-----------------------------------------

In order to push an image to the repository you need to have committed your container to a named image including your
repository username. e.g. by doing: docker commit <container_id> dhrp/nodejs

Now you can commit this image to the repository

::

    docker push image-name

    # for example docker push dhrp/nodejs

