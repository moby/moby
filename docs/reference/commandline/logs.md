<!--[metadata]>
+++
title = "logs"
description = "The logs command description and usage"
keywords = ["logs, retrieve, docker"]
[menu.main]
parent = "smn_cli"
+++
<![end-metadata]-->

# logs

    Usage: docker logs [OPTIONS] CONTAINER

    Fetch the logs of a container

      -f, --follow=false        Follow log output
      --help=false              Print usage
      --since=""                Show logs since timestamp
      -t, --timestamps=false    Show timestamps
      --tail="all"              Number of lines to show from the end of the logs

> **Note**: this command is available only for containers with `json-file` and
> `journald` logging drivers.

The `docker logs` command batch-retrieves logs present at the time of execution.

The `docker logs --follow` command will continue streaming the new output from
the container's `STDOUT` and `STDERR`.

Passing a negative number or a non-integer to `--tail` is invalid and the
value is set to `all` in that case.

The `docker logs --timestamps` command will add an [RFC3339Nano timestamp](https://golang.org/pkg/time/#pkg-constants)
, for example `2014-09-16T06:17:46.000000000Z`, to each
log entry. To ensure that the timestamps are aligned the
nano-second part of the timestamp will be padded with zero when necessary.

The `--since` option shows only the container logs generated after
a given date. You can specify the date as an RFC 3339 date, a UNIX
timestamp, or a Go duration string (e.g. `1m30s`, `3h`). Besides RFC3339 date
format you may also use RFC3339Nano, `2006-01-02T15:04:05`,
`2006-01-02T15:04:05.999999999`, `2006-01-02Z07:00`, and `2006-01-02`. The local
timezone on the client will be used if you do not provide either a `Z` or a
`+-00:00` timezone offset at the end of the timestamp. When providing Unix
timestamps enter seconds[.nanoseconds], where seconds is the number of seconds
that have elapsed since January 1, 1970 (midnight UTC/GMT), not counting leap
seconds (aka Unix epoch or Unix time), and the optional .nanoseconds field is a
fraction of a second no more than nine digits long. You can combine the
`--since` option with either or both of the `--follow` or `--tail` options.
