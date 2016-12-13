---
title: "stack rm"
description: "The stack rm command description and usage"
keywords: "stack, rm, remove, down"
---

<!-- This file is maintained within the docker/docker Github
     repository at https://github.com/docker/docker/. Make all
     pull requests against that repo. If you see this file in
     another repository, consider it read-only there, as it will
     periodically be overwritten by the definitive file. Pull
     requests which include edits to this file in other repositories
     will be rejected.
-->

# stack rm

```markdown
Usage:  docker stack rm STACK

Remove the stack

Aliases:
  rm, remove, down

Options:
      --help   Print usage
```

Remove the stack from the swarm. This command has to be run targeting
a manager node.

## Related information

* [stack deploy](stack_deploy.md)
* [stack ls](stack_ls.md)
* [stack ps](stack_ps.md)
* [stack services](stack_services.md)
