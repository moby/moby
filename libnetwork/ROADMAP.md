# Roadmap

Libnetwork is a young project and is still being defined.
This document defines the high-level goals of the project and defines the release-relationship to the Docker Platform.

* [Goals](#goals)
* [Project Planning](#project-planning): release-relationship to the Docker Platform.

## Goals

- Combine the networking logic in Docker Engine and libcontainer in to a single, reusable library
- Replace the networking subsystem of Docker Engine, with libnetwork
- Define a flexible model that allows local and remote drivers to provide networking to containers
- Provide a stand-alone tool for using/testing libnetwork

## Project Planning

Libnetwork versions do not map 1:1 with Docker Platform releases.
Milestones and Project Pages are used to define the set of features that are included in each release.

| Platform Version | Libnetwork Version | Planning |
|------------------|--------------------|----------|
| Docker 1.7       | [0.3](https://github.com/docker/libnetwork/milestones/0.3) | [Project Page](https://github.com/docker/libnetwork/wiki/Docker-1.7-Project-Page) |
| Docker 1.8       | [1.0](https://github.com/docker/libnetwork/milestones/1.0) | [Project Page](https://github.com/docker/libnetwork/wiki/Docker-1.8-Project-Page) |
