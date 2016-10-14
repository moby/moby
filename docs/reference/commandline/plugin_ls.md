---
title: "plugin ls"
description: "The plugin ls command description and usage"
keywords: ["plugin, list"]
advisory: "experimental"
---

# plugin ls (experimental)

```markdown
Usage:  docker plugin ls [OPTIONS]

List plugins

Aliases:
  ls, list

Options:
      --help	   Print usage
      --no-trunc   Don't truncate output
```

Lists all the plugins that are currently installed. You can install plugins
using the [`docker plugin install`](plugin_install.md) command.

Example output:

```bash
$ docker plugin ls

NAME                  TAG                 DESCRIPTION                ENABLED
tiborvass/no-remove   latest              A test plugin for Docker   true
```

## Related information

* [plugin enable](plugin_enable.md)
* [plugin disable](plugin_disable.md)
* [plugin inspect](plugin_inspect.md)
* [plugin install](plugin_install.md)
* [plugin rm](plugin_rm.md)
