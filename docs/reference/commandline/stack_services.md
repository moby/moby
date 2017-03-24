---
title: "stack services"
description: "The stack services command description and usage"
keywords: "stack, services"
advisory: "experimental"
---

<!-- This file is maintained within the docker/docker Github
     repository at https://github.com/docker/docker/. Make all
     pull requests against that repo. If you see this file in
     another repository, consider it read-only there, as it will
     periodically be overwritten by the definitive file. Pull
     requests which include edits to this file in other repositories
     will be rejected.
-->

# stack services (experimental)

```markdown
Usage:	docker stack services [OPTIONS] STACK

List the services in the stack

Options:
  -f, --filter filter   Filter output based on conditions provided
      --format string   Pretty-print services using a Go template
      --help            Print usage
  -q, --quiet           Only display IDs
```

## Description

Lists the services that are running as part of the specified stack. This
command has to be run targeting a manager node.

## Examples

The following command shows all services in the `myapp` stack:

```bash
$ docker stack services myapp

ID            NAME            REPLICAS  IMAGE                                                                          COMMAND
7be5ei6sqeye  myapp_web       1/1       nginx@sha256:23f809e7fd5952e7d5be065b4d3643fbbceccd349d537b62a123ef2201bc886f
dn7m7nhhfb9y  myapp_db        1/1       mysql@sha256:a9a5b559f8821fe73d58c3606c812d1c044868d42c63817fa5125fd9d8b7b539
```

### Filtering

The filtering flag (`-f` or `--filter`) format is a `key=value` pair. If there
is more than one filter, then pass multiple flags (e.g. `--filter "foo=bar" --filter "bif=baz"`).
Multiple filter flags are combined as an `OR` filter.

The following command shows both the `web` and `db` services:

```bash
$ docker stack services --filter name=myapp_web --filter name=myapp_db myapp

ID            NAME            REPLICAS  IMAGE                                                                          COMMAND
7be5ei6sqeye  myapp_web       1/1       nginx@sha256:23f809e7fd5952e7d5be065b4d3643fbbceccd349d537b62a123ef2201bc886f
dn7m7nhhfb9y  myapp_db        1/1       mysql@sha256:a9a5b559f8821fe73d58c3606c812d1c044868d42c63817fa5125fd9d8b7b539
```

The currently supported filters are:

* id / ID (`--filter id=7be5ei6sqeye`, or `--filter ID=7be5ei6sqeye`)
* name (`--filter name=myapp_web`)
* label (`--filter label=key=value`)

### Formatting

The formatting options (`--format`) pretty-prints services output
using a Go template.

Valid placeholders for the Go template are listed below:

Placeholder | Description
------------|------------------------------------------------------------------------------------------
`.ID`       | Service ID
`.Name`     | Service name
`.Mode`     | Service mode (replicated, global)
`.Replicas` | Service replicas
`.Image`    | Service image

When using the `--format` option, the `stack services` command will either
output the data exactly as the template declares or, when using the
`table` directive, includes column headers as well.

The following example uses a template without headers and outputs the
`ID`, `Mode`, and `Replicas` entries separated by a colon for all services:

```bash
$ docker stack services --format "{{.ID}}: {{.Mode}} {{.Replicas}}"

0zmvwuiu3vue: replicated 10/10
fm6uf97exkul: global 5/5
```


## Related commands

* [stack deploy](stack_deploy.md)
* [stack ls](stack_ls.md)
* [stack ps](stack_ps.md)
* [stack rm](stack_rm.md)
