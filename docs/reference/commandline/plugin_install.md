---
title: "plugin install"
description: "the plugin install command description and usage"
keywords: "plugin, install"
---

<!-- This file is maintained within the docker/docker Github
     repository at https://github.com/docker/docker/. Make all
     pull requests against that repo. If you see this file in
     another repository, consider it read-only there, as it will
     periodically be overwritten by the definitive file. Pull
     requests which include edits to this file in other repositories
     will be rejected.
-->

# plugin install

```markdown
Usage:  docker plugin install [OPTIONS] PLUGIN [KEY=VALUE...]

Install a plugin

Options:
      --alias string            Local name for plugin
      --disable                 Do not enable the plugin on install
      --grant-all-permissions   Grant all permissions necessary to run the plugin
      --help                    Print usage
```

Installs and enables a plugin. Docker looks first for the plugin on your Docker
host. If the plugin does not exist locally, then the plugin is pulled from
the registry. Note that the minimum required registry version to distribute
plugins is 2.3.0


The following example installs `no-remove` plugin and [set](plugin_set.md) it's env variable
`DEBUG` to 1. Install consists of pulling the plugin from Docker Hub, prompting
the user to accept the list of privileges that the plugin needs, settings parameters
 and enabling the plugin.

```bash
$ docker plugin install tiborvass/no-remove DEBUG=1

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

ID                  NAME                  TAG                 DESCRIPTION                ENABLED
69553ca1d123        tiborvass/no-remove   latest              A test plugin for Docker   true
```

## Related information

* [plugin create](plugin_create.md)
* [plugin disable](plugin_disable.md)
* [plugin enable](plugin_enable.md)
* [plugin inspect](plugin_inspect.md)
* [plugin ls](plugin_ls.md)
* [plugin push](plugin_push.md)
* [plugin rm](plugin_rm.md)
* [plugin set](plugin_set.md)
