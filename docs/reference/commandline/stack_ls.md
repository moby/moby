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
      --help   Print usage
```

## Descriptino

Lists the stacks.

## Examples

The following command shows all stacks and some additional information:

```bash
$ docker stack ls

ID                 SERVICES
vossibility-stack  6
myapp              2
```

## Related commands

* [stack deploy](stack_deploy.md)
* [stack ps](stack_ps.md)
* [stack rm](stack_rm.md)
* [stack services](stack_services.md)
