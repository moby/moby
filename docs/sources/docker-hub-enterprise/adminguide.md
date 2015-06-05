no_version_dropdown: true
page_title: Docker Hub Enterprise: Admin guide
page_description: Documentation describing administration of Docker Hub Enterprise
page_keywords: docker, documentation, about, technology, hub, enterprise

# Docker Hub Enterprise Administrator's Guide

This guide covers tasks and functions an administrator of Docker Hub Enterprise
(DHE) will need to know about, such as reporting, logging, system management,
performance metrics, etc.
For tasks DHE users need to accomplish, such as using DHE to push and pull
images, please visit the [User's Guide](./userguide).

## Reporting

### System Health

![System Health page</admin/metrics/>](../assets/admin-metrics.png)

The "System Health" tab displays resource utilization metrics for the DHE host
as well as for each of its contained services. The CPU and RAM usage meters at
the top indicate overall resource usage for the host, while detailed time-series
charts are provided below for each service. You can mouse-over the charts or
meters to see detailed data points.

Clicking on a service name (i.e., "load_balancer", "admin_server", etc.) will
display the network, CPU, and memory (RAM) utilization data for the specified
service. See below for a
[detailed explanation of the available services](#services).

### Logs

![System Logs page</admin/logs/>](../assets/admin-logs.png)

Click the "Logs" tab to view all logs related to your DHE instance. You will see
log sections on this page for each service in your DHE instance. Older or newer
logs can be loaded by scrolling up or down. See below for a
[detailed explanation of the available services](#services).

DHE's log files can be found on the host in `/usr/local/etc/dhe/logs/`. The
files are limited to a maximum size of 64mb. They are rotated every two weeks,
when the aggregator sends logs to the collection server, or they are rotated if
a logfile would exceed 64mb without rotation. Log files are named `<component
name>-<timestamp at rotation>`, where the "component name" is the service it
provides (`manager`, `admin-server`, etc.).

### Usage statistics and crash reports

During normal use, DHE generates usage statistics and crash reports. This
information is collected by Docker, Inc. to help us prioritize features, fix
bugs, and improve our products. Specifically, Docker, Inc. collects the
following information:

* Error logs
* Crash logs

## Emergency access to DHE

If your authenticated or public access to the DHE web interface has stopped
working, but your DHE admin container is still running, you can add an
[ambassador container](https://docs.docker.com/articles/ambassador_pattern_linking/)
to get temporary unsecure access to it by running:

    $ docker run --rm -it --link docker_hub_enterprise_admin_server:admin -p 9999:80 svendowideit/ambassador

> **Note:** This guide assumes you can run Docker commands from a machine where
> you are a member of the `docker` group, or have root privileges. Otherwise,
> you may need to add `sudo` to the example command above.

This will give you access on port `9999` on your DHE server - `http://<dhe-host-ip>:9999/admin/`.

## Services

DHE runs several Docker services which are essential to its reliability and
usability. The following services are included; you can see their details by
running queries on the [System Health](#system-health) and [Logs](#logs) pages:

* `admin_server`: Used for displaying system health, performing upgrades,
configuring settings, and viewing logs.
* `load_balancer`: Used for maintaining high availability by distributing load
to each image storage service (`image_storage_X`).
* `log_aggregator`: A microservice used for aggregating logs from each of the
other services. Handles log persistence and rotation on disk.
* `image_storage_X`: Stores Docker images using the [Docker Registry HTTP API V2](https://github.com/docker/distribution/blob/master/doc/SPEC.md). Typically,
multiple image storage services are used in order to provide greater uptime and
faster, more efficient resource utilization.

## DHE system management

The `dockerhubenterprise/manager` image is used to control the DHE system. This
image uses the Docker socket to orchestrate the multiple services that comprise
DHE.

     $ sudo bash -c "$(sudo docker run dockerhubenterprise/manager [COMMAND])"

Supported commands are: `install`, `start`, `stop`, `restart`, `status`, and
`upgrade`.

> **Note**: `sudo` is needed for `dockerhubenterprise/manager` commands to
> ensure that the Bash script is run with full access to the Docker host.

## Next Steps

For information on installing DHE, take a look at the [Installation instructions](./install.md).
