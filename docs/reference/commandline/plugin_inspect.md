---
title: "plugin inspect"
description: "The plugin inspect command description and usage"
keywords: "plugin, inspect"
---

<!-- This file is maintained within the docker/docker Github
     repository at https://github.com/docker/docker/. Make all
     pull requests against that repo. If you see this file in
     another repository, consider it read-only there, as it will
     periodically be overwritten by the definitive file. Pull
     requests which include edits to this file in other repositories
     will be rejected.
-->

# plugin inspect

```markdown
Usage:	docker plugin inspect [OPTIONS] PLUGIN|ID [PLUGIN|ID...]

Display detailed information on one or more plugins

Options:
  -f, --format string   Format the output using the given Go template
      --help            Print usage
```

Returns information about a plugin. By default, this command renders all results
in a JSON array.

Example output:

```bash
$ docker plugin inspect tiborvass/no-remove:latest
```
```JSON
{
  "Id": "8c74c978c434745c3ade82f1bc0acf38d04990eaf494fa507c16d9f1daa99c21",
  "Name": "tiborvass/no-remove:latest",
  "Enabled": true,
  "Config": {
    "Mounts": [
      {
        "Name": "",
        "Description": "",
        "Settable": null,
        "Source": "/data",
        "Destination": "/data",
        "Type": "bind",
        "Options": [
          "shared",
          "rbind"
        ]
      },
      {
        "Name": "",
        "Description": "",
        "Settable": null,
        "Source": null,
        "Destination": "/foobar",
        "Type": "tmpfs",
        "Options": null
      }
    ],
    "Env": [
      "DEBUG=1"
    ],
    "Args": null,
    "Devices": null
  },
  "Manifest": {
    "ManifestVersion": "v0",
    "Description": "A test plugin for Docker",
    "Documentation": "https://docs.docker.com/engine/extend/plugins/",
    "Interface": {
      "Types": [
        "docker.volumedriver/1.0"
      ],
      "Socket": "plugins.sock"
    },
    "Entrypoint": [
      "plugin-no-remove",
      "/data"
    ],
    "Workdir": "",
    "User": {
    },
    "Network": {
      "Type": "host"
    },
    "Capabilities": null,
    "Mounts": [
      {
        "Name": "",
        "Description": "",
        "Settable": null,
        "Source": "/data",
        "Destination": "/data",
        "Type": "bind",
        "Options": [
          "shared",
          "rbind"
        ]
      },
      {
        "Name": "",
        "Description": "",
        "Settable": null,
        "Source": null,
        "Destination": "/foobar",
        "Type": "tmpfs",
        "Options": null
      }
    ],
    "Devices": [
      {
        "Name": "device",
        "Description": "a host device to mount",
        "Settable": null,
        "Path": "/dev/cpu_dma_latency"
      }
    ],
    "Env": [
      {
        "Name": "DEBUG",
        "Description": "If set, prints debug messages",
        "Settable": null,
        "Value": "1"
      }
    ],
    "Args": {
      "Name": "args",
      "Description": "command line arguments",
      "Settable": null,
      "Value": [

      ]
    }
  }
}
```
(output formatted for readability)


```bash
$ docker plugin inspect -f '{{.Id}}' tiborvass/no-remove:latest
```
```
8c74c978c434745c3ade82f1bc0acf38d04990eaf494fa507c16d9f1daa99c21
```


## Related information

* [plugin create](plugin_create.md)
* [plugin enable](plugin_enable.md)
* [plugin disable](plugin_disable.md)
* [plugin install](plugin_install.md)
* [plugin ls](plugin_ls.md)
* [plugin push](plugin_push.md)
* [plugin rm](plugin_rm.md)
* [plugin set](plugin_set.md)
