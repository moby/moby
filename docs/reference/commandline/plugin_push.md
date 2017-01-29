---
title: "plugin push"
description: "the plugin push command description and usage"
keywords: "plugin, push"
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
Usage:	docker plugin push [OPTIONS] PLUGIN[:TAG]

Push a plugin to a registry

Options:
      --disable-content-trust   Skip image signing (default true)
      --help                    Print usage
```

Use `docker plugin create` to create the plugin. Once the plugin is ready for distribution,
use `docker plugin push` to share your images to the Docker Hub registry or to a self-hosted one.

Registry credentials are managed by [docker login](login.md).

The following example shows how to push a sample `user/plugin`.

```bash

$ docker plugin ls
ID                  NAME                  TAG                 DESCRIPTION                ENABLED
69553ca1d456        user/plugin           latest              A sample plugin for Docker false
$ docker plugin push user/plugin
```

## Related information

* [plugin create](plugin_create.md)
* [plugin disable](plugin_disable.md)
* [plugin enable](plugin_enable.md)
* [plugin inspect](plugin_inspect.md)
* [plugin install](plugin_install.md)
* [plugin ls](plugin_ls.md)
* [plugin rm](plugin_rm.md)
* [plugin set](plugin_set.md)
* [plugin upgrade](plugin_upgrade.md)
