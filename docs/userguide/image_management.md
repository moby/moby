<!--[metadata]>
+++
alias = [ "/reference/api/hub_registry_spec/"]
title = "Image management"
description = "Documentation for docker Registry and Registry API"
keywords = ["docker, registry, api,  hub"]
[menu.main]
parent="mn_docker_hub"
weight=-1
+++
<![end-metadata]-->

# Image management

The Docker Engine provides a client which you can use to create images on the command line or through a build process. You can run these images in a container or publish them for others to use. Storing the images you create, searching for images you might want, or publishing images others might use are all elements of image management.

This section provides an overview of the major features and products Docker provides for image management.


## Docker Hub

The [Docker Hub](https://docs.docker.com/docker-hub/) is responsible for centralizing information about user accounts, images, and public name spaces. It has different components:

 - Web UI
 - Meta-data store (comments, stars, list public repositories)
 - Authentication service
 - Tokenization

There is only one instance of the Docker Hub, run and managed by Docker Inc. This public Hub is useful for most individuals and smaller companies.

## Docker Registry and the Docker Trusted Registry

The Docker Registry is a component of Docker's ecosystem. A registry is a
storage and content delivery system, holding named Docker images, available in
different tagged versions. For example, the image `distribution/registry`, with
tags `2.0` and `latest`. Users interact with a registry by using docker push and
pull commands such as, `docker pull myregistry.com/stevvooe/batman:voice`.

The Docker Hub has its own registry which, like the Hub itself, is run and managed by Docker. However, there are other ways to obtain a registry. You can purchase the [Docker Trusted Registry](https://docs.docker.com/docker-trusted-registry) product to run on your company's network. Alternatively, you can use the Docker Registry component to build a private registry. For information about using a registry, see overview for the [Docker Registry](https://docs.docker.com/registry).


## Content Trust

When transferring data among networked systems, *trust* is a central concern. In
particular, when communicating over an untrusted medium such as the internet, it
is critical to ensure the integrity and publisher of all of the data a system
operates on. You use Docker to push and pull images (data) to a registry.
Content trust gives you the ability to both verify the integrity and the
publisher of all the data received from a registry over any channel.

[Content trust](../security/trust/) is currently only available for users of the
public Docker Hub. It is currently not available for the Docker Trusted Registry
or for private registries.
