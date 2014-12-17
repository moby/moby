page_title: Docker 1.x Series Release Notes
page_description: Release Notes for Docker 1.x.
page_keywords: docker, documentation, about, technology, understanding, release

#Release Notes

You can view release notes for earlier version of Docker by selecting the
desired version from the drop-down list at the top right of this page.

##Version 1.4.1
(2014-12-17)
 
This release fixes an issue related to mounting volumes on `create`. Details available in the [Github milestone](https://github.com/docker/docker/issues?q=milestone%3A1.4.1+is%3Aclosed).

##Version 1.4.0
(2014-12-11)
 
This release provides a number of new features, but is mainly focused on bug
fixes and improvements to platform stability and security.

For a complete list of patches, fixes, and other improvements, see the [merge PR on GitHub](https://github.com/docker/docker/pull/9345).
 
*New Features*

* You can now add labels to the Docker daemon using key=value pairs defined with
the new `--label` flag. The labels are displayed by running `docker info`. In
addition, `docker info` also now returns an ID and hostname field. For more
information, see  the 
[command line reference](http://docs.docker.com/reference/commandline/cli/#daemon).
* The `ENV` instruction in the `Dockerfile` now supports arguments in the form 
of `ENV name=value name2=value2..`. For more information, see the 
[command line reference](http://docs.docker.com/reference/builder/#env)
* Introducing a new, still 
[experimental, overlayfs storage driver](https://github.com/docker/docker/pull/7619/).
* You can now add filters to `docker events` to filter events by event name, 
container, or image. For more information, see  the 
[command line reference](http://docs.docker.com/reference/commandline/cli/#events).
* The `docker cp` command now supports copying files from the filesystem of a
container's volumes. For more information, see  the 
[remote API reference](http://docs.docker.com/reference/api/docker_remote_api/).
* The `docker tag` command has been fixed so that it correctly honors `--force`
when overriding a tag for existing image. For more information, see 
the [command line reference](http://docs.docker.com/reference/commandline/cli/#tag).

* Container volumes are now initialized during `docker create`. For more information, see 
the [command line reference](http://docs.docker.com/reference/commandline/cli/#create). 

*Security Fixes*

Patches and changes were made to address the following vulnerabilities:

* CVE-2014-9356: Path traversal during processing of absolute symlinks. 
Absolute symlinks were not adequately checked for  traversal which created a
vulnerability via image extraction and/or volume mounts.
* CVE-2014-9357: Escalation of privileges during decompression of LZMA (.xz)
archives. Docker 1.3.2 added `chroot` for archive extraction. This created a
vulnerability that could allow malicious images or builds to write files to the
host system and escape containerization, leading to privilege escalation.
* CVE-2014-9358: Path traversal and spoofing opportunities via image
identifiers. Image IDs passed either via `docker load` or registry communications
were not sufficiently validated. This created a vulnerability to path traversal
attacks wherein malicious images or repository spoofing could lead to graph
corruption and manipulation.

> **Note:** the above CVEs are also patched in Docker 1.3.3, which was released
> concurrently with 1.4.0.

*Runtime fixes*

* Fixed an issue that caused image archives to be read slowly.

*Client fixes*
 
* Fixed a regression related to STDIN redirection.
* Fixed a regression involving `docker cp` when the current directory is the
destination.

> **Note:**
> Development history prior to version 1.0 can be found by
> searching in the [Docker GitHub repo](https://github.com/docker/docker).

