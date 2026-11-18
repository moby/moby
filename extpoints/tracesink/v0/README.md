# Daemon trace sinks

An extension can provide `tracesinkv0.Point` to receive sampled dockerd traces.
Its `ExportTraces` method accepts the standard OTLP `ExportTraceServiceRequest`.
Out-of-process extensions pass `sdk.WithServerPoints(tracesinkv0.ServerPoint)` to `sdk.Main`; dockerd uses the existing private extension connection.
In-process extensions only need to provide the point; dockerd invokes `ExportTraces` directly.
Do not offer the point on the daemon API socket.

To authorize a loaded extension to receive traces, add its ID to `daemon.json` and restart dockerd:

```json
{
  "extensions": {
    "trace-sinks": ["org.example.traces.v1"]
  }
}
```

This setting does not install or enable extensions.
Trace sinks work without an environment-configured OTLP exporter and do not replace that exporter when it is configured.
Only list extensions you trust with daemon trace data.

Delivery is best effort.
Each sink has a bounded queue and a deadline for each export.
Sinks must honor cancellation and must not use the daemon's tracer provider during `ExportTraces`, since that could recursively generate more exports.
This point does not export metrics.
