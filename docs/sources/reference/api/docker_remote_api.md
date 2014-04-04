page_title: Remote API
page_description: API Documentation for Docker
page_keywords: API, Docker, rcli, REST, documentation

# Docker Remote API

## 1. Brief introduction

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

## 2. Versions

The current version of the API is 1.10

Calling /images/\<name\>/insert is the same as calling
/v1.10/images/\<name\>/insert

You can still call an old version of the api using
/v1.0/images/\<name\>/insert

### v1.10

#### Full Documentation

[*Docker Remote API v1.10*](../docker_remote_api_v1.10/)

#### What’s new

 `DELETE `{.descname}`/images/`{.descname}(*name*)
:   **New!** You can now use the force parameter to force delete of an
    image, even if it’s tagged in multiple repositories.

### v1.9

#### Full Documentation

[*Docker Remote API v1.9*](../docker_remote_api_v1.9/)

#### What’s new

 `POST `{.descname}`/build`{.descname}
:   **New!** This endpoint now takes a serialized ConfigFile which it
    uses to resolve the proper registry auth credentials for pulling the
    base image. Clients which previously implemented the version
    accepting an AuthConfig object must be updated.

### v1.8

#### Full Documentation

[*Docker Remote API v1.8*](../docker_remote_api_v1.8/)

#### What’s new

 `POST `{.descname}`/build`{.descname}
:   **New!** This endpoint now returns build status as json stream. In
    case of a build error, it returns the exit status of the failed
    command.

 `GET `{.descname}`/containers/`{.descname}(*id*)`/json`{.descname}
:   **New!** This endpoint now returns the host config for the
    container.

 `POST `{.descname}`/images/create`{.descname}
:   

 `POST `{.descname}`/images/`{.descname}(*name*)`/insert`{.descname}
:   

 `POST `{.descname}`/images/`{.descname}(*name*)`/push`{.descname}
:   **New!** progressDetail object was added in the JSON. It’s now
    possible to get the current value and the total of the progress
    without having to parse the string.

### v1.7

#### Full Documentation

[*Docker Remote API v1.7*](../docker_remote_api_v1.7/)

#### What’s new

 `GET `{.descname}`/images/json`{.descname}
:   The format of the json returned from this uri changed. Instead of an
    entry for each repo/tag on an image, each image is only represented
    once, with a nested attribute indicating the repo/tags that apply to
    that image.

    Instead of:

        HTTP/1.1 200 OK
        Content-Type: application/json

        [
          {
            "VirtualSize": 131506275,
            "Size": 131506275,
            "Created": 1365714795,
            "Id": "8dbd9e392a964056420e5d58ca5cc376ef18e2de93b5cc90e868a1bbc8318c1c",
            "Tag": "12.04",
            "Repository": "ubuntu"
          },
          {
            "VirtualSize": 131506275,
            "Size": 131506275,
            "Created": 1365714795,
            "Id": "8dbd9e392a964056420e5d58ca5cc376ef18e2de93b5cc90e868a1bbc8318c1c",
            "Tag": "latest",
            "Repository": "ubuntu"
          },
          {
            "VirtualSize": 131506275,
            "Size": 131506275,
            "Created": 1365714795,
            "Id": "8dbd9e392a964056420e5d58ca5cc376ef18e2de93b5cc90e868a1bbc8318c1c",
            "Tag": "precise",
            "Repository": "ubuntu"
          },
          {
            "VirtualSize": 180116135,
            "Size": 24653,
            "Created": 1364102658,
            "Id": "b750fe79269d2ec9a3c593ef05b4332b1d1a02a62b4accb2c21d589ff2f5f2dc",
            "Tag": "12.10",
            "Repository": "ubuntu"
          },
          {
            "VirtualSize": 180116135,
            "Size": 24653,
            "Created": 1364102658,
            "Id": "b750fe79269d2ec9a3c593ef05b4332b1d1a02a62b4accb2c21d589ff2f5f2dc",
            "Tag": "quantal",
            "Repository": "ubuntu"
          }
        ]

    The returned json looks like this:

        HTTP/1.1 200 OK
        Content-Type: application/json

        [
          {
             "RepoTags": [
               "ubuntu:12.04",
               "ubuntu:precise",
               "ubuntu:latest"
             ],
             "Id": "8dbd9e392a964056420e5d58ca5cc376ef18e2de93b5cc90e868a1bbc8318c1c",
             "Created": 1365714795,
             "Size": 131506275,
             "VirtualSize": 131506275
          },
          {
             "RepoTags": [
               "ubuntu:12.10",
               "ubuntu:quantal"
             ],
             "ParentId": "27cf784147099545",
             "Id": "b750fe79269d2ec9a3c593ef05b4332b1d1a02a62b4accb2c21d589ff2f5f2dc",
             "Created": 1364102658,
             "Size": 24653,
             "VirtualSize": 180116135
          }
        ]

 `GET `{.descname}`/images/viz`{.descname}
:   This URI no longer exists. The `images -viz`
    output is now generated in the client, using the
    `/images/json` data.

### v1.6

#### Full Documentation

[*Docker Remote API v1.6*](../docker_remote_api_v1.6/)

#### What’s new

 `POST `{.descname}`/containers/`{.descname}(*id*)`/attach`{.descname}
:   **New!** You can now split stderr from stdout. This is done by
    prefixing a header to each transmition. See
    [`POST /containers/(id)/attach`{.xref .http .http-post .docutils
    .literal}](../docker_remote_api_v1.9/#post--containers-(id)-attach "POST /containers/(id)/attach").
    The WebSocket attach is unchanged. Note that attach calls on the
    previous API version didn’t change. Stdout and stderr are merged.

### v1.5

#### Full Documentation

[*Docker Remote API v1.5*](../docker_remote_api_v1.5/)

#### What’s new

 `POST `{.descname}`/images/create`{.descname}
:   **New!** You can now pass registry credentials (via an AuthConfig
    object) through the X-Registry-Auth header

 `POST `{.descname}`/images/`{.descname}(*name*)`/push`{.descname}
:   **New!** The AuthConfig object now needs to be passed through the
    X-Registry-Auth header

 `GET `{.descname}`/containers/json`{.descname}
:   **New!** The format of the Ports entry has been changed to a list of
    dicts each containing PublicPort, PrivatePort and Type describing a
    port mapping.

### v1.4

#### Full Documentation

[*Docker Remote API v1.4*](../docker_remote_api_v1.4/)

#### What’s new

 `POST `{.descname}`/images/create`{.descname}
:   **New!** When pulling a repo, all images are now downloaded in
    parallel.

 `GET `{.descname}`/containers/`{.descname}(*id*)`/top`{.descname}
:   **New!** You can now use ps args with docker top, like docker top
    \<container\_id\> aux

 `GET `{.descname}`/events:`{.descname}
:   **New!** Image’s name added in the events

### v1.3

docker v0.5.0
[51f6c4a](https://github.com/dotcloud/docker/commit/51f6c4a7372450d164c61e0054daf0223ddbd909)

#### Full Documentation

[*Docker Remote API v1.3*](../docker_remote_api_v1.3/)

#### What’s new

 `GET `{.descname}`/containers/`{.descname}(*id*)`/top`{.descname}
:   List the processes running inside a container.

 `GET `{.descname}`/events:`{.descname}
:   **New!** Monitor docker’s events via streaming or via polling

Builder (/build):

-   Simplify the upload of the build context
-   Simply stream a tarball instead of multipart upload with 4
    intermediary buffers
-   Simpler, less memory usage, less disk usage and faster

Warning

The /build improvements are not reverse-compatible. Pre 1.3 clients will
break on /build.

List containers (/containers/json):

-   You can use size=1 to get the size of the containers

Start containers (/containers/\<id\>/start):

-   You can now pass host-specific configuration (e.g. bind mounts) in
    the POST body for start calls

### v1.2

docker v0.4.2
[2e7649b](https://github.com/dotcloud/docker/commit/2e7649beda7c820793bd46766cbc2cfeace7b168)

#### Full Documentation

[*Docker Remote API v1.2*](../docker_remote_api_v1.2/)

#### What’s new

The auth configuration is now handled by the client.

The client should send it’s authConfig as POST on each call of
/images/(name)/push

 `GET `{.descname}`/auth`{.descname}
:   **Deprecated.**

 `POST `{.descname}`/auth`{.descname}
:   Only checks the configuration but doesn’t store it on the server

    Deleting an image is now improved, will only untag the image if it
    has children and remove all the untagged parents if has any.

 `POST `{.descname}`/images/<name>/delete`{.descname}
:   Now returns a JSON structure with the list of images
    deleted/untagged.

### v1.1

docker v0.4.0
[a8ae398](https://github.com/dotcloud/docker/commit/a8ae398bf52e97148ee7bd0d5868de2e15bd297f)

#### Full Documentation

[*Docker Remote API v1.1*](../docker_remote_api_v1.1/)

#### What’s new

 `POST `{.descname}`/images/create`{.descname}
:   

 `POST `{.descname}`/images/`{.descname}(*name*)`/insert`{.descname}
:   

 `POST `{.descname}`/images/`{.descname}(*name*)`/push`{.descname}
:   Uses json stream instead of HTML hijack, it looks like this:

    >     HTTP/1.1 200 OK
    >     Content-Type: application/json
    >
    >     {"status":"Pushing..."}
    >     {"status":"Pushing", "progress":"1/? (n/a)"}
    >     {"error":"Invalid..."}
    >     ...

### v1.0

docker v0.3.4
[8d73740](https://github.com/dotcloud/docker/commit/8d73740343778651c09160cde9661f5f387b36f4)

#### Full Documentation

[*Docker Remote API v1.0*](../docker_remote_api_v1.0/)

#### What’s new

Initial version
