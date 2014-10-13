libvirt-go
============

[![Build Status](http://ci.serversaurus.com/github.com/alexzorin/libvirt-go/status.svg?branch=master)](http://ci.serversaurus.com/github.com/alexzorin/libvirt-go)

Go bindings for libvirt.

Versions
--------------
Please use the `0.9` branch for libvirt 0.9.8 and below, which will be tagged `v1.x` releases: `gopkg.in/alexzorin/libvirt-go.v1` [(docs)](http://gopkg.in/alexzorin/libvirt-go.v1).

The `master` branch targets the 1.x version of libvirt, which will be tagged `v2.x` releases: `gopkg.in/alexzorin/libvirt-go.v2` [(docs)](http://gopkg.in/alexzorin/libvirt-go.v2).

Make sure to have libvirt-dev package (or the development files otherwise somewhere in your include path)

Documentation
--------------

* [api documentation for the bindings](http://godoc.org/github.com/alexzorin/libvirt-go)
* [api documentation for libvirt](http://libvirt.org/html/libvirt-libvirt.html)

Contributing
-------------

Please fork and write tests.

Integration tests are available where functionality isn't provided by the test driver, see `integration_test.go`.

`Vagrantfiles` are included to provision testing environments to run the integration tests:

* `cd ./vagrant/{branch}` (i.e `./vagrant/master`, where you will find a `Vagrantfile` for the `master` branch)
* `vagrant up` to provision the virtual machine
* `vagrant ssh` to login to the virtual machine

Once inside, `sudo su -`, `cd /libvirt-go` and `go test -tags integration`.
