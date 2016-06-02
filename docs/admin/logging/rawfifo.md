<!--[metadata]>
+++
aliases = ["/engine/reference/logging/rawfifo/"]
title = "Raw FIFO logging driver"
description = "Describes how to use the Raw FIFO logging driver."
keywords = ["logging, driver"]
[menu.main]
parent = "smn_logging"
+++
<![end-metadata]-->

# Raw FIFO logging driver

The `rawfifo` logging driver sends raw container logs to UNIX named pipes.

Users can implement their own logging driver plugin by reading from the pipes.
The output from the container can be buffered up to the number of bytes allowed
by the system. (see also `fcntl(2)` and `proc(5)`).
The output is blocked if the buffer is full.

The driver does not support [tags](log_tags.md).

## Usage

You can set the logging driver for a specific container by using the
`--log-driver` option to `docker run`:

    docker run --log-driver=rawfifo ...

## Raw FIFO options

You can use the `--log-opt NAME=VALUE` flag to specify Raw FIFO logging driver
options.

### rawfifo-dir

The `rawfifo` logging driver sends your Docker logs to FIFOs named `stdout` and 
`stderr` under a specific directory. Use the `rawfifo-dir` log option to set the
directory.
Currently, the driver has no default value for `rawfifo-dir` and specifying the 
option is mandatory.

    docker run --log-driver=rawfifo --log-opt rawfifo-dir=/tmp/log1 ...

Since the driver does not support [tags](log_tags.md), it is recommended to
specify the directory which path contains the container name and other
information instead.

    docker run --log-driver=rawfifo --log-opt rawfifo-dir=/tmp/alpine/c1 --name c1 alpine

