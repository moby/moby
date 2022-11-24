## Cloud Logging [![Go Reference](https://pkg.go.dev/badge/cloud.google.com/go/logging.svg)](https://pkg.go.dev/cloud.google.com/go/logging)

- [About Cloud Logging](https://cloud.google.com/logging/)
- [API documentation](https://cloud.google.com/logging/docs)
- [Go client documentation](https://pkg.go.dev/cloud.google.com/go/logging)
- [Complete sample programs](https://github.com/GoogleCloudPlatform/golang-samples/tree/main/logging)

For an interactive tutorial on using the client library in a Go application, click [Guide Me](https://console.cloud.google.com/?walkthrough_id=logging__logging-go).
### Example Usage

First create a `logging.Client` to use throughout your application:
[snip]:# (logging-1)

```go
ctx := context.Background()
client, err := logging.NewClient(ctx, "my-project")
if err != nil {
   // TODO: Handle error.
}
```

Usually, you'll want to add log entries to a buffer to be periodically flushed
(automatically and asynchronously) to the Cloud Logging service.
[snip]:# (logging-2)

```go
logger := client.Logger("my-log")
logger.Log(logging.Entry{Payload: "something happened!"})
```

If you need to write a critical log entry use synchronous ingestion method.
[snip]:# (logging-3)

```go
logger := client.Logger("my-log")
logger.LogSync(context.Background(), logging.Entry{Payload: "something happened!"})
```

Close your client before your program exits, to flush any buffered log entries.
[snip]:# (logging-4)

```go
err = client.Close()
if err != nil {
   // TODO: Handle error.
}
```

### Logger configuration options

Creating a Logger using `logging.Logger` accept configuration [LoggerOption](loggeroption.go#L25) arguments. The following options are supported:

| Configuration option | Arguments | Description |
| -------------------- | --------- | ----------- |
| CommonLabels | `map[string]string` | The set of labels that will be ingested for all log entries ingested by Logger. |
| ConcurrentWriteLimit | `int` | Number of parallel goroutine the Logger will use to ingest logs asynchronously. High number of routines may exhaust API quota. The default is 1. |
| DelayThreshold | `time.Duration` | Maximum time a log entry is buffered on client before being ingested. The default is 1 second. |
| EntryCountThreshold | `int` | Maximum number of log entries to be buffered on client before being ingested. The default is 1000. |
| EntryByteThreshold | `int` | Maximum size in bytes of log entries to be buffered on client before being ingested. The default is 8MiB. |
| EntryByteLimit | `int` | Maximum size in bytes of the single write call to ingest log entries. If EntryByteLimit is smaller than EntryByteThreshold, the latter has no effect. The default is zero, meaning there is no limit. |
| BufferedByteLimit | `int` | Maximum number of bytes that the Logger will keep in memory before returning ErrOverflow. This option limits the total memory consumption of the Logger (but note that each Logger has its own, separate limit). It is possible to reach BufferedByteLimit even if it is larger than EntryByteThreshold or EntryByteLimit, because calls triggered by the latter two options may be enqueued (and hence occupying memory) while new log entries are being added. |
| ContextFunc | `func() (ctx context.Context, afterCall func())` | Callback function to be called to obtain `context.Context` during async log ingestion. |
| SourceLocationPopulation | One of `logging.DoNotPopulateSourceLocation`, `logging.PopulateSourceLocationForDebugEntries` or `logging.AlwaysPopulateSourceLocation` | Controls auto-population of the logging.Entry.SourceLocation field when ingesting log entries. Allows to disable population of source location info, allowing it only for log entries at Debug severity or enable it for all log entries. Enabling it for all entries may result in degradation in performance. Use `logging_test.BenchmarkSourceLocationPopulation` to test performance with and without the option. The default is set to `logging.DoNotPopulateSourceLocation`. |
| PartialSuccess | | Make each write call to Logging service with [partialSuccess flag](https://cloud.google.com/logging/docs/reference/v2/rest/v2/entries/write#body.request_body.FIELDS.partial_success) set. The default is to make calls without setting the flag. |
| RedirectAsJSON | `io.Writer` | Converts log entries to Jsonified one line string according to the [structured logging format](https://cloud.google.com/logging/docs/structured-logging#special-payload-fields) and writes it to provided `io.Writer`. Users should use this option with `os.Stdout` and `os.Stderr` to leverage the out-of-process ingestion of logs using logging agents that are deployed in Cloud Logging environments. |
