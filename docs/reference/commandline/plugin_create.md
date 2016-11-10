---
title: "plugin create (experimental)"
description: "the plugin create command description and usage"
keywords: "plugin, create"
advisory: "experimental"
---

<!-- This file is maintained within the docker/docker Github
     repository at https://github.com/docker/docker/. Make all
     pull requests against that repo. If you see this file in
     another repository, consider it read-only there, as it will
     periodically be overwritten by the definitive file. Pull
     requests which include edits to this file in other repositories
     will be rejected.
-->

```markdown
Usage:  docker plugin create [OPTIONS] reponame[:tag] PATH-TO-ROOTFS

create a plugin from the given PATH-TO-ROOTFS, which contains the plugin's root filesystem and the manifest file, manifest.json

Options:
      --compress   Compress the context using gzip 
      --help       Print usage
```

Creates a plugin. Before creating the plugin, prepare the plugin's root filesystem as well as
the manifest.json (https://github.com/docker/docker/blob/master/docs/extend/manifest.md)


The following example shows how to create a sample `plugin`.

```bash

$ ls -ls /home/pluginDir

4 -rw-r--r--  1 root root 431 Nov  7 01:40 manifest.json
0 drwxr-xr-x 19 root root 420 Nov  7 01:40 rootfs

$ docker plugin create plugin /home/pluginDir
plugin

NAME                  	TAG                 DESCRIPTION                  ENABLED
plugin                  latest              A sample plugin for Docker   true
```

The plugin can subsequently be enabled for local use or pushed to the public registry.

## Related information

* [plugin ls](plugin_ls.md)
* [plugin enable](plugin_enable.md)
* [plugin disable](plugin_disable.md)
* [plugin inspect](plugin_inspect.md)
* [plugin install](plugin_install.md)
* [plugin rm](plugin_rm.md)
* [plugin set](plugin_set.md)
