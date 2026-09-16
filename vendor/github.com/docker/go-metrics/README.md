# go-metrics

[![Go Reference](https://pkg.go.dev/badge/github.com/docker/go-metrics.svg)](https://pkg.go.dev/github.com/docker/go-metrics)

This package is small wrapper around the prometheus go client to help enforce convention and best practices for metrics collection in Docker projects.

## Best Practices

This packages is meant to be used for collecting metrics. It is not meant to be
used as a replacement for the prometheus client but to help enforce consistent
naming across metrics collected. If you have not already read the prometheus
best practices around naming and labels you can read the naming section in the
[prometheus documentation](https://prometheus.io/docs/practices/naming/).

The following are a few conventions that help naming metrics in your project.

### Namespace and Subsystem

This package provides you with a namespace type that allows you to specify the
same namespace and subsystem for your metrics.

```go
ns := metrics.NewNamespace("engine", "daemon", metrics.Labels{
        "version": daemon.Version,
        "commit":  daemon.GitCommit,
})
```

In the example above we are creating metrics for the daemon package.
`engine` would be the namespace in this example where `daemon` is the subsystem
or package where we are collecting the metrics.

A namespace also allows you to attach constant labels to the metrics such as
the git commit and version that it is collecting.

### Declaring your Metrics

Try to keep all your metric declarations in one file. This makes it easy for
others to see what constant labels are defined on the namespace and what
labels are defined on the metrics when they are created.

### Use labels instead of multiple metrics

Labels allow you to define one metric such as the time it takes to perform a
certain action on an object. If we wanted to collect timings on various container
actions such as create, start, and delete then we can define one metric called
`container_actions` and use labels to specify the type of action.


```go
containerActions = ns.NewLabeledTimer("container_actions", "The number of milliseconds it takes to process each container action", "action")
```

The last parameter is the label name or key. When adding a data point to the
metric you can use the `WithValues` function to specify the `action` that you
are collecting for.

```go
containerActions.WithValues("create").UpdateSince(start)
```

### Always use a unit

The metric name should describe what you are measuring but you also need to
provide the unit that it is being measured with. For a timer, the standard
unit is seconds and a counter's standard unit is a total. For gauges you must
provide the unit. This package provides a standard set of units for use within
your projects.

```go
Nanoseconds Unit = "nanoseconds"
Seconds     Unit = "seconds"
Bytes       Unit = "bytes"
Total       Unit = "total"
```

If you need to use a unit but it is not defined in the package please open
a PR to add it but first try to see if one of the existing units work for
your metric, i.e., seconds or nanoseconds vs adding milliseconds.

## HTTP Metrics

To instrument a http handler, you can wrap the code like this:

```go
namespace := metrics.NewNamespace("api_server", "http", metrics.Labels{"handler": "your_http_handler_name"})
httpMetrics := namespace.NewDefaultHttpMetrics()
metrics.Register(namespace)
instrumentedHandler = metrics.InstrumentHandler(httpMetrics, unInstrumentedHandler)
```
Note: The `handler` label must be provided when a new namespace is created.

## Additional Metrics

Additional metrics are also defined here that are not available in the prometheus client.
If you need a custom metrics and it is generic enough to be used by multiple projects, define it here.


## Copyright and license

Copyright © 2016 Docker, Inc. All rights reserved, except as follows. Code is released under the Apache 2.0 license.
