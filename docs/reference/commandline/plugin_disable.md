---
title: "plugin disable"
description: "the plugin disable command description and usage"
keywords: ["plugin, disable"]
advisory: "experimental"
---

# plugin disable (experimental)

```markdown
Usage:  docker plugin disable PLUGIN

Disable a plugin

Options:
      --help   Print usage
```

Disables a plugin. The plugin must be installed before it can be disabled,
see [`docker plugin install`](plugin_install.md).


The following example shows that the `no-remove` plugin is installed
and enabled:

```bash
$ docker plugin ls

NAME                  TAG                 DESCRIPTION                ENABLED
tiborvass/no-remove   latest              A test plugin for Docker   true
```

To disable the plugin, use the following command:

```bash
$ docker plugin disable tiborvass/no-remove

tiborvass/no-remove

$ docker plugin ls

NAME                  TAG                 DESCRIPTION                ENABLED
tiborvass/no-remove   latest              A test plugin for Docker   false
```

## Related information

* [plugin ls](plugin_ls.md)
* [plugin enable](plugin_enable.md)
* [plugin inspect](plugin_inspect.md)
* [plugin install](plugin_install.md)
* [plugin rm](plugin_rm.md)
