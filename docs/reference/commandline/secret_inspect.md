---
title: "secret inspect"
description: "The secret inspect command description and usage"
keywords: ["secret, inspect"]
---

<!-- This file is maintained within the docker/docker Github
     repository at https://github.com/docker/docker/. Make all
     pull requests against that repo. If you see this file in
     another repository, consider it read-only there, as it will
     periodically be overwritten by the definitive file. Pull
     requests which include edits to this file in other repositories
     will be rejected.
-->

# secret inspect

```Markdown
Usage:  docker secret inspect [OPTIONS] SECRET [SECRET...]

Display detailed information on one or more secrets

Options:
  -f, --format string   Format the output using the given Go template
      --help            Print usage
```


Inspects the specified secret. This command has to be run targeting a manager
node.

By default, this renders all results in a JSON array. If a format is specified,
the given template will be executed for each result.

Go's [text/template](http://golang.org/pkg/text/template/) package
describes all the details of the format.

## Examples

### Inspecting a secret by name or ID

You can inspect a secret, either by its *name*, or *ID*

For example, given the following secret:

```bash
$ docker secret ls
ID                          NAME                    CREATED                                   UPDATED
mhv17xfe3gh6xc4rij5orpfds   secret.json             2016-10-27 23:25:43.909181089 +0000 UTC   2016-10-27 23:25:43.909181089 +0000 UTC
```

```bash
$ docker secret inspect secret.json
[
    {
        "ID": "mhv17xfe3gh6xc4rij5orpfds",
            "Version": {
            "Index": 1198
        },
        "CreatedAt": "2016-10-27T23:25:43.909181089Z",
        "UpdatedAt": "2016-10-27T23:25:43.909181089Z",
        "Spec": {
            "Name": "secret.json"
        }
    }
]
```

### Formatting secret output

You can use the --format option to obtain specific information about a
secret. The following example command outputs the creation time of the
secret.

```bash{% raw %}
$ docker secret inspect --format='{{.CreatedAt}}' mhv17xfe3gh6xc4rij5orpfds
2016-10-27 23:25:43.909181089 +0000 UTC
{% endraw %}```


## Related information

* [secret create](secret_create.md)
* [secret ls](secret_ls.md)
* [secret rm](secret_rm.md)
