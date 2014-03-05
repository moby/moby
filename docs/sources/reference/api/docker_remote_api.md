title
:   Remote API

description
:   API Documentation for Docker

keywords
:   API, Docker, rcli, REST, documentation

Docker Remote API
=================

1. Brief introduction
---------------------

-   The Remote API is replacing rcli
-   By default the Docker daemon listens on unix:///var/run/docker.sock
    and the client must have root access to interact with the daemon
-   If a group named *docker* exists on your system, docker will apply
    ownership of the socket to the group
-   The API tends to be REST, but for some complex commands, like attach
    or pull, the HTTP connection is hijacked to transport stdout stdin
    and stderr
-   Since API version 1.2, the auth configuration is now handled client
    side, so the client has to send the authConfig as POST in
    /images/(name)/push

2. Versions
-----------

The current version of the API is 1.10

Calling /images/\<name\>/insert is the same as calling
/v1.10/images/\<name\>/insert

You can still call an old version of the api using
/v1.0/images/\<name\>/insert

### v1.10

#### Full Documentation

docker\_remote\_api\_v1.10

#### What's new

### v1.9

#### Full Documentation

docker\_remote\_api\_v1.9

#### What's new

### v1.8

#### Full Documentation

docker\_remote\_api\_v1.8

#### What's new

### v1.7

#### Full Documentation

docker\_remote\_api\_v1.7

#### What's new

### v1.6

#### Full Documentation

docker\_remote\_api\_v1.6

#### What's new

### v1.5

#### Full Documentation

docker\_remote\_api\_v1.5

#### What's new

### v1.4

#### Full Documentation

docker\_remote\_api\_v1.4

#### What's new

### v1.3

docker v0.5.0
[51f6c4a](https://github.com/dotcloud/docker/commit/51f6c4a7372450d164c61e0054daf0223ddbd909)

#### Full Documentation

docker\_remote\_api\_v1.3

#### What's new

Builder (/build):

-   Simplify the upload of the build context
-   Simply stream a tarball instead of multipart upload with 4
    intermediary buffers
-   Simpler, less memory usage, less disk usage and faster

> **warning**
>
> The /build improvements are not reverse-compatible. Pre 1.3 clients
> will break on /build.

List containers (/containers/json):

-   You can use size=1 to get the size of the containers

Start containers (/containers/\<id\>/start):

-   You can now pass host-specific configuration (e.g. bind mounts) in
    the POST body for start calls

### v1.2

docker v0.4.2
[2e7649b](https://github.com/dotcloud/docker/commit/2e7649beda7c820793bd46766cbc2cfeace7b168)

#### Full Documentation

docker\_remote\_api\_v1.2

#### What's new

The auth configuration is now handled by the client.

The client should send it's authConfig as POST on each call of
/images/(name)/push

### v1.1

docker v0.4.0
[a8ae398](https://github.com/dotcloud/docker/commit/a8ae398bf52e97148ee7bd0d5868de2e15bd297f)

#### Full Documentation

docker\_remote\_api\_v1.1

#### What's new

### v1.0

docker v0.3.4
[8d73740](https://github.com/dotcloud/docker/commit/8d73740343778651c09160cde9661f5f387b36f4)

#### Full Documentation

docker\_remote\_api\_v1.0

#### What's new

Initial version
