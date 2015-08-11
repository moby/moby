<!--[metadata]>
+++
title = "Remote API"
description = "API Documentation for Docker"
keywords = ["API, Docker, rcli, REST,  documentation"]
[menu.main]
parent = "smn_remoteapi"
+++
<![end-metadata]-->

# Docker Remote API

 - By default the Docker daemon listens on `unix:///var/run/docker.sock`
   and the client must have `root` access to interact with the daemon.
 - If the Docker daemon is set to use an encrypted TCP socket (`--tls`,
   or `--tlsverify`) as with Boot2Docker 1.3.0, then you need to add extra
   parameters to `curl` or `wget` when making test API requests:
   `curl --insecure --cert ~/.docker/cert.pem --key ~/.docker/key.pem https://boot2docker:2376/images/json`
   or 
   `wget --no-check-certificate --certificate=$DOCKER_CERT_PATH/cert.pem --private-key=$DOCKER_CERT_PATH/key.pem https://boot2docker:2376/images/json -O - -q`
 - If a group named `docker` exists on your system, docker will apply
   ownership of the socket to the group.
 - The API tends to be REST, but for some complex commands, like attach
   or pull, the HTTP connection is hijacked to transport STDOUT, STDIN,
   and STDERR.
 - Since API version 1.2, the auth configuration is now handled client
   side, so the client has to send the `authConfig` as a `POST` in `/images/(name)/push`.
 - authConfig, set as the `X-Registry-Auth` header, is currently a Base64
   encoded (JSON) string with the following structure:
   `{"username": "string", "password": "string", "email": "string",
   "serveraddress" : "string", "auth": ""}`. Notice that `auth` is to be left
   empty, `serveraddress` is a domain/ip without protocol, and that double
   quotes (instead of single ones) are required.
 - The Remote API uses an open schema model.  In this model, unknown 
   properties in incoming messages will be ignored.
   Client applications need to take this into account to ensure
   they will not break when talking to newer Docker daemons.

The current version of the API is v1.20

Calling `/info` is the same as calling
`/v1.20/info`.

You can still call an old version of the API using
`/v1.19/info`.

## Docker Events

The following diagram depicts the container states accessible through the API.

![States](../images/event_state.png)

Some container-related events are not affected by container state, so they are not included in this diagram. These events are:

* **export** emitted by `docker export`
* **exec_create** emitted by `docker exec`
* **exec_start** emitted by `docker exec` after **exec_create**

Running `docker rmi` emits an **untag** event when removing an image name.  The `rmi` command may also emit **delete** events when images are deleted by ID directly or by deleting the last tag referring to the image.

> **Acknowledgement**: This diagram and the accompanying text were used with the permission of Matt Good and Gilder Labs. See Matt's original blog post [Docker Events Explained](http://gliderlabs.com/blog/2015/04/14/docker-events-explained/).

## v1.20

### Full documentation

[*Docker Remote API v1.20*](/reference/api/docker_remote_api_v1.20/)

### What's new

`GET /containers/(id)/archive`

**New!**
Get an archive of filesystem content from a container.

`PUT /containers/(id)/archive`

**New!**
Upload an archive of content to be extracted to an
existing directory inside a container's filesystem.

`POST /containers/(id)/copy`

**Deprecated!**
This copy endpoint has been deprecated in favor of the above `archive` endpoint
which can be used to download files and directories from a container.

**New!**
The `hostConfig` option now accepts the field `GroupAdd`, which specifies a list of additional
groups that the container process will run as.

## v1.19

### Full documentation

[*Docker Remote API v1.19*](/reference/api/docker_remote_api_v1.19/)

### What's new

**New!**
When the daemon detects a version mismatch with the client, usually when
the client is newer than the daemon, an HTTP 400 is now returned instead
of a 404.

`GET /containers/(id)/stats`

**New!**
You can now supply a `stream` bool to get only one set of stats and
disconnect

`GET /containers/(id)/logs`

**New!**

This endpoint now accepts a `since` timestamp parameter.

`GET /info`

**New!**

The fields `Debug`, `IPv4Forwarding`, `MemoryLimit`, and `SwapLimit`
are now returned as boolean instead of as an int.

In addition, the end point now returns the new boolean fields
`CpuCfsPeriod`, `CpuCfsQuota`, and `OomKillDisable`.

## v1.18

### Full documentation

[*Docker Remote API v1.18*](/reference/api/docker_remote_api_v1.18/)

### What's new

`GET /version`

**New!**
This endpoint now returns `Os`, `Arch` and `KernelVersion`.

`POST /containers/create`

`POST /containers/(id)/start`

**New!**
You can set ulimit settings to be used within the container.

`GET /info`

**New!**
This endpoint now returns `SystemTime`, `HttpProxy`,`HttpsProxy` and `NoProxy`.

`GET /images/json`

**New!**
Added a `RepoDigests` field to include image digest information.

`POST /build`

**New!**
Builds can now set resource constraints for all containers created for the build.

**New!**
(`CgroupParent`) can be passed in the host config to setup container cgroups under a specific cgroup.

`POST /build`

**New!**
Closing the HTTP request will now cause the build to be canceled.

`POST /containers/(id)/exec`

**New!**
Add `Warnings` field to response.

## v1.17

### Full documentation

[*Docker Remote API v1.17*](/reference/api/docker_remote_api_v1.17/)

### What's new

The build supports `LABEL` command. Use this to add metadata
to an image. For example you could add data describing the content of an image.

`LABEL "com.example.vendor"="ACME Incorporated"`

**New!**
`POST /containers/(id)/attach` and `POST /exec/(id)/start`

**New!**
Docker client now hints potential proxies about connection hijacking using HTTP Upgrade headers.

`POST /containers/create`

**New!**
You can set labels on container create describing the container.

`GET /containers/json`

**New!**
The endpoint returns the labels associated with the containers (`Labels`).

`GET /containers/(id)/json`

**New!**
This endpoint now returns the list current execs associated with the container (`ExecIDs`).
This endpoint now returns the container labels (`Config.Labels`).

`POST /containers/(id)/rename`

**New!**
New endpoint to rename a container `id` to a new name.

`POST /containers/create`
`POST /containers/(id)/start`

**New!**
(`ReadonlyRootfs`) can be passed in the host config to mount the container's
root filesystem as read only.

`GET /containers/(id)/stats`

**New!**
This endpoint returns a live stream of a container's resource usage statistics.

`GET /images/json`

**New!**
This endpoint now returns the labels associated with each image (`Labels`).


## v1.16

### Full documentation

[*Docker Remote API v1.16*](/reference/api/docker_remote_api_v1.16/)

### What's new

`GET /info`

**New!**
`info` now returns the number of CPUs available on the machine (`NCPU`),
total memory available (`MemTotal`), a user-friendly name describing the running Docker daemon (`Name`), a unique ID identifying the daemon (`ID`), and
a list of daemon labels (`Labels`).

`POST /containers/create`

**New!**
You can set the new container's MAC address explicitly.

**New!**
Volumes are now initialized when the container is created.

`POST /containers/(id)/copy`

**New!**
You can now copy data which is contained in a volume.

## v1.15

### Full documentation

[*Docker Remote API v1.15*](/reference/api/docker_remote_api_v1.15/)

### What's new

`POST /containers/create`

**New!**
It is now possible to set a container's HostConfig when creating a container.
Previously this was only available when starting a container.

## v1.14

### Full documentation

[*Docker Remote API v1.14*](/reference/api/docker_remote_api_v1.14/)

### What's new

`DELETE /containers/(id)`

**New!**
When using `force`, the container will be immediately killed with SIGKILL.

`POST /containers/(id)/start`

**New!**
The `hostConfig` option now accepts the field `CapAdd`, which specifies a list of capabilities
to add, and the field `CapDrop`, which specifies a list of capabilities to drop.

`POST /images/create`

**New!**
The `fromImage` and `repo` parameters now supports the `repo:tag` format.
Consequently,  the `tag` parameter is now obsolete. Using the new format and
the `tag` parameter at the same time will return an error.


