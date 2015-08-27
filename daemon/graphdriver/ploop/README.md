## Docker ploop graphdriver

This document describes how to run the Docker ploop graphdriver.

Ploop is the enhanced block loop device, developed for
[OpenVZ](https://openvz.org/) since around 2008 and used in production
since 2012. To know more about ploop, see
[openvz.org/Ploop](https://openvz.org/Ploop).

This driver is built upon the following stack of software:
* [kernel ploop driver](https://github.com/OpenVZ/vzkernel/tree/branch-rh7-3.10.0-229.7.2-ovz/drivers/block/ploop)
* [ploop command line tool](https://github.com/kolyshkin/ploop)
* [Go ploop library](https://github.com/kolyshkin/goploop-cli)

### Prerequisites

To use this, you need to have:
* ploop Linux kernel modules
* ploop command line tool
* ext4 file system for ``/var/lib/docker`` (or wherever the root of the Docker runtime is)

Any system with ploop kernel modules and working Docker will do,
but currently it appears that VZ7 is the only such system.

A work is currently underway to provide ploop kernel modules
compilable for the upstream kernel. Once finished, this document will be
updated accordingly.

Note you can either install the full VZ7 system, or just the kernel.
For the latter option, you need a CentOS/RHEL 7 system ready.
For VZ7 installation, see
[openvz.org/Quick_install](https://openvz.org/Quick_install)).

### Testing

The driver undergone some functionality, performance, and stress testing.
Currently there are no known bugs.

One good stress test creating and removing many images in parallel is
[github.com/crosbymichael/docker-stress](https://github.com/crosbymichael/docker-stress):

```
go get github.com/crosbymichael/docker-stress
docker-stress --containers 50 -c 200 -k 3s
```

Another stress test used is [github.com/spotify/docker-stress](https://github.com/spotify/docker-stress).

### Limitations

* This driver uses shared deltas (via hardlinks), a feature not officially supported by ''libploop'' yet (but it works fine).
* Ploop dynamic resize is not used, all images and containers are of the same size (seems that Docker does not have a concept of a per-container disk space limit).

### TODO

* Maybe move clone with hardlinking code to ``goploop-cli`` (or even ``libploop``)?
* Implement changed files tracker to optimize ``Diff()``/``Changes()``

## See also
* [openvz.org/Ploop](https://openvz.org/Ploop) -- ploop home page
* [github.com/kolyshkin/ploop](https://github.com/kolyshkin/ploop) -- ploop C library and tool
* [github.com/kolyshkin/goploop-cli](https://github.com/kolyshkin/goploop-cli) -- Go ploop library that uses ploop command line tool
* [github.com/kolyshkin/goploop](https://github.com/kolyshkin/goploop-cli) -- Go ploop library that uses ploop C library
