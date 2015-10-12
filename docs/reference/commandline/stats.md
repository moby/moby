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

    Usage: docker stats [OPTIONS] CONTAINER [CONTAINER...]

    Display a live stream of one or more containers' resource usage statistics

      --help=false       Print usage
      --no-stream=false  Disable streaming stats and only pull the first result

Running `docker stats` on multiple containers

    $ docker stats redis1 redis2
    CONTAINER           CPU %               MEM USAGE / LIMIT     MEM %               NET I/O             BLOCK I/O
    redis1              0.07%               796 KB / 64 MB        1.21%               788 B / 648 B       3.568 MB / 512 KB
    redis2              0.07%               2.746 MB / 64 MB      4.29%               1.266 KB / 648 B    12.4 MB / 0 B


The `docker stats` command will only return a live stream of data for running
containers. Stopped containers will not return any data.

> **Note:**
> If you want more detailed information about a container's resource
> usage, use the API endpoint.
