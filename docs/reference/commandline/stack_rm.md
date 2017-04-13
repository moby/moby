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
Usage:  docker stack rm STACK [STACK...]

Remove one or more stacks

Aliases:
  rm, remove, down

Options:
      --help   Print usage
```

## Description

Remove the stack from the swarm. This command has to be run targeting
a manager node.

## Examples

### Remove a stack

This will remove the stack with the name `myapp`. Services, networks, and secrets associated with the stack will be removed.

```bash
$ docker stack rm myapp

Removing service myapp_redis
Removing service myapp_web
Removing service myapp_lb
Removing network myapp_default
Removing network myapp_frontend
```

### Remove multiple stacks

This will remove all the specified stacks, `myapp` and `vossibility`. Services, networks, and secrets associated with all the specified stacks will be removed.

```bash
$ docker stack rm myapp vossibility

Removing service myapp_redis
Removing service myapp_web
Removing service myapp_lb
Removing network myapp_default
Removing network myapp_frontend
Removing service vossibility_nsqd
Removing service vossibility_logstash
Removing service vossibility_elasticsearch
Removing service vossibility_kibana
Removing service vossibility_ghollector
Removing service vossibility_lookupd
Removing network vossibility_default
Removing network vossibility_vossibility
```

## Related commands

* [stack deploy](stack_deploy.md)
* [stack ls](stack_ls.md)
* [stack ps](stack_ps.md)
* [stack services](stack_services.md)
