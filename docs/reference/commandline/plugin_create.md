---
title: "plugin create"
description: "the plugin create command description and usage"
keywords: "plugin, create"
---

<!-- This file is maintained within the docker/docker Github
     repository at https://github.com/docker/docker/. Make all
     pull requests against that repo. If you see this file in
     another repository, consider it read-only there, as it will
     periodically be overwritten by the definitive file. Pull
     requests which include edits to this file in other repositories
     will be rejected.
-->

# plugin create

```markdown
Usage:  docker plugin create [OPTIONS] PLUGIN PLUGIN-DATA-DIR

Create a plugin from a rootfs and configuration. Plugin data directory must contain config.json and rootfs directory.

Options:
      --compress   Compress the context using gzip 
      --help       Print usage
```

Creates a plugin. Before creating the plugin, prepare the plugin's root filesystem as well as
[the config.json](../../extend/config.md)


The following example shows how to create a sample `plugin`.

```bash

$ ls -ls /home/pluginDir

4 -rw-r--r--  1 root root 431 Nov  7 01:40 config.json
0 drwxr-xr-x 19 root root 420 Nov  7 01:40 rootfs

$ docker plugin create plugin /home/pluginDir
plugin

$ docker plugin ls
ID                  NAME                TAG                 DESCRIPTION                  ENABLED
672d8144ec02        plugin              latest              A sample plugin for Docker   false
```

The plugin can subsequently be enabled for local use or pushed to the public registry.

## Related information

* [plugin disable](plugin_disable.md)
* [plugin enable](plugin_enable.md)
* [plugin inspect](plugin_inspect.md)
* [plugin install](plugin_install.md)
* [plugin ls](plugin_ls.md)
* [plugin push](plugin_push.md)
* [plugin rm](plugin_rm.md)
* [plugin set](plugin_set.md)
