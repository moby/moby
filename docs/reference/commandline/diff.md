---
title: "diff"
description: "The diff command description and usage"
keywords: "list, changed, files, container"
---

<!-- This file is maintained within the docker/docker Github
     repository at https://github.com/docker/docker/. Make all
     pull requests against that repo. If you see this file in
     another repository, consider it read-only there, as it will
     periodically be overwritten by the definitive file. Pull
     requests which include edits to this file in other repositories
     will be rejected.
-->

# diff

```markdown
Usage:  docker diff CONTAINER

Inspect changes on a container's filesystem

Options:
      --help   Print usage
```

List the changed files and directories in a container᾿s filesystem.
 There are 3 events that are listed in the `diff`:

1. `A` - Add
2. `D` - Delete
3. `C` - Change

For example:

    $ docker diff 7bb0e258aefe

    C /dev
    A /dev/kmsg
    C /etc
    A /etc/mtab
    A /go
    A /go/src
    A /go/src/github.com
    A /go/src/github.com/docker
    A /go/src/github.com/docker/docker
    A /go/src/github.com/docker/docker/.git
    ....
