---
title: "plugin save"
description: "the plugin save command description and usage"
keywords: "plugin, save"
---

<!-- This file is maintained within the docker/docker Github
     repository at https://github.com/docker/docker/. Make all
     pull requests against that repo. If you see this file in
     another repository, consider it read-only there, as it will
     periodically be overwritten by the definitive file. Pull
     requests which include edits to this file in other repositories
     will be rejected.
-->

# plugin save

```markdown
Usage:  docker plugin save [OPTIONS] PLUGIN

Save a plugin to a tar archive (streamed to STDOUT by default)

Options:
      --help            Print usage
  -o, --output string   Write to a file, instead of STDOUT

```

## Description

Saves a plugin to a tar stream.

## Examples

The following example saves the installed`tiborvass/sample-volume-plugin` to a tar stream.

```bash

$ docker plugin save tiborvass/sample-volume-plugin > /tmp/volplugin.tar

```

After the plugin is saved, it can be distributed and loaded using `docker plugin load` 

## Related commands

* [plugin install](plugin_install.md)
* [plugin create](plugin_create.md)
* [plugin disable](plugin_disable.md)
* [plugin enable](plugin_enable.md)
* [plugin inspect](plugin_inspect.md)
* [plugin ls](plugin_ls.md)
* [plugin push](plugin_push.md)
* [plugin rm](plugin_rm.md)
* [plugin set](plugin_set.md)
* [plugin upgrade](plugin_upgrade.md)
* [plugin load](plugin_load.md)
