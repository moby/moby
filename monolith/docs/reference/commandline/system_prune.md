---
title: "system prune"
description: "Remove unused data"
keywords: "system, prune, delete, remove"
---

<!-- This file is maintained within the docker/docker Github
     repository at https://github.com/docker/docker/. Make all
     pull requests against that repo. If you see this file in
     another repository, consider it read-only there, as it will
     periodically be overwritten by the definitive file. Pull
     requests which include edits to this file in other repositories
     will be rejected.
-->

# system prune

```markdown
Usage:	docker system prune [OPTIONS]

Delete unused data

Options:
  -a, --all             Remove all unused images not just dangling ones
      --filter filter   Provide filter values (e.g. 'until=<timestamp>')
  -f, --force           Do not prompt for confirmation
      --help            Print usage
```

## Description

Remove all unused containers, volumes, networks and images (both dangling and unreferenced).

## Examples

```bash
$ docker system prune -a

WARNING! This will remove:
	- all stopped containers
	- all volumes not used by at least one container
	- all networks not used by at least one container
	- all images without at least one container associated to them
Are you sure you want to continue? [y/N] y
Deleted Containers:
0998aa37185a1a7036b0e12cf1ac1b6442dcfa30a5c9650a42ed5010046f195b
73958bfb884fa81fa4cc6baf61055667e940ea2357b4036acbbe25a60f442a4d

Deleted Volumes:
named-vol

Deleted Images:
untagged: my-curl:latest
deleted: sha256:7d88582121f2a29031d92017754d62a0d1a215c97e8f0106c586546e7404447d
deleted: sha256:dd14a93d83593d4024152f85d7c63f76aaa4e73e228377ba1d130ef5149f4d8b
untagged: alpine:3.3
deleted: sha256:695f3d04125db3266d4ab7bbb3c6b23aa4293923e762aa2562c54f49a28f009f
untagged: alpine:latest
deleted: sha256:ee4603260daafe1a8c2f3b78fd760922918ab2441cbb2853ed5c439e59c52f96
deleted: sha256:9007f5987db353ec398a223bc5a135c5a9601798ba20a1abba537ea2f8ac765f
deleted: sha256:71fa90c8f04769c9721459d5aa0936db640b92c8c91c9b589b54abd412d120ab
deleted: sha256:bb1c3357b3c30ece26e6604aea7d2ec0ace4166ff34c3616701279c22444c0f3
untagged: my-jq:latest
deleted: sha256:6e66d724542af9bc4c4abf4a909791d7260b6d0110d8e220708b09e4ee1322e1
deleted: sha256:07b3fa89d4b17009eb3988dfc592c7d30ab3ba52d2007832dffcf6d40e3eda7f
deleted: sha256:3a88a5c81eb5c283e72db2dbc6d65cbfd8e80b6c89bb6e714cfaaa0eed99c548

Total reclaimed space: 13.5 MB
```

### Filtering

The filtering flag (`-f` or `--filter`) format is of "key=value". If there is more
than one filter, then pass multiple flags (e.g., `--filter "foo=bar" --filter "bif=baz"`)

The currently supported filters are:

* until (`<timestamp>`) - only remove containers, images, and networks created before given timestamp
* label (`label=<key>`, `label=<key>=<value>`, `label!=<key>`, or `label!=<key>=<value>`) - only remove containers, images, networks, and volumes with (or without, in case `label!=...` is used) the specified labels.

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

The `label` filter accepts two formats. One is the `label=...` (`label=<key>` or `label=<key>=<value>`),
which removes containers, images, networks, and volumes with the specified labels. The other
format is the `label!=...` (`label!=<key>` or `label!=<key>=<value>`), which removes
containers, images, networks, and volumes without the specified labels.

## Related commands

* [volume create](volume_create.md)
* [volume ls](volume_ls.md)
* [volume inspect](volume_inspect.md)
* [volume rm](volume_rm.md)
* [volume prune](volume_prune.md)
* [Understand Data Volumes](https://docs.docker.com/engine/tutorials/dockervolumes/)
* [system df](system_df.md)
* [container prune](container_prune.md)
* [image prune](image_prune.md)
* [network prune](network_prune.md)
* [system prune](system_prune.md)
