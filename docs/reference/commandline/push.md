---
title: "push"
description: "The push command description and usage"
keywords: "share, push, image"
---

<!-- This file is maintained within the docker/docker Github
     repository at https://github.com/docker/docker/. Make all
     pull requests against that repo. If you see this file in
     another repository, consider it read-only there, as it will
     periodically be overwritten by the definitive file. Pull
     requests which include edits to this file in other repositories
     will be rejected.
-->

# push

```markdown
Usage:  docker push [OPTIONS] NAME[:TAG]

Push an image or a repository to a registry

Options:
      --disable-content-trust   Skip image signing (default true)
      --help                    Print usage
```

## Description

Use `docker push` to share your images to the [Docker Hub](https://hub.docker.com)
registry or to a self-hosted one.

Refer to the [`docker tag`](tag.md) reference for more information about valid
image and tag names.

Killing the `docker push` process, for example by pressing `CTRL-c` while it is
running in a terminal, terminates the push operation.

Progress bars are shown during docker push, which show the uncompressed size. The 
actual amount of data that's pushed will be compressed before sending, so the uploaded
 size will not be reflected by the progress bar. 

Registry credentials are managed by [docker login](login.md).

### Concurrent uploads

By default the Docker daemon will push five layers of an image at a time.
If you are on a low bandwidth connection this may cause timeout issues and you may want to lower
this via the `--max-concurrent-uploads` daemon option. See the
[daemon documentation](dockerd.md) for more details.

## Examples

### Push a new image to a registry

First save the new image by finding the container ID (using [`docker ps`](ps.md))
and then committing it to a new image name.  Note that only `a-z0-9-_.` are
allowed when naming images:

```bash
$ docker commit c16378f943fe rhel-httpd
```

Now, push the image to the registry using the image ID. In this example the
registry is on host named `registry-host` and listening on port `5000`. To do
this, tag the image with the host name or IP address, and the port of the
registry:

```bash
$ docker tag rhel-httpd registry-host:5000/myadmin/rhel-httpd

$ docker push registry-host:5000/myadmin/rhel-httpd
```

Check that this worked by running:

```bash
$ docker images
```

You should see both `rhel-httpd` and `registry-host:5000/myadmin/rhel-httpd`
listed.
