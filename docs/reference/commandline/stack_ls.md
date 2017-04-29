---
title: "stack ls"
description: "The stack ls command description and usage"
keywords: "stack, ls"
---

<!-- This file is maintained within the docker/docker Github
     repository at https://github.com/docker/docker/. Make all
     pull requests against that repo. If you see this file in
     another repository, consider it read-only there, as it will
     periodically be overwritten by the definitive file. Pull
     requests which include edits to this file in other repositories
     will be rejected.
-->

# stack ls

```markdown
Usage:	docker stack ls

List stacks

Aliases:
  ls, list

Options:
      --help            Print usage
      --format string   Pretty-print stacks using a Go template
```

## Description

Lists the stacks.

## Examples

The following command shows all stacks and some additional information:

```bash
$ docker stack ls

ID                 SERVICES
vossibility-stack  6
myapp              2
```

### Formatting

The formatting option (`--format`) pretty-prints stacks using a Go template.

Valid placeholders for the Go template are listed below:

| Placeholder | Description        |
| ----------- | ------------------ |
| `.Name`     | Stack name         |
| `.Services` | Number of services |

When using the `--format` option, the `stack ls` command either outputs
the data exactly as the template declares or, when using the
`table` directive, includes column headers as well.

The following example uses a template without headers and outputs the
`Name` and `Services` entries separated by a colon for all stacks:

```bash
$ docker stack ls --format "{{.Name}}: {{.Services}}"
web-server: 1
web-cache: 4
```

## Related commands

* [stack deploy](stack_deploy.md)
* [stack ps](stack_ps.md)
* [stack rm](stack_rm.md)
* [stack services](stack_services.md)
