title
:   Remote API v1.4

description
:   API Documentation for Docker

keywords
:   API, Docker, rcli, REST, documentation

orphan
:   

Docker Remote API v1.4
======================

1. Brief introduction
---------------------

-   The Remote API is replacing rcli
-   Default port in the docker daemon is 4243
-   The API tends to be REST, but for some complex commands, like attach
    or pull, the HTTP connection is hijacked to transport stdout stdin
    and stderr

2. Endpoints
------------

### 2.1 Containers

#### List containers

#### Create a container

#### Inspect a container

#### List processes running inside a container

#### Inspect changes on a container's filesystem

#### Export a container

#### Start a container

#### Stop a container

#### Restart a container

#### Kill a container

#### Attach to a container

#### Wait a container

#### Remove a container

#### Copy files or folders from a container

### 2.2 Images

#### List Images

#### Create an image

#### Insert a file in an image

#### Inspect an image

#### Get the history of an image

> Content-Type: application/json
>
> [
> :   {
>     :   "Id":"b750fe79269d", "Created":1364102658,
>         "CreatedBy":"/bin/bash"
>
>     }, { "Id":"27cf78414709", "Created":1364068391, "CreatedBy":"" }
>
> ]
>
> > statuscode 200
> > :   no error
> >
> > statuscode 404
> > :   no such image
> >
> > statuscode 500
> > :   server error
> >
#### Push an image on the registry

#### Tag an image into a repository

#### Remove an image

#### Search images

### 2.3 Misc

#### Build an image from Dockerfile via stdin

#### Check auth configuration

> Content-Type: application/json
>
> {
> :   "username":"hannibal", "password:"xxxx", "email":
>     "hannibal@a-team.com" ("hannibal at a-team dot com")
>     ,
>     "serveraddress":"[https://index.docker.io/v1/](https://index.docker.io/v1/)"
>
> }
>
> > **Example response**:
> >
> > statuscode 200
> > :   no error
> >
> > statuscode 204
> > :   no error
> >
> > statuscode 500
> > :   server error
> >
#### Display system-wide information

#### Show the docker version information

#### Create a new image from a container's changes

#### Monitor Docker's events

3. Going further
----------------

### 3.1 Inside 'docker run'

Here are the steps of 'docker run' :

-   Create the container
-   If the status code is 404, it means the image doesn't exists:
    :   -   Try to pull it
        -   Then retry to create the container

-   Start the container
-   If you are not in detached mode:
    :   -   Attach to the container, using logs=1 (to have stdout and
            stderr from the container's start) and stream=1

-   If in detached mode or only stdin is attached:
    :   -   Display the container's id

### 3.2 Hijacking

In this version of the API, /attach, uses hijacking to transport stdin,
stdout and stderr on the same socket. This might change in the future.

### 3.3 CORS Requests

To enable cross origin requests to the remote api add the flag
"-api-enable-cors" when running docker in daemon mode.

~~~~ {.sourceCode .bash}
docker -d -H="192.168.1.9:4243" -api-enable-cors
~~~~
