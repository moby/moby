<!--[metadata]>
+++
title = "stats"
description = "The stats command description and usage"
keywords = ["container, resource, statistics"]
[menu.main]
parent = "smn_cli"
+++
<![end-metadata]-->

# stats

```markdown
Usage:  docker stats [OPTIONS] [CONTAINER...]

Display a live stream of container(s) resource usage statistics

Options:
  -a, --all         Show all containers (default shows just running)
      --help        Print usage
      --no-stream   Disable streaming stats and only pull the first result
```

The `docker stats` command returns a live data stream for running containers. To limit data to one or more specific containers, specify a list of container names or ids separated by a space. You can specify a stopped container but stopped containers do not return any data.

If you want more detailed information about a container's resource usage, use the `/containers/(id)/stats` API endpoint. 

## Examples

Running `docker stats` on all running containers

    $ docker stats
    CONTAINER           CPU %               MEM USAGE / LIMIT     MEM %               NET I/O             BLOCK I/O
    1285939c1fd3        0.07%               796 KiB / 64 MiB        1.21%               788 B / 648 B       3.568 MB / 512 KB
    9c76f7834ae2        0.07%               2.746 MiB / 64 MiB      4.29%               1.266 KB / 648 B    12.4 MB / 0 B
    d1ea048f04e4        0.03%               4.583 MiB / 64 MiB      6.30%               2.854 KB / 648 B    27.7 MB / 0 B

Running `docker stats` on multiple containers by name and id.

    $ docker stats fervent_panini 5acfcb1b4fd1
    CONTAINER           CPU %               MEM USAGE/LIMIT     MEM %               NET I/O
    5acfcb1b4fd1        0.00%               115.2 MiB/1.045 GiB   11.03%              1.422 kB/648 B
    fervent_panini      0.02%               11.08 MiB/1.045 GiB   1.06%               648 B/648 B
