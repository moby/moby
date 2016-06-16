<!--[metadata]>
+++
title = "plugin ls"
description = "The plugin ls command description and usage"
keywords = ["plugin, list"]
[menu.main]
parent = "smn_cli"
advisory = "experimental"
+++
<![end-metadata]-->

# plugin ls (experimental)

    Usage: docker plugin ls

    List plugins

      --help   Print usage

    Aliases:
      ls, list

Lists all the plugins that are currently installed. You can install plugins
using the [`docker plugin install`](plugin_install.md) command.

Example output:

```bash
$ docker plugin ls
NAME                	VERSION             ACTIVE
tiborvass/no-remove	latest              true
```

## Related information

* [plugin enable](plugin_enable.md)
* [plugin disable](plugin_disable.md)
* [plugin inspect](plugin_inspect.md)
* [plugin install](plugin_install.md)
* [plugin rm](plugin_rm.md)
