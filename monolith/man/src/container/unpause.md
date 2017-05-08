The `docker container unpause` command un-suspends all processes in a container.
On Linux, it does this using the cgroups freezer.

See the [cgroups freezer documentation]
(https://www.kernel.org/doc/Documentation/cgroup-v1/freezer-subsystem.txt) for
further details.
