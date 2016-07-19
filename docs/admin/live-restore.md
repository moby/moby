<!--[metadata]>
+++
title = "Keep containers alive during daemon downtime"
description = "How to keep containers running when the daemon isn't available."
keywords = ["docker, upgrade, daemon, dockerd, live-restore, daemonless container"]
[menu.main]
parent = "engine_admin"
weight="6"
+++
<![end-metadata]-->

# Keep containers alive during daemon downtime

By default, when the Docker daemon terminates, it shuts down running containers.
Starting with Docker Engine 1.12, you can configure the daemon so that containers remain
running if the daemon becomes unavailable. The live restore option helps reduce
container downtime due to daemon crashes, planned outages, or upgrades.

## Enable the live restore option

There are two ways to enable the live restore setting to keep containers alive
when the daemon becomes unavailable:

* If the daemon is already running and you don't want to stop it, you can add
the configuration to the daemon configuration file. For example, on a linux
system the default configuration file is `/etc/docker/daemon.json`.

Use your favorite editor to enable the `live-restore` option in the
`daemon.json`.

```bash
{
"live-restore": true
}
```

You have to send a `SIGHUP` signal to the daemon process for it to reload the
configuration. For more information on how to configure the Docker daemon using
config.json, see [daemon configuration file](../reference/commandline/dockerd.md#daemon-configuration-file)

* When you start the Docker daemon, pass the `--live-restore` flag:

    ```bash
    $ sudo dockerd --live-restore
    ```

## Live restore during upgrades

The live restore feature supports restoring containers to the daemon for
upgrades from one minor release to the next. For example from Docker Engine
1.12.1 to 1.13.2.

If you skip releases during an upgrade, the daemon may not restore connection
the containers. If the daemon is unable restore connection, it ignores the
running containers and you must manage them manually. The daemon won't shut down
the disconnected containers.

## Live restore upon restart

The live restore option only works to restore the same set of daemon options
as the daemon had before it stopped. For example, live restore may not work if
the daemon restarts with a different bridge IP or a different graphdriver.

## Impact of live restore on running containers

A lengthy absence of the daemon can impact running containers. The containers
process writes to FIFO logs for daemon consumption. If the daemon is unavailable
to consume the output, the buffer will fill up and block further writes to the
log. A full log blocks the process until further space is available. The default
buffer size is typically 64K.

You must restart Docker to flush the buffers.

You can modify the kernel's buffer size by changing `/proc/sys/fs/pipe-max-size`.

## Live restore and swarm mode

The live restore option is not compatible with Docker Engine swarm mode. When
the Docker Engine runs in swarm mode, the orchestration feature manages tasks
and keeps containers running according to a service specification.
