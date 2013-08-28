:title: Working With Repositories
:description: Repositories allow users to share images.
:keywords: repo, repositories, usage, pull image, push image, image, documentation

.. _working_with_the_repository:

Working with Repositories
=========================

A *repository* is a hosted collection of tagged :ref:`images
<image_def>` that together create the file system for a container. The
repository's name is a tag that indicates the provenance of the
repository, i.e. who created it and where the original copy is
located.

You can find one or more repositories hosted on a *registry*. There
can be an implicit or explicit host name as part of the repository
tag. The implicit registry is located at ``index.docker.io``, the home
of "top-level" repositories and the Central Index. This registry may
also include public "user" repositories.

So Docker is not only a tool for creating and managing your own
:ref:`containers <container_def>` -- **Docker is also a tool for
sharing**. The Docker project provides a Central Registry to host
public repositories, namespaced by user, and a Central Index which
provides user authentication and search over all the public
repositories. You can host your own Registry too! Docker acts as a
client for these services via ``docker search, pull, login`` and
``push``.

.. _using_public_repositories:

Public Repositories
-------------------

There are two types of public repositories: *top-level* repositories
which are controlled by the Docker team, and *user* repositories
created by individual contributors. Anyone can read from these
repositories -- they really help people get started quickly! You could
also use :ref:`using_private_repositories` if you need to keep control
of who accesses your images, but we will only refer to public
repositories in these examples.

* Top-level repositories can easily be recognized by **not** having a
  ``/`` (slash) in their name. These repositories can generally be
  trusted.
* User repositories always come in the form of
  ``<username>/<repo_name>``. This is what your published images will
  look like if you push to the public Central Registry.
* Only the authenticated user can push to their *username* namespace
  on the Central Registry.
* User images are not checked, it is therefore up to you whether or
  not you trust the creator of this image.

Find public images available on the Central Index
-------------------------------------------------

Search by name, namespace or description

.. code-block:: bash

    sudo docker search <value>


Download them simply by their name

.. code-block:: bash

    sudo docker pull <value>


Very similarly you can search for and browse the index online on
https://index.docker.io


Connecting to the Central Registry
----------------------------------

You can create a user on the central Docker Index online, or by running

.. code-block:: bash

    sudo docker login

This will prompt you for a username, which will become a public
namespace for your public repositories.

If your username does not exist it will prompt you to also enter a
password and your e-mail address. It will then automatically log you
in.

.. _container_commit:

Committing a container to a named image
---------------------------------------

In order to commit to the repository it is required to have committed
your container to an image within your username namespace.

.. code-block:: bash

    # for example docker commit $CONTAINER_ID dhrp/kickassapp
    sudo docker commit <container_id> <username>/<repo_name>

.. _image_push:

Pushing an image to its repository
----------------------------------

In order to push an image to its repository you need to have committed
your container to a named image (see above)

Now you can commit this image to the repository designated by its name
or tag.

.. code-block:: bash

    # for example docker push dhrp/kickassapp
    sudo docker push <username>/<repo_name>

.. _using_private_repositories:

Private Repositories
--------------------

Right now (version 0.5), private repositories are only possible by
hosting `your own registry
<https://github.com/dotcloud/docker-registry>`_.  To push or pull to a
repository on your own registry, you must prefix the tag with the
address of the registry's host, like this:

.. code-block:: bash

    # Tag to create a repository with the full registry location.
    # The location (e.g. localhost.localdomain:5000) becomes
    # a permanent part of the repository name
    sudo docker tag 0u812deadbeef localhost.localdomain:5000/repo_name

    # Push the new repository to its home location on localhost
    sudo docker push localhost.localdomain:5000/repo_name

Once a repository has your registry's host name as part of the tag,
you can push and pull it like any other repository, but it will
**not** be searchable (or indexed at all) in the Central Index, and
there will be no user name checking performed. Your registry will
function completely independently from the Central Index.
