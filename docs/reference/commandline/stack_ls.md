---
title: "stack ls"
description: "The stack ls command description and usage"
keywords: ["stack, ls"]
advisory: "experimental"
---

# stack ls (experimental)

```markdown
Usage:	docker stack ls

List stacks
```

Lists the stacks.

For example, the following command shows all stacks and some additional information:

```bash
$ docker stack ls

ID                 SERVICES
vossibility-stack  6
myapp              2
```

## Related information

* [stack config](stack_config.md)
* [stack deploy](stack_deploy.md)
* [stack rm](stack_rm.md)
* [stack ps](stack_ps.md)
