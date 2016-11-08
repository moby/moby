---
redirect_from:
  - /reference/commandline/plugin_rm/
advisory: experimental
description: the plugin rm command description and usage
keywords:
- plugin, rm
title: docker plugin rm (experimental)
---

```markdown
Usage:  docker plugin rm PLUGIN

Remove one or more plugins

Aliases:
  rm, remove

Options:
      --help   Print usage
```

Removes a plugin. You cannot remove a plugin if it is active, you must disable
a plugin using the [`docker plugin disable`](plugin_disable.md) before removing
it.

The following example disables and removes the `no-remove:latest` plugin;

```bash
$ docker plugin disable tiborvass/no-remove
tiborvass/no-remove

$ docker plugin rm tiborvass/no-remove:latest
tiborvass/no-remove
```

## Related information

* [plugin ls](plugin_ls.md)
* [plugin enable](plugin_enable.md)
* [plugin disable](plugin_disable.md)
* [plugin inspect](plugin_inspect.md)
* [plugin install](plugin_install.md)
