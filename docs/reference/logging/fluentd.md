<!--[metadata]>
+++
title = "Fluentd logging driver"
description = "Describes how to use the fluentd logging driver."
keywords = ["Fluentd, docker, logging, driver"]
[menu.main]
parent = "smn_logging"
weight=2
+++
<![end-metadata]-->

# Fluentd logging driver

The `fluentd` logging driver sends container logs to the
[Fluentd](http://www.fluentd.org/) collector as structured log data. Then, users
can use any of the [various output plugins of
Fluentd](http://www.fluentd.org/plugins) to write these logs to various
destinations.

In addition to the log message itself, the `fluentd` log
driver sends the following metadata in the structured log message:

| Field            | Description                         |
-------------------|-------------------------------------|
| `container_id`   | The full 64-character container ID. |
| `container_name` | The container name at the time it was started. If you use `docker rename` to rename a container, the new name is not reflected in the journal entries.                                         |
| `source`         | `stdout` or `stderr`                |

The `docker logs` command is not available for this logging driver.

## Usage

Some options are supported by specifying `--log-opt` as many times as needed:

 - `fluentd-address`: specify `host:port` to connect `localhost:24224`
 - `tag`: specify tag for fluentd message, which interpret some markup, ex `{{.ID}}`, `{{.FullID}}` or `{{.Name}}` `docker.{{.ID}}`


Configure the default logging driver by passing the
`--log-driver` option to the Docker daemon:

    docker --log-driver=fluentd

To set the logging driver for a specific container, pass the
`--log-driver` option to `docker run`:

    docker run --log-driver=fluentd ...

Before using this logging driver, launch a Fluentd daemon. The logging driver
connects to this daemon through `localhost:24224` by default. Use the
`fluentd-address` option to connect to a different address.

    docker run --log-driver=fluentd --log-opt fluentd-address=myhost.local:24224

If container cannot connect to the Fluentd daemon, the container stops
immediately.

## Options

Users can use the `--log-opt NAME=VALUE` flag to specify additional Fluentd logging driver options.

### fluentd-address

By default, the logging driver connects to `localhost:24224`. Supply the
`fluentd-address` option to connect to a different address.

    docker run --log-driver=fluentd --log-opt fluentd-address=myhost.local:24224

### tag

By default, Docker uses the first 12 characters of the container ID to tag log messages.
Refer to the [log tag option documentation](log_tags.md) for customizing
the log tag format.


## Fluentd daemon management with Docker

About `Fluentd` itself, see [the project webpage](http://www.fluentd.org)
and [its documents](http://docs.fluentd.org/).

To use this logging driver, start the `fluentd` daemon on a host. We recommend
that you use [the Fluentd docker
image](https://registry.hub.docker.com/u/fluent/fluentd/). This image is
especially useful if you want to aggregate multiple container logs on a each
host then, later, transfer the logs to another Fluentd node to create an
aggregate store.

### Testing container loggers

1. Write a configuration file (`test.conf`) to dump input logs:

        <source>
          @type forward
        </source>
    
        <match docker.**>
          @type stdout
        </match>

2. Launch Fluentd container with this configuration file:

        $ docker run -it -p 24224:24224 -v /path/to/conf/test.conf:/fluentd/etc -e FLUENTD_CONF=test.conf fluent/fluentd:latest

3. Start one or more containers with the `fluentd` logging driver:

        $ docker run --log-driver=fluentd your/application
