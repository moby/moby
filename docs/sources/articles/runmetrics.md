page_title: Docker Metrics
page_description: Measure the behavior of running containers
page_keywords: docker, metrics, CPU, memory, disk, IO, run, runtime

# Metrics

Docker provides various metrics both about the containers it manages
as well as about its internal state and health.

You can access the most imporant metrics via the `docker metrics`
command.

To integrate all metrics into your existing monitoring system, you can
use the Docker API. The `/metrics` endpoint exposes all available
metrics for bulk import into existing monitoring infrastructure.

All metric endpoints return a list of metrics in the following format:

    {
      "type": "COUNTER",
      "help": "Total seconds of cpu time consumed.",
      "name": "cpu_usage_seconds_total",
      "metrics": [
        {
          "value": 53.21,
          "labels": {
            "space": "kernel"
          }
        },
        {
          "value": 2778.09,
          "labels": {
            "space": "user"
          }
        }
      ]
    }

As you see, this metric has a set of labels as keys. Those labels may
refer to things like sub system, in this case whether the cpu time is
spent in user- or kernel space.

In the global `/metrics` endpoint, the labels are used to specify the
container a metric is about.

## CGroup Metrics

The following metrics are available for each container, running or not,
and for the host system itself.

- cpu_usage_seconds_total: Total seconds of cpu time. Labels: space =
  kernel|user. *Counter*
- cpu_throttling_periods_total: Total number of periods, as defined by
  cpu.cfs_period_us, with throttling enabled. *Counter*
- cpu_throttling_throttled_periods_total: Total number of periods with
  throttling enabled. *Counter*
- cpu_throttled_time_seconds_total: Total time the container was
  throttled for in seconds. *Counter*
- memory_usage_bytes: Current memory usage in bytes. *Gauge*
- memory_max_usage_bytes: Maximum memory usage ever recorded for
  container in bytes. *Gauge*
- memory_failures_total: Total number of times the container hit its
  memory limit. *Counter*
- memory_stats_*: Various metrics from group/memory/memory.stat. See 
  [cgroup documentation](https://www.kernel.org/doc/Documentation/cgroups/memory.txt),
  section 5.2 for more details. *Counter*s and *Gauge*s
- blkio_io_service_bytes_total: Number
  of bytes transferred by `operation` from/to `device`. Labels: device,
  operation. *Counter*
- blkio_io_serviced_total: Number of `operation`s from/to `device`.
  Labels: device, operation. *Counter*
- blkio_io_queued: Number of`operation`s currently queued up. Labels:
  device, operation. *Gauge*
- blkio_sectors_total: Number of sectors transferred by `operation`
  to/from `device. Labels: device, operation. *Counter*


## Global Docker Metrics

- http_requests_total: Total number of HTTP requests to the API. Labels: method, path. *Counter*
- http_request_duration_seconds: HTTP request latencies for API handlers in seconds. Labels: method, path. *Gauge*
- jobs_created_total: Total number of Docker internal jobs created. Labels: name. *Counter*
- jobs_ran_total: Total number of Docker internal jobs ran. Labels: name. *Counter*
- jobs_run_failed_total: Total number of Docker internal jobs failed during run. Labels: name. *Counter*
- jobs_run_duration_ms: Job run latency. Labels: name. *Counter*
