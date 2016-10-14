---
title: "plugin install"
description: "the plugin install command description and usage"
keywords: ["plugin, install"]
advisory: "experimental"
---

# plugin install (experimental)

```markdown
Usage:  docker plugin install [OPTIONS] PLUGIN

Install a plugin

Options:
      --disable                 Do not enable the plugin on install
      --grant-all-permissions   Grant all permissions necessary to run the plugin
      --help                    Print usage
```

Installs and enables a plugin. Docker looks first for the plugin on your Docker
host. If the plugin does not exist locally, then the plugin is pulled from
the registry. Note that the minimum required registry version to distribute
plugins is 2.3.0


The following example installs `no-remove` plugin. Install consists of pulling the
plugin from Docker Hub, prompting the user to accept the list of privileges that
the plugin needs and enabling the plugin.

```bash
$ docker plugin install tiborvass/no-remove

Plugin "tiborvass/no-remove" is requesting the following privileges:
 - network: [host]
 - mount: [/data]
 - device: [/dev/cpu_dma_latency]
Do you grant the above permissions? [y/N] y
tiborvass/no-remove
```

After the plugin is installed, it appears in the list of plugins:

```bash
$ docker plugin ls

NAME                  TAG                 DESCRIPTION                ENABLED
tiborvass/no-remove   latest              A test plugin for Docker   true
```

## Related information

* [plugin ls](plugin_ls.md)
* [plugin enable](plugin_enable.md)
* [plugin disable](plugin_disable.md)
* [plugin inspect](plugin_inspect.md)
* [plugin rm](plugin_rm.md)
