title
:   Remote API v1.9

description
:   API Documentation for Docker

keywords
:   API, Docker, rcli, REST, documentation

orphan
:   

Docker Remote API v1.9
======================

1. Brief introduction
---------------------

-   The Remote API has replaced rcli
-   The daemon listens on `unix:///var/run/docker.sock`, but you can
    bind\_docker.
-   The API tends to be REST, but for some complex commands, like
    `attach` or `pull`, the HTTP connection is hijacked to transport
    `stdout, stdin` and `stderr`

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

#### Push an image on the registry

#### Tag an image into a repository

#### Remove an image

#### Search images

### 2.3 Misc

#### Build an image from Dockerfile

#### Check auth configuration

#### Display system-wide information

#### Show the docker version information

#### Create a new image from a container's changes

#### Monitor Docker's events

#### Get a tarball containing all images and tags in a repository

#### Load a tarball with a set of images and tags into docker

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
