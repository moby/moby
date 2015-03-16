page_title: Docker 1.x Series Release Notes
page_description: Release Notes for Docker 1.x.
page_keywords: docker, documentation, about, technology, understanding, release

# Release Notes

You can view release notes for earlier version of Docker by selecting the
desired version from the drop-down list at the top right of this page.

## Version 1.5.0
(2015-02-03)

For a complete list of patches, fixes, and other improvements, see the
[merge PR on GitHub](https://github.com/docker/docker/pull/10286).

*New Features*

* [1.6] The Docker daemon will no longer ignore unknown commands
  while processing a `Dockerfile`. Instead it will generate an error and halt
  processing.
* The Docker daemon has now supports for IPv6 networking between containers
  and on the `docker0` bridge. For more information see the
  [IPv6 networking reference](/articles/networking/#ipv6).
* Docker container filesystems can now be set to`--read-only`, restricting your
  container to writing to volumes [PR# 10093](https://github.com/docker/docker/pull/10093).
* A new `docker stats CONTAINERID` command has been added to allow users to view a
  continuously updating stream of container resource usage statistics. See the
  [`stats` command line reference](/reference/commandline/cli/#stats) and the
  [container `stats` API reference](/reference/api/docker_remote_api_v1.17/#get-container-stats-based-on-resource-usage).
  **Note**: this feature is only enabled for the `libcontainer` exec-driver at this point.
* Users can now specify the file to use as the `Dockerfile` by running
  `docker build -f alternate.dockerfile .`. This will allow the definition of multiple
  `Dockerfile`s for a single project. See the [`docker build` command reference](
/reference/commandline/cli/#build) for more information.
* The v1 Open Image specification has been created to document the current Docker image
  format and metadata. Please see [the Open Image specification document](
https://github.com/docker/docker/blob/master/image/spec/v1.md) for more details.
* This release also includes a number of significant performance improvements in
  build and image management ([PR #9720](https://github.com/docker/docker/pull/9720),
  [PR #8827](https://github.com/docker/docker/pull/8827))
* The `docker inspect` command now lists ExecIDs generated for each `docker exec` process.
  See [PR #9800](https://github.com/docker/docker/pull/9800)) for more details.
* The `docker inspect` command now shows the number of container restarts when there
  is a restart policy ([PR #9621](https://github.com/docker/docker/pull/9621))
* This version of Docker is built using Go 1.4

> **Note:**
> Development history prior to version 1.0 can be found by
> searching in the [Docker GitHub repo](https://github.com/docker/docker).

## Known Issues

This section lists significant known issues present in Docker as of release
date. It is not exhaustive; it lists only issues with potentially significant
impact on users. This list will be updated as issues are resolved.

* **Unexpected File Permissions in Containers**
An idiosyncrasy in AUFS prevents permissions from propagating predictably
between upper and lower layers. This can cause issues with accessing private
keys, database instances, etc. For complete information and workarounds see
[Github Issue 783](https://github.com/docker/docker/issues/783).

* **Docker Hub incompatible with Safari 8**
Docker Hub has multiple issues displaying on Safari 8, the default browser
for OS X 10.10 (Yosemite). Users should access the hub using a different
browser. Most notably, changes in the way Safari handles cookies means that the
user is repeatedly logged out. For more information, see the [Docker
forum post](https://forums.docker.com/t/new-safari-in-yosemite-issue/300).
