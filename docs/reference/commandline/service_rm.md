<!--[metadata]>
+++
title = "service rm"
description = "The service rm command description and usage"
keywords = ["service, rm"]
[menu.main]
parent = "smn_cli"
+++
<![end-metadata]-->

**Warning:** this command is part of the Swarm management feature introduced in Docker 1.12, and might be subject to non backward-compatible changes.

# service rm

```Markdown
Usage:	docker service rm [OPTIONS] SERVICE

Remove a service

Aliases:
  rm, remove

Options:
      --help   Print usage
```

Removes the specified services from the swarm. This command has to be run
targeting a manager node.

For example, to remove the redis service:

```bash
$ docker service rm redis
redis
$ docker service ls
ID            NAME   SCALE  IMAGE        COMMAND
```

> **Warning**: Unlike `docker rm`, this command does not ask for confirmation
> before removing a running service.



## Related information

* [service create](service_create.md)
* [service inspect](service_inspect.md)
* [service ls](service_ls.md)
* [service scale](service_scale.md)
* [service ps](service_ps.md)
* [service update](service_update.md)
