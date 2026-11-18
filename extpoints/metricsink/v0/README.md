# Daemon metric sinks

An extension can provide `metricsinkv0.Point` to receive dockerd metrics.
Its `ExportMetrics` method accepts the standard OTLP `ExportMetricsServiceRequest`.
Out-of-process extensions pass `sdk.WithServerPoints(metricsinkv0.ServerPoint)` to `sdk.Main`; dockerd uses the existing private extension connection.
In-process extensions only need to provide the point; dockerd invokes `ExportMetrics` directly.
Do not offer the point on the daemon API socket.

An extension that also provides `tracesinkv0.Point` can implement both `ExportTraces` and `ExportMetrics` on one type and pass both server points to `sdk.Main`.

To authorize a loaded extension to receive metrics, add its ID to `daemon.json` and restart dockerd:

```json
{
  "extensions": {
    "metric-sinks": ["org.example.metrics.v1"]
  }
}
```

This setting does not install or enable extensions, and it is separate from `trace-sinks`.
Only list extensions you trust with daemon metric data.

Sinks receive metrics recorded through OpenTelemetry instruments in dockerd.
Metrics that are only exposed on the Prometheus `/metrics` endpoint are not included.
Metrics use cumulative temporality.
dockerd collects and exports them periodically, every 60 seconds unless `OTEL_METRIC_EXPORT_INTERVAL` says otherwise, and once more when it shuts down.

Delivery is best effort.
Each export has a deadline.
Sinks must honor cancellation and must not record measurements on the daemon's meter provider during `ExportMetrics`, since that could recursively generate more exports.
