---
title: "plugin ls"
description: "The plugin ls command description and usage"
keywords: "plugin, list"
---

<!-- This file is maintained within the docker/docker Github
     repository at https://github.com/docker/docker/. Make all
     pull requests against that repo. If you see this file in
     another repository, consider it read-only there, as it will
     periodically be overwritten by the definitive file. Pull
     requests which include edits to this file in other repositories
     will be rejected.
-->

# plugin ls

```markdown
Usage:  docker plugin ls [OPTIONS]

List plugins

Aliases:
  ls, list

Options:
      --help	   Print usage
      --no-trunc   Don't truncate output
```

Lists all the plugins that are currently installed. You can install plugins
using the [`docker plugin install`](plugin_install.md) command.

Example output:

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
* [plugin install](plugin_install.md)
* [plugin push](plugin_push.md)
* [plugin rm](plugin_rm.md)
* [plugin set](plugin_set.md)
