## Stackdriver Logging [![GoDoc](https://godoc.org/cloud.google.com/go/logging?status.svg)](https://godoc.org/cloud.google.com/go/logging)

- [About Stackdriver Logging](https://cloud.google.com/logging/)
- [API documentation](https://cloud.google.com/logging/docs)
- [Go client documentation](https://godoc.org/cloud.google.com/go/logging)
- [Complete sample programs](https://github.com/GoogleCloudPlatform/golang-samples/tree/master/logging)

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
(automatically and asynchronously) to the Stackdriver Logging service.
[snip]:# (logging-2)
```go
logger := client.Logger("my-log")
logger.Log(logging.Entry{Payload: "something happened!"})
```

Close your client before your program exits, to flush any buffered log entries.
[snip]:# (logging-3)
```go
err = client.Close()
if err != nil {
	// TODO: Handle error.
}
```