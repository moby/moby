---
title: "stats"
description: "The stats command description and usage"
keywords: "container, resource, statistics"
---

<!-- This file is maintained within the docker/docker Github
     repository at https://github.com/docker/docker/. Make all
     pull requests against that repo. If you see this file in
     another repository, consider it read-only there, as it will
     periodically be overwritten by the definitive file. Pull
     requests which include edits to this file in other repositories
     will be rejected.
-->

# stats

```markdown
Usage:  docker stats [OPTIONS] [CONTAINER...]

Display a live stream of container(s) resource usage statistics

Options:
  -a, --all             Show all containers (default shows just running)
      --format string   Pretty-print images using a Go template
      --help            Print usage
      --no-stream       Disable streaming stats and only pull the first result
```

## Description

The `docker stats` command returns a live data stream for running containers. To limit data to one or more specific containers, specify a list of container names or ids separated by a space. You can specify a stopped container but stopped containers do not return any data.

If you want more detailed information about a container's resource usage, use the `/containers/(id)/stats` API endpoint.

## Examples

Running `docker stats` on all running containers against a Linux daemon.

```bash
$ docker stats
CONTAINER           CPU %               MEM USAGE / LIMIT     MEM %               NET I/O             BLOCK I/O
1285939c1fd3        0.07%               796 KiB / 64 MiB        1.21%               788 B / 648 B       3.568 MB / 512 KB
9c76f7834ae2        0.07%               2.746 MiB / 64 MiB      4.29%               1.266 KB / 648 B    12.4 MB / 0 B
d1ea048f04e4        0.03%               4.583 MiB / 64 MiB      6.30%               2.854 KB / 648 B    27.7 MB / 0 B
```

Running `docker stats` on multiple containers by name and id against a Linux daemon.

```bash
$ docker stats fervent_panini 5acfcb1b4fd1
CONTAINER           CPU %               MEM USAGE/LIMIT       MEM %               NET I/O
5acfcb1b4fd1        0.00%               115.2 MiB/1.045 GiB   11.03%              1.422 kB/648 B
fervent_panini      0.02%               11.08 MiB/1.045 GiB   1.06%               648 B/648 B
```

Running `docker stats` with customized format on all (Running and Stopped) containers.

```bash
$ docker stats --all --format "table {{.ID}}\t{{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}"
CONTAINER ID                                                       NAME                     CPU %               MEM USAGE / LIMIT
c9dfa83f0317f87637d5b7e67aa4223337d947215c5a9947e697e4f7d3e0f834   ecstatic_noether         0.00%               56KiB / 15.57GiB
8f92d01cf3b29b4f5fca4cd33d907e05def7af5a3684711b20a2369d211ec67f   stoic_goodall            0.07%               32.86MiB / 15.57GiB
38dd23dba00f307d53d040c1d18a91361bbdcccbf592315927d56cf13d8b7343   drunk_visvesvaraya       0.00%               0B / 0B
5a8b07ec4cc52823f3cbfdb964018623c1ba307bce2c057ccdbde5f4f6990833   big_heisenberg           0.00%               0B / 0B
```

`drunk_visvesvaraya` and `big_heisenberg` are stopped containers in the above example.

Running `docker stats` on all running containers against a Windows daemon.

```powershell
PS E:\> docker stats
CONTAINER           CPU %               PRIV WORKING SET    NET I/O             BLOCK I/O
09d3bb5b1604        6.61%               38.21 MiB           17.1 kB / 7.73 kB   10.7 MB / 3.57 MB
9db7aa4d986d        9.19%               38.26 MiB           15.2 kB / 7.65 kB   10.6 MB / 3.3 MB
3f214c61ad1d        0.00%               28.64 MiB           64 kB / 6.84 kB     4.42 MB / 6.93 MB
```

Running `docker stats` on multiple containers by name and id against a Windows daemon.

```powershell
PS E:\> docker ps -a
CONTAINER ID        IMAGE               COMMAND             CREATED             STATUS              PORTS               NAMES
3f214c61ad1d        nanoserver          "cmd"               2 minutes ago       Up 2 minutes                            big_minsky
9db7aa4d986d        windowsservercore   "cmd"               2 minutes ago       Up 2 minutes                            mad_wilson
09d3bb5b1604        windowsservercore   "cmd"               2 minutes ago       Up 2 minutes                            affectionate_easley

PS E:\> docker stats 3f214c61ad1d mad_wilson
CONTAINER           CPU %               PRIV WORKING SET    NET I/O             BLOCK I/O
3f214c61ad1d        0.00%               46.25 MiB           76.3 kB / 7.92 kB   10.3 MB / 14.7 MB
mad_wilson          9.59%               40.09 MiB           27.6 kB / 8.81 kB   17 MB / 20.1 MB
```

### Formatting

The formatting option (`--format`) pretty prints container output
using a Go template.

Valid placeholders for the Go template are listed below:

Placeholder  | Description
------------ | --------------------------------------------
`.Container` | Container name or ID (user input)
`.Name`      | Container name
`.ID`        | Container ID
`.CPUPerc`   | CPU percentage
`.MemUsage`  | Memory usage
`.NetIO`     | Network IO
`.BlockIO`   | Block IO
`.MemPerc`   | Memory percentage (Not available on Windows)
`.PIDs`      | Number of PIDs (Not available on Windows)


When using the `--format` option, the `stats` command either
outputs the data exactly as the template declares or, when using the
`table` directive, includes column headers as well.

The following example uses a template without headers and outputs the
`Container` and `CPUPerc` entries separated by a colon for all images:

```bash
$ docker stats --format "{{.Container}}: {{.CPUPerc}}"

09d3bb5b1604: 6.61%
9db7aa4d986d: 9.19%
3f214c61ad1d: 0.00%
```

To list all containers statistics with their name, CPU percentage and memory
usage in a table format you can use:

```bash
$ docker stats --format "table {{.Container}}\t{{.CPUPerc}}\t{{.MemUsage}}"

CONTAINER           CPU %               PRIV WORKING SET
1285939c1fd3        0.07%               796 KiB / 64 MiB
9c76f7834ae2        0.07%               2.746 MiB / 64 MiB
d1ea048f04e4        0.03%               4.583 MiB / 64 MiB
```
