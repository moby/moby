---
redirect_from:
  - /reference/commandline/plugin_ls/
advisory: experimental
description: The plugin ls command description and usage
keywords:
- plugin, list
title: docker plugin ls (experimental)
---

```markdown
Usage:  docker plugin ls

List plugins

Aliases:
  ls, list

Options:
      --help   Print usage
```

Lists all the plugins that are currently installed. You can install plugins
using the [`docker plugin install`](plugin_install.md) command.

Example output:

```bash
$ docker plugin ls

NAME                  VERSION             ACTIVE
tiborvass/no-remove   latest              true
```

## Related information

* [plugin enable](plugin_enable.md)
* [plugin disable](plugin_disable.md)
* [plugin inspect](plugin_inspect.md)
* [plugin install](plugin_install.md)
* [plugin rm](plugin_rm.md)
