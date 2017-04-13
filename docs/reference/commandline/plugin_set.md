---
title: "plugin set"
description: "the plugin set command description and usage"
keywords: "plugin, set"
---

<!-- This file is maintained within the docker/docker Github
     repository at https://github.com/docker/docker/. Make all
     pull requests against that repo. If you see this file in
     another repository, consider it read-only there, as it will
     periodically be overwritten by the definitive file. Pull
     requests which include edits to this file in other repositories
     will be rejected.
-->

# plugin set

```markdown
Usage:  docker plugin set PLUGIN KEY=VALUE [KEY=VALUE...]

Change settings for a plugin

Options:
      --help                    Print usage
```

## Description

Change settings for a plugin. The plugin must be disabled.

The settings currently supported are:
 * env variables
 * source of mounts
 * path of devices
 * args

## What is settable ?

Look at the plugin manifest, it's easy to see what fields are settable,
by looking at the `Settable` field.

Here is an extract of a plugin manifest:

```
{
        "config": {
            ...
            "args": {
                "name": "myargs",
                "settable": ["value"],
                "value": ["foo", "bar"]
            },
            "env": [
                {
                    "name": "DEBUG",
                    "settable": ["value"],
                    "value": "0"
                },
                {
                    "name": "LOGGING",
                    "value": "1"
                }
            ],
	    "devices": [
                {
                    "name": "mydevice",
                    "path": "/dev/foo",
                    "settable": ["path"]
                }
            ],
	    "mounts": [
                {
                    "destination": "/baz",
                    "name": "mymount",
                    "options": ["rbind"],
                    "settable": ["source"],
                    "source": "/foo",
                    "type": "bind"
                }
            ],
	    ...
	 }
}
```

In this example, we can see that the `value` of the `DEBUG` environment variable is settable,
the `source` of the `mymount` mount is also settable. Same for the `path` of `mydevice` and `value` of `myargs`.

On the contrary, the `LOGGING` environment variable doesn't have any settable field, which implies that user cannot tweak it.

## Examples

### Change an environment variable

The following example change the env variable `DEBUG` on the
`sample-volume-plugin` plugin.

```bash
$ docker plugin inspect -f {{.Settings.Env}} tiborvass/sample-volume-plugin
[DEBUG=0]

$ docker plugin set tiborvass/sample-volume-plugin DEBUG=1

$ docker plugin inspect -f {{.Settings.Env}} tiborvass/sample-volume-plugin
[DEBUG=1]
```

### Change the source of a mount

The following example change the source of the `mymount` mount on
the `myplugin` plugin.

```bash
$ docker plugin inspect -f '{{with $mount := index .Settings.Mounts 0}}{{$mount.Source}}{{end}}' myplugin
/foo

$ docker plugins set myplugin mymount.source=/bar

$ docker plugin inspect -f '{{with $mount := index .Settings.Mounts 0}}{{$mount.Source}}{{end}}' myplugin
/bar
```

> **Note**: Since only `source` is settable in `mymount`,
> `docker plugins set mymount=/bar myplugin` would work too.

### Change a device path

The following example change the path of the `mydevice` device on
the `myplugin` plugin.

```bash
$ docker plugin inspect -f '{{with $device := index .Settings.Devices 0}}{{$device.Path}}{{end}}' myplugin
/dev/foo

$ docker plugins set myplugin mydevice.path=/dev/bar

$ docker plugin inspect -f '{{with $device := index .Settings.Devices 0}}{{$device.Path}}{{end}}' myplugin
/dev/bar
```

> **Note**: Since only `path` is settable in `mydevice`,
> `docker plugins set mydevice=/dev/bar myplugin` would work too.

### Change the source of the arguments

The following example change the value of the args on the `myplugin` plugin.

```bash
$ docker plugin inspect -f '{{.Settings.Args}}' myplugin
["foo", "bar"]

$ docker plugins set myplugin myargs="foo bar baz"

$ docker plugin inspect -f '{{.Settings.Args}}' myplugin
["foo", "bar", "baz"]
```

## Related commands

* [plugin create](plugin_create.md)
* [plugin disable](plugin_disable.md)
* [plugin enable](plugin_enable.md)
* [plugin inspect](plugin_inspect.md)
* [plugin install](plugin_install.md)
* [plugin ls](plugin_ls.md)
* [plugin push](plugin_push.md)
* [plugin rm](plugin_rm.md)
