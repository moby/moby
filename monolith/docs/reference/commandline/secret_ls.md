---
title: "secret ls"
description: "The secret ls command description and usage"
keywords: ["secret, ls"]
---

<!-- This file is maintained within the docker/docker Github
     repository at https://github.com/docker/docker/. Make all
     pull requests against that repo. If you see this file in
     another repository, consider it read-only there, as it will
     periodically be overwritten by the definitive file. Pull
     requests which include edits to this file in other repositories
     will be rejected.
-->

# secret ls

```Markdown
Usage:	docker secret ls [OPTIONS]

List secrets

Aliases:
  ls, list

Options:
  -f, --filter filter   Filter output based on conditions provided
      --format string   Pretty-print secrets using a Go template
      --help            Print usage
  -q, --quiet           Only display IDs
```

## Description

Run this command on a manager node to list the secrets in the swarm.

For detailed information about using secrets, refer to [manage sensitive data with Docker secrets](https://docs.docker.com/engine/swarm/secrets/).

## Examples

```bash
$ docker secret ls

ID                          NAME                        CREATED             UPDATED
6697bflskwj1998km1gnnjr38   q5s5570vtvnimefos1fyeo2u2   6 weeks ago         6 weeks ago
9u9hk4br2ej0wgngkga6rp4hq   my_secret                   5 weeks ago         5 weeks ago
mem02h8n73mybpgqjf0kfi1n0   test_secret                 3 seconds ago       3 seconds ago
```

### Filtering

The filtering flag (`-f` or `--filter`) format is a `key=value` pair. If there is more
than one filter, then pass multiple flags (e.g., `--filter "foo=bar" --filter "bif=baz"`)

The currently supported filters are:

* [id](secret_ls.md#id) (secret's ID)
* [label](secret_ls.md#label) (`label=<key>` or `label=<key>=<value>`)
* [name](secret_ls.md#name) (secret's name)

#### id

The `id` filter matches all or prefix of a secret's id.

```bash
$ docker secret ls -f "id=6697bflskwj1998km1gnnjr38"

ID                          NAME                        CREATED             UPDATED
6697bflskwj1998km1gnnjr38   q5s5570vtvnimefos1fyeo2u2   6 weeks ago         6 weeks ago
```

#### label

The `label` filter matches secrets based on the presence of a `label` alone or
a `label` and a value.

The following filter matches all secrets with a `project` label regardless of
its value:

```bash
$ docker secret ls --filter label=project

ID                          NAME                        CREATED             UPDATED
mem02h8n73mybpgqjf0kfi1n0   test_secret                 About an hour ago   About an hour ago
```

The following filter matches only services with the `project` label with the
`project-a` value.

```bash
$ docker service ls --filter label=project=test

ID                          NAME                        CREATED             UPDATED
mem02h8n73mybpgqjf0kfi1n0   test_secret                 About an hour ago   About an hour ago
```

#### name

The `name` filter matches on all or prefix of a secret's name.

The following filter matches secret with a name containing a prefix of `test`.

```bash
$ docker secret ls --filter name=test_secret

ID                          NAME                        CREATED             UPDATED
mem02h8n73mybpgqjf0kfi1n0   test_secret                 About an hour ago   About an hour ago
```

### Format the output

The formatting option (`--format`) pretty prints secrets output
using a Go template.

Valid placeholders for the Go template are listed below:

| Placeholder  | Description                                                                          |
| ------------ | ------------------------------------------------------------------------------------ |
| `.ID`        | Secret ID                                                                            |
| `.Name`      | Secret name                                                                          |
| `.CreatedAt` | Time when the secret was created                                                     |
| `.UpdatedAt` | Time when the secret was updated                                                     |
| `.Labels`    | All labels assigned to the secret                                                    |
| `.Label`     | Value of a specific label for this secret. For example `{{.Label "secret.ssh.key"}}` |

When using the `--format` option, the `secret ls` command will either
output the data exactly as the template declares or, when using the
`table` directive, will include column headers as well.

The following example uses a template without headers and outputs the
`ID` and `Name` entries separated by a colon for all images:

```bash
$ docker secret ls --format "{{.ID}}: {{.Name}}"

77af4d6b9913: secret-1
b6fa739cedf5: secret-2
78a85c484f71: secret-3
```

To list all secrets with their name and created date in a table format you
can use:

```bash
$ docker secret ls --format "table {{.ID}}\t{{.Name}}\t{{.CreatedAt}}"

ID                  NAME                      CREATED
77af4d6b9913        secret-1                  5 minutes ago
b6fa739cedf5        secret-2                  3 hours ago
78a85c484f71        secret-3                  10 days ago
```

## Related commands

* [secret create](secret_create.md)
* [secret inspect](secret_inspect.md)
* [secret rm](secret_rm.md)
