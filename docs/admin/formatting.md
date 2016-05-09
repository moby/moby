<!--[metadata]>
+++
title = "Format command and log output"
description = "CLI and log output formatting reference"
keywords = ["format, formatting, output, templates, log"]
[menu.main]
parent = "engine_admin"
weight=-90
+++
<![end-metadata]-->

# Formatting reference

Docker uses [Go templates](https://golang.org/pkg/text/template/) to allow users manipulate the output format
of certain commands and log drivers. Each command a driver provides a detailed
list of elements they support in their templates:

- [Docker Images formatting](https://docs.docker.com/engine/reference/commandline/images/#formatting)
- [Docker Inspect formatting](https://docs.docker.com/engine/reference/commandline/inspect/#examples)
- [Docker Log Tag formatting](https://docs.docker.com/engine/admin/logging/log_tags/)
- [Docker Network Inspect formatting](https://docs.docker.com/engine/reference/commandline/network_inspect/)
- [Docker PS formatting](https://docs.docker.com/engine/reference/commandline/ps/#formatting)
- [Docker Volume Inspect formatting](https://docs.docker.com/engine/reference/commandline/volume_inspect/)
- [Docker Version formatting](https://docs.docker.com/engine/reference/commandline/version/#examples)

## Template functions

Docker provides a set of basic functions to manipulate template elements.
This is the complete list of the available functions with examples:

### Join

Join concatenates a list of strings to create a single string.
It puts a separator between each element in the list.

	$ docker ps --format '{{join .Names " or "}}'

### Json

Json encodes an element as a json string.

	$ docker inspect --format '{{json .Mounts}}' container

### Lower

Lower turns a string into its lower case representation.

	$ docker inspect --format "{{lower .Name}}" container

### Split

Split slices a string into a list of strings separated by a separator.

	# docker inspect --format '{{split (join .Names "/") "/"}}' container

### Title

Title capitalizes a string.

	$ docker inspect --format "{{title .Name}}" container

### Upper

Upper turms a string into its upper case representation.

	$ docker inspect --format "{{upper .Name}}" container
