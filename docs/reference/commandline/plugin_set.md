---
title: "plugin set"
description: "the plugin set command description and usage"
keywords: "plugin, set"
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

# plugin set (experimental)

```markdown
Usage:  docker plugin set PLUGIN KEY=VALUE [KEY=VALUE...]

Change settings for a plugin

Options:
      --help                    Print usage
```

Change settings for a plugin. The plugin must be disabled.


The following example installs change the env variable `DEBUG` of the
`no-remove` plugin.

```bash
$ docker plugin inspect -f {{.Config.Env}} tiborvass/no-remove
[DEBUG=0]

$ docker plugin set DEBUG=1 tiborvass/no-remove

$ docker plugin inspect -f {{.Config.Env}} tiborvass/no-remove
[DEBUG=1]
```

## Related information

* [plugin create](plugin_create.md)
* [plugin ls](plugin_ls.md)
* [plugin enable](plugin_enable.md)
* [plugin disable](plugin_disable.md)
* [plugin inspect](plugin_inspect.md)
* [plugin install](plugin_install.md)
* [plugin rm](plugin_rm.md)
