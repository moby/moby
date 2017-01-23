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
      --format string   Pretty-print plugins using a Go template
      --help            Print usage
      --no-trunc        Don't truncate output
  -q, --quiet           Only display plugin IDs
```

Lists all the plugins that are currently installed. You can install plugins
using the [`docker plugin install`](plugin_install.md) command.

Example output:

```bash
$ docker plugin ls

ID                  NAME                             TAG                 DESCRIPTION                ENABLED
69553ca1d123        tiborvass/sample-volume-plugin   latest              A test plugin for Docker   true
```

## Formatting

The formatting options (`--format`) pretty-prints plugins output
using a Go template.

Valid placeholders for the Go template are listed below:

Placeholder    | Description
---------------|------------------------------------------------------------------------------------------
`.ID`          | Plugin ID
`.Name`        | Plugin name
`.Description` | Plugin description
`.Enabled`     | Whether plugin is enabled or not

When using the `--format` option, the `plugin ls` command will either
output the data exactly as the template declares or, when using the
`table` directive, includes column headers as well.

The following example uses a template without headers and outputs the
`ID` and `Name` entries separated by a colon for all plugins:

```bash
$ docker plugin ls --format "{{.ID}}: {{.Name}}"
4be01827a72e: tiborvass/no-remove
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
