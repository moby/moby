<!--[metadata]>
+++
aliases = ["/engine/articles/configuring/"]
title = "Configuring and running Docker"
description = "Configuring and running the Docker daemon on various distributions"
keywords = ["docker, daemon, configuration, running,  process managers"]
[menu.main]
parent = "engine_admin"
weight = 3
+++
<![end-metadata]-->

# Configuring and running Docker on various distributions

After successfully installing Docker, the `docker` daemon runs with its default
configuration.

In a production environment, system administrators typically configure the
`docker` daemon to start and stop according to an organization's requirements. In most
cases, the system administrator configures a process manager such as `SysVinit`, `Upstart`,
or `systemd` to manage the `docker` daemon's start and stop.

### Running the docker daemon directly

The `docker` daemon can be run directly using the `dockerd` command. By default it listens on
the Unix socket `unix:///var/run/docker.sock`

    $ dockerd

    INFO[0000] +job init_networkdriver()
    INFO[0000] +job serveapi(unix:///var/run/docker.sock)
    INFO[0000] Listening for HTTP on unix (/var/run/docker.sock)
    ...
    ...

### Configuring the docker daemon directly

If you're running the `docker` daemon directly by running `docker daemon` instead
of using a process manager, you can append the configuration options to the `docker` run
command directly. Other options can be passed to the `docker` daemon to configure it.

Some of the daemon's options are:

| Flag                  | Description                                               |
|-----------------------|-----------------------------------------------------------|
| `-D`, `--debug=false` | Enable or disable debug mode. By default, this is false. |
| `-H`,`--host=[]`      | Daemon socket(s) to connect to.                           |
| `--tls=false`         | Enable or disable TLS. By default, this is false.         |


Here is an example of running the `docker` daemon with configuration options:

    $ dockerd -D --tls=true --tlscert=/var/docker/server.pem --tlskey=/var/docker/serverkey.pem -H tcp://192.168.59.3:2376

These options :

- Enable `-D` (debug) mode
- Set `tls` to true with the server certificate and key specified using `--tlscert` and `--tlskey` respectively
- Listen for connections on `tcp://192.168.59.3:2376`

The command line reference has the [complete list of daemon flags](../reference/commandline/dockerd.md)
with explanations.

### Daemon debugging

As noted above, setting the log level of the daemon to "debug" or enabling debug mode
with `-D` allows the administrator or operator to gain much more knowledge about the
runtime activity of the daemon. If faced with a non-responsive daemon, the administrator
can force a full stack trace of all threads to be added to the daemon log by sending the
`SIGUSR1` signal to the Docker daemon. A common way to send this signal is using the `kill`
command on Linux systems. For example, `kill -USR1 <daemon-pid>` sends the `SIGUSR1`
signal to the daemon process, causing the stack dump to be added to the daemon log.

> **Note:** The log level setting of the daemon must be at least "info" level and above for
> the stack trace to be saved to the logfile. By default the daemon's log level is set to
> "info".

The daemon will continue operating after handling the `SIGUSR1` signal and dumping the stack
traces to the log. The stack traces can be used to determine the state of all goroutines and
threads within the daemon.

## Ubuntu

As of `14.04`, Ubuntu uses Upstart as a process manager. By default, Upstart jobs
are located in  `/etc/init` and the `docker` Upstart job can be found at `/etc/init/docker.conf`.

After successfully [installing Docker for Ubuntu](../installation/linux/ubuntulinux.md),
you can check the running status using Upstart in this way:

    $ sudo status docker

    docker start/running, process 989

### Running Docker

You can start/stop/restart the `docker` daemon using

    $ sudo start docker

    $ sudo stop docker

    $ sudo restart docker


### Configuring Docker

The instructions below depict configuring Docker on a system that uses `upstart`
as the process manager. As of Ubuntu 15.04, Ubuntu uses `systemd` as its process
manager. For Ubuntu 15.04 and higher, refer to [control and configure Docker with systemd](systemd.md).

You configure the `docker` daemon in the `/etc/default/docker` file on your
system. You do this by specifying values in a `DOCKER_OPTS` variable.

To configure Docker options:

1. Log into your host as a user with `sudo` or `root` privileges.

2. If you don't have one, create the `/etc/default/docker` file on your host. Depending on how
you installed Docker, you may already have this file.

3. Open the file with your favorite editor.

    ```
    $ sudo vi /etc/default/docker
    ```

4. Add a `DOCKER_OPTS` variable with the following options. These options are appended to the
`docker` daemon's run command.

```
    DOCKER_OPTS="-D --tls=true --tlscert=/var/docker/server.pem --tlskey=/var/docker/serverkey.pem -H tcp://192.168.59.3:2376"
```

These options :

- Enable `-D` (debug) mode
- Set `tls` to true with the server certificate and key specified using `--tlscert` and `--tlskey` respectively
- Listen for connections on `tcp://192.168.59.3:2376`

The command line reference has the [complete list of daemon flags](../reference/commandline/dockerd.md)
with explanations.


5. Save and close the file.

6. Restart the `docker` daemon.

    ```
    $ sudo restart docker
    ```

7. Verify that the `docker` daemon is running as specified with the `ps` command.

    ```
    $ ps aux | grep docker | grep -v grep
    ```

### Logs

By default logs for Upstart jobs are located in `/var/log/upstart` and the logs for `docker` daemon
can be located at `/var/log/upstart/docker.log`

    $ tail -f /var/log/upstart/docker.log
    INFO[0000] Loading containers: done.
    INFO[0000] Docker daemon commit=1b09a95-unsupported graphdriver=aufs version=1.11.0-dev
    INFO[0000] +job acceptconnections()
    INFO[0000] -job acceptconnections() = OK (0)
    INFO[0000] Daemon has completed initialization


## CentOS / Red Hat Enterprise Linux / Fedora

As of `7.x`, CentOS and RHEL use `systemd` as the process manager. As of `21`, Fedora uses
`systemd` as its process manager.

After successfully installing Docker for [CentOS](../installation/linux/centos.md)/[Red Hat Enterprise Linux](../installation/linux/rhel.md)/[Fedora](../installation/linux/fedora.md), you can check the running status in this way:

    $ sudo systemctl status docker

### Running Docker

You can start/stop/restart the `docker` daemon using

    $ sudo systemctl start docker

    $ sudo systemctl stop docker

    $ sudo systemctl restart docker

If you want Docker to start at boot, you should also:

    $ sudo systemctl enable docker

### Configuring Docker

For CentOS 7.x and RHEL 7.x you can [control and configure Docker with systemd](systemd.md).

Previously, for CentOS 6.x and RHEL 6.x you would configure the `docker` daemon in
the `/etc/sysconfig/docker` file on your system. You would do this by specifying
values in a `other_args` variable. For a short time in CentOS 7.x and RHEL 7.x you
would specify values in a `OPTIONS` variable. This is no longer recommended in favor
of using systemd directly.

For this section, we will use CentOS 7.x as an example to configure the `docker` daemon.

To configure Docker options:

1. Log into your host as a user with `sudo` or `root` privileges.

2. Create the `/etc/systemd/system/docker.service.d` directory.

    ```
    $ sudo mkdir /etc/systemd/system/docker.service.d
    ```

3. Create a  `/etc/systemd/system/docker.service.d/docker.conf` file.

4. Open the file with your favorite editor.

    ```
    $ sudo vi /etc/systemd/system/docker.service.d/docker.conf
    ```

5. Override the `ExecStart` configuration from your `docker.service` file to customize
the `docker` daemon. To modify the `ExecStart` configuration you have to specify
an empty configuration followed by a new one as follows:

```
[Service]
ExecStart=
ExecStart=/usr/bin/dockerd -H fd:// -D --tls=true --tlscert=/var/docker/server.pem --tlskey=/var/docker/serverkey.pem -H tcp://192.168.59.3:2376
```

These options :

- Enable `-D` (debug) mode
- Set `tls` to true with the server certificate and key specified using `--tlscert` and `--tlskey` respectively
- Listen for connections on `tcp://192.168.59.3:2376`

The command line reference has the [complete list of daemon flags](../reference/commandline/dockerd.md)
with explanations.

6. Save and close the file.

7. Flush changes.

    ```
    $ sudo systemctl daemon-reload
    ```

8. Restart the `docker` daemon.

    ```
    $ sudo systemctl restart docker
    ```

9. Verify that the `docker` daemon is running as specified with the `ps` command.

    ```
    $ ps aux | grep docker | grep -v grep
    ```

### Logs

systemd has its own logging system called the journal. The logs for the `docker` daemon can
be viewed using `journalctl -u docker`

    $ sudo journalctl -u docker
    May 06 00:22:05 localhost.localdomain systemd[1]: Starting Docker Application Container Engine...
    May 06 00:22:05 localhost.localdomain docker[2495]: time="2015-05-06T00:22:05Z" level="info" msg="+job serveapi(unix:///var/run/docker.sock)"
    May 06 00:22:05 localhost.localdomain docker[2495]: time="2015-05-06T00:22:05Z" level="info" msg="Listening for HTTP on unix (/var/run/docker.sock)"
    May 06 00:22:06 localhost.localdomain docker[2495]: time="2015-05-06T00:22:06Z" level="info" msg="+job init_networkdriver()"
    May 06 00:22:06 localhost.localdomain docker[2495]: time="2015-05-06T00:22:06Z" level="info" msg="-job init_networkdriver() = OK (0)"
    May 06 00:22:06 localhost.localdomain docker[2495]: time="2015-05-06T00:22:06Z" level="info" msg="Loading containers: start."
    May 06 00:22:06 localhost.localdomain docker[2495]: time="2015-05-06T00:22:06Z" level="info" msg="Loading containers: done."
    May 06 00:22:06 localhost.localdomain docker[2495]: time="2015-05-06T00:22:06Z" level="info" msg="Docker daemon commit=1b09a95-unsupported graphdriver=aufs version=1.11.0-dev"
    May 06 00:22:06 localhost.localdomain docker[2495]: time="2015-05-06T00:22:06Z" level="info" msg="+job acceptconnections()"
    May 06 00:22:06 localhost.localdomain docker[2495]: time="2015-05-06T00:22:06Z" level="info" msg="-job acceptconnections() = OK (0)"

_Note: Using and configuring journal is an advanced topic and is beyond the scope of this article._
