# goploop

This is a Go wrapper for [libploop](https://github.com/kolyshkin/ploop/tree/master/lib),
a C library to manage ploop.

## What is ploop?

Ploop is a loopback block device (a.k.a. "filesystem in a file"), not unlike [loop](https://en.wikipedia.org/wiki/Loop_device) but with better performance
and more features, including:

* thin provisioning (image grows on demand)
* dynamic resize (both grow and shrink)
* instant online snapshots
* online snapshot merge
* optimized image migration with write tracker (ploop copy)

Ploop is implemented in the kernel and is currently available in OpenVZ RHEL6 and RHEL7 based kernels. For more information about ploop, see [openvz.org/Ploop](https://openvz.org/Ploop).

## Prerequisites

You need to have
* ext4 formatted partition (note RHEL/CentOS 7 installer uses xfs by default, that won't work!)
* ploop-enabled kernel installed
* ploop kernel modules loaded
* ploop-lib and ploop-devel packages installed

Currently, all the above comes with OpenVZ, please see [openvz.org/Quick_installation](https://openvz.org/Quick_installation).
After installing OpenVZ, you might need to run:

    yum install ploop-devel

## Usage

For examples of how to use the package, see [ploop_test.go](ploop_test.go).
