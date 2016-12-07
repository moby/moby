---
title: "network prune"
description: "Remove unused networks"
keywords: "network, prune, delete"
---

# network prune

```markdown
Usage:	docker network prune [OPTIONS]

Remove all unused networks

Options:
      --filter filter   Provide filter values (e.g. 'until=<timestamp>')
  -f, --force           Do not prompt for confirmation
      --help            Print usage
```

Remove all unused networks. Unused networks are those which are not referenced by any containers.

Example output:

```bash
$ docker network prune
WARNING! This will remove all networks not used by at least one container.
Are you sure you want to continue? [y/N] y
Deleted Networks:
n1
n2
```

## Filtering

The filtering flag (`-f` or `--filter`) format is of "key=value". If there is more
than one filter, then pass multiple flags (e.g., `--filter "foo=bar" --filter "bif=baz"`)

The currently supported filters are:

* until (`<timestamp>`) - only remove networks created before given timestamp

The `until` filter can be Unix timestamps, date formatted
timestamps, or Go duration strings (e.g. `10m`, `1h30m`) computed
relative to the daemon machine’s time. Supported formats for date
formatted time stamps include RFC3339Nano, RFC3339, `2006-01-02T15:04:05`,
`2006-01-02T15:04:05.999999999`, `2006-01-02Z07:00`, and `2006-01-02`. The local
timezone on the daemon will be used if you do not provide either a `Z` or a
`+-00:00` timezone offset at the end of the timestamp.  When providing Unix
timestamps enter seconds[.nanoseconds], where seconds is the number of seconds
that have elapsed since January 1, 1970 (midnight UTC/GMT), not counting leap
seconds (aka Unix epoch or Unix time), and the optional .nanoseconds field is a
fraction of a second no more than nine digits long.

The following removes networks created more than 5 minutes ago. Note that
system networks such as `bridge`, `host`, and `none` will never be pruned:

```bash
$ docker network ls
NETWORK ID          NAME                DRIVER              SCOPE
7430df902d7a        bridge              bridge              local
ea92373fd499        foo-1-day-ago       bridge              local
ab53663ed3c7        foo-1-min-ago       bridge              local
97b91972bc3b        host                host                local
f949d337b1f5        none                null                local

$ docker network prune --force --filter until=5m
Deleted Networks:
foo-1-day-ago

$ docker network ls
NETWORK ID          NAME                DRIVER              SCOPE
7430df902d7a        bridge              bridge              local
ab53663ed3c7        foo-1-min-ago       bridge              local
97b91972bc3b        host                host                local
f949d337b1f5        none                null                local
```

## Related information

* [network disconnect ](network_disconnect.md)
* [network connect](network_connect.md)
* [network create](network_create.md)
* [network ls](network_ls.md)
* [network inspect](network_inspect.md)
* [network rm](network_rm.md)
* [Understand Docker container networks](https://docs.docker.com/engine/userguide/networking/)
* [system df](system_df.md)
* [container prune](container_prune.md)
* [image prune](image_prune.md)
* [volume prune](volume_prune.md)
* [system prune](system_prune.md)
