# containerd

containerd is a daemon to control runC, built for performance and density. 
containerd leverages runC's advanced features such as seccomp and user namespace support as well
as checkpoint and restore for cloning and live migration of containers.

## Getting started

The easiest way to start using containerd is to download binaries from the [releases page](https://github.com/containerd/containerd/releases).

The included `ctr` command-line tool allows you interact with the containerd daemon:

```
$ sudo ctr containers start redis /containers/redis
$ sudo ctr containers list
ID                  PATH                STATUS              PROCESSES
redis               /containers/redis   running             14063
```

`/containers/redis` is the path to an OCI bundle. [See the docs for more information.](docs/bundle.md)

## Docs

 * [Client CLI reference (`ctr`)](docs/cli.md)
 * [Daemon CLI reference (`containerd`)](docs/daemon.md)
 * [Creating OCI bundles](docs/bundle.md)
 * [containerd changes to the bundle](docs/bundle-changes.md)
 * [Attaching to STDIO or TTY](docs/attach.md)
 * [Telemetry and metrics](docs/telemetry.md)

All documentation is contained in the `/docs` directory in this repository.

## Building

You will need to make sure that you have Go installed on your system and the containerd repository is cloned
in your `$GOPATH`.  You will also need to make sure that you have all the dependencies cloned as well.
Currently, contributing to containerd is not for the first time devs as many dependencies are not vendored and 
work is being completed at a high rate.  

After that just run `make` and the binaries for the daemon and client will be localed in the `bin/` directory.

## Performance

Starting 1000 containers concurrently runs at 126-140 containers per second.

Overall start times:

```
[containerd] 2015/12/04 15:00:54   count:        1000
[containerd] 2015/12/04 14:59:54   min:          23ms
[containerd] 2015/12/04 14:59:54   max:         355ms
[containerd] 2015/12/04 14:59:54   mean:         78ms
[containerd] 2015/12/04 14:59:54   stddev:       34ms
[containerd] 2015/12/04 14:59:54   median:       73ms
[containerd] 2015/12/04 14:59:54   75%:          91ms
[containerd] 2015/12/04 14:59:54   95%:         123ms
[containerd] 2015/12/04 14:59:54   99%:         287ms
[containerd] 2015/12/04 14:59:54   99.9%:       355ms
```

## Roadmap

The current roadmap and milestones for alpha and beta completion are in the github issues on this repository.  Please refer to these issues for what is being worked on and completed for the various stages of development.

## Copyright and license

Copyright © 2016 Docker, Inc. All rights reserved, except as follows. Code
is released under the Apache 2.0 license. The README.md file, and files in the
"docs" folder are licensed under the Creative Commons Attribution 4.0
International License under the terms and conditions set forth in the file
"LICENSE.docs". You may obtain a duplicate copy of the same license, titled
CC-BY-SA-4.0, at http://creativecommons.org/licenses/by/4.0/.
