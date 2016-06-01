<!--[metadata]>
+++
title = "Google Cloud Logging driver"
description = "Describes how to use the Google Cloud Logging driver."
keywords = ["gcplogs, google, docker, logging, driver"]
[menu.main]
parent = "smn_logging"
+++
<![end-metadata]-->

# Google Cloud Logging driver

The Google Cloud Logging driver sends container logs to <a href="https://cloud.google.com/logging/docs/" target="_blank">Google Cloud
Logging</a>.

## Usage

You can configure the default logging driver by passing the `--log-driver`
option to the Docker daemon:

    docker daemon --log-driver=gcplogs

You can set the logging driver for a specific container by using the
`--log-driver` option to `docker run`:

    docker run --log-driver=gcplogs ...

This log driver does not implement a reader so it is incompatible with
`docker logs`.

If Docker detects that it is running in a Google Cloud Project, it will discover configuration
from the <a href="https://cloud.google.com/compute/docs/metadata" target="_blank">instance metadata service</a>.
Otherwise, the user must specify which project to log to using the `--gcp-project`
log option and Docker will attempt to obtain credentials from the
<a href="https://developers.google.com/identity/protocols/application-default-credentials" target="_blank">Google Application Default Credential</a>.
The `--gcp-project` takes precedence over information discovered from the metadata server
so a Docker daemon running in a Google Cloud Project can be overriden to log to a different
Google Cloud Project using `--gcp-project`.

## gcplogs options

You can use the `--log-opt NAME=VALUE` flag to specify these additional Google
Cloud Logging driver options:

| Option                      | Required | Description                                                                                                                                 |
|-----------------------------|----------|---------------------------------------------------------------------------------------------------------------------------------------------|
| `gcp-project`               | optional | Which GCP project to log to. Defaults to discovering this value from the GCE metadata service.                                              |
| `gcp-log-cmd`               | optional | Whether to log the command that the container was started with. Defaults to false.                                                          |
| `labels`                    | optional | Comma-separated list of keys of labels, which should be included in message, if these labels are specified for container.                   |
| `env`                       | optional | Comma-separated list of keys of environment variables, which should be included in message, if these variables are specified for container. |

If there is collision between `label` and `env` keys, the value of the `env`
takes precedence. Both options add additional fields to the attributes of a
logging message.

Below is an example of the logging options required to log to the default
logging destination which is discovered by querying the GCE metadata server.

    docker run --log-driver=gcplogs \
        --log-opt labels=location
        --log-opt env=TEST
        --log-opt gcp-log-cmd=true
        --env "TEST=false"
        --label location=west
        your/application

This configuration also directs the driver to include in the payload the label
`location`, the environment variable `ENV`, and the command used to start the
container.
