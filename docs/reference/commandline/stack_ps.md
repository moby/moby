---
title: "stack ps"
description: "The stack ps command description and usage"
keywords: "stack, ps"
---

<!-- This file is maintained within the docker/docker Github
     repository at https://github.com/docker/docker/. Make all
     pull requests against that repo. If you see this file in
     another repository, consider it read-only there, as it will
     periodically be overwritten by the definitive file. Pull
     requests which include edits to this file in other repositories
     will be rejected.
-->

# stack ps

```markdown
Usage:  docker stack ps [OPTIONS] STACK

List the tasks in the stack

Options:
  -f, --filter filter   Filter output based on conditions provided
      --help            Print usage
      --no-resolve      Do not map IDs to Names
      --no-trunc        Do not truncate output
```

Lists the tasks that are running as part of the specified stack. This
command has to be run targeting a manager node.

## Filtering

The filtering flag (`-f` or `--filter`) format is a `key=value` pair. If there
is more than one filter, then pass multiple flags (e.g. `--filter "foo=bar" --filter "bif=baz"`).
Multiple filter flags are combined as an `OR` filter. For example,
`-f name=redis.1 -f name=redis.7` returns both `redis.1` and `redis.7` tasks.

The currently supported filters are:

* id
* name
* desired-state

## Related information

* [stack deploy](stack_deploy.md)
* [stack rm](stack_ls.md)
* [stack rm](stack_rm.md)
* [stack services](stack_services.md)
