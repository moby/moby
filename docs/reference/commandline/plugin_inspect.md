<!--[metadata]>
+++
title = "plugin inspect"
description = "The plugin inspect command description and usage"
keywords = ["plugin, inspect"]
advisory = "experimental"
[menu.main]
parent = "smn_cli"
+++
<![end-metadata]-->

# plugin inspect (experimental)

```markdown
Usage:  docker plugin inspect PLUGIN

Inspect a plugin

Options:
      --help   Print usage
```

Returns information about a plugin. By default, this command renders all results
in a JSON array.

Example output:

```bash
$ docker plugin inspect tiborvass/no-remove:latest
```
```JSON
{
    "Manifest": {
        "ManifestVersion": "",
        "Description": "A test plugin for Docker",
        "Documentation": "https://docs.docker.com/engine/extend/plugins/",
        "Entrypoint": [
            "plugin-no-remove",
            "/data"
        ],
        "Interface": {
            "Types": [
                "docker.volumedriver/1.0"
            ],
            "Socket": "plugins.sock"
        },
        "Network": {
            "Type": "host"
        },
        "Capabilities": null,
        "Mounts": [
            {
                "Name": "",
                "Description": "",
                "Settable": false,
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
                "Settable": false,
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
                "Settable": false,
                "Path": null
            }
        ],
        "Env": [
            {
                "Name": "DEBUG",
                "Description": "If set, prints debug messages",
                "Settable": false,
                "Value": null
            }
        ],
        "Args": [
            {
                "Name": "arg1",
                "Description": "a command line argument",
                "Settable": false,
                "Value": null
            }
        ]
    },
    "Config": {
        "Mounts": [
            {
                "Source": "/data",
                "Destination": "/data",
                "Type": "bind",
                "Options": [
                    "shared",
                    "rbind"
                ]
            },
            {
                "Source": null,
                "Destination": "/foobar",
                "Type": "tmpfs",
                "Options": null
            }
        ],
        "Env": [],
        "Args": [],
        "Devices": null
    },
    "Active": true,
    "Name": "tiborvass/no-remove",
    "Tag": "latest",
    "ID": "ac9d36b664921d61813254f7e9946f10e3cadbb676346539f1705fcaf039c01f"
}
```
(output formatted for readability)



## Related information

* [plugin ls](plugin_ls.md)
* [plugin enable](plugin_enable.md)
* [plugin disable](plugin_disable.md)
* [plugin install](plugin_install.md)
* [plugin rm](plugin_rm.md)
