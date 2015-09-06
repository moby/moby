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

Docker's Remote API uses an open schema model.  In this model, unknown 
properties in incoming messages are ignored. Client applications need to take
this behavior into account to ensure they do not break when talking to newer
Docker daemons.

The API tends to be REST, but for some complex commands, like attach or pull,
the HTTP connection is hijacked to transport STDOUT, STDIN, and STDERR.
   
By default the Docker daemon listens on `unix:///var/run/docker.sock` and the
client must have `root` access to interact with the daemon. If a group named
`docker` exists on your system, `docker` applies ownership of the socket to the
group.

The current version of the API is v1.21 which means calling `/info` is the same
as calling `/v1.21/info`. To call an older version of the API use
`/v1.20/info`.

## Authentication

Since API version 1.2, the auth configuration is now handled client side, so the
client has to send the `authConfig` as a `POST` in `/images/(name)/push`. The
`authConfig`, set as the `X-Registry-Auth` header, is currently a Base64 encoded
(JSON) string with the following structure:

```
{"username": "string", "password": "string", "email": "string",
   "serveraddress" : "string", "auth": ""}
```
   
Callers should leave the `auth` empty. The `serveraddress` is a domain/ip
without protocol. Throughout this structure, double quotes are required.

## Using Docker Machine with the API

If you are using `docker-machine`, the Docker daemon is on a virtual host that uses an encrypted TCP socket. This means, for Docker Machine users, you need to add extra parameters to `curl` or `wget` when making test API requests, for example:

```
curl --insecure --cert ~/.docker/cert.pem --key ~/.docker/key.pem https://YOUR_VM_IP:2376/images/json

wget --no-check-certificate --certificate=$DOCKER_CERT_PATH/cert.pem --private-key=$DOCKER_CERT_PATH/key.pem https://your_vm_ip:2376/images/json -O - -q
```

## Docker Events

The following diagram depicts the container states accessible through the API.

![States](../images/event_state.png)

Some container-related events are not affected by container state, so they are not included in this diagram. These events are:

* **export** emitted by `docker export`
* **exec_create** emitted by `docker exec`
* **exec_start** emitted by `docker exec` after **exec_create**

Running `docker rmi` emits an **untag** event when removing an image name.  The `rmi` command may also emit **delete** events when images are deleted by ID directly or by deleting the last tag referring to the image.

> **Acknowledgement**: This diagram and the accompanying text were used with the permission of Matt Good and Gilder Labs. See Matt's original blog post [Docker Events Explained](http://gliderlabs.com/blog/2015/04/14/docker-events-explained/).

## Version history

This section lists each version from latest to oldest.  Each listing includes a link to the full documentation set and the changes relevant in that release.

### v1.21 API changes

[Docker Remote API v1.21](/reference/api/docker_remote_api_v1.21/) documentation

* `GET /volumes` lists volumes from all volume drivers.
* `POST /volumes` to create a volume.
* `GET /volumes/(name)` get low-level information about a volume.
* `DELETE /volumes/(name)`remove a volume with the specified name.


### v1.20 API changes

[Docker Remote API v1.20](/reference/api/docker_remote_api_v1.20/) documentation

* `GET /containers/(id)/archive` get an archive of filesystem content from a container.
* `PUT /containers/(id)/archive` upload an archive of content to be extracted to
an existing directory inside a container's filesystem.
* `POST /containers/(id)/copy` is deprecated in favor of the above `archive`
endpoint which can be used to download files and directories from a container.
* The `hostConfig` option now accepts the field `GroupAdd`, which specifies a
list of additional groups that the container process will run as.

### v1.19 API changes

[Docker Remote API v1.19](/reference/api/docker_remote_api_v1.19/) documentation

* When the daemon detects a version mismatch with the client, usually when
the client is newer than the daemon, an HTTP 400 is now returned instead
of a 404.
* `GET /containers/(id)/stats` now accepts `stream` bool to get only one set of stats and disconnect.
* `GET /containers/(id)/logs` now accepts a `since` timestamp parameter.
* `GET /info` The fields `Debug`, `IPv4Forwarding`, `MemoryLimit`, and
`SwapLimit` are now returned as boolean instead of as an int. In addition, the
end point now returns the new boolean fields `CpuCfsPeriod`, `CpuCfsQuota`, and
`OomKillDisable`.

### v1.18 API changes

[Docker Remote API v1.18](/reference/api/docker_remote_api_v1.18/) documentation

* `GET /version` now returns `Os`, `Arch` and `KernelVersion`.
* `POST /containers/create` and `POST /containers/(id)/start`allow you to  set ulimit settings for use in the container.
* `GET /info` now returns `SystemTime`, `HttpProxy`,`HttpsProxy` and `NoProxy`.
* `GET /images/json` added a `RepoDigests` field to include image digest information.
* `POST /build` can now set resource constraints for all containers created for the build.
* `CgroupParent` can be passed in the host config to setup container cgroups under a specific cgroup.
* `POST /build` closing the HTTP request cancels the build
* `POST /containers/(id)/exec` includes `Warnings` field to response.

### v1.17 API changes

[Docker Remote API v1.17](/reference/api/docker_remote_api_v1.17/) documentation

* The build supports `LABEL` command. Use this to add metadata to an image. For
example you could add data describing the content of an image. `LABEL
"com.example.vendor"="ACME Incorporated"`
* `POST /containers/(id)/attach` and `POST /exec/(id)/start`
* The Docker client now hints potential proxies about connection hijacking using HTTP Upgrade headers.
* `POST /containers/create` sets labels on container create describing the container.
* `GET /containers/json` returns the labels associated with the containers (`Labels`).
* `GET /containers/(id)/json` returns the list current execs associated with the
container (`ExecIDs`). This endpoint now returns the container labels
(`Config.Labels`).
* `POST /containers/(id)/rename` renames a container `id` to a new name.* 
* `POST /containers/create` and `POST /containers/(id)/start` callers can pass
`ReadonlyRootfs` in the host config to mount the container's root filesystem as
read only.
* `GET /containers/(id)/stats` returns a live stream of a container's resource usage statistics.
* `GET /images/json` returns the labels associated with each image (`Labels`).


### v1.16 API changes

[Docker Remote API v1.16](/reference/api/docker_remote_api_v1.16/)

* `GET /info` returns the number of CPUs available on the machine (`NCPU`),
total memory available (`MemTotal`), a user-friendly name describing the running Docker daemon (`Name`), a unique ID identifying the daemon (`ID`), and
a list of daemon labels (`Labels`).
* `POST /containers/create` callers can set the new container's MAC address explicitly.
* Volumes are now initialized when the container is created.
* `POST /containers/(id)/copy` copies data which is contained in a volume.

### v1.15 API changes

[Docker Remote API v1.15](/reference/api/docker_remote_api_v1.15/) documentation

`POST /containers/create` you can set a container's `HostConfig` when creating a
container. Previously this was only available when starting a container.

### v1.14 API changes

[Docker Remote API v1.14](/reference/api/docker_remote_api_v1.14/) documentation

* `DELETE /containers/(id)` when using `force`, the container will be immediately killed with SIGKILL.
* `POST /containers/(id)/start` the `hostConfig` option accepts the field `CapAdd`, which specifies a list of capabilities
to add, and the field `CapDrop`, which specifies a list of capabilities to drop.
* `POST /images/create` th `fromImage` and `repo` parameters supportthe
`repo:tag` format. Consequently,  the `tag` parameter is now obsolete. Using the
new format and the `tag` parameter at the same time will return an error.



