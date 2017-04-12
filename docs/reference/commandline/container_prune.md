---
title: "container prune"
description: "Remove all stopped containers"
keywords: container, prune, delete, remove
---

<!-- This file is maintained within the docker/docker Github
     repository at https://github.com/docker/docker/. Make all
     pull requests against that repo. If you see this file in
     another repository, consider it read-only there, as it will
     periodically be overwritten by the definitive file. Pull
     requests which include edits to this file in other repositories
     will be rejected.
-->

# container prune

```markdown
Usage:	docker container prune [OPTIONS]

Remove all stopped containers

Options:
Options:
      --filter filter   Provide filter values (e.g. 'until=<timestamp>')
  -f, --force           Do not prompt for confirmation
      --help            Print usage
```

## Description

Removes all stopped containers.

## Examples

### Prune containers

```bash
$ docker container prune
WARNING! This will remove all stopped containers.
Are you sure you want to continue? [y/N] y
Deleted Containers:
4a7f7eebae0f63178aff7eb0aa39cd3f0627a203ab2df258c1a00b456cf20063
f98f9c2aa1eaf727e4ec9c0283bc7d4aa4762fbdba7f26191f26c97f64090360

Total reclaimed space: 212 B
```

### Filtering

The filtering flag (`-f` or `--filter`) format is of "key=value". If there is more
than one filter, then pass multiple flags (e.g., `--filter "foo=bar" --filter "bif=baz"`)

The currently supported filters are:

* until (`<timestamp>`) - only remove containers created before given timestamp

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

The following removes containers created more than 5 minutes ago:

```bash
{% raw %}
$ docker ps -a --format 'table {{.ID}}\t{{.Image}}\t{{.Command}}\t{{.CreatedAt}}\t{{.Status}}'

CONTAINER ID        IMAGE               COMMAND             CREATED AT                      STATUS
61b9efa71024        busybox             "sh"                2017-01-04 13:23:33 -0800 PST   Exited (0) 41 seconds ago
53a9bc23a516        busybox             "sh"                2017-01-04 13:11:59 -0800 PST   Exited (0) 12 minutes ago

$ docker container prune --force --filter "until=5m"

Deleted Containers:
53a9bc23a5168b6caa2bfbefddf1b30f93c7ad57f3dec271fd32707497cb9369

Total reclaimed space: 25 B

$ docker ps -a --format 'table {{.ID}}\t{{.Image}}\t{{.Command}}\t{{.CreatedAt}}\t{{.Status}}'

CONTAINER ID        IMAGE               COMMAND             CREATED AT                      STATUS
61b9efa71024        busybox             "sh"                2017-01-04 13:23:33 -0800 PST   Exited (0) 44 seconds ago
{% endraw %}
```

The following removes containers created before `2017-01-04T13:10:00`:

```bash
{% raw %}
$ docker ps -a --format 'table {{.ID}}\t{{.Image}}\t{{.Command}}\t{{.CreatedAt}}\t{{.Status}}'

CONTAINER ID        IMAGE               COMMAND             CREATED AT                      STATUS
53a9bc23a516        busybox             "sh"                2017-01-04 13:11:59 -0800 PST   Exited (0) 7 minutes ago
4a75091a6d61        busybox             "sh"                2017-01-04 13:09:53 -0800 PST   Exited (0) 9 minutes ago

$ docker container prune --force --filter "until=2017-01-04T13:10:00"

Deleted Containers:
4a75091a6d618526fcd8b33ccd6e5928ca2a64415466f768a6180004b0c72c6c

Total reclaimed space: 27 B

$ docker ps -a --format 'table {{.ID}}\t{{.Image}}\t{{.Command}}\t{{.CreatedAt}}\t{{.Status}}'

CONTAINER ID        IMAGE               COMMAND             CREATED AT                      STATUS
53a9bc23a516        busybox             "sh"                2017-01-04 13:11:59 -0800 PST   Exited (0) 9 minutes ago
{% endraw %}
```

## Related commands

* [system df](system_df.md)
* [volume prune](volume_prune.md)
* [image prune](image_prune.md)
* [network prune](network_prune.md)
* [system prune](system_prune.md)
