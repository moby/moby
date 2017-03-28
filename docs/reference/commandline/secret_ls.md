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
  -q, --quiet          Only display IDs
  -format string       Pretty-print secrets using a Go template
```

## Description

Run this command on a manager node to list the secrets in the swarm.

For detailed information about using secrets, refer to [manage sensitive data with Docker secrets](https://docs.docker.com/engine/swarm/secrets/).

## Examples

```bash
$ docker secret ls

ID                          NAME                CREATED             UPDATED
eo7jnzguqgtpdah3cm5srfb97   my_secret           11 minutes ago      11 minutes ago
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
