---
title: "plugin enable"
description: "the plugin enable command description and usage"
keywords: "plugin, enable"
---

<!-- This file is maintained within the docker/docker Github
     repository at https://github.com/docker/docker/. Make all
     pull requests against that repo. If you see this file in
     another repository, consider it read-only there, as it will
     periodically be overwritten by the definitive file. Pull
     requests which include edits to this file in other repositories
     will be rejected.
-->

# plugin enable

```markdown
Usage:  docker plugin enable [OPTIONS] PLUGIN

Enable a plugin

Options:
      --help          Print usage
      --timeout int   HTTP client timeout (in seconds)
```

Enables a plugin. The plugin must be installed before it can be enabled,
see [`docker plugin install`](plugin_install.md).


The following example shows that the `sample-volume-plugin` plugin is installed,
but disabled:

```bash
$ docker plugin ls

ID                  NAME                             TAG                 DESCRIPTION                ENABLED
69553ca1d123        tiborvass/sample-volume-plugin   latest              A test plugin for Docker   false
```

To enable the plugin, use the following command:

```bash
$ docker plugin enable tiborvass/sample-volume-plugin

tiborvass/sample-volume-plugin

$ docker plugin ls

ID                  NAME                             TAG                 DESCRIPTION                ENABLED
69553ca1d123        tiborvass/sample-volume-plugin   latest              A test plugin for Docker   true
```

## Related information

* [plugin create](plugin_create.md)
* [plugin disable](plugin_disable.md)
* [plugin inspect](plugin_inspect.md)
* [plugin install](plugin_install.md)
* [plugin ls](plugin_ls.md)
* [plugin push](plugin_push.md)
* [plugin rm](plugin_rm.md)
* [plugin set](plugin_set.md)
