:title: Remote API
:description: API Documentation for Docker
:keywords: API, Docker, rcli, REST, documentation

=================
Docker Remote API
=================

.. contents:: Table of Contents

1. Brief introduction
=====================

- The Remote API is replacing rcli
- Default port in the docker deamon is 4243 
- The API tends to be REST, but for some complex commands, like attach or pull, the HTTP connection is hijacked to transport stdout stdin and stderr
- Since API version 1.2, the auth configuration is now handled client side, so the client has to send the authConfig as POST in /images/(name)/push

2. Versions
===========

The current verson of the API is 1.2
Calling /images/<name>/insert is the same as calling /v1.2/images/<name>/insert
You can still call an old version of the api using /v1.0/images/<name>/insert

:doc:`docker_remote_api_v1.2`
*****************************

What's new
----------

The auth configuration is now handled by the client.
The client should send it's authConfig as POST on each call of /images/(name)/push

.. http:get:: /auth is now deprecated
.. http:post:: /auth only checks the configuration but doesn't store it on the server

Deleting an image is now improved, will only untag the image if it has chidrens and remove all the untagged parents if has any.
.. http:post:: /images/<name>/delete now returns a JSON with the list of images deleted/untagged


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


docker v0.3.4 8d73740_

           HTTP/1.1 200 OK
	   Content-Type: application/vnd.docker.raw-stream

	   {{ STREAM }}

	:query registry: the registry you wan to push, optional
	:statuscode 200: no error
        :statuscode 404: no such image
        :statuscode 500: server error


Tag an image into a repository
******************************

.. http:post:: /images/(name)/tag

	Tag the image ``name`` into a repository

        **Example request**:

        .. sourcecode:: http
			
	   POST /images/test/tag?repo=myrepo&force=0 HTTP/1.1

	**Example response**:

        .. sourcecode:: http

           HTTP/1.1 200 OK

	:query repo: The repository to tag in
	:query force: 1/True/true or 0/False/false, default false
	:statuscode 200: no error
	:statuscode 400: bad parameter
	:statuscode 404: no such image
        :statuscode 500: server error


Remove an image
***************

.. http:delete:: /images/(name)

	Remove the image ``name`` from the filesystem 
	
	**Example request**:

	.. sourcecode:: http

	   DELETE /images/test HTTP/1.1

	**Example response**:

        .. sourcecode:: http

           HTTP/1.1 204 OK

	:statuscode 204: no error
        :statuscode 404: no such image
        :statuscode 500: server error


Search images
*************

.. http:get:: /images/search

	Search for an image in the docker index
	
	**Example request**:

        .. sourcecode:: http

           GET /images/search?term=sshd HTTP/1.1

	**Example response**:

	.. sourcecode:: http

	   HTTP/1.1 200 OK
	   Content-Type: application/json
	   
	   [
		{
			"Name":"cespare/sshd",
			"Description":""
		},
		{
			"Name":"johnfuller/sshd",
			"Description":""
		},
		{
			"Name":"dhrp/mongodb-sshd",
			"Description":""
		}
	   ]

	   :query term: term to search
	   :statuscode 200: no error
	   :statuscode 500: server error


3.3 Misc
--------

Build an image from Dockerfile via stdin
****************************************

.. http:post:: /build

	Build an image from Dockerfile via stdin

	**Example request**:

        .. sourcecode:: http

           POST /build HTTP/1.1
	   
	   {{ STREAM }}

	**Example response**:

        .. sourcecode:: http

           HTTP/1.1 200 OK
	   
	   {{ STREAM }}

	:query t: tag to be applied to the resulting image in case of success
	:query remote: URL to be fetch. Either a single Dockerfile or a Git repository
	:statuscode 200: no error
        :statuscode 500: server error


Get default username and email
******************************

.. http:get:: /auth

	Get the default username and email

	**Example request**:

        .. sourcecode:: http

           GET /auth HTTP/1.1

        **Example response**:

        .. sourcecode:: http

           HTTP/1.1 200 OK
	   Content-Type: application/json

	   {
		"username":"hannibal",
		"email":"hannibal@a-team.com"
	   }

        :statuscode 200: no error
        :statuscode 500: server error


Set auth configuration
**********************

.. http:post:: /auth

        Get the default username and email

        **Example request**:

        .. sourcecode:: http

           POST /auth HTTP/1.1
	   Content-Type: application/json

	   {
		"username":"hannibal",
		"password:"xxxx",
		"email":"hannibal@a-team.com"
	   }

        **Example response**:

        .. sourcecode:: http

           HTTP/1.1 200 OK

        :statuscode 200: no error
        :statuscode 204: no error
        :statuscode 500: server error


Display system-wide information
*******************************

.. http:get:: /info

	Display system-wide information
	
	**Example request**:

        .. sourcecode:: http

           GET /info HTTP/1.1

        **Example response**:

        .. sourcecode:: http

           HTTP/1.1 200 OK
	   Content-Type: application/json

	   {
		"Containers":11,
		"Images":16,
		"Debug":false,
		"NFd": 11,
		"NGoroutines":21,
		"MemoryLimit":true,
		"SwapLimit":false
	   }

        :statuscode 200: no error
        :statuscode 500: server error


Show the docker version information
***********************************

.. http:get:: /version

	Show the docker version information

	**Example request**:

        .. sourcecode:: http

           GET /version HTTP/1.1

        **Example response**:

        .. sourcecode:: http

           HTTP/1.1 200 OK
	   Content-Type: application/json

	   {
		"Version":"0.2.2",
		"GitCommit":"5a2a5cc+CHANGES",
		"GoVersion":"go1.0.3"
	   }

        :statuscode 200: no error
	:statuscode 500: server error


Create a new image from a container's changes
*********************************************

.. http:post:: /commit

	Create a new image from a container's changes

	**Example request**:

        .. sourcecode:: http

           POST /commit?container=44c004db4b17&m=message&repo=myrepo HTTP/1.1

        **Example response**:

        .. sourcecode:: http

           HTTP/1.1 201 OK
	   Content-Type: application/vnd.docker.raw-stream

           {"Id":"596069db4bf5"}

Initial version


.. _a8ae398: https://github.com/dotcloud/docker/commit/a8ae398bf52e97148ee7bd0d5868de2e15bd297f
.. _8d73740: https://github.com/dotcloud/docker/commit/8d73740343778651c09160cde9661f5f387b36f4

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
| Javascript           | docker-js      | https://github.com/dgoujard/docker-js      |
+----------------------+----------------+--------------------------------------------+
| Javascript (Angular) | dockerui       | https://github.com/crosbymichael/dockerui  |
| **WebUI**            |                |                                            |
+----------------------+----------------+--------------------------------------------+
