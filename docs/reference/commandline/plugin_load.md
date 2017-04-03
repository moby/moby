---
title: "plugin load"
description: "the plugin load command description and usage"
keywords: "plugin, load"
---

<!-- This file is maintained within the docker/docker Github
     repository at https://github.com/docker/docker/. Make all
     pull requests against that repo. If you see this file in
     another repository, consider it read-only there, as it will
     periodically be overwritten by the definitive file. Pull
     requests which include edits to this file in other repositories
     will be rejected.
-->

# plugin load

```markdown
Usage:  docker plugin load [OPTIONS]

Load a plugin from a tar archive or STDIN

Options:
      --help           Print usage
  -i, --input string   Read from tar archive file, instead of STDIN
  -q, --quiet          Suppress the load output

```

## Description

Loads a plugin from the tarstream generated from `docker plugin save`.

## Examples

The following example loads a saved tarstream of the plugin `tiborvass/sample-volume-plugin`

```bash
$ docker plugin load < /tmp/volplugin.tar
Loaded plugin ID: 792e00e0251b114f44dfd4e855cba43d92de94b1517e29b430a68e00614e9f38
```

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
* [plugin save](plugin_save.md)
