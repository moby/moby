:title: Remote API v1.6
:description: API Documentation for Docker
:keywords: API, Docker, rcli, REST, documentation

:orphan:

======================
Docker Remote API v1.6
======================

.. contents:: Table of Contents

1. Brief introduction
=====================

- The Remote API has replaced rcli
- The daemon listens on ``unix:///var/run/docker.sock``, but you can
  :ref:`bind_docker`.
- The API tends to be REST, but for some complex commands, like
  ``attach`` or ``pull``, the HTTP connection is hijacked to transport
  ``stdout, stdin`` and ``stderr``

2. Endpoints
============

2.1 Containers
--------------

List containers
***************

.. http:get:: /containers/json

	List containers

	**Example request**:

	.. sourcecode:: http

	   GET /containers/json?all=1&before=8dfafdbc3a40&size=1 HTTP/1.1
	   
	**Example response**:

	.. sourcecode:: http

	   HTTP/1.1 200 OK
	   Content-Type: application/json
	   
	   [
		{
			"Id": "8dfafdbc3a40",
			"Image": "base:latest",
			"Command": "echo 1",
			"Created": 1367854155,
			"Status": "Exit 0",
			"Ports":[{"PrivatePort": 2222, "PublicPort": 3333, "Type": "tcp"}],
			"SizeRw":12288,
			"SizeRootFs":0
		},
		{
			"Id": "9cd87474be90",
			"Image": "base:latest",
			"Command": "echo 222222",
			"Created": 1367854155,
			"Status": "Exit 0",
			"Ports":[],
			"SizeRw":12288,
			"SizeRootFs":0
		},
		{
			"Id": "3176a2479c92",
			"Image": "base:latest",
			"Command": "echo 3333333333333333",
			"Created": 1367854154,
			"Status": "Exit 0",
			"Ports":[],
			"SizeRw":12288,
			"SizeRootFs":0
		},
		{
			"Id": "4cb07b47f9fb",
			"Image": "base:latest",
			"Command": "echo 444444444444444444444444444444444",
			"Created": 1367854152,
			"Status": "Exit 0",
			"Ports":[],
			"SizeRw":12288,
			"SizeRootFs":0
		}
	   ]
 
	:query all: 1/True/true or 0/False/false, Show all containers. Only running containers are shown by default
	:query limit: Show ``limit`` last created containers, include non-running ones.
	:query since: Show only containers created since Id, include non-running ones.
	:query before: Show only containers created before Id, include non-running ones.
	:query size: 1/True/true or 0/False/false, Show the containers sizes
	:statuscode 200: no error
	:statuscode 400: bad parameter
	:statuscode 500: server error


Create a container
******************

.. http:post:: /containers/create

	Create a container

	**Example request**:

	.. sourcecode:: http

	   POST /containers/create HTTP/1.1
	   Content-Type: application/json

	   {
		"Hostname":"",
		"User":"",
		"Memory":0,
		"MemorySwap":0,
		"AttachStdin":false,
		"AttachStdout":true,
		"AttachStderr":true,
		"ExposedPorts":{},
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
		"VolumesFrom":"",
		"WorkingDir":""
	   }
	   
	**Example response**:

	.. sourcecode:: http

	   HTTP/1.1 201 OK
	   Content-Type: application/json

	   {
		"Id":"e90e34656806"
		"Warnings":[]
	   }
	
	:jsonparam config: the container's configuration
 	:query name: container name to use
	:statuscode 201: no error
	:statuscode 404: no such container
	:statuscode 406: impossible to attach (container not running)
	:statuscode 500: server error

	**More Complex Example request, in 2 steps.**
	**First, use create to expose a Private Port, which can be bound back to a Public Port at startup**:

	.. sourcecode:: http

	   POST /containers/create HTTP/1.1
	   Content-Type: application/json

	   {
		"Cmd":[
			"/usr/sbin/sshd","-D"
		],
		"Image":"image-with-sshd",
		"ExposedPorts":{"22/tcp":{}}
		}

	**Example response**:

	.. sourcecode:: http

	   HTTP/1.1 201 OK
	   Content-Type: application/json

	   {
		"Id":"e90e34656806"
		"Warnings":[]
	   }

	**Second, start (using the ID returned above) the image we just created, mapping the ssh port 22 to something on the host**:

	.. sourcecode:: http

	   POST /containers/e90e34656806/start HTTP/1.1
	   Content-Type: application/json

	   {
		"PortBindings": { "22/tcp": [{ "HostPort": "11022" }]} 
		}

	**Example response**:

	.. sourcecode:: http

		HTTP/1.1 204 No Content
		Content-Type: text/plain; charset=utf-8
		Content-Length: 0

	**Now you can ssh into your new container on port 11022.**




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
	   Content-Type: application/json

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
				"ExposedPorts": {},
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
				"VolumesFrom": "",
				"WorkingDir":""

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


List processes running inside a container
*****************************************

.. http:get:: /containers/(id)/top

	List processes running inside the container ``id``

	**Example request**:

	.. sourcecode:: http

	   GET /containers/4fa6e0f0c678/top HTTP/1.1

	**Example response**:

	.. sourcecode:: http

	   HTTP/1.1 200 OK
	   Content-Type: application/json

	   {
		"Titles":[
			"USER",
			"PID",
			"%CPU",
			"%MEM",
			"VSZ",
			"RSS",
			"TTY",
			"STAT",
			"START",
			"TIME",
			"COMMAND"
			],
		"Processes":[
			["root","20147","0.0","0.1","18060","1864","pts/4","S","10:06","0:00","bash"],
			["root","20271","0.0","0.0","4312","352","pts/4","S+","10:07","0:00","sleep","10"]
		]
	   }

	:query ps_args: ps arguments to use (eg. aux)
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
	   Content-Type: application/json
	   
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
	   Content-Type: application/octet-stream
	   
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

           POST /containers/(id)/start HTTP/1.1
           Content-Type: application/json

           {
                "Binds":["/tmp:/tmp"],
                "LxcConf":{"lxc.utsname":"docker"},
                "ContainerIDFile": "",
                "Privileged": false,
                "PortBindings": {"22/tcp": [{HostIp:"", HostPort:""}]},
                "Links": [],
                "PublishAllPorts": false
           }

        **Example response**:

        .. sourcecode:: http

           HTTP/1.1 204 No Content
           Content-Type: text/plain

        :jsonparam hostConfig: the container's host configuration (optional)
        :statuscode 204: no error
        :statuscode 404: no such container
        :statuscode 500: server error


Stop a container
****************

.. http:post:: /containers/(id)/stop

	Stop the container ``id``

	**Example request**:

	.. sourcecode:: http

	   POST /containers/e90e34656806/stop?t=5 HTTP/1.1
	   
	**Example response**:

	.. sourcecode:: http

	   HTTP/1.1 204 OK
	   	
	:query t: number of seconds to wait before killing the container
	:statuscode 204: no error
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

	   HTTP/1.1 204 OK
	   	
	:query t: number of seconds to wait before killing the container
	:statuscode 204: no error
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

	   HTTP/1.1 204 OK

	:query signal: Signal to send to the container (integer). When not set, SIGKILL is assumed and the call will waits for the container to exit.
	:statuscode 204: no error
	:statuscode 404: no such container
	:statuscode 500: server error


Attach to a container
*********************

.. http:post:: /containers/(id)/attach

	Attach to the container ``id``

	**Example request**:

	.. sourcecode:: http

	   POST /containers/16253994b7c4/attach?logs=1&stream=0&stdout=1 HTTP/1.1
	   
	**Example response**:

	.. sourcecode:: http

	   HTTP/1.1 200 OK
	   Content-Type: application/vnd.docker.raw-stream

	   {{ STREAM }}
	   	
	:query logs: 1/True/true or 0/False/false, return logs. Default false
	:query stream: 1/True/true or 0/False/false, return stream. Default false
	:query stdin: 1/True/true or 0/False/false, if stream=true, attach to stdin. Default false
	:query stdout: 1/True/true or 0/False/false, if logs=true, return stdout log, if stream=true, attach to stdout. Default false
	:query stderr: 1/True/true or 0/False/false, if logs=true, return stderr log, if stream=true, attach to stderr. Default false
	:statuscode 200: no error
	:statuscode 400: bad parameter
	:statuscode 404: no such container
	:statuscode 500: server error

	**Stream details**:

	When using the TTY setting is enabled in
	:http:post:`/containers/create`, the stream is the raw data
	from the process PTY and client's stdin.  When the TTY is
	disabled, then the stream is multiplexed to separate stdout
	and stderr.

	The format is a **Header** and a **Payload** (frame).

	**HEADER**

	The header will contain the information on which stream write
	the stream (stdout or stderr). It also contain the size of
	the associated frame encoded on the last 4 bytes (uint32).

	It is encoded on the first 8 bytes like this::

	    header := [8]byte{STREAM_TYPE, 0, 0, 0, SIZE1, SIZE2, SIZE3, SIZE4}

	``STREAM_TYPE`` can be:

	- 0: stdin (will be writen on stdout)
	- 1: stdout
	- 2: stderr

	``SIZE1, SIZE2, SIZE3, SIZE4`` are the 4 bytes of the uint32 size encoded as big endian.

	**PAYLOAD**

	The payload is the raw stream.

	**IMPLEMENTATION**

	The simplest way to implement the Attach protocol is the following:

	1) Read 8 bytes
	2) chose stdout or stderr depending on the first byte
	3) Extract the frame size from the last 4 byets
	4) Read the extracted size and output it on the correct output
	5) Goto 1)



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
	   Content-Type: application/json

	   {"StatusCode":0}
	   	
	:statuscode 200: no error
	:statuscode 404: no such container
	:statuscode 500: server error


Remove a container
*******************

.. http:delete:: /containers/(id)

	Remove the container ``id`` from the filesystem

	**Example request**:

        .. sourcecode:: http

           DELETE /containers/16253994b7c4?v=1 HTTP/1.1

        **Example response**:

        .. sourcecode:: http

	   HTTP/1.1 204 OK

	:query v: 1/True/true or 0/False/false, Remove the volumes associated to the container. Default false
        :statuscode 204: no error
	:statuscode 400: bad parameter
        :statuscode 404: no such container
        :statuscode 500: server error


Copy files or folders from a container
**************************************

.. http:post:: /containers/(id)/copy

	Copy files or folders of container ``id``

	**Example request**:

	.. sourcecode:: http

	   POST /containers/4fa6e0f0c678/copy HTTP/1.1
	   Content-Type: application/json

	   {
		"Resource":"test.txt"
	   }

	**Example response**:

	.. sourcecode:: http

	   HTTP/1.1 200 OK
	   Content-Type: application/octet-stream
	   
	   {{ STREAM }}

	:statuscode 200: no error
	:statuscode 404: no such container
	:statuscode 500: server error


2.2 Images
----------

List Images
***********

.. http:get:: /images/(format)

	List images ``format`` could be json or viz (json default)

	**Example request**:

	.. sourcecode:: http

	   GET /images/json?all=0 HTTP/1.1

	**Example response**:

	.. sourcecode:: http

	   HTTP/1.1 200 OK
	   Content-Type: application/json
	   
	   [
		{
			"Repository":"base",
			"Tag":"ubuntu-12.10",
			"Id":"b750fe79269d",
			"Created":1364102658,
			"Size":24653,
			"VirtualSize":180116135
		},
		{
			"Repository":"base",
			"Tag":"ubuntu-quantal",
			"Id":"b750fe79269d",
			"Created":1364102658,
			"Size":24653,
			"VirtualSize":180116135
		}
	   ]


	**Example request**:

	.. sourcecode:: http

	   GET /images/viz HTTP/1.1

	**Example response**:

	.. sourcecode:: http

	   HTTP/1.1 200 OK
	   Content-Type: text/plain

	   digraph docker {
	   "d82cbacda43a" -> "074be284591f"
	   "1496068ca813" -> "08306dc45919"
	   "08306dc45919" -> "0e7893146ac2"
	   "b750fe79269d" -> "1496068ca813"
	   base -> "27cf78414709" [style=invis]
	   "f71189fff3de" -> "9a33b36209ed"
	   "27cf78414709" -> "b750fe79269d"
	   "0e7893146ac2" -> "d6434d954665"
	   "d6434d954665" -> "d82cbacda43a"
	   base -> "e9aa60c60128" [style=invis]
	   "074be284591f" -> "f71189fff3de"
	   "b750fe79269d" [label="b750fe79269d\nbase",shape=box,fillcolor="paleturquoise",style="filled,rounded"];
	   "e9aa60c60128" [label="e9aa60c60128\nbase2",shape=box,fillcolor="paleturquoise",style="filled,rounded"];
	   "9a33b36209ed" [label="9a33b36209ed\ntest",shape=box,fillcolor="paleturquoise",style="filled,rounded"];
	   base [style=invisible]
	   }
 
	:query all: 1/True/true or 0/False/false, Show all containers. Only running containers are shown by default
	:statuscode 200: no error
	:statuscode 400: bad parameter
	:statuscode 500: server error


Create an image
***************

.. http:post:: /images/create

	Create an image, either by pull it from the registry or by importing it

	**Example request**:

        .. sourcecode:: http

           POST /images/create?fromImage=base HTTP/1.1

        **Example response**:

        .. sourcecode:: http

           HTTP/1.1 200 OK
	   Content-Type: application/json

	   {"status":"Pulling..."}
	   {"status":"Pulling", "progress":"1/? (n/a)"}
	   {"error":"Invalid..."}
	   ...

	When using this endpoint to pull an image from the registry,
	the ``X-Registry-Auth`` header can be used to include a
	base64-encoded AuthConfig object.

        :query fromImage: name of the image to pull
	:query fromSrc: source to import, - means stdin
        :query repo: repository
	:query tag: tag
	:query registry: the registry to pull from
        :statuscode 200: no error
        :statuscode 500: server error


Insert a file in an image
*************************

.. http:post:: /images/(name)/insert

	Insert a file from ``url`` in the image ``name`` at ``path``

	**Example request**:

        .. sourcecode:: http

           POST /images/test/insert?path=/usr&url=myurl HTTP/1.1

	**Example response**:

        .. sourcecode:: http

           HTTP/1.1 200 OK
	   Content-Type: application/json

	   {"status":"Inserting..."}
	   {"status":"Inserting", "progress":"1/? (n/a)"}
	   {"error":"Invalid..."}
	   ...

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
	   Content-Type: application/json

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
				"ExposedPorts":{},
				"Tty":true,
				"OpenStdin":true,
				"StdinOnce":false,
				"Env":null,
				"Cmd": ["/bin/bash"]
				,"Dns":null,
				"Image":"base",
				"Volumes":null,
				"VolumesFrom":"",
				"WorkingDir":""
			},
		"Size": 6824592
	   }

	:statuscode 200: no error
	:statuscode 404: no such image
        :statuscode 500: server error


Get the history of an image
***************************

.. http:get:: /images/(name)/history

        Return the history of the image ``name``

        **Example request**:

        .. sourcecode:: http

           GET /images/base/history HTTP/1.1

        **Example response**:

        .. sourcecode:: http

           HTTP/1.1 200 OK
	   Content-Type: application/json

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
    Content-Type: application/json

   {"status":"Pushing..."}
   {"status":"Pushing", "progress":"1/? (n/a)"}
   {"error":"Invalid..."}
   ...

	The ``X-Registry-Auth`` header can be used to include a
	base64-encoded AuthConfig object.

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

           HTTP/1.1 201 OK

	:query repo: The repository to tag in
	:query force: 1/True/true or 0/False/false, default false
	:statuscode 201: no error
	:statuscode 400: bad parameter
	:statuscode 404: no such image
	:statuscode 409: conflict
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
	   Content-type: application/json

	   [
	    {"Untagged":"3e2f21a89f"},
	    {"Deleted":"3e2f21a89f"},
	    {"Deleted":"53b4f83ac9"}
	   ]

	:statuscode 200: no error
        :statuscode 404: no such image
	:statuscode 409: conflict
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


    The stream must be a tar archive compressed with one of the following algorithms:
    identity (no compression), gzip, bzip2, xz. The archive must include a file called
    `Dockerfile` at its root. It may include any number of other files, which will be
    accessible in the build context (See the ADD build command).
    
    The Content-type header should be set to "application/tar".

    :query t: repository name (and optionally a tag) to be applied to the resulting image in case of success
    :query q: suppress verbose build output
    :query nocache: do not use the cache when building the image
    :statuscode 200: no error
    :statuscode 500: server error


Check auth configuration
************************

.. http:post:: /auth

        Get the default username and email

        **Example request**:

        .. sourcecode:: http

           POST /auth HTTP/1.1
	   Content-Type: application/json

	   {
		"username":"hannibal",
		"password:"xxxx",
		"email":"hannibal@a-team.com",
		"serveraddress":"https://index.docker.io/v1/"
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
		"SwapLimit":false,
		"IPv4Forwarding":true
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
       Content-Type: application/json
       
       {
           "Cmd": ["cat", "/world"],
           "ExposedPorts":{"22/tcp":{}}
       }

    **Example response**:

    .. sourcecode:: http

        HTTP/1.1 201 OK
	    Content-Type: application/vnd.docker.raw-stream

        {"Id":"596069db4bf5"}

    :query container: source container
    :query repo: repository
    :query tag: tag
    :query m: commit message
    :query author: author (eg. "John Hannibal Smith <hannibal@a-team.com>")
    :statuscode 201: no error
    :statuscode 404: no such container
    :statuscode 500: server error


Monitor Docker's events
***********************

.. http:get:: /events

	Get events from docker, either in real time via streaming, or via polling (using `since`)

	**Example request**:

	.. sourcecode:: http

           GET /events?since=1374067924

        **Example response**:

        .. sourcecode:: http

           HTTP/1.1 200 OK
	   Content-Type: application/json

	   {"status":"create","id":"dfdf82bd3881","from":"base:latest","time":1374067924}
	   {"status":"start","id":"dfdf82bd3881","from":"base:latest","time":1374067924}
	   {"status":"stop","id":"dfdf82bd3881","from":"base:latest","time":1374067966}
	   {"status":"destroy","id":"dfdf82bd3881","from":"base:latest","time":1374067970}

	:query since: timestamp used for polling
        :statuscode 200: no error
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

In this version of the API, /attach, uses hijacking to transport stdin, stdout and stderr on the same socket. This might change in the future.

3.3 CORS Requests
-----------------

To enable cross origin requests to the remote api add the flag "--api-enable-cors" when running docker in daemon mode.

.. code-block:: bash

   docker -d -H="192.168.1.9:4243" --api-enable-cors

