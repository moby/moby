<!--[metadata]>
+++
aliases = ["/engine/reference/logging/logentries/"]
title = "Logentries logging driver"
description = "Describes how to use the Logentries logging driver."
keywords = ["logentries, docker, logging, driver"]
[menu.main]
parent = "smn_logging"
weight = 2
+++
<![end-metadata]-->

# Logentries logging driver

The `logentries` logging driver sends container logs to the Logentries server.

## Usage

You can configure the default logging driver by passing the `--log-driver`
option to the Docker daemon:

    docker daemon --log-driver=logentries

You can set the logging driver for a specific container by using the
`--log-driver` option to `docker run`:

    docker run --log-driver=logentries ...

## Logentries options

You can use the `--log-opt NAME=VALUE` flag to specify these additional
Logentries logging driver options:

| Option                      | Required | Description                                                                                                                                                                                                        |
|-----------------------------|----------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `logentries-token`              | required | Logentries token.                                                                                                                                                                                 |

An example usage would be somethig like:

    docker run --log-driver=logentries \
        --log-opt logentries-token=176FCEBF-4CF5-4EDF-91BC-703796522D20 \
        your/application
