---
title: "service logs"
description: "The service logs command description and usage"
keywords: "service, task, logs"
---

<!-- This file is maintained within the docker/docker Github
     repository at https://github.com/docker/docker/. Make all
     pull requests against that repo. If you see this file in
     another repository, consider it read-only there, as it will
     periodically be overwritten by the definitive file. Pull
     requests which include edits to this file in other repositories
     will be rejected.
-->

# service logs

```Markdown
Usage:  docker service logs [OPTIONS] SERVICE|TASK

Fetch the logs of a service or task

Options:
  -f, --follow         Follow log output
      --help           Print usage
      --no-resolve     Do not map IDs to Names in output
      --no-task-ids    Do not include task IDs in output
      --no-trunc        Do not truncate output
      --since string   Show logs since timestamp
      --tail string    Number of lines to show from the end of the logs (default "all")
  -t, --timestamps     Show timestamps
```

## Description

The `docker service logs` command batch-retrieves logs present at the time of execution.

The `docker service logs` command can be used with either the name or ID of a
service, or with the ID of a task. If a service is passed, it will display logs
for all of the containers in that service. If a task is passed, it will only
display logs from that particular task.

> **Note**: This command is only functional for services that are started with
> the `json-file` or `journald` logging driver.

For more information about selecting and configuring logging drivers, refer to
[Configure logging drivers](https://docs.docker.com/engine/admin/logging/overview/).

The `docker service logs --follow` command will continue streaming the new output from
the service's `STDOUT` and `STDERR`.

Passing a negative number or a non-integer to `--tail` is invalid and the
value is set to `all` in that case.

The `docker service logs --timestamps` command will add an [RFC3339Nano timestamp](https://golang.org/pkg/time/#pkg-constants)
, for example `2014-09-16T06:17:46.000000000Z`, to each
log entry. To ensure that the timestamps are aligned the
nano-second part of the timestamp will be padded with zero when necessary.

The `docker service logs --details` command will add on extra attributes, such as
environment variables and labels, provided to `--log-opt` when creating the
service.

The `--since` option shows only the service logs generated after
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

## Related commands

* [service create](service_create.md)
* [service inspect](service_inspect.md)
* [service ls](service_ls.md)
* [service rm](service_rm.md)
* [service scale](service_scale.md)
* [service ps](service_ps.md)
* [service update](service_update.md)
