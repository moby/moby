:title: Kill Command
:description: Kill a running container
:keywords: kill, container, docker, documentation

====================================
``kill`` -- Kill a running container
====================================

::

    Usage: docker kill [OPTIONS] CONTAINER [CONTAINER...]

    Kill a running container

Examples
--------

.. code-block:: bash

	$ docker ps -a | head
	ID                  IMAGE               COMMAND                CREATED             STATUS              PORTS
	9bbd19793b44        docker:latest       hack/dind /bin/hash    7 hours ago         Exit 1                                  
	3cbe352798b0        ubuntu:12.04        /bin/bash              26 hours ago        Up 26 hours                             
	$ docker kill 9bbd19793b44
	9bbd19793b44

	$ docker ps -a | head
	ID                  IMAGE               COMMAND                CREATED             STATUS              PORTS
	9bbd19793b44        docker:latest       hack/dind /bin/hash    7 hours ago         Exit 1                                  
	3cbe352798b0        ubuntu:12.04        /bin/bash              26 hours ago        Up 26 hours                             
	$ docker kill 3cbe352798b0
	3cbe352798b0

	$ docker ps -a | head
	ID                  IMAGE               COMMAND                CREATED             STATUS              PORTS
	9bbd19793b44        docker:latest       hack/dind /bin/hash    7 hours ago         Exit 1                                  
	3cbe352798b0        ubuntu:12.04        /bin/bash              26 hours ago        Exit 137  
	
	$docker_orig kill 3cbe352798b3
	Error: No such container: 3cbe352798b3

