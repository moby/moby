---
title: "plugin"
description: "The plugin command description and usage"
keywords: "plugin"
---

<!-- This file is maintained within the docker/docker Github
     repository at https://github.com/docker/docker/. Make all
     pull requests against that repo. If you see this file in
     another repository, consider it read-only there, as it will
     periodically be overwritten by the definitive file. Pull
     requests which include edits to this file in other repositories
     will be rejected.
-->

# plugin

```markdown
Usage:  docker plugin COMMAND

Manage plugins

Options:
      --help   Print usage

Commands:
  create      Create a plugin from a rootfs and configuration. Plugin data directory must contain config.json and rootfs directory.
  disable     Disable a plugin
  enable      Enable a plugin
  inspect     Display detailed information on one or more plugins
  install     Install a plugin
  ls          List plugins
  push        Push a plugin to a registry
  rm          Remove one or more plugins
  set         Change settings for a plugin
  upgrade     Upgrade an existing plugin

Run 'docker plugin COMMAND --help' for more information on a command.

```

## Description

Manage plugins.
