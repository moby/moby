# Experimental Features

The Metric SDK contains features that have not yet stabilized in the OpenTelemetry specification.
These features are added to the OpenTelemetry Go Metric SDK prior to stabilization in the specification so that users can start experimenting with them and provide feedback.

These feature may change in backwards incompatible ways as feedback is applied.
See the [Compatibility and Stability](#compatibility-and-stability) section for more information.

## Features

- [Metric Export Batch Size](#metric-export-batch-size)

### Metric Export Batch Size

The metric export can be split into batches before exporting by specifying a maximum number of data points per batch.

This experimental feature can be enabled by setting the `OTEL_GO_X_METRIC_EXPORT_BATCH_SIZE` environment variable.
The value MUST be a positive integer.
All other values or an empty value will result in the default behavior of not batching.

#### Examples

Enable metrics to be batched by maximum export batch size of 200.

```console
export OTEL_GO_X_METRIC_EXPORT_BATCH_SIZE=200
```

Disable metric export batching.

```console
unset OTEL_GO_X_METRIC_EXPORT_BATCH_SIZE
```

## Compatibility and Stability

Experimental features do not fall within the scope of the OpenTelemetry Go versioning and stability [policy](../../../../VERSIONING.md).
These features may be removed or modified in successive version releases, including patch versions.

When an experimental feature is promoted to a stable feature, a migration path will be included in the changelog entry of the release.
There is no guarantee that any environment variable feature flags that enabled the experimental feature will be supported by the stable version.
If they are supported, they may be accompanied with a deprecation notice stating a timeline for the removal of that support.
