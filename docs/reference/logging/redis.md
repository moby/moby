<!--[metadata]>
+++
title = "Redis logging driver"
description = "Describes how to use the redis logging driver."
keywords = ["redis, docker, logging, driver"]
[menu.main]
parent = "smn_logging"
weight=2
+++
<![end-metadata]-->

# Redis logging driver

The `redis` logging driver sends container logs to a
`redis` key-value store. About `redis` itself, see [the project webpage](http://www.redis.io)
and [its documentation](http://redis.io/documentation). To use this logging driver, start a `redis` server on a host.
The logs are stored in a list. Each log is a string that contains a vaild json.

Besides the log message itself, the `redis` log
driver sends the following metadata:

| Field            | Description                         |
-------------------|-------------------------------------|
| `container`      | The container name at the time it was started. If you use `docker rename` to rename a container, the new name is not reflected in the journal entries. |
| `host`           | The host name the container is run. |
| `attrs`       | All specified labels and env options are stored in this field. |

The `docker logs` command is not available for this logging driver.

## Usage

Configure the default logging driver by passing the
`--log-driver` option to the Docker daemon:

    docker daemon --log-driver=redis --log-opt redis-address=tcp://host.port

To set the logging driver for a specific container, pass the
`--log-driver` option to `docker run`:

    docker run --log-driver=redis --log-opt redis-address=tcp://host.port ...

Before using this logging driver, launch a redis server. Use the
`redis-address` option to connect to the redis server.

    docker run --log-driver=redis --log-opt redis-address=tcp://host.port

If container cannot connect to the redis server, the container stops
immediately.

## Options

Users can use the `--log-opt NAME=VALUE` flag to specify more redis logging driver options.

### redis-address

This option is mandatory. Supply the
`redis-address` option to connect to the redis-server.

    docker run --log-driver=redis --log-opt redis-address=tcp://host.port

### redis-key

Supply the `redis-key` option to specify the key used in redis to store the list of logs. If not provided the redis logging driver will log to the key `docker-logger` by default.

    docker run --log-driver=redis --log-opt redis-key=key

### redis-database

Supply the `redis-database` option to specify a redis database index. You will only need this option if you have databases specified on your redis server.

    docker run --log-driver=redis --log-opt redis-database=index

### redis-password

Supply the `redis-password` option if your redis server is password protected.

    docker run --log-driver=redis --log-opt redis-password=password

### tag

By default, Docker uses the first 12 characters of the container ID to tag log messages.
Refer to the [log tag option documentation](log_tags.md) for customizing
the log tag format.


### labels and env

The `labels` and `env` options each take a comma-separated list of keys. If there is collision between `label` and `env` keys, the value of the `env` takes precedence. Both options add extra fields to the attrs of a logging message.
