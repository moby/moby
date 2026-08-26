# Jobs

Jobs are one-shot workloads managed by the daemon: a **job** is a container
spec plus a trigger declaration, and every execution is a **run** with a
stable, durable identity that survives container removal and daemon
restarts. Jobs fire manually or on a cron schedule; the daemon evaluates
schedule triggers on its own clock, so scheduled work keeps firing while no
client is connected.

Jobs are an experimental feature, delivered as a builtin extension of the
daemon and exposed through a gRPC service on the daemon socket. The API
contract is versioned by its extension point, currently
`org.mobyproject.extension.jobs.api.v0`, and may change between releases
while the feature is experimental.

## Enabling the feature

The jobs extension is off by default. Enable it with the daemon's feature
flags, either on the command line:

```console
$ dockerd --feature jobs
```

or in `daemon.json`:

```json
{
  "features": { "jobs": true }
}
```

The flag is read once at startup; changing it takes effect on the next
daemon start. When the feature is disabled the Jobs service is not
registered at all, and calls to it fail with the gRPC `Unimplemented` code.

> **Note**: the gRPC endpoint of the daemon socket is not evaluated by
> authorization plugins, which gate the HTTP API only. Deployments relying
> on such plugins should take this into account before enabling the
> feature.

## Talking to the API

The service is defined in `extpoints/jobs/api/v0/jobs.proto` (generated
from the Go contract in the same directory, which is the reference
documentation for every RPC and message). Any gRPC client works; for
example, with a generated Go client:

```go
conn, _ := grpc.NewClient("unix:///var/run/docker.sock",
    grpc.WithTransportCredentials(insecure.NewCredentials()))
jobs := protogen.NewJobsClient(conn)
```

or interactively with grpcurl (all flags before the socket address, which
comes before the method):

```console
$ grpcurl -plaintext -unix -proto extpoints/jobs/api/v0/jobs.proto \
    -d '{"name": "backup", "spec": {"container_spec": "eyJJbWFnZSI6ICJidXN5Ym94In0="}}' \
    /var/run/docker.sock org.mobyproject.extension.jobs.api.v0.Jobs/Create
```

The container definition travels as a JSON payload (`container_spec`) in
the exact format of the container-create API request body; unknown fields
are rejected.

## Semantics in brief

- **Registration is idempotent.** `Create` never starts a run: it registers
  the job and arms its trigger. Re-submitting an identical name and spec is
  a no-op; the same name with a different spec fails with `AlreadyExists`.
  Identity is a canonical hash of the spec, exposed as `spec_hash`, so JSON
  formatting differences do not create a new identity. Specs are immutable:
  remove and re-create to change one.
- **Triggers.** A job is either manual (fired only by explicit `Run` calls)
  or scheduled by a five-field cron expression — numbers, lists, ranges and
  steps; names and `@daily`-style shortcuts are not accepted on the wire.
  Expressions are evaluated on the wall clock of the job's IANA timezone
  (empty means UTC). Event-driven triggers are planned for a later
  iteration.
- **Concurrency.** When a schedule fires while a run is in flight, the
  `forbid` policy (default) drops the fire and `queue` defers a single one
  until the current run completes. An explicit `Run` on a running job fails
  with `FailedPrecondition`.
- **Missed fires.** After daemon downtime, the `one` policy (default) fires
  a single catch-up run and `skip` drops missed occurrences; either way the
  schedule then re-arms from the next occurrence, never replaying a
  backlog.
- **Runs.** The run record is persisted before its container is created, so
  a failure or crash always leaves a discoverable record. Runs end
  `succeeded`, `failed`, `timed_out` (the spec's timeout, enforced by the
  daemon) or `cancelled`; terminal records are immutable. Run history is
  capped per job (10000 by default, oldest terminal runs evicted first).
  Removing a job keeps its run history by default.
- **Reschedule.** `Run` with `reschedule` makes the manual fire stand in
  for the job's next scheduled occurrence, which is skipped; later
  occurrences keep the cron alignment.

## Run containers and logs

Run containers carry the reserved labels `com.docker.job.id` and
`com.docker.job.run-id`, correlating them with job and run records across
the container API (`docker ps --filter label=com.docker.job.id=<id>`).

The jobs service does not proxy container logs. Runs expose their
`container_id` (and a `container_gone` flag once the container was
removed); read logs from the standard container logs API, for example
`docker logs <container_id>`. Following logs live works the same way —
`/containers/{id}/logs?follow=1` replays from the beginning and then
follows, so nothing is missed by attaching after the run started. Logs are
gone when the container is removed: runs configured with
`remove_on_success` or `remove_on_failure` trade postmortem logs for
automatic cleanup, keeping exit code and error on the run record.

## Restarts

State lives under the daemon data root and survives restarts. On startup
the extension reconciles runs left in flight by an unclean stop: a run
whose container exited during the downtime is resolved with its actual
exit code, a run whose container still runs (for example under
`live-restore`) is re-attached and watched to completion, timeouts are
re-armed from the original start time, and a run whose start was never
recorded is failed rather than guessed at. Only then are schedule triggers
re-armed, applying the missed-fires policy.
