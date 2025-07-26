Moby Project Roadmap
====================

### How should I use this document?

This document provides description of items that the project decided to prioritize. This should
serve as a reference point for Moby contributors to understand where the project is going, and
help determine if a contribution could be conflicting with some longer term plans.

The fact that a feature isn't listed here doesn't mean that a patch for it will automatically be
refused! We are always happy to receive patches for new cool features we haven't thought about,
or didn't judge to be a priority. Please however understand that such patches might take longer
for us to review.

### How can I help?

Short term objectives are listed in
[Issues](https://github.com/moby/moby/issues?q=is%3Aopen+is%3Aissue+label%3Aroadmap). Our
goal is to split down the workload in such way that anybody can jump in and help. Please comment on
issues if you want to work on it to avoid duplicating effort! Similarly, if a maintainer is already
assigned on an issue you'd like to participate in, pinging him on GitHub to offer your help is
the best way to go.

### How can I add something to the roadmap?

The roadmap process is new to the Moby Project: we are only beginning to structure and document the
project objectives. Our immediate goal is to be more transparent, and work with our community to
focus our efforts on fewer prioritized topics.

We hope to offer in the near future a process allowing anyone to propose a topic to the roadmap, but
we are not quite there yet. For the time being, it is best to discuss with the maintainers on an
issue, in the Slack channel, or in person at the Moby Summits that happen every few months.

# 1. Features and refactoring

## 1.1 Testing

Moby has many tests, both unit and integration. Moby needs more tests which can
cover the full spectrum functionality and edge cases out there.

Tests in the `integration-cli` folder should also be migrated into (both in
location and style) the `integration` folder. These newer tests are simpler to
run in isolation, simpler to read, simpler to write, and more fully exercise the
API. Meanwhile tests of the docker CLI should generally live in docker/cli.

Tracking issues:

- [#32866](https://github.com/moby/moby/issues/32866) Replace integration-cli suite with API test suite

## 1.2 Internal decoupling

Much of the internal code structure, and in particular the
["Daemon"](https://godoc.org/github.com/docker/docker/daemon#Daemon) object,
should be split into smaller, more manageable, and more testable components.
