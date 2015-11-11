<!--[metadata]>
+++
title = "Configuring Logging Drivers"
description = "Configure logging driver."
keywords = ["docker, logging, driver, Fluentd"]
[menu.main]
parent = "smn_logging"
weight=-1
+++
<![end-metadata]-->


# Configure logging drivers

The container can have a different logging driver than the Docker daemon. Use
the `--log-driver=VALUE` with the `docker run` command to configure the
container's logging driver. The following options are supported:

| `none`      | Disables any logging for the container. `docker logs` won't be available with this driver.                                    |
|-------------|-------------------------------------------------------------------------------------------------------------------------------|
| `json-file` | Default logging driver for Docker. Writes JSON messages to file.                                                              |
| `syslog`    | Syslog logging driver for Docker. Writes log messages to syslog.                                                              |
| `journald`  | Journald logging driver for Docker. Writes log messages to `journald`.                                                        |
| `gelf`      | Graylog Extended Log Format (GELF) logging driver for Docker. Writes log messages to a GELF endpoint likeGraylog or Logstash. |
| `fluentd`   | Fluentd logging driver for Docker. Writes log messages to `fluentd` (forward input).                                          |
| `awslogs`   | Amazon CloudWatch Logs logging driver for Docker. Writes log messages to Amazon CloudWatch Logs.                              |
| `splunk`    | Splunk logging driver for Docker. Writes log messages to `splunk` using HTTP Event Collector.                                 |

The `docker logs`command is available only for the `json-file` and `journald`
logging drivers.

The `labels` and `env` options add additional attributes for use with logging drivers that accept them. Each option takes a comma-separated list of keys. If there is collision between `label` and `env` keys, the value of the `env` takes precedence.

To use attributes, specify them when you start the Docker daemon.

```
docker daemon --log-driver=json-file --log-opt labels=foo --log-opt env=foo,fizz
```

Then, run a container and specify values for the `labels` or `env`.  For example, you might use this:

```
docker run --label foo=bar -e fizz=buzz -d -P training/webapp python app.py
```

This adds additional fields to the log depending on the driver, e.g. for
`json-file` that looks like:

    "attrs":{"fizz":"buzz","foo":"bar"}


## json-file options

The following logging options are supported for the `json-file` logging driver:

    --log-opt max-size=[0-9+][k|m|g]
    --log-opt max-file=[0-9+]
    --log-opt labels=label1,label2
    --log-opt env=env1,env2

Logs that reach `max-size` are rolled over. You can set the size in kilobytes(k), megabytes(m), or gigabytes(g). eg `--log-opt max-size=50m`. If `max-size` is not set, then logs are not rolled over.

`max-file` specifies the maximum number of files that a log is rolled over before being discarded. eg `--log-opt max-file=100`. If `max-size` is not set, then `max-file` is not honored.

If `max-size` and `max-file` are set, `docker logs` only returns the log lines from the newest log file.


## syslog options

The following logging options are supported for the `syslog` logging driver:

    --log-opt syslog-address=[tcp|udp]://host:port
    --log-opt syslog-address=unix://path
    --log-opt syslog-facility=daemon
    --log-opt tag="mailer"

`syslog-address` specifies the remote syslog server address where the driver connects to.
If not specified it defaults to the local unix socket of the running system.
If transport is either `tcp` or `udp` and `port` is not specified it defaults to `514`
The following example shows how to have the `syslog` driver connect to a `syslog`
remote server at `192.168.0.42` on port `123`

    $ docker run --log-driver=syslog --log-opt syslog-address=tcp://192.168.0.42:123

The `syslog-facility` option configures the syslog facility. By default, the system uses the
`daemon` value. To override this behavior, you can provide an integer of 0 to 23 or any of
the following named facilities:

* `kern`
* `user`
* `mail`
* `daemon`
* `auth`
* `syslog`
* `lpr`
* `news`
* `uucp`
* `cron`
* `authpriv`
* `ftp`
* `local0`
* `local1`
* `local2`
* `local3`
* `local4`
* `local5`
* `local6`
* `local7`

By default, Docker uses the first 12 characters of the container ID to tag log messages.
Refer to the [log tag option documentation](log_tags.md) for customizing
the log tag format.


## journald options

The `journald` logging driver stores the container id in the journal's `CONTAINER_ID` field. For detailed information on
working with this logging driver, see [the journald logging driver](journald.md)
reference documentation.

## gelf options

The GELF logging driver supports the following options:

    --log-opt gelf-address=udp://host:port
    --log-opt tag="database"
    --log-opt labels=label1,label2
    --log-opt env=env1,env2

The `gelf-address` option specifies the remote GELF server address that the
driver connects to. Currently, only `udp` is supported as the transport and you must
specify a `port` value. The following example shows how to connect the `gelf`
driver to a GELF remote server at `192.168.0.42` on port `12201`

    $ docker run --log-driver=gelf --log-opt gelf-address=udp://192.168.0.42:12201

By default, Docker uses the first 12 characters of the container ID to tag log messages.
Refer to the [log tag option documentation](log_tags.md) for customizing
the log tag format.

The `labels` and `env` options are supported by the gelf logging
driver. It adds additional key on the `extra` fields, prefixed by an
underscore (`_`).

    // […]
    "_foo": "bar",
    "_fizz": "buzz",
    // […]


## fluentd options

You can use the `--log-opt NAME=VALUE` flag to specify these additional Fluentd logging driver options.

 - `fluentd-address`: specify `host:port` to connect [localhost:24224]
 - `tag`: specify tag for `fluentd` message,

For example, to specify both additional options:

`docker run --log-driver=fluentd --log-opt fluentd-address=localhost:24224 --log-opt tag=docker.{{.Name}}`

If container cannot connect to the Fluentd daemon on the specified address,
the container stops immediately. For detailed information on working with this
logging driver, see [the fluentd logging driver](fluentd.md)


## Specify Amazon CloudWatch Logs options

The Amazon CloudWatch Logs logging driver supports the following options:

    --log-opt awslogs-region=<aws_region>
    --log-opt awslogs-group=<log_group_name>
    --log-opt awslogs-stream=<log_stream_name>


For detailed information on working with this logging driver, see [the awslogs logging driver](awslogs.md) reference documentation.

## Splunk options

The Splunk logging driver requires the following options:

    --log-opt splunk-token=<splunk_http_event_collector_token>
    --log-opt splunk-url=https://your_splunk_instance:8088

For detailed information about working with this logging driver, see the [Splunk logging driver](splunk.md)
reference documentation.
