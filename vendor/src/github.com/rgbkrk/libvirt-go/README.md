# Carving out the future

Go 1.6 is incompatible with libvirt's implementation of Domain Events (those things that callback to your code to let you know something changed in a domain's state), so builds of this under golang 1.6 will not include Domain Events unless someone comes up with a workaround.

# libvirt-go

[![Build Status](http://ci.serversaurus.com/github.com/alexzorin/libvirt-go/status.svg?branch=master)](http://ci.serversaurus.com/github.com/alexzorin/libvirt-go)

Go bindings for libvirt.

Make sure to have `libvirt-dev` package (or the development files otherwise somewhere in your include path)

## Version Support
Currently, the only supported version of libvirt is **1.2.2**, tagged as `v2.x` releases `gopkg.in/alexzorin/libvirt-go.v2` [(docs)](http://gopkg.in/alexzorin/libvirt-go.v2).

The bindings will probably work with versions of libvirt that are higher than 1.2.2, depending on what is added in those releases. However, no features are currently being added that will cause the build or tests to break against 1.2.2.

### OS Compatibility Matrix

To quickly see what version of libvirt your OS can easily support (may be outdated). Obviously, nothing below 1.2.2 is usable with these bindings.

| OS Release   | libvirt Version                |
| ------------ | ------------------------------ |
| FC19         | 1.2.9 from libvirt.org/sources |
| Debian 7     | 1.2.4 from wheezy-backports    |
| Debian 6     | 0.9.12 from squeeze-backports  |
| Ubuntu 14.04 | 1.2.2 from trusty              |
| RHEL 6       | 0.10.x                         |
| RHEL 5       | 0.8.x                          |


### 0.9.x Support

Previously there was support for libvirt 0.9.8 and below, however this is no longer being updated. These releases were tagged `v1.x` at `gopkg.in/alexzorin/libvirt-go.v1` [(docs)](http://gopkg.in/alexzorin/libvirt-go.v1).

## Documentation

* [api documentation for the bindings](http://godoc.org/github.com/rgbkrk/libvirt-go)
* [api documentation for libvirt](http://libvirt.org/html/libvirt-libvirt.html)

## Contributing

Please fork and write tests.

Integration tests are available where functionality isn't provided by the test driver, see `integration_test.go`.

A `Vagrantfile` is included to run the integration tests:

* `cd ./vagrant/{branch}` (i.e `./vagrant/master`, where you will find a `Vagrantfile` for the `master` branch)
* `vagrant up` to provision the virtual machine
* `vagrant ssh` to login to the virtual machine

Once inside, `sudo su -`, `cd /libvirt-go` and `go test -tags integration`.
