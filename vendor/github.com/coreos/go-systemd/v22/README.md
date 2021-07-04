# go-systemd

[![Build Status](https://travis-ci.org/coreos/go-systemd.png?branch=master)](https://travis-ci.org/coreos/go-systemd)
[![godoc](https://img.shields.io/badge/godoc-reference-5272B4)](https://pkg.go.dev/mod/github.com/coreos/go-systemd/v22/?tab=packages)
![minimum golang 1.12](https://img.shields.io/badge/golang-1.12%2B-orange.svg)


Go bindings to systemd. The project has several packages:

- `activation` - for writing and using socket activation from Go
- `daemon` - for notifying systemd of service status changes
- `dbus` - for starting/stopping/inspecting running services and units
- `journal` - for writing to systemd's logging service, journald
- `sdjournal` - for reading from journald by wrapping its C API
- `login1` - for integration with the systemd logind API
- `machine1` - for registering machines/containers with systemd
- `unit` - for (de)serialization and comparison of unit files

## Socket Activation

An example HTTP server using socket activation can be quickly set up by following this README on a Linux machine running systemd:

https://github.com/coreos/go-systemd/tree/master/examples/activation/httpserver

## systemd Service Notification

The `daemon` package is an implementation of the [sd_notify protocol](https://www.freedesktop.org/software/systemd/man/sd_notify.html#Description).
It can be used to inform systemd of service start-up completion, watchdog events, and other status changes.

## D-Bus

The `dbus` package connects to the [systemd D-Bus API](http://www.freedesktop.org/wiki/Software/systemd/dbus/) and lets you start, stop and introspect systemd units.
[API documentation][dbus-doc] is available online.

[dbus-doc]: https://pkg.go.dev/github.com/coreos/go-systemd/v22/dbus?tab=doc

### Debugging

Create `/etc/dbus-1/system-local.conf` that looks like this:

```
<!DOCTYPE busconfig PUBLIC
"-//freedesktop//DTD D-Bus Bus Configuration 1.0//EN"
"http://www.freedesktop.org/standards/dbus/1.0/busconfig.dtd">
<busconfig>
    <policy user="root">
        <allow eavesdrop="true"/>
        <allow eavesdrop="true" send_destination="*"/>
    </policy>
</busconfig>
```

## Journal

### Writing to the Journal

Using the pure-Go `journal` package you can submit journal entries directly to systemd's journal, taking advantage of features like indexed key/value pairs for each log entry.

### Reading from the Journal

The `sdjournal` package provides read access to the journal by wrapping around journald's native C API; consequently it requires cgo and the journal headers to be available.

## logind

The `login1` package provides functions to integrate with the [systemd logind API](http://www.freedesktop.org/wiki/Software/systemd/logind/).

## machined

The `machine1` package allows interaction with the [systemd machined D-Bus API](http://www.freedesktop.org/wiki/Software/systemd/machined/).

## Units

The `unit` package provides various functions for working with [systemd unit files](http://www.freedesktop.org/software/systemd/man/systemd.unit.html).
