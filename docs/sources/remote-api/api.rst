=================
Docker Remote API
=================

.. contents:: Table of Contents

1. Brief introduction
=====================

- The Remote API is replacing rcli
- Default port in the docker deamon is 4243 
- The API tends to be REST, but for some complex commands, like attach or pull, the HTTP connection in hijacked to transport stdout stdin and stderr

2. Endpoints
============

2.1 Containers
--------------

List containers
***************

.. http:get:: /containers

	List containers

	**Example request**:

	.. sourcecode:: http

	   GET /containers?trunc_cmd=0&all=1&only_ids=0&before=8dfafdbc3a40 HTTP/1.1
	   
	**Example response**:

	.. sourcecode:: http

	   HTTP/1.1 200 OK
	   
	   [
		{
			"Id": "8dfafdbc3a40",
			"Image": "base:latest",
			"Command": "echo 1",
			"Created": 1367854155,
			"Status": "Exit 0"
		},
		{
			"Id": "9cd87474be90",
			"Image": "base:latest",
			"Command": "echo 222222",
			"Created": 1367854155,
			"Status": "Exit 0"
		},
		{
			"Id": "3176a2479c92",
			"Image": "base:latest",
			"Command": "echo 3333333333333333",
			"Created": 1367854154,
			"Status": "Exit 0"
		},
		{
			"Id": "4cb07b47f9fb",
			"Image": "base:latest",
			"Command": "echo 444444444444444444444444444444444",
			"Created": 1367854152,
			"Status": "Exit 0"
		}
	   ]
 
	:query only_ids: 1 or 0, Only display numeric IDs. Default 0
	:query all: 1 or 0, Show all containers. Only running containers are shown by default
	:query trunc_cmd: 1 or 0, Truncate output. Output is truncated by default  
	:query limit: Show ``limit`` last created containers, include non-running ones.
	:query since: Show only containers created since Id, include non-running ones.
	:query before: Show only containers created before Id, include non-running ones.
	:statuscode 200: no error
	:statuscode 500: server error


Create a container
******************

.. http:post:: /containers

	Create a container

	**Example request**:

	.. sourcecode:: http

	   POST /containers HTTP/1.1
	   
	   {
		"Hostname":"",
		"User":"",
		"Memory":0,
		"MemorySwap":0,
		"AttachStdin":false,
		"AttachStdout":true,
		"AttachStderr":true,
		"PortSpecs":null,
		"Tty":false,
		"OpenStdin":false,
		"StdinOnce":false,
		"Env":null,
		"Cmd":[
			"date"
		],
		"Dns":null,
		"Image":"base",
		"Volumes":{},
		"VolumesFrom":""
	   }
	   
	**Example response**:

	.. sourcecode:: http

	   HTTP/1.1 200 OK
	   
	   {
		"Id":"e90e34656806"
		"Warnings":[]
	   }
	
	:jsonparam config: the container's configuration
	:statuscode 200: no error
	:statuscode 404: no such container
	:statuscode 500: server error


Inspect a container
*******************

.. http:get:: /containers/(id)/json

	Return low-level information on the container ``id``

	**Example request**:

	.. sourcecode:: http

	   GET /containers/4fa6e0f0c678/json HTTP/1.1
	   
	**Example response**:

	.. sourcecode:: http

	   HTTP/1.1 200 OK
	   
	   {
			"Id": "4fa6e0f0c6786287e131c3852c58a2e01cc697a68231826813597e4994f1d6e2",
			"Created": "2013-05-07T14:51:42.041847+02:00",
			"Path": "date",
			"Args": [],
			"Config": {
				"Hostname": "4fa6e0f0c678",
				"User": "",
				"Memory": 0,
				"MemorySwap": 0,
				"AttachStdin": false,
				"AttachStdout": true,
				"AttachStderr": true,
				"PortSpecs": null,
				"Tty": false,
				"OpenStdin": false,
				"StdinOnce": false,
				"Env": null,
				"Cmd": [
					"date"
				],
				"Dns": null,
				"Image": "base",
				"Volumes": {},
				"VolumesFrom": ""
			},
			"State": {
				"Running": false,
				"Pid": 0,
				"ExitCode": 0,
				"StartedAt": "2013-05-07T14:51:42.087658+02:01360",
				"Ghost": false
			},
			"Image": "b750fe79269d2ec9a3c593ef05b4332b1d1a02a62b4accb2c21d589ff2f5f2dc",
			"NetworkSettings": {
				"IpAddress": "",
				"IpPrefixLen": 0,
				"Gateway": "",
				"Bridge": "",
				"PortMapping": null
			},
			"SysInitPath": "/home/kitty/go/src/github.com/dotcloud/docker/bin/docker",
			"ResolvConfPath": "/etc/resolv.conf",
			"Volumes": {}
	   }

	:statuscode 200: no error
	:statuscode 404: no such container
	:statuscode 500: server error


Inspect changes on a container's filesystem
*******************************************

.. http:get:: /containers/(id)/changes

	Inspect changes on container ``id`` 's filesystem

	**Example request**:

	.. sourcecode:: http

	   GET /containers/4fa6e0f0c678/changes HTTP/1.1

	   
	**Example response**:

	.. sourcecode:: http

	   HTTP/1.1 200 OK
	   
	   [
		{
			"Path":"/dev",
			"Kind":0
		},
		{
			"Path":"/dev/kmsg",
			"Kind":1
		},
		{
			"Path":"/test",
			"Kind":1
		}
	   ]

	:statuscode 200: no error
	:statuscode 404: no such container
	:statuscode 500: server error


Export a container
******************

.. http:get:: /containers/(id)/export

	Export the contents of container ``id``

	**Example request**:

	.. sourcecode:: http

	   GET /containers/4fa6e0f0c678/export HTTP/1.1

	   
	**Example response**:

	.. sourcecode:: http

	   HTTP/1.1 200 OK
	   Content-Type: raw-stream-hijack
	   
	   {{ STREAM }}

	:statuscode 200: no error
	:statuscode 404: no such container
	:statuscode 500: server error


Start a container
*****************

.. http:post:: /containers/(id)/start

	Start the container ``id``

	**Example request**:

	.. sourcecode:: http

	   POST /containers/e90e34656806/start HTTP/1.1
	   
	**Example response**:

	.. sourcecode:: http

	   HTTP/1.1 200 OK
	   	
	:statuscode 200: no error
	:statuscode 404: no such container
	:statuscode 500: server error


Stop a contaier
***************

.. http:post:: /containers/(id)/stop

	Stop the container ``id``

	**Example request**:

	.. sourcecode:: http

	   POST /containers/e90e34656806/stop?t=5 HTTP/1.1
	   
	**Example response**:

	.. sourcecode:: http

	   HTTP/1.1 200 OK
	   	
	:query t: number of seconds to wait before killing the container
	:statuscode 200: no error
	:statuscode 404: no such container
	:statuscode 500: server error


Restart a container
*******************

.. http:post:: /containers/(id)/restart

	Restart the container ``id``

	**Example request**:

	.. sourcecode:: http

	   POST /containers/e90e34656806/restart?t=5 HTTP/1.1
	   
	**Example response**:

	.. sourcecode:: http

	   HTTP/1.1 200 OK
	   	
	:query t: number of seconds to wait before killing the container
	:statuscode 200: no error
	:statuscode 404: no such container
	:statuscode 500: server error


Kill a container
****************

.. http:post:: /containers/(id)/kill

	Kill the container ``id``

	**Example request**:

	.. sourcecode:: http

	   POST /containers/e90e34656806/kill HTTP/1.1
	   
	**Example response**:

	.. sourcecode:: http

	   HTTP/1.1 200 OK
	   	
	:statuscode 200: no error
	:statuscode 404: no such container
	:statuscode 500: server error


Attach to a container
*********************

.. http:post:: /containers/(id)/attach

	Stop the container ``id``

	**Example request**:

	.. sourcecode:: http

	   POST /containers/16253994b7c4/attach?logs=1&stream=0&stdout=1 HTTP/1.1
	   
	**Example response**:

	.. sourcecode:: http

	   HTTP/1.1 200 OK
	   Content-Type: raw-stream-hijack

	   {{ STREAM }}
	   	
	:query logs: 1 or 0, return logs. Default 0
	:query stream: 1 or 0, return stream. Default 0
	:query stdin: 1 or 0, if stream=1, attach to stdin. Default 0
	:query stdout: 1 or 0, if logs=1, return stdout log, if stream=1, attach to stdout. Default 0
	:query stderr: 1 or 0, if logs=1, return stderr log, if stream=1, attach to stderr. Default 0
	:statuscode 200: no error
	:statuscode 404: no such container
	:statuscode 500: server error


Wait a container
****************

.. http:post:: /containers/(id)/wait

	Block until container ``id`` stops, then returns the exit code

	**Example request**:

	.. sourcecode:: http

	   POST /containers/16253994b7c4/wait HTTP/1.1
	   
	**Example response**:

	.. sourcecode:: http

	   HTTP/1.1 200 OK

	   {"StatusCode":0}
	   	
	:statuscode 200: no error
	:statuscode 404: no such container
	:statuscode 500: server error


Remove a container
*******************

.. http:delete:: /container/(id)

	Remove the container ``id`` from the filesystem

	**Example request**:

        .. sourcecode:: http

           DELETE /containers/16253994b7c4?v=1 HTTP/1.1

        **Example response**:

        .. sourcecode:: http

	   HTTP/1.1 200 OK

	:query v: 1 or 0, Remove the volumes associated to the container. Default 0
        :statuscode 200: no error
        :statuscode 404: no such container
        :statuscode 500: server error


2.2 Images
----------

List Images
***********

.. http:get:: /images

	List images

	**Example request**:

	.. sourcecode:: http

	   GET /images?all=0&only_ids=0 HTTP/1.1
	   
	**Example response**:

	.. sourcecode:: http

	   HTTP/1.1 200 OK
	   
	   [
		{
			"Repository":"base",
			"Tag":"ubuntu-12.10",
			"Id":"b750fe79269d",
			"Created":1364102658
		},
		{
			"Repository":"base",
			"Tag":"ubuntu-quantal",
			"Id":"b750fe79269d",
			"Created":1364102658
		}
	   ]
 
	:query only_ids: 1 or 0, Only display numeric IDs. Default 0
	:query all: 1 or 0, Show all containers. Only running containers are shown by default
	:statuscode 200: no error
	:statuscode 500: server error


Create an image
***************

.. http:post:: /images

	Create an image, either by pull it from the registry or by importing it

	**Example request**:

        .. sourcecode:: http

           POST /images?fromImage=base HTTP/1.1

        **Example response**:

        .. sourcecode:: http

           HTTP/1.1 200 OK
	   Content-Type: raw-stream-hijack

	   {{ STREAM }}

        :query fromImage: name of the image to pull
	:query fromSrc: source to import, - means stdin
        :query repo: repository
	:query tag: tag
	:query registry: the registry to pull from
        :statuscode 200: no error
        :statuscode 500: server error


Insert a file in a image
************************

.. http:post:: /images/(name)/insert

	Insert a file from ``url`` in the image ``name`` at ``path``

	**Example request**:

        .. sourcecode:: http

           POST /images/test/insert?path=/usr&url=myurl HTTP/1.1

	**Example response**:

        .. sourcecode:: http

           HTTP/1.1 200 OK

	   {{ STREAM }}

	:statuscode 200: no error
        :statuscode 500: server error


Inspect an image
****************

.. http:get:: /images/(name)/json

	Return low-level information on the image ``name``

	**Example request**:

	.. sourcecode:: http

	   GET /images/base/json HTTP/1.1

	**Example response**:

        .. sourcecode:: http

           HTTP/1.1 200 OK

	   {
		"id":"b750fe79269d2ec9a3c593ef05b4332b1d1a02a62b4accb2c21d589ff2f5f2dc",
		"parent":"27cf784147099545",
		"created":"2013-03-23T22:24:18.818426-07:00",
		"container":"3d67245a8d72ecf13f33dffac9f79dcdf70f75acb84d308770391510e0c23ad0",
		"container_config":
			{
				"Hostname":"",
				"User":"",
				"Memory":0,
				"MemorySwap":0,
				"AttachStdin":false,
				"AttachStdout":false,
				"AttachStderr":false,
				"PortSpecs":null,
				"Tty":true,
				"OpenStdin":true,
				"StdinOnce":false,
				"Env":null,
				"Cmd": ["/bin/bash"]
				,"Dns":null,
				"Image":"base",
				"Volumes":null,
				"VolumesFrom":""
			}
	   }

	:statuscode 200: no error
	:statuscode 404: no such image
        :statuscode 500: server error


Get the history of an image
***************************

.. http:get:: /images/(name)

        Return the history of the image ``name``

        **Example request**:

        .. sourcecode:: http

           GET /images/base/history HTTP/1.1

        **Example response**:

        .. sourcecode:: http

           HTTP/1.1 200 OK

	   [
		{
			"Id":"b750fe79269d",
			"Created":1364102658,
			"CreatedBy":"/bin/bash"
		},
		{
			"Id":"27cf78414709",
			"Created":1364068391,
			"CreatedBy":""
		}
	   ]

        :statuscode 200: no error
        :statuscode 404: no such image
        :statuscode 500: server error


Push an image on the registry
*****************************

.. http:post:: /images/(name)/push

	Push the image ``name`` on the registry

	 **Example request**:

	 .. sourcecode:: http

	    POST /images/test/push HTTP/1.1

	 **Example response**:

        .. sourcecode:: http

           HTTP/1.1 200 OK
	   Content-Type: raw-stream-hijack

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
	:query force: 1 or 0, default 0
	:statuscode 200: no error
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

           HTTP/1.1 200 OK

	:statuscode 200: no error
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


2.3 Misc
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

	   {
		"username":"hannibal",
		"password:"xxxx",
		"email":"hannibal@a-team.com"
	   }

        **Example response**:

        .. sourcecode:: http

           HTTP/1.1 200 OK

        :statuscode 200: no error
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

	   {
		"Containers":11,
		"Version":"0.2.2",
		"Images":16,
		"Debug":false
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
	   
	   {
		"Version":"0.2.2",
		"GitCommit":"5a2a5cc+CHANGES",
		"MemoryLimit":true,
		"SwapLimit":false
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

           HTTP/1.1 200 OK
	   Content-Type: raw-stream-hijack

           {{ STREAM }}

	:query container: source container
	:query repo: repository
	:query tag: tag
	:query m: commit message
	:query author: author (eg. "John Hannibal Smith <hannibal@a-team.com>")
	:query run: config automatically applied when the image is run. (ex: {"Cmd": ["cat", "/world"], "PortSpecs":["22"]})
        :statuscode 200: no error
	:statuscode 404: no such container
        :statuscode 500: server error


3. Going further
================

3.1 Inside 'docker run'
-----------------------

Here are the steps of 'docker run' :

* Create the container
* If the status code is 404, it means the image doesn't exists:
        * Try to pull it
        * Then retry to create the container
* Start the container
* If you are not in detached mode:
        * Attach to the container, using logs=1 (to have stdout and stderr from the container's start) and stream=1
* If in detached mode or only stdin is attached:
	* Display the container's id


3.2 Hijacking
-------------

In this first version of the API, some of the endpoints, like /attach, /pull or /push uses hijacking to transport stdin,
stdout and stderr on the same socket. This might change in the future.
