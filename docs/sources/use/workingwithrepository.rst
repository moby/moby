:title: Share Images via Repositories
:description: Repositories allow users to share images.
:keywords: repo, repositories, usage, pull image, push image, image, documentation

.. _working_with_the_repository:

Share Images via Repositories
=============================

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

.. _searching_central_index:

Find Public Images on the Central Index
---------------------------------------

You can search the Central Index `online <https://index.docker.io>`_
or by the CLI. Searching can find images by name, user name or
description:

.. code-block:: bash

    $ sudo docker help search
    Usage: docker search NAME

    Search the docker index for images

      -notrunc=false: Don't truncate output
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

Pushing an image to its repository
----------------------------------

In order to push an image to its repository you need to have committed
your container to a named image (see above)

Now you can commit this image to the repository designated by its name
or tag.

.. code-block:: bash

    # format is "docker push <username>/<repo_name>"
    $ sudo docker push myname/kickassapp

.. _using_private_repositories:

Trusted Builds
--------------

Trusted Builds automate the building and updating of images from GitHub, directly 
on docker.io servers. It works by adding a commit hook to your selected repository,
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

Once you've created a Trusted Build you can deactive or delete it. You cannot
however push to a Trusted Build with the ``docker push`` command. You can only
manage it by committing code to your GitHub repository.

You can create multiple Trusted Builds per repository and configure them to
point to specific ``Dockerfile``'s or Git branches.

Private Repositories
--------------------

Right now (version 0.6), private repositories are only possible by
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
