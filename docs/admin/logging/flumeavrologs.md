<!--[metadata]>
+++
aliases = ["/engine/reference/logging/flume/"]
title = "Flume logging driver"
description = "Describes how to use the flume logging driver."
keywords = ["Flume, avro, docker, logging, driver"]
[menu.main]
parent = "smn_logging"
+++
<![end-metadata]-->

# Flume logging driver

The `flume-avor` logging driver send container logs to the [Apache Flume](https://flume.apache.org/) collector as structured log data. Then, isers can use any of the [various sink of Flume](https://flume.apache.org/FlumeUserGuide.html#flume-sinks) to write these logs to various destinations;

In addition to the log message itself, the `flume`log driver sends the following metadata in the structured log message:

| Field            | Description                         |
-------------------|-------------------------------------|
| `container_id`   | The full 64-character container ID. |
| `container_name` | The container name at the time it was started. If you use `docker rename` to rename a container, the new name is not reflected in the journal entries.                                         |
| `source`         | `stdout` or `stderr`                |

The `docker logs` command is not available for this logging driver.

## usage

Some options are supported by specifying `--log-opt` as many times as needed:

- `avro-host`: specify `host` to connect `localhost`
- `avro-port`: specify `port` to connect `63001`

Configure the default logging driver by passing the
`--log-driver` option to the Docker daemon:

    docker daemon --log-driver=flume-avro

To set the logging driver for a specific container, pass the
`--log-driver` option to `docker run`:

    docker run --log-driver=flume-avro ...

Before using this logging driver, launch a Flume daemon. The logging driver connects to this daemon through `localhost:63001` by default. Use the `flume-host=localhost` and `flume-port=63001` option to connect to a different address.


    docker run --log-driver=flume<F4>avro --log-opt flume-host=myhost.local --log-opt flume-port=24224

If container cannot connect to the Flume daemon, the container stops immediately.

## Options

Users can use the `--log-opt NAME=VALUE` flag to specify additional Flume logging driver options.

### avro-host

By default, the logging driver connects to `localhost`. Supply the `avr-host` option to connect to a different address.

### avro-port

By default, the logging driver connects to `63001`. Supply the `avr-port` option to connect to a different port.


## Flume daemon management with Docker

About `Apache Flume` itself, see [the project webpage](http://flume.apache.org) and [its documents](http://flume.apache.org/FlumeUserGuide.html).

To use this logging driver,s tart the `flume` daemon on a host. We recommend that use [the Flume docker image](). This 

## Testing container loggers

1. Write a configuration file (`flume.conf`) to dump input logs:

```
agentLocal.sources = s1
agentLocal.channels = c1
agentLocal.sinks = k1

# Channels
#---------

agentLocal.channels.c1.type = memory

# Source Avro
#------------
agentLocal.sources.s1.type=avro
agentLocal.sources.s1.port=63001
agentLocal.sources.s1.bind=0.0.0.0
agentLocal.sources.s1.channels=c1
agentLocal.sources.s1.byteCapacityBufferPercentage=20
# Configuration de la destination
#--------------------------------
agentLocal.sinks.k1.type = logger
agentLocal.sinks.k1.channel = c1
```


2. Launch Flume container with this configuration file:

       $ docker run -it -p 63001:63001 -v /path/to/conf/test.conf:/var/tmp/flume.conf -e FLUME_CONF_FILE=/var/tmp/flume.conf -e FLUME_AGENT_NAME=agentLocal probablyfine/flume:latest

3. Start one or more containers with the `flume` logging driver:

        $ docker run --log-driver=flume-avro hello-world

