:title: Repository
:description: Definition of an Repository
:keywords: containers, lxc, concepts, explanation, image, repository, container

.. _repository_def:

Repository
==========

A repository is a tagged set of images either on your local docker server, or
shared, by pushing it to a :ref:`Registry<registry_def>` server.

Images can be labeld into a repository using ``sudo docker build -t LABEL``, 
``sudo docker commit CONTAINERID LABEL`` or ``sudo docker tag IMAGEID LABEL``.

The label can be made up of 3 parts:

[registry_hostname[:port]/][user_name/]( repository_name[:version_tag] | image_id )
[REGISTRYHOST/][USERNAME/]NAME[:TAG]

TAG defaults to ``latest``, USERNAME and REGISTRYHOST default to an empty string.
When REGISTRYHOST is an empty string, then ``docker push`` will push to ``index.docker.io:80``.

If you create a new repository which you want to share, you will need to set the 
first part, as the 'default' blank REPOSITORY prefix is reserved for official Docker images.

For more information see :ref:`Working with Repositories<working_with_the_repository>`
