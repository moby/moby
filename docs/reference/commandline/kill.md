<!--[metadata]>
+++
title = "kill"
description = "The kill command description and usage"
keywords = ["container, kill, signal"]
[menu.main]
parent = "smn_cli"
+++
<![end-metadata]-->

# kill

    Usage: docker kill [OPTIONS] CONTAINER [CONTAINER...]

    Kill a running container using SIGKILL or a specified signal

      --help=false           Print usage
      -s, --signal="KILL"    Signal to send to the container

The main process inside the container will be sent `SIGKILL`, or any
signal specified with option `--signal`.

> **Note:**
> `ENTRYPOINT` and `CMD` in the *shell* form run as a subcommand of `/bin/sh -c`,
> which does not pass signals. This means that the executable is not the container’s PID 1
> and does not receive Unix signals.

If a restart policy is set on the container, the restart policy is disabled until
the next time the container is started.
When using `--signal`, if the signal is considered a "non-fatal" signal, such as
`SIGUSR1`, the restart policy is left in place.

The container's `--stop-signal` is always considered a fatal signal even when it
is normally consided non-fatal, such as `SIGUSR1`.

On Linux fatal signals are:

- SIGABRT
- SIGINT
- SIGKILL
- SIGQUIT
- SIGTERM
