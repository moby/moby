## Why is this change needed or what are the use cases?

Sometimes after an outage, a user may want to access logs through a specific
time window. Right now, `docker logs` only allows you to retrieve logs after
a specific timestamp, but it lacks an `--until` flag.

Many other logging tools like journalctl allow this kind of relativistic
logging, so it'd be great if Docker could too.

## What are the requirements this change should meet?

- Should allow users to inspect logs within a time window
- Should not affect any existing functionality, such as `--since`

## What are some ways to design/implement this feature?

Add a new `--until` flag which operates in a similar way to `--since`.

Users can use `--until` by itself (all logs older than a specific date), or
in tandem with `--since` (all logs in a time window).

## Which design/implementation do you think is best and why?

There are two steps in the process: adding it to the daemon and to the client.

### 1. Daemon

- New `string` Until field in [ContainerLogsOptions](https://github.com/moby/moby/blob/d40a17ffc2f6592396a3dfc0f5ebe396c2107536/api/types/client.go#L73-L81)
- New `time.Time` Until field in [ReadConfig](https://github.com/moby/moby/blob/d40a17ffc2f6592396a3dfc0f5ebe396c2107536/daemon/logger/logger.go#L84-L88)
- Refactor all logging adaptors to use until logic:
  - [`daemon/logger/adapter.go`](https://github.com/moby/moby/blob/d40a17ffc2f6592396a3dfc0f5ebe396c2107536/daemon/logger/adapter.go#L81
  - [`daemon/logger/journald/read.go`](https://github.com/moby/moby/blob/d40a17ffc2f6592396a3dfc0f5ebe396c2107536/daemon/logger/journald/read.go#L413
  - [`daemon/logger/jsonfilelog/read.go`](https://github.com/moby/moby/blob/d40a17ffc2f6592396a3dfc0f5ebe396c2107536/daemon/logger/jsonfilelog/read.go#L41
  - [`daemon/logger/proxy.go`](https://github.com/moby/moby/blob/d40a17ffc2f6592396a3dfc0f5ebe396c2107536/daemon/logger/proxy.go#L99
  - [`daemon/logger/ring.go`](https://github.com/moby/moby/blob/d40a17ffc2f6592396a3dfc0f5ebe396c2107536/daemon/logger/ring.go#L28

### 2. Client/CLI

- Add conditional logic to [`client/container_logs.go`](https://github.com/moby/moby/blob/d40a17ffc2f6592396a3dfc0f5ebe396c2107536/client/container_logs.go)
- Populate correct options in [`cli/command/container/logs.go`](https://github.com/moby/moby/blob/d40a17ffc2f6592396a3dfc0f5ebe396c2107536/cli/command/container/logs.go)

### 3. Docs

- Update docs and any examples
