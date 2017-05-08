The `docker container pause` command suspends all processes in the specified containers.
On Linux, this uses the cgroups freezer. Traditionally, when suspending a process
the `SIGSTOP` signal is used, which is observable by the process being suspended.
With the cgroups freezer the process is unaware, and unable to capture,
that it is being suspended, and subsequently resumed. On Windows, only Hyper-V
containers can be paused.

See the [cgroups freezer documentation]
(https://www.kernel.org/doc/Documentation/cgroup-v1/freezer-subsystem.txt) for
further details.

**docker-container-unpause(1)** to unpause all processes within a container.
