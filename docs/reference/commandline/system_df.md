---
title: "system df"
description: "The system df command description and usage"
keywords: [system, data, usage, disk]
---

# system df

```markdown
Usage:	docker system df [OPTIONS]

Show docker filesystem usage

Options:
      --help      Print usage
  -v, --verbose   Show detailed information on space usage
```

The `docker system df` command displays information regarding the
amount of disk space used by the docker daemon.

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

## Related Information
* [system prune](system_prune.md)
* [container prune](container_prune.md)
* [volume prune](volume_prune.md)
* [image prune](image_prune.md)
