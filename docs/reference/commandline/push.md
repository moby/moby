---
title: "push"
description: "The push command description and usage"
keywords: ["share, push, image"]
---

# push

```markdown
Usage:  docker push [OPTIONS] NAME[:TAG]

Push an image or a repository to a registry

Options:
      --disable-content-trust   Skip image verification (default true)
      --help                    Print usage
```

Use `docker push` to share your images to the [Docker Hub](https://hub.docker.com)
registry or to a self-hosted one.

Refer to the [`docker tag`](tag.md) reference for more information about valid
image and tag names.

Killing the `docker push` process, for example by pressing `CTRL-c` while it is
running in a terminal, terminates the push operation.

Registry credentials are managed by [docker login](login.md).

## Examples

### Pushing a new image to a registry

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
