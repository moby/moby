## Docker ploop graphdriver

This document describes how to run the Docker ploop graphdriver.

Ploop is the enhanced block loop device, developed for
[OpenVZ](https://openvz.org/) since around 2008.
To know more about ploop, see
[openvz.org/Ploop](https://openvz.org/Ploop).

This driver is built upon the following stack of software:
* kernel ploop driver (available in OpenVZ kernels)
* ploop C library [github.com/kolyshkin/ploop](https://github.com/kolyshkin/ploop)
* goploop wrapper of the abovementioned library [github.com/kolyshkin/goploop](https://github.com/kolyshkin/goploop)

Note this driver uses shared deltas (via hardlinks), a feature
not officially supported by libploop (but apparently it works fine).

### Prerequisites

To try this, you need to have VZ7 installed,
see [openvz.org/Quick_install](https://openvz.org/Quick_install)).
Technically, any system with ploop kernel driver and working Docker
will do, but currently it appears that VZ7 is the only such system.

Note you can either install the full VZ7 system, or just the kernel.
For the latter option, you need a CentOS/RHEL 7 system ready.

Once VZ7 (or its kernel) is up and running, you need to install docker:
```bash
yum install docker
service docker start
```

### Building

Download my docker fork from github, and switch to ploop branch:
```bash
yum install git
git clone https://github.com/kolyshkin/docker
cd docker
git checkout ploop
```

All the following commands assume you are in docker git root directory.

To build it:
```bash
make
```

### Using

If the above works (takes 10-15 minutes for the first time, consecutive
runs are faster thanks to caching), you can try starting it:

```bash
service docker stop
rm /var/lib/docker/*
./bundles/latest/binary/docker -D -d -s ploop
# Options: -d for daemon, -D for debug, -s to use ploop graphdriver
```

Now you can try some docker commands. The fastest one is probably this:
```bash
docker run busybox ps
```

You might notice docker daemon complains about incompatible
client version. You can fix it, too.
```bash
export PATH=`pwd`/bundles/latest/binary/:$PATH
hash -r
which docker # make sure it shows the one you built
```

For something slower, you can try rebuilding yourself:
```bash
make
```

### Testing

A good stress test creating and removing many images in parallel is
[github.com/crosbymichael/docker-stress](https://github.com/crosbymichael/docker-stress).

```bash
git clone https://github.com/crosbymichael/docker-stress
cd doocker-stress
go build -v .
./docker-stress --containers 50 -c 200 -k 3s
```

Another stress test I used is [github.com/spotify/docker-stress](https://vgithub.com/spotify/docker-stress).

### Limitations

* The driver is a work in progress: not optimized for speed, probably buggy (this is my first Go project, not counting goploop).
* Ploop dynamic resize is not used, all images and containers are of the same size (frankly I'm not sure if Docker has a concept of a per-container disk space limit).

### TODO

* Move clone code to goploop (or even libploop)?
* More stress testing
* Performance comparison
* Implement "changed files" tracker to optimize Diff()/Changes()

## See also
* [openvz.org](https://openvz.org)
* [openvz.org/Ploop](https://openvz.org/Ploop)
* [C ploop library](https://github.com/kolyshkin/ploop)
* [Go ploop library](https://github.com/kolyshkin/goploop) (a wrapper around C lib)
