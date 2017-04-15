---
title: "system df"
description: "The system df command description and usage"
keywords: "system, data, usage, disk"
---

<!-- This file is maintained within the docker/docker Github
     repository at https://github.com/docker/docker/. Make all
     pull requests against that repo. If you see this file in
     another repository, consider it read-only there, as it will
     periodically be overwritten by the definitive file. Pull
     requests which include edits to this file in other repositories
     will be rejected.
-->

# system df

```markdown
Usage:	docker system df [OPTIONS]

Show docker filesystem usage

Options:
      --help      Print usage
  -v, --verbose   Show detailed information on space usage
```

## Description

The `docker system df` command displays information regarding the
amount of disk space used by the docker daemon.

## Examples

By default the command will just show a summary of the data used:

```bash
$ docker system df

TYPE                TOTAL               ACTIVE              SIZE                RECLAIMABLE
Images              5                   2                   16.43 MB            11.63 MB (70%)
Containers          2                   0                   212 B               212 B (100%)
Local Volumes       2                   1                   36 B                0 B (0%)
```

A more detailed view can be requested using the `-v, --verbose` flag:

```bash
$ docker system df -v

Images space usage:

REPOSITORY          TAG                 IMAGE ID            CREATED             SIZE                SHARED SIZE         UNIQUE SIZE         CONTAINERS
my-curl             latest              b2789dd875bf        6 minutes ago       11 MB               11 MB               5 B                 0
my-jq               latest              ae67841be6d0        6 minutes ago       9.623 MB            8.991 MB            632.1 kB            0
<none>              <none>              a0971c4015c1        6 minutes ago       11 MB               11 MB               0 B                 0
alpine              latest              4e38e38c8ce0        9 weeks ago         4.799 MB            0 B                 4.799 MB            1
alpine              3.3                 47cf20d8c26c        9 weeks ago         4.797 MB            4.797 MB            0 B                 1

Containers space usage:

CONTAINER ID        IMAGE               COMMAND             LOCAL VOLUMES       SIZE                CREATED             STATUS                      NAMES
4a7f7eebae0f        alpine:latest       "sh"                1                   0 B                 16 minutes ago      Exited (0) 5 minutes ago    hopeful_yalow
f98f9c2aa1ea        alpine:3.3          "sh"                1                   212 B               16 minutes ago      Exited (0) 48 seconds ago   anon-vol

Local Volumes space usage:

NAME                                                               LINKS               SIZE
07c7bdf3e34ab76d921894c2b834f073721fccfbbcba792aa7648e3a7a664c2e   2                   36 B
my-named-vol                                                       0                   0 B
```

* `SHARED SIZE` is the amount of space that an image shares with another one (i.e. their common data)
* `UNIQUE SIZE` is the amount of space that is only used by a given image
* `SIZE` is the virtual size of the image, it is the sum of `SHARED SIZE` and `UNIQUE SIZE`

> **Note**: Network information is not shown because it doesn't consume the disk
> space.

## Performance

The `system df` command can be very resource-intensive. It traverses the
filesystem of every image, container, and volume in the system. You should be
careful running this command in systems with lots of images, containers, or
volumes or in systems where some images, containers, or volumes have very large
filesystems with many files. You should also be careful not to run this command
in systems where performance is critical.

## Format the output

The formatting option (`--format`) pretty prints the disk usage output
using a Go template.

Valid placeholders for the Go template are listed below:

| Placeholder    | Description                                |
| -------------- | ------------------------------------------ |
| `.Type`        | `Images`, `Containers` and `Local Volumes` |
| `.TotalCount`  | Total number of items                      |
| `.Active`      | Number of active items                     |
| `.Size`        | Available size                             |
| `.Reclaimable` | Reclaimable size                           |

When using the `--format` option, the `system df` command outputs
the data exactly as the template declares or, when using the
`table` directive, will include column headers as well.

The following example uses a template without headers and outputs the
`Type` and `TotalCount` entries separated by a colon:

```bash
$ docker system df --format "{{.Type}}: {{.TotalCount}}"

Images: 2
Containers: 4
Local Volumes: 1
```

To list the disk usage with size and reclaimable size in a table format you
can use:

```bash
$ docker system df --format "table {{.Type}}\t{{.Size}}\t{{.Reclaimable}}"

TYPE                SIZE                RECLAIMABLE
Images              2.547 GB            2.342 GB (91%)
Containers          0 B                 0 B
Local Volumes       150.3 MB            150.3 MB (100%)
<Paste>
```

**Note** the format option is meaningless when verbose is true.

## Related commands
* [system prune](system_prune.md)
* [container prune](container_prune.md)
* [volume prune](volume_prune.md)
* [image prune](image_prune.md)
* [network prune](network_prune.md)
