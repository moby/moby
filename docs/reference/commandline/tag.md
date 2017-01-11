---
title: "tag"
description: "The tag command description and usage"
keywords: "tag, name, image"
---

<!-- This file is maintained within the docker/docker Github
     repository at https://github.com/docker/docker/. Make all
     pull requests against that repo. If you see this file in
     another repository, consider it read-only there, as it will
     periodically be overwritten by the definitive file. Pull
     requests which include edits to this file in other repositories
     will be rejected.
-->

# tag

```markdown
Usage:  docker tag SOURCE_IMAGE[:TAG] TARGET_IMAGE[:TAG]

Create a tag TARGET_IMAGE that refers to SOURCE_IMAGE

Options:
      --help   Print usage
```

An image name is made up of slash-separated name components, optionally prefixed
by a registry hostname. The hostname must comply with standard DNS rules, but
may not contain underscores. If a hostname is present, it may optionally be
followed by a port number in the format `:8080`. If not present, the command
uses Docker's public registry located at `registry-1.docker.io` by default. Name
components may contain lowercase characters, digits and separators. A separator
is defined as a period, one or two underscores, or one or more dashes. A name
component may not start or end with a separator.

A tag name may contain lowercase and uppercase characters, digits, underscores,
periods and dashes. A tag name may not start with a period or a dash and may
contain a maximum of 128 characters.

You can group your images together using names and tags, and then upload them
to [*Share Images via Repositories*](https://docs.docker.com/engine/tutorials/dockerrepos/#/contributing-to-docker-hub).

# Examples

## Tagging an image referenced by ID

To tag a local image with ID "0e5574283393" into the "fedora" repository with
"version1.0":

    docker tag 0e5574283393 fedora/httpd:version1.0

## Tagging an image referenced by Name

To tag a local image with name "httpd" into the "fedora" repository with
"version1.0":

    docker tag httpd fedora/httpd:version1.0

Note that since the tag name is not specified, the alias is created for an
existing local version `httpd:latest`.

## Tagging an image referenced by Name and Tag

To tag a local image with name "httpd" and tag "test" into the "fedora"
repository with "version1.0.test":

    docker tag httpd:test fedora/httpd:version1.0.test

## Tagging an image for a private repository

To push an image to a private registry and not the central Docker
registry you must tag it with the registry hostname and port (if needed).

    docker tag 0e5574283393 myregistryhost:5000/fedora/httpd:version1.0
