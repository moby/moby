page_title: Docker 1.x Series Release Notes
page_description: Release Notes for Docker 1.x.
page_keywords: docker, documentation, about, technology, understanding, release

#Release Notes

##Version 1.3.3
(2014-12-11)
 
This release fixes several security issues. In order to encourage immediate
upgrading, this release also patches some critical bugs. All users are highly
encouraged to upgrade as soon as possible.
 
*Security fixes*
 
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
 
*Runtime fixes*
 
* Fixed an issue that cause image archives to be read slowly.
 
*Client fixes*
 
* Fixed a regression related to STDIN redirection.
* Fixed a regression involving `docker cp` when the current directory is the
destination.

##Version 1.3.2
(2014-11-24)

This release fixes some bugs and addresses some security issues. We have also
made improvements to aspects of `docker run`.

*Security fixes*

Patches and changes were made to address CVE-2014-6407 and CVE-2014-6408.
Specifically, changes were made in order to:

* Prevent host privilege escalation from an image extraction vulnerability (CVE-2014-6407).

* Prevent container escalation from malicious security options applied to images (CVE-2014-6408).

*Daemon fixes*

The `--insecure-registry` flag of the `docker run` command has undergone
several refinements and additions. For details, please see the
[command-line reference](http://docs.docker.com/reference/commandline/cli/#run).

* You can now specify a sub-net in order to set a range of registries which the Docker daemon will consider insecure.

* By default, Docker now defines `localhost` as an insecure registry.

* Registries can now be referenced using the Classless Inter-Domain Routing (CIDR) format.

* When mirroring is enabled, the experimental registry v2 API is skipped.

##Version 1.3.1
(2014-10-28)

This release fixes some bugs and addresses some security issues.

*Security fixes*

Patches and changes were made to address [CVE-2014-5277 and CVE-2014-3566](https://groups.google.com/forum/#!topic/docker-user/oYm0i3xShJU).
Specifically, changes were made to:

* Prevent fallback to SSL protocols < TLS 1.0 for client, daemon and registry
* Secure HTTPS connection to registries with certificate verification and without HTTP fallback unless [`--insecure-registry`](/reference/commandline/cli/#run) is specified.

*Runtime fixes*

* Fixed issue where volumes would not be shared.

*Client fixes*

* Fixed issue with `--iptables=false` not automatically setting
`--ip-masq=false`.
* Fixed docker run output to non-TTY stdout.

*Builder fixes*

* Fixed escaping `$` for environment variables.
* Fixed issue with lowercase `onbuild` instruction in a `Dockerfile`.
* Restricted environment variable expansion to `ENV`, `ADD`, `COPY`, `WORKDIR`,
`EXPOSE`, `VOLUME`, and `USER`

##Version 1.3.0

This version fixes a number of bugs and issues and adds new functions and other
improvements. The [GitHub 1.3milestone](https://github.com/docker/docker/issues?q=milestone%3A1.3.0+) has
more detailed information. Major additions and changes include:

###New Features

*New command: `docker exec`*

The new `docker exec` command lets you run a process in an existing, active
container. The command has APIs for both the daemon and the client. With `docker
exec`, you'll be able to do things like add or remove devices from running
containers, debug running containers, and run commands that are not part of the
container's static specification. Details in the [command line reference](/reference/commandline/cli/#exec).

*New command: `docker create`*

Traditionally, the `docker run` command has been used to both create a container
and spawn a process to run it. The new `docker create` command breaks this
apart, letting you set up a container without actually starting it. This
provides more control over management of the container lifecycle, giving you the
ability to configure things like volumes or port mappings before the container
is started. For example, in a rapid-response scaling situation, you could use
`create` to prepare and stage ten containers in anticipation of heavy loads.
Details in the [command line reference](/reference/commandline/cli/#create).

*Tech preview of new provenance features*

This release offers a sneak peek at new image signing capabilities that are
currently under development. Soon, these capabilities will allow any image
author to sign their images to certify they have not been tampered with. For
this release, Official images are now signed by Docker, Inc. Not only does this
demonstrate the new functionality, we hope it will improve your confidence in
the security of Official images. Look for the blue ribbons denoting signed
images on the [Docker Hub](https://hub.docker.com/). The Docker Engine has been
updated to automatically verify that a given Official Repo has a current, valid
signature. When pulling a signed image, you'll see a message stating `the image
you are pulling has been verified`. If no valid signature is detected, Docker
Engine will fall back to pulling a regular, unsigned image.

###Other improvements & changes*

* We've added a new security options flag to the `docker run` command,
`--security-opt`, that lets you set SELinux and AppArmor labels and profiles.
This means you'll  no longer have to use `docker run --privileged` on kernels
that support SE Linux or AppArmor. For more information, see the [command line
reference](/reference/commandline/cli/#run).

* A new flag, `--add-host`, has been added to `docker run` that lets you add
lines to `/etc/hosts`. This allows you to specify different name resolution for
the container than it would get via DNS. For more information, see the [command
line reference](/reference/commandline/cli/#run).

* You can now set a `DOCKER_TLS_VERIFY` environment variable to secure
connections by default (rather than having to pass the `--tlsverify` flag on
every call). For more information, see the [https guide](/articles/https).

* Three security issues have been addressed in this release: [CVE-2014-5280,
CVE-2014-5270, and
CVE-2014-5282](https://groups.google.com/forum/#!msg/docker-announce/aQoVmQlcE0A/smPuBNYf8VwJ).

##Version 1.2.0

This version fixes a number of bugs and issues and adds new functions and other
improvements. These include:

###New Features

*New restart policies*

We added a `--restart flag` to `docker run` to specify a restart policy for your
container. Currently, there are three policies available:

* `no` – Do not restart the container if it dies. (default) * `on-failure` –
Restart the container if it exits with a non-zero exit code. This can also
accept an optional maximum restart count (e.g. `on-failure:5`). * `always` –
Always restart the container no matter what exit code is returned. This
deprecates the `--restart` flag on the Docker daemon.

*New flags for `docker run`: `--cap-add` and `--cap-drop`*

In previous releases, Docker containers could either be given complete
capabilities or they could all follow a whitelist of allowed capabilities while
dropping all others. Further, using `--privileged` would grant all capabilities
inside a container, rather than applying a whitelist. This was not recommended
for production use because it’s really unsafe; it’s as if you were directly in
the host.

This release introduces two new flags for `docker run`, `--cap-add` and
`--cap-drop`, that give you fine-grain control over the specific capabilities
you want grant to a particular container.

*New `--device` flag for `docker run`*

Previously, you could only use devices inside your containers by bind mounting
them (with `-v`) in a `--privileged` container. With this release, we introduce
the `--device flag` to `docker run` which lets you use a device without
requiring a privileged container.

*Writable `/etc/hosts`, `/etc/hostname` and `/etc/resolv.conf`*

You can now edit `/etc/hosts`, `/etc/hostname` and `/etc/resolve.conf` in a
running container. This is useful if you need to install BIND or other services
that might override one of those files.

Note, however, that changes to these files are not saved when running `docker
build` and so will not be preserved in the resulting image. The changes will
only “stick” in a running container.

*Docker proxy in a separate process*

The Docker userland proxy that routes outbound traffic to your containers now
has its own separate process (one process per connection). This greatly reduces
the load on the daemon, which increases stability and efficiency.

###Other improvements & changes

* When using `docker rm -f`, Docker now kills the container (instead of stopping
it) before removing it . If you intend to stop the container cleanly, you can
use `docker stop`.

* Added support for IPv6 addresses in `--dns`

* Added search capability in private registries

##Version 1.1.0

###New Features

*`.dockerignore` support*

You can now add a `.dockerignore` file next to your `Dockerfile` and Docker will
ignore files and directories specified in that file when sending the build
context to the daemon. Example:
https://github.com/docker/docker/blob/master/.dockerignore

*Pause containers during commit*

Doing a commit on a running container was not recommended because you could end
up with files in an inconsistent state (for example, if they were being written
during the commit). Containers are now paused when a commit is made to them. You
can disable this feature by doing a `docker commit --pause=false <container_id>`

*Tailing logs*

You can now tail the logs of a container. For example, you can get the last ten
lines of a log by using `docker logs --tail 10 <container_id>`. You can also
follow the logs of a container without having to read the whole log file with
`docker logs --tail 0 -f <container_id>`.

*Allow a tar file as context for docker build*

You can now pass a tar archive to `docker build` as context. This can be used to
automate docker builds, for example: `cat context.tar | docker build -` or
`docker run builder_image | docker build -`

*Bind mounting your whole filesystem in a container*

`/` is now allowed as source of `--volumes`. This means you can bind-mount your
whole system in a container if you need to. For example: `docker run -v
/:/my_host ubuntu:ro ls /my_host`. However, it is now forbidden to mount to /.


###Other Improvements & Changes

* Port allocation has been improved. In the previous release, Docker could
prevent you from starting a container with previously allocated ports which
seemed to be in use when in fact they were not. This has been fixed.

* A bug in `docker save` was introduced in the last release. The `docker save`
command could produce images with invalid metadata. The command now produces
images with correct metadata.

* Running `docker inspect` in a container now returns which containers it is
linked to.

* Parsing of the `docker commit` flag has improved validation, to better prevent
you from committing an image with a name such as  `-m`. Image names with dashes
in them potentially conflict with command line flags.

* The API now has Improved status codes for  `start` and `stop`. Trying to start
a running container will now return a 304 error.

* Performance has been improved overall. Starting the daemon is faster than in
previous releases. The daemon’s performance has also been improved when it is
working with large numbers of images and containers.

* Fixed an issue with white-spaces and multi-lines in Dockerfiles.

##Version 1.1.0

###New Features

*`.dockerignore` support*

You can now add a `.dockerignore` file next to your `Dockerfile` and Docker will
ignore files and directories specified in that file when sending the build
context to the daemon. Example:
https://github.com/dotcloud/docker/blob/master/.dockerignore

*Pause containers during commit*

Doing a commit on a running container was not recommended because you could end
up with files in an inconsistent state (for example, if they were being written
during the commit). Containers are now paused when a commit is made to them. You
can disable this feature by doing a `docker commit --pause=false <container_id>`

*Tailing logs*

You can now tail the logs of a container. For example, you can get the last ten
lines of a log by using `docker logs --tail 10 <container_id>`. You can also
follow the logs of a container without having to read the whole log file with
`docker logs --tail 0 -f <container_id>`.

*Allow a tar file as context for docker build*

You can now pass a tar archive to `docker build` as context. This can be used to
automate docker builds, for example: `cat context.tar | docker build -` or
`docker run builder_image | docker build -`

*Bind mounting your whole filesystem in a container*

`/` is now allowed as source of `--volumes`. This means you can bind-mount your
whole system in a container if you need to. For example: `docker run -v
/:/my_host ubuntu:ro ls /my_host`. However, it is now forbidden to mount to /.


###Other Improvements & Changes

* Port allocation has been improved. In the previous release, Docker could
prevent you from starting a container with previously allocated ports which
seemed to be in use when in fact they were not. This has been fixed.

* A bug in `docker save` was introduced in the last release. The `docker save`
command could produce images with invalid metadata. The command now produces
images with correct metadata.

* Running `docker inspect` in a container now returns which containers it is
linked to.

* Parsing of the `docker commit` flag has improved validation, to better prevent
you from committing an image with a name such as  `-m`. Image names with dashes
in them potentially conflict with command line flags.

* The API now has Improved status codes for  `start` and `stop`. Trying to start
a running container will now return a 304 error.

* Performance has been improved overall. Starting the daemon is faster than in
previous releases. The daemon’s performance has also been improved when it is
working with large numbers of images and containers.

* Fixed an issue with white-spaces and multi-lines in Dockerfiles.

##Version 1.0.0

First production-ready release. Prior development history can be found by
searching in [GitHub](https://github.com/docker/docker).
