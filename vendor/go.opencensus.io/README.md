# OpenCensus Libraries for Go

[![Build Status][travis-image]][travis-url]
[![Windows Build Status][appveyor-image]][appveyor-url]
[![GoDoc][godoc-image]][godoc-url]
[![Gitter chat][gitter-image]][gitter-url]

OpenCensus Go is a Go implementation of OpenCensus, a toolkit for
collecting application performance and behavior monitoring data.
Currently it consists of three major components: tags, stats, and tracing.

## Installation

```
$ go get -u go.opencensus.io
```

The API of this project is still evolving, see: [Deprecation Policy](#deprecation-policy).
The use of vendoring or a dependency management tool is recommended.

## Prerequisites

OpenCensus Go libraries require Go 1.8 or later.

## Exporters

OpenCensus can export instrumentation data to various backends. 
Currently, OpenCensus supports:

* [Prometheus][exporter-prom] for stats
* [OpenZipkin][exporter-zipkin] for traces
* Stackdriver [Monitoring][exporter-stackdriver] and [Trace][exporter-stackdriver]
* [Jaeger][exporter-jaeger] for traces
* [AWS X-Ray][exporter-xray] for traces


## Overview

![OpenCensus Overview](https://i.imgur.com/cf4ElHE.jpg)

In a microservices environment, a user request may go through
multiple services until there is a response. OpenCensus allows
you to instrument your services and collect diagnostics data all
through your services end-to-end.

Start with instrumenting HTTP and gRPC clients and servers,
then add additional custom instrumentation if needed.

* [HTTP guide](https://github.com/census-instrumentation/opencensus-go/tree/master/examples/http)
* [gRPC guide](https://github.com/census-instrumentation/opencensus-go/tree/master/examples/grpc)


## Tags

Tags represent propagated key-value pairs. They are propagated using `context.Context`
in the same process or can be encoded to be transmitted on the wire. Usually, this will
be handled by an integration plugin, e.g. `ocgrpc.ServerHandler` and `ocgrpc.ClientHandler`
for gRPC.

Package tag allows adding or modifying tags in the current context.

[embedmd]:# (internal/readme/tags.go new)
```go
ctx, err = tag.New(ctx,
	tag.Insert(osKey, "macOS-10.12.5"),
	tag.Upsert(userIDKey, "cde36753ed"),
)
if err != nil {
	log.Fatal(err)
}
```

## Stats

OpenCensus is a low-overhead framework even if instrumentation is always enabled.
In order to be so, it is optimized to make recording of data points fast
and separate from the data aggregation.

OpenCensus stats collection happens in two stages:

* Definition of measures and recording of data points
* Definition of views and aggregation of the recorded data

### Recording

Measurements are data points associated with a measure.
Recording implicitly tags the set of Measurements with the tags from the
provided context:

[embedmd]:# (internal/readme/stats.go record)
```go
stats.Record(ctx, videoSize.M(102478))
```

### Views

Views are how Measures are aggregated. You can think of them as queries over the
set of recorded data points (measurements).

Views have two parts: the tags to group by and the aggregation type used.

Currently three types of aggregations are supported:
* CountAggregation is used to count the number of times a sample was recorded.
* DistributionAggregation is used to provide a histogram of the values of the samples.
* SumAggregation is used to sum up all sample values.

[embedmd]:# (internal/readme/stats.go aggs)
```go
distAgg := view.Distribution(0, 1<<32, 2<<32, 3<<32)
countAgg := view.Count()
sumAgg := view.Sum()
```

Here we create a view with the DistributionAggregation over our measure.

[embedmd]:# (internal/readme/stats.go view)
```go
if err := view.Register(&view.View{
	Name:        "my.org/video_size_distribution",
	Description: "distribution of processed video size over time",
	Measure:     videoSize,
	Aggregation: view.Distribution(0, 1<<32, 2<<32, 3<<32),
}); err != nil {
	log.Fatalf("Failed to subscribe to view: %v", err)
}
```

Subscribe begins collecting data for the view. Subscribed views' data will be
exported via the registered exporters.

## Traces

[embedmd]:# (internal/readme/trace.go startend)
```go
ctx, span := trace.StartSpan(ctx, "your choice of name")
defer span.End()
```

## Profiles

OpenCensus tags can be applied as profiler labels
for users who are on Go 1.9 and above.

[embedmd]:# (internal/readme/tags.go profiler)
```go
ctx, err = tag.New(ctx,
	tag.Insert(osKey, "macOS-10.12.5"),
	tag.Insert(userIDKey, "fff0989878"),
)
if err != nil {
	log.Fatal(err)
}
tag.Do(ctx, func(ctx context.Context) {
	// Do work.
	// When profiling is on, samples will be
	// recorded with the key/values from the tag map.
})
```

A screenshot of the CPU profile from the program above:

![CPU profile](https://i.imgur.com/jBKjlkw.png)

## Deprecation Policy

Before version 1.0.0, the following deprecation policy will be observed:

No backwards-incompatible changes will be made except for the removal of symbols that have
been marked as *Deprecated* for at least one minor release (e.g. 0.9.0 to 0.10.0). A release
removing the *Deprecated* functionality will be made no sooner than 28 days after the first 
release in which the functionality was marked *Deprecated*.

[travis-image]: https://travis-ci.org/census-instrumentation/opencensus-go.svg?branch=master
[travis-url]: https://travis-ci.org/census-instrumentation/opencensus-go
[appveyor-image]: https://ci.appveyor.com/api/projects/status/vgtt29ps1783ig38?svg=true
[appveyor-url]: https://ci.appveyor.com/project/opencensusgoteam/opencensus-go/branch/master
[godoc-image]: https://godoc.org/go.opencensus.io?status.svg
[godoc-url]: https://godoc.org/go.opencensus.io
[gitter-image]: https://badges.gitter.im/census-instrumentation/lobby.svg
[gitter-url]: https://gitter.im/census-instrumentation/lobby?utm_source=badge&utm_medium=badge&utm_campaign=pr-badge&utm_content=badge


[new-ex]: https://godoc.org/go.opencensus.io/tag#example-NewMap
[new-replace-ex]: https://godoc.org/go.opencensus.io/tag#example-NewMap--Replace

[exporter-prom]: https://godoc.org/go.opencensus.io/exporter/prometheus
[exporter-stackdriver]: https://godoc.org/contrib.go.opencensus.io/exporter/stackdriver
[exporter-zipkin]: https://godoc.org/go.opencensus.io/exporter/zipkin
[exporter-jaeger]: https://godoc.org/go.opencensus.io/exporter/jaeger
[exporter-xray]: https://github.com/census-instrumentation/opencensus-go-exporter-aws
