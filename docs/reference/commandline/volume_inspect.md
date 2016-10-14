---
title: "volume inspect"
description: "The volume inspect command description and usage"
keywords: ["volume, inspect"]
---

# volume inspect

```markdown
Usage:  docker volume inspect [OPTIONS] VOLUME [VOLUME...]

Display detailed information on one or more volumes

Options:
  -f, --format string   Format the output using the given go template
      --help            Print usage
```

Returns information about a volume. By default, this command renders all results
in a JSON array. You can specify an alternate format to execute a
given template for each result. Go's
[text/template](http://golang.org/pkg/text/template/) package describes all the
details of the format.

Example output:

    $ docker volume create
    85bffb0677236974f93955d8ecc4df55ef5070117b0e53333cc1b443777be24d
    $ docker volume inspect 85bffb0677236974f93955d8ecc4df55ef5070117b0e53333cc1b443777be24d
    [
      {
          "Name": "85bffb0677236974f93955d8ecc4df55ef5070117b0e53333cc1b443777be24d",
          "Driver": "local",
          "Mountpoint": "/var/lib/docker/volumes/85bffb0677236974f93955d8ecc4df55ef5070117b0e53333cc1b443777be24d/_data",
          "Status": null
      }
    ]

    $ docker volume inspect --format '{{ .Mountpoint }}' 85bffb0677236974f93955d8ecc4df55ef5070117b0e53333cc1b443777be24d
    /var/lib/docker/volumes/85bffb0677236974f93955d8ecc4df55ef5070117b0e53333cc1b443777be24d/_data

## Related information

* [volume create](volume_create.md)
* [volume ls](volume_ls.md)
* [volume rm](volume_rm.md)
* [Understand Data Volumes](../../tutorials/dockervolumes.md)
