<!--[metadata]>
+++
title = "push"
description = "The push command description and usage"
keywords = ["share, push, image"]
[menu.main]
parent = "smn_cli"
+++
<![end-metadata]-->

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
[Read more about valid image names and tags](tag.md).

Killing the `docker push` process, for example by pressing `CTRL-c` while it is
running in a terminal, will terminate the push operation.

Registry credentials are managed by [docker login](login.md).
