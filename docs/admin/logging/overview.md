<!--[metadata]>
+++
aliases = ["/engine/reference/logging/overview/"]
title = "Configuring Logging Drivers"
description = "Configure logging driver."
keywords = ["docker, logging, driver, Fluentd"]
[menu.main]
parent = "smn_logging"
weight=-99
+++
<![end-metadata]-->


# Configure logging drivers

The container can have a different logging driver than the Docker daemon. Use
the `--log-driver=VALUE` with the `docker run` command to configure the
container's logging driver. If the `--log-driver` option is not set, docker
uses the default (`json-file`) logging driver. The following options are
supported:

| Driver      | Description                                                                                                                   |
|-------------|-------------------------------------------------------------------------------------------------------------------------------|
| `none`      | Disables any logging for the container. `docker logs` won't be available with this driver.                                    |
| `json-file` | Default logging driver for Docker. Writes JSON messages to file.                                                              |
| `syslog`    | Syslog logging driver for Docker. Writes log messages to syslog.                                                              |
| `journald`  | Journald logging driver for Docker. Writes log messages to `journald`.                                                        |
| `gelf`      | Graylog Extended Log Format (GELF) logging driver for Docker. Writes log messages to a GELF endpoint likeGraylog or Logstash. |
| `fluentd`   | Fluentd logging driver for Docker. Writes log messages to `fluentd` (forward input).                                          |
| `awslogs`   | Amazon CloudWatch Logs logging driver for Docker. Writes log messages to Amazon CloudWatch Logs.                              |
| `splunk`    | Splunk logging driver for Docker. Writes log messages to `splunk` using HTTP Event Collector.                                 |
| `etwlogs`   | ETW logging driver for Docker on Windows. Writes log messages as ETW events.                                                  |
| `gcplogs`   | Google Cloud Logging driver for Docker. Writes log messages to Google Cloud Logging.                                          |

The `docker logs`command is available only for the `json-file` and `journald`
logging drivers.

The `labels` and `env` options add additional attributes for use with logging
drivers that accept them. Each option takes a comma-separated list of keys. If
there is collision between `label` and `env` keys, the value of the `env` takes
precedence.

To use attributes, specify them when you start the Docker daemon. For example,
to manually start the daemon with the `json-file` driver, and include additional
attributes in the output, run the following command:

```bash
$ docker daemon \
    --log-driver=json-file \
    --log-opt labels=foo \
    --log-opt env=foo,fizz
```

Then, run a container and specify values for the `labels` or `env`. For
example, you might use this:

```bash
$ docker run -dit --label foo=bar -e fizz=buzz alpine sh
```

This adds additional fields to the log depending on the driver, e.g. for
`json-file` that looks like:

```json
"attrs":{"fizz":"buzz","foo":"bar"}
```


## json-file options

The following logging options are supported for the `json-file` logging driver:

```bash
--log-opt max-size=[0-9+][k|m|g]
--log-opt max-file=[0-9+]
--log-opt labels=label1,label2
--log-opt env=env1,env2
```

Logs that reach `max-size` are rolled over. You can set the size in
kilobytes(k), megabytes(m), or gigabytes(g). eg `--log-opt max-size=50m`. If
`max-size` is not set, then logs are not rolled over.

`max-file` specifies the maximum number of files that a log is rolled over
before being discarded. eg `--log-opt max-file=100`. If `max-size` is not set,
then `max-file` is not honored.

If `max-size` and `max-file` are set, `docker logs` only returns the log lines
from the newest log file.


## syslog options

The following logging options are supported for the `syslog` logging driver:

```bash
--log-opt syslog-address=[tcp|udp|tcp+tls]://host:port
--log-opt syslog-address=unix://path
--log-opt syslog-address=unixgram://path
--log-opt syslog-facility=daemon
--log-opt syslog-tls-ca-cert=/etc/ca-certificates/custom/ca.pem
--log-opt syslog-tls-cert=/etc/ca-certificates/custom/cert.pem
--log-opt syslog-tls-key=/etc/ca-certificates/custom/key.pem
--log-opt syslog-tls-skip-verify=true
--log-opt tag="mailer"
--log-opt syslog-format=[rfc5424|rfc5424micro|rfc3164]
--log-opt env=ENV1,ENV2,ENV3
--log-opt labels=label1,label2,label3
```

`syslog-address` specifies the remote syslog server address where the driver
connects to. If not specified it defaults to the local unix socket of the
running system. If transport is either `tcp` or `udp` and `port` is not
specified it defaults to `514` The following example shows how to have the
`syslog` driver connect to a `syslog` remote server at `192.168.0.42` on port
`123`

```bash
$ docker run --log-driver=syslog --log-opt syslog-address=tcp://192.168.0.42:123
```

The `syslog-facility` option configures the syslog facility. By default, the
system uses the `daemon` value. To override this behavior, you can provide an
integer of 0 to 23 or any of the following named facilities:

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

`syslog-tls-ca-cert` specifies the absolute path to the trust certificates
signed by the CA. This option is ignored if the address protocol is not
`tcp+tls`.

`syslog-tls-cert` specifies the absolute path to the TLS certificate file. This
option is ignored if the address protocol is not `tcp+tls`.

`syslog-tls-key` specifies the absolute path to the TLS key file. This option
is ignored if the address protocol is not `tcp+tls`.

`syslog-tls-skip-verify` configures the TLS verification. This verification is
enabled by default, but it can be overriden by setting this option to `true`.
This option is ignored if the address protocol is not `tcp+tls`.

`tag` configures a string that is appended to the APP-NAME in the syslog
message. By default, Docker uses the first 12 characters of the container ID to
tag log messages. Refer to the [log tag option documentation](log_tags.md) for
customizing the log tag format.

`syslog-format` specifies syslog message format to use when logging. If not
specified it defaults to the local unix syslog format without hostname
specification. Specify rfc3164 to perform logging in RFC-3164 compatible
format. Specify rfc5424 to perform logging in RFC-5424 compatible format.
Specify rfc5424micro to perform logging in RFC-5424 compatible format with
microsecond timestamp resolution.

`env` is a comma-separated list of keys of environment variables. Used for
advanced [log tag options](log_tags.md).

`labels` is a comma-separated list of keys of labels. Used for advanced [log
tag options](log_tags.md).

## journald options

The `journald` logging driver stores the container id in the journal's
`CONTAINER_ID` field. For detailed information on working with this logging
driver, see [the journald logging driver](journald.md) reference documentation.

## GELF options

The GELF logging driver supports the following options:

```bash
--log-opt gelf-address=udp://host:port
--log-opt tag="database"
--log-opt labels=label1,label2
--log-opt env=env1,env2
--log-opt gelf-compression-type=gzip
--log-opt gelf-compression-level=1
```

The `gelf-address` option specifies the remote GELF server address that the
driver connects to. Currently, only `udp` is supported as the transport and you
must specify a `port` value. The following example shows how to connect the
`gelf` driver to a GELF remote server at `192.168.0.42` on port `12201`

```bash
$ docker run -dit \
    --log-driver=gelf \
    --log-opt gelf-address=udp://192.168.0.42:12201 \
    alpine sh
```

By default, Docker uses the first 12 characters of the container ID to tag log
messages. Refer to the [log tag option documentation](log_tags.md) for
customizing the log tag format.

The `labels` and `env` options are supported by the gelf logging
driver. It adds additional key on the `extra` fields, prefixed by an
underscore (`_`).

    // […]
    "_foo": "bar",
    "_fizz": "buzz",
    // […]

The `gelf-compression-type` option can be used to change how the GELF driver
compresses each log message. The accepted values are `gzip`, `zlib` and `none`.
`gzip` is chosen by default.

The `gelf-compression-level` option can be used to change the level of
compresssion when `gzip` or `zlib` is selected as `gelf-compression-type`.
Accepted value must be from from -1 to 9 (BestCompression). Higher levels
typically run slower but compress more. Default value is 1 (BestSpeed).

## Fluentd options

You can use the `--log-opt NAME=VALUE` flag to specify these additional Fluentd
logging driver options.

 - `fluentd-address`: specify `host:port` to connect [localhost:24224]
 - `tag`: specify tag for `fluentd` message
 - `fluentd-buffer-limit`: specify the maximum size of the fluentd log buffer [8MB]
 - `fluentd-retry-wait`: initial delay before a connection retry (after which it increases exponentially) [1000ms]
 - `fluentd-max-retries`: maximum number of connection retries before abrupt failure of docker [1073741824]
 - `fluentd-async-connect`: whether to block on initial connection or not [false]

For example, to specify both additional options:

```bash
$ docker run -dit \
    --log-driver=fluentd \
    --log-opt fluentd-address=localhost:24224 \
    --log-opt tag="docker.{{.Name}}" \
    alpine sh
```

If container cannot connect to the Fluentd daemon on the specified address and
`fluentd-async-connect` is not enabled, the container stops immediately.
For detailed information on working with this logging driver,
see [the fluentd logging driver](fluentd.md)


## Amazon CloudWatch Logs options

The Amazon CloudWatch Logs logging driver supports the following options:

```bash
--log-opt awslogs-region=<aws_region>
--log-opt awslogs-group=<log_group_name>
--log-opt awslogs-stream=<log_stream_name>
```

For detailed information on working with this logging driver, see [the awslogs
logging driver](awslogs.md) reference documentation.

## Splunk options

The Splunk logging driver requires the following options:

```bash
--log-opt splunk-token=<splunk_http_event_collector_token>
--log-opt splunk-url=https://your_splunk_instance:8088
```

For detailed information about working with this logging driver, see the
[Splunk logging driver](splunk.md) reference documentation.

## ETW logging driver options

The etwlogs logging driver does not require any options to be specified. This
logging driver forwards each log message as an ETW event. An ETW listener
can then be created to listen for these events.

The ETW logging driver is only available on Windows. For detailed information
on working with this logging driver, see [the ETW logging driver](etwlogs.md)
reference documentation.

## Google Cloud Logging options

The Google Cloud Logging driver supports the following options:

```bash
--log-opt gcp-project=<gcp_projext>
--log-opt labels=<label1>,<label2>
--log-opt env=<envvar1>,<envvar2>
--log-opt log-cmd=true
```

For detailed information about working with this logging driver, see the
[Google Cloud Logging driver](gcplogs.md). reference documentation.
