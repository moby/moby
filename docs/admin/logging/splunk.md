<!--[metadata]>
+++
aliases = ["/engine/reference/logging/splunk/"]
title = "Splunk logging driver"
description = "Describes how to use the Splunk logging driver."
keywords = ["splunk, docker, logging, driver"]
[menu.main]
parent = "smn_logging"
+++
<![end-metadata]-->

# Splunk logging driver

The `splunk` logging driver sends container logs to
[HTTP Event Collector](http://dev.splunk.com/view/event-collector/SP-CAAAE6M)
in Splunk Enterprise and Splunk Cloud.

## Usage

You can configure the default logging driver by passing the `--log-driver`
option to the Docker daemon:

    dockerd --log-driver=splunk

You can set the logging driver for a specific container by using the
`--log-driver` option to `docker run`:

    docker run --log-driver=splunk ...

## Splunk options

You can use the `--log-opt NAME=VALUE` flag to specify these additional Splunk
logging driver options:

| Option                      | Required | Description                                                                                                                                                                                                             |
|-----------------------------|----------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `splunk-token`              | required | Splunk HTTP Event Collector token.                                                                                                                                                                                      |
| `splunk-url`                | required | Path to your Splunk Enterprise or Splunk Cloud instance (including port and scheme used by HTTP Event Collector) `https://your_splunk_instance:8088`.                                                                   |
| `splunk-source`             | optional | Event source.                                                                                                                                                                                                           |
| `splunk-sourcetype`         | optional | Event source type.                                                                                                                                                                                                      |
| `splunk-index`              | optional | Event index.                                                                                                                                                                                                            |
| `splunk-capath`             | optional | Path to root certificate.                                                                                                                                                                                               |
| `splunk-caname`             | optional | Name to use for validating server certificate; by default the hostname of the `splunk-url` will be used.                                                                                                                |
| `splunk-insecureskipverify` | optional | Ignore server certificate validation.                                                                                                                                                                                   |
| `splunk-format`             | optional | Message format. Can be `inline`, `json` or `raw`. Defaults to `inline`.                                                                                                                                                 |
| `splunk-verify-connection`  | optional | Verify on start, that docker can connect to Splunk server. Defaults to true.                                                                                                                                            |
| `splunk-gzip`               | optional | Enable/disable gzip compression to send events to Splunk Enterprise or Splunk Cloud instance. Defaults to false.                                                                                                         |
| `splunk-gzip-level`         | optional | Set compression level for gzip. Valid values are -1 (default), 0 (no compression), 1 (best speed) ... 9 (best compression). Defaults to [DefaultCompression](https://golang.org/pkg/compress/gzip/#DefaultCompression). |
| `tag`                       | optional | Specify tag for message, which interpret some markup. Default value is `{{.ID}}` (12 characters of the container ID). Refer to the [log tag option documentation](log_tags.md) for customizing the log tag format.      |
| `labels`                    | optional | Comma-separated list of keys of labels, which should be included in message, if these labels are specified for container.                                                                                               |
| `env`                       | optional | Comma-separated list of keys of environment variables, which should be included in message, if these variables are specified for container.                                                                             |

If there is collision between `label` and `env` keys, the value of the `env` takes precedence.
Both options add additional fields to the attributes of a logging message.

Below is an example of the logging option specified for the Splunk Enterprise
instance. The instance is installed locally on the same machine on which the
Docker daemon is running. The path to the root certificate and Common Name is
specified using an HTTPS scheme. This is used for verification.
The `SplunkServerDefaultCert` is automatically generated by Splunk certificates.

    docker run --log-driver=splunk \
        --log-opt splunk-token=176FCEBF-4CF5-4EDF-91BC-703796522D20 \
        --log-opt splunk-url=https://splunkhost:8088 \
        --log-opt splunk-capath=/path/to/cert/cacert.pem \
        --log-opt splunk-caname=SplunkServerDefaultCert
        --log-opt tag="{{.Name}}/{{.FullID}}"
        --log-opt labels=location
        --log-opt env=TEST
        --env "TEST=false"
        --label location=west
        your/application

### Message formats

By default Logging Driver sends messages as `inline` format, where each message
will be embedded as a string, for example

```
{
    "attrs": {
        "env1": "val1",
        "label1": "label1"
    },
    "tag": "MyImage/MyContainer",
    "source":  "stdout",
    "line": "my message"
}
{
    "attrs": {
        "env1": "val1",
        "label1": "label1"
    },
    "tag": "MyImage/MyContainer",
    "source":  "stdout",
    "line": "{\"foo\": \"bar\"}"
}
```

In case if your messages are JSON objects you may want to embed them in the
message we send to Splunk. By specifying `--log-opt splunk-format=json` driver
will try to parse every line as a JSON object and send it as embedded object. In
case if it cannot parse it - message will be send as `inline`. For example


```
{
    "attrs": {
        "env1": "val1",
        "label1": "label1"
    },
    "tag": "MyImage/MyContainer",
    "source":  "stdout",
    "line": "my message"
}
{
    "attrs": {
        "env1": "val1",
        "label1": "label1"
    },
    "tag": "MyImage/MyContainer",
    "source":  "stdout",
    "line": {
        "foo": "bar"
    }
}
```

Third format is a `raw` message. You can specify it by using
`--log-opt splunk-format=raw`. Attributes (environment variables and labels) and
tag will be prefixed to the message. For example

```
MyImage/MyContainer env1=val1 label1=label1 my message
MyImage/MyContainer env1=val1 label1=label1 {"foo": "bar"}
```

## Advanced options

Splunk Logging Driver allows you to configure few advanced options by specifying next environment variables for the Docker daemon.

| Environment variable name                        | Default value | Description                                                                                                                                        |
|--------------------------------------------------|---------------|----------------------------------------------------------------------------------------------------------------------------------------------------|
| `SPLUNK_LOGGING_DRIVER_POST_MESSAGES_FREQUENCY`  | `5s`          | If there is nothing to batch how often driver will post messages. You can think about this as the maximum time to wait for more messages to batch. |
| `SPLUNK_LOGGING_DRIVER_POST_MESSAGES_BATCH_SIZE` | `1000`        | How many messages driver should wait before sending them in one batch.                                                                             |
| `SPLUNK_LOGGING_DRIVER_BUFFER_MAX`               | `10 * 1000`   | If driver cannot connect to remote server, what is the maximum amount of messages it can hold in buffer for retries.                               |
| `SPLUNK_LOGGING_DRIVER_CHANNEL_SIZE`             | `4 * 1000`    | How many pending messages can be in the channel which is used to send messages to background logger worker, which batches them.                    |
