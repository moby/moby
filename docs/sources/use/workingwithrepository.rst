:title: Working With Images And Repositories
:description: Repositories allow users to share images.
:keywords: repo, repositories, usage, pull image, push image, image, documentation

.. _working_with_the_repository:

Working With Images And Repositories
====================================

Introduction
------------

Docker :ref:`images<image_def>` can be tagged (e.g. ``ubuntu:latest``)
like a commit in Git and they can also be collected in *Docker repositories*
together as a shareable set. Each repository's name is a label that indicates 
lineage, showing who created it (e.g. ``crosbymichael/sentry``) and possibly 
original copy's location.

Repositories are hosted on *Docker Registries*. When declaring a repository
name, you can explicitly set a registry location, or use the default one.

The central (i.e., default) Docker registry is the *Docker Index*, located at
`index.docker.io <https://index.docker.io/>`_ where base images and those
that are shared by the Docker community exist, grouped by username. This
service, as a registry, also serves as a hub for authentication.

By joining free of charge, you can use the Docker Index to share your 
repositories *publicly*.  ``docker`` works as a client to talk with the registry 
services via ``search``, ``pull``, ``login`` and ``push``.

For private usage, you can always sign up for a plan at Docker Index. And if
you prefer, you can host your own `Docker Registry <https://github.com/dotcloud
/docker-registry>`_ as well. 

**In this article, we are going to learn:**

 - Local, public and private repositories;
 - Working with repositories;
 - Trusted builds, and;
 - Authentication files.

Local Repositories
------------------

Docker images which have been created and labeled on your local Docker server
need to be pushed to a Public or Private registry to be shared.

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

.. _searching_central_index:

Find Public Images on the Central Index
---------------------------------------

You can search the Central Index `online <https://index.docker.io>`_
or using the command line interface. Searching can find images by name, user
name or description:

.. code-block:: bash

    $ sudo docker help search
    Usage: docker search NAME

    Search the docker index for images

      --no-trunc=false: Don't truncate output
    $ sudo docker search centos
    Found 25 results matching your query ("centos")
    NAME                             DESCRIPTION
    centos                           
    slantview/centos-chef-solo       CentOS 6.4 with chef-solo.
    ...

There you can see two example results: ``centos`` and
``slantview/centos-chef-solo``. The second result shows that it comes
from the public repository of a user, ``slantview/``, while the first
result (``centos``) doesn't explicitly list a repository so it comes
from the trusted Central Repository. The ``/`` character separates a
user's repository and the image name.

Once you have found the image name, you can download it:

.. code-block:: bash

    # sudo docker pull <value>
    $ sudo docker pull centos
    Pulling repository centos
    539c0211cd76: Download complete

What can you do with that image? Check out the :ref:`example_list`
and, when you're ready with your own image, come back here to learn
how to share it.

Contributing to the Central Registry
------------------------------------

Anyone can pull public images from the Central Registry, but if you
would like to share one of your own images, then you must register a
unique user name first. You can create your username and login on the
`central Docker Index online
<https://index.docker.io/account/signup/>`_, or by running

.. code-block:: bash

    sudo docker login

This will prompt you for a username, which will become a public
namespace for your public repositories.

If your username is available then ``docker`` will also prompt you to
enter a password and your e-mail address. It will then automatically
log you in. Now you're ready to commit and push your own images!

.. _container_commit:

Committing a Container to a Named Image
---------------------------------------

When you make changes to an existing image, those changes get saved to
a container's file system. You can then promote that container to
become an image by making a ``commit``. In addition to converting the
container to an image, this is also your opportunity to name the
image, specifically a name that includes your user name from the
Central Docker Index (as you did a ``login`` above) and a meaningful
name for the image.

.. code-block:: bash

    # format is "sudo docker commit <container_id> <username>/<imagename>"
    $ sudo docker commit $CONTAINER_ID myname/kickassapp

.. _image_push:

Pushing a repository to its registry
------------------------------------

In order to push an repository to its registry you need to have named an image, 
or committed your container to a named image (see above)

Now you can push this repository to the registry designated by its name
or tag.

.. code-block:: bash

    # format is "docker push <username>/<repo_name>"
    $ sudo docker push myname/kickassapp

.. _using_private_repositories:

Trusted Builds
--------------

Trusted Builds automate the building and updating of images from GitHub, directly 
on ``docker.io`` servers. It works by adding a commit hook to your selected repository,
triggering a build and update when you push a commit.

To setup a trusted build
++++++++++++++++++++++++

#. Create a `Docker Index account <https://index.docker.io/>`_ and login.
#. Link your GitHub account through the ``Link Accounts`` menu.
#. `Configure a Trusted build <https://index.docker.io/builds/>`_.
#. Pick a GitHub project that has a ``Dockerfile`` that you want to build.
#. Pick the branch you want to build (the default is the  ``master`` branch).
#. Give the Trusted Build a name.
#. Assign an optional Docker tag to the Build.
#. Specify where the ``Dockerfile`` is located. The default is ``/``.

Once the Trusted Build is configured it will automatically trigger a build, and
in a few minutes, if there are no errors, you will see your new trusted build
on the Docker Index. It will will stay in sync with your GitHub repo until you
deactivate the Trusted Build.

If you want to see the status of your Trusted Builds you can go to your
`Trusted Builds page <https://index.docker.io/builds/>`_ on the Docker index,
and it will show you the status of your builds, and the build history.

Once you've created a Trusted Build you can deactivate or delete it. You cannot
however push to a Trusted Build with the ``docker push`` command. You can only
manage it by committing code to your GitHub repository.

You can create multiple Trusted Builds per repository and configure them to
point to specific ``Dockerfile``'s or Git branches.

Private Registry
----------------

Docker registries allow you - and those you choose - to exclusively create
and host nonpublic Docker images in private Docker repositories.

There are two ways you can start working privately:

 - By `signing up <https://index.docker.io/plans/>`_ for the **Docker Index**
   managed by `Docker IO <https://www.docker.io/>`_, or; 
 - By hosting your own `Docker registry <https://github.com/dotcloud/docker-registry>`_. 

To push or pull to a repository on your own registry, you must prefix the tag with the
address of the registry's host (a ``.`` or ``:`` is used to identify a host), 
like this:

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

.. raw:: html

   <iframe width="640" height="360"
   src="//www.youtube.com/embed/CAewZCBT4PI?rel=0" frameborder="0"
   allowfullscreen></iframe>

.. seealso:: `Docker Blog: How to use your own registry 
   <http://blog.docker.io/2013/07/how-to-use-your-own-registry/>`_

Authentication file
-------------------

The authentication is stored in a json file, ``.dockercfg`` located in your
home directory. It supports multiple registry urls.

``docker login`` will create the "https://index.docker.io/v1/" key.

``docker login https://my-registry.com`` will create the "https://my-registry.com" key.

For example:

.. code-block:: json

   {
	"https://index.docker.io/v1/": {
		"auth": "xXxXxXxXxXx=",
		"email": "email@example.com"
	},
	"https://my-registry.com": {
		"auth": "XxXxXxXxXxX=",
		"email": "email@my-registry.com"
	}
   }

The ``auth`` field represents ``base64(<username>:<password>)``
