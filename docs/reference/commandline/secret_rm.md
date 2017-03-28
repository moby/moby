---
title: "secret rm"
description: "The secret rm command description and usage"
keywords: ["secret, rm"]
---

<!-- This file is maintained within the docker/docker Github
     repository at https://github.com/docker/docker/. Make all
     pull requests against that repo. If you see this file in
     another repository, consider it read-only there, as it will
     periodically be overwritten by the definitive file. Pull
     requests which include edits to this file in other repositories
     will be rejected.
-->

# secret rm

```Markdown
Usage:	docker secret rm SECRET [SECRET...]

Remove one or more secrets

Aliases:
  rm, remove

Options:
      --help   Print usage
```

## Description

Removes the specified secrets from the swarm. This command has to be run
targeting a manager node.

For detailed information about using secrets, refer to [manage sensitive data with Docker secrets](https://docs.docker.com/engine/swarm/secrets/).

## Examples

This example removes a secret:

```bash
$ docker secret rm secret.json
sapth4csdo5b6wz2p5uimh5xg
```

> **Warning**: Unlike `docker rm`, this command does not ask for confirmation
> before removing a secret.


## Related commands

* [secret create](secret_create.md)
* [secret inspect](secret_inspect.md)
* [secret ls](secret_ls.md)
