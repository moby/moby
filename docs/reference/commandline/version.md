---
title: "version"
description: "The version command description and usage"
keywords: "version, architecture, api"
---

<!-- This file is maintained within the docker/docker Github
     repository at https://github.com/docker/docker/. Make all
     pull requests against that repo. If you see this file in
     another repository, consider it read-only there, as it will
     periodically be overwritten by the definitive file. Pull
     requests which include edits to this file in other repositories
     will be rejected.
-->

# version

```markdown
Usage:  docker version [OPTIONS]

Show the Docker version information

Options:
  -f, --format string   Format the output using the given Go template
      --help            Print usage
```

## Description

By default, this will render all version information in an easy to read
layout. If a format is specified, the given template will be executed instead.

Go's [text/template](http://golang.org/pkg/text/template/) package
describes all the details of the format.

## Examples

### Default output

```bash
$ docker version

Client:
Version:      1.8.0
API version:  1.20
Go version:   go1.4.2
Git commit:   f5bae0a
Built:        Tue Jun 23 17:56:00 UTC 2015
OS/Arch:      linux/amd64

Server:
Version:      1.8.0
API version:  1.20
Go version:   go1.4.2
Git commit:   f5bae0a
Built:        Tue Jun 23 17:56:00 UTC 2015
OS/Arch:      linux/amd64
```

### Get the server version

```bash
{% raw %}
$ docker version --format '{{.Server.Version}}'

1.8.0
{% endraw %}
```

### Dump raw JSON data

```bash
{% raw %}
$ docker version --format '{{json .}}'

{"Client":{"Version":"1.8.0","ApiVersion":"1.20","GitCommit":"f5bae0a","GoVersion":"go1.4.2","Os":"linux","Arch":"amd64","BuildTime":"Tue Jun 23 17:56:00 UTC 2015"},"ServerOK":true,"Server":{"Version":"1.8.0","ApiVersion":"1.20","GitCommit":"f5bae0a","GoVersion":"go1.4.2","Os":"linux","Arch":"amd64","KernelVersion":"3.13.2-gentoo","BuildTime":"Tue Jun 23 17:56:00 UTC 2015"}}
{% endraw %}
```
