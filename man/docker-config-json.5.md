% CONFIG.JSON(5) Docker User Manuals
% Docker Community
% JANUARY 2016
# NAME
HOME/.docker/config.json - Default Docker configuration file

# INTRODUCTION

By default, the Docker command line stores its configuration files in a
directory called `.docker` within your `$HOME` directory.  Docker manages most of
the files in the configuration directory and you should not modify them.
However, you *can modify* the `config.json` file to control certain aspects of
how the `docker` command behaves.

Currently, you can modify the `docker` command behavior using environment
variables or command-line options. You can also use options within
`config.json` to modify some of the same behavior. When using these
mechanisms, you must keep in mind the order of precedence among them. Command
line options override environment variables and environment variables override
properties you specify in a `config.json` file.

The `config.json` file stores a JSON encoding of several properties:

* The `HttpHeaders` property specifies a set of headers to include in all messages
sent from the Docker client to the daemon. Docker does not try to interpret or
understand these header; it simply puts them into the messages. Docker does not
allow these headers to change any headers it sets for itself.

* The `psFormat` property specifies the default format for `docker ps` output.
When the `--format` flag is not provided with the `docker ps` command,
Docker's client uses this property. If this property is not set, the client
falls back to the default table format. For a list of supported formatting
directives, see **docker-ps(1)**.

* The `detachKeys` property specifies the default key sequence which
detaches the container. When the `--detach-keys` flag is not provide
with the `docker attach`, `docker exec`, `docker run` or `docker
start`, Docker's client uses this property. If this property is not
set, the client falls back to the default sequence `ctrl-p,ctrl-q`.


* The `imagesFormat` property  specifies the default format for `docker images`
output. When the `--format` flag is not provided with the `docker images`
command, Docker's client uses this property. If this property is not set, the
client falls back to the default table format. For a list of supported
formatting directives, see **docker-images(1)**.

You can specify a different location for the configuration files via the
`DOCKER_CONFIG` environment variable or the `--config` command line option. If
both are specified, then the `--config` option overrides the `DOCKER_CONFIG`
environment variable:

    docker --config ~/testconfigs/ ps

This command instructs Docker to use the configuration files in the
`~/testconfigs/` directory when running the `ps` command.

## Examples

Following is a sample `config.json` file:

    {
      "HttpHeaders": {
        "MyHeader": "MyValue"
      },
      "psFormat": "table {{.ID}}\\t{{.Image}}\\t{{.Command}}\\t{{.Labels}}",
      "imagesFormat": "table {{.ID}}\\t{{.Repository}}\\t{{.Tag}}\\t{{.CreatedAt}}",
      "detachKeys": "ctrl-e,e"
    }

# HISTORY
January 2016, created by Moxiegirl <mary@docker.com>
