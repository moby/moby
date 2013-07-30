:title: Remote API
:description: API Documentation for Docker
:keywords: API, Docker, rcli, REST, documentation

.. COMMENT use http://pythonhosted.org/sphinxcontrib-httpdomain/ to
.. document the REST API.

=================
Docker Remote API
=================

.. contents:: Table of Contents

1. Brief introduction
=====================

- The Remote API is replacing rcli
- Default port in the docker deamon is 4243 
- The API tends to be REST, but for some complex commands, like attach
  or pull, the HTTP connection is hijacked to transport stdout stdin
  and stderr
- Since API version 1.2, the auth configuration is now handled client
  side, so the client has to send the authConfig as POST in
  /images/(name)/push

2. Versions
===========

The current verson of the API is 1.4

Calling /images/<name>/insert is the same as calling
/v1.4/images/<name>/insert 

You can still call an old version of the api using
/v1.0/images/<name>/insert

:doc:`docker_remote_api_v1.4`
*****************************

What's new
----------

.. http:get:: /containers/(id)/top

   **New!** You can now use ps args with docker top, like `docker top <container_id> aux`

:doc:`docker_remote_api_v1.3`
*****************************

docker v0.5.0 51f6c4a_

What's new
----------

.. http:get:: /containers/(id)/top

   List the processes running inside a container.

.. http:get:: /events:

   **New!** Monitor docker's events via streaming or via polling

Builder (/build):

- Simplify the upload of the build context
- Simply stream a tarball instead of multipart upload with 4
  intermediary buffers
- Simpler, less memory usage, less disk usage and faster

.. Warning::

  The /build improvements are not reverse-compatible. Pre 1.3 clients
  will break on /build.

List containers (/containers/json):

- You can use size=1 to get the size of the containers

Start containers (/containers/<id>/start):

- You can now pass host-specific configuration (e.g. bind mounts) in
  the POST body for start calls

:doc:`docker_remote_api_v1.2`
*****************************

docker v0.4.2 2e7649b_

What's new
----------

The auth configuration is now handled by the client.

The client should send it's authConfig as POST on each call of
/images/(name)/push

.. http:get:: /auth 

  **Deprecated.**

.. http:post:: /auth 

  Only checks the configuration but doesn't store it on the server

  Deleting an image is now improved, will only untag the image if it
  has chidren and remove all the untagged parents if has any.

.. http:post:: /images/<name>/delete 

  Now returns a JSON structure with the list of images
  deleted/untagged.


:doc:`docker_remote_api_v1.1`
*****************************

docker v0.4.0 a8ae398_

What's new
----------

.. http:post:: /images/create
.. http:post:: /images/(name)/insert
.. http:post:: /images/(name)/push

   Uses json stream instead of HTML hijack, it looks like this:

        .. sourcecode:: http

           HTTP/1.1 200 OK
	   Content-Type: application/json

	   {"status":"Pushing..."}
	   {"status":"Pushing", "progress":"1/? (n/a)"}
	   {"error":"Invalid..."}
	   ...


:doc:`docker_remote_api_v1.0`
*****************************

docker v0.3.4 8d73740_

What's new
----------

Initial version


.. _a8ae398: https://github.com/dotcloud/docker/commit/a8ae398bf52e97148ee7bd0d5868de2e15bd297f
.. _8d73740: https://github.com/dotcloud/docker/commit/8d73740343778651c09160cde9661f5f387b36f4
.. _2e7649b: https://github.com/dotcloud/docker/commit/2e7649beda7c820793bd46766cbc2cfeace7b168
.. _51f6c4a: https://github.com/dotcloud/docker/commit/51f6c4a7372450d164c61e0054daf0223ddbd909

==================================
Docker Remote API Client Libraries
==================================

These libraries have been not tested by the Docker Maintainers for
compatibility. Please file issues with the library owners.  If you
find more library implementations, please list them in Docker doc bugs
and we will add the libraries here.

+----------------------+----------------+--------------------------------------------+
| Language/Framework   | Name           | Repository                                 |
+======================+================+============================================+
| Python               | docker-py      | https://github.com/dotcloud/docker-py      |
+----------------------+----------------+--------------------------------------------+
| Ruby                 | docker-ruby    | https://github.com/ActiveState/docker-ruby |
+----------------------+----------------+--------------------------------------------+
| Ruby                 | docker-client  | https://github.com/geku/docker-client      |
+----------------------+----------------+--------------------------------------------+
| Ruby                 | docker-api     | https://github.com/swipely/docker-api      |
+----------------------+----------------+--------------------------------------------+
| Javascript           | docker-js      | https://github.com/dgoujard/docker-js      |
+----------------------+----------------+--------------------------------------------+
| Javascript (Angular) | dockerui       | https://github.com/crosbymichael/dockerui  |
| **WebUI**            |                |                                            |
+----------------------+----------------+--------------------------------------------+
| Java                 | docker-java    | https://github.com/kpelykh/docker-java     |
+----------------------+----------------+--------------------------------------------+

