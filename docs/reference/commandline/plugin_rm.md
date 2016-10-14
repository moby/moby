---
title: "plugin rm"
description: "the plugin rm command description and usage"
keywords: ["plugin, rm"]
advisory: "experimental"
---

# plugin rm (experimental)

```markdown
Usage:  docker plugin rm [OPTIONS] PLUGIN [PLUGIN...]

Remove one or more plugins

Aliases:
  rm, remove

Options:
      -f, --force  Force the removal of an active plugin
          --help   Print usage
```

Removes a plugin. You cannot remove a plugin if it is enabled, you must disable
a plugin using the [`docker plugin disable`](plugin_disable.md) before removing
it (or use --force, use of force is not recommended, since it can affect
functioning of running containers using the plugin).

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
