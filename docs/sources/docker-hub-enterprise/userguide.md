page_title: Docker Hub Enterprise: User guide
page_description: Documentation describing basic use of Docker Hub Enterprise
page_keywords: docker, documentation, about, technology, hub, enterprise


# Docker Hub Enterprise User's Guide

This guide covers tasks and functions a user of Docker Hub Enterprise (DHE) will
need to know about, such as pushing or pulling images, etc. For tasks DHE
administrators need to accomplish, such as configuring or monitoring DHE, please
visit the [Administrator's Guide](./adminguide.md).

## Overview

The primary use case for DHE users is to push and pull images to and from the
DHE image storage service. For example, you might pull an Official Image for
Ubuntu from the Docker Hub, customize it with configuration settings for your
infrastructure and then push it to your DHE image storage for other developers
to pull and use for their development environments.

Pushing and pulling images with DHE works very much like any other Docker
registry: you use the `docker pull` command to retrieve images and the `docker
push` command to add an image to a DHE repository. To learn more about Docker
images, see
[User Guide: Working with Docker Images](https://docs.docker.com/userguide/dockerimages/). For a step-by-step
example of the entire process, see the
[Quick Start: Basic Workflow Guide](./quick-start.md).

> **Note**: If your DHE instance has authentication enabled, you will need to
>use your command line to `docker login <dhe-hostname>` (e.g., `docker login
> dhe.yourdomain.com`).
>
> Failures due to unauthenticated `docker push` and `docker pull` commands will
> look like :
>
>     $ docker pull dhe.yourdomain.com/hello-world
>     Pulling repository dhe.yourdomain.com/hello-world
>     FATA[0001] Error: image hello-world:latest not found
>
>     $ docker push dhe.yourdomain.com/hello-world
>     The push refers to a repository [dhe.yourdomain.com/hello-world] (len: 1)
>     e45a5af57b00: Image push failed
>     FATA[0001] Error pushing to registry: token auth attempt for registry
>     https://dhe.yourdomain.com/v2/:
>     https://dhe.yourdomain.com/auth/v2/token/?scope=
>     repository%3Ahello-world%3Apull%2Cpush&service=dhe.yourdomain.com
>     request failed with status: 401 Unauthorized

## Pushing Images

You push an image up to a DHE repository by using the
[`docker push` command](https://docs.docker.com/reference/commandline/cli/#push).

You can add a `tag` to your image so that you can more easily identify it
amongst other variants and so that it refers to your DHE server.

    `$ docker tag hello-world:latest dhe.yourdomain.com/yourusername/hello-mine:latest`

The command labels a `hello-world:latest` image using a new tag in the
`[REGISTRYHOST/][USERNAME/]NAME[:TAG]` format.  The `REGISTRYHOST` in this
case is your DHE server, `dhe.yourdomain.com`, and the `USERNAME` is
`yourusername`. Lastly, the image tag is set to `hello-mine:latest`.

Once an image is tagged, you can push it to DHE with:

    `$ docker push dhe.yourdomain.com/demouser/hello-mine:latest`
    
> **Note**: If the Docker daemon on which you are running `docker push` doesn't
> have the right certificates set up, you will get an error similar to:
>
>     $ docker push dhe.yourdomain.com/demouser/hello-world
>     FATA[0000] Error response from daemon: v1 ping attempt failed with error:
>     Get https://dhe.yourdomain.com/v1/_ping: x509: certificate signed by
>     unknown authority. If this private registry supports only HTTP or HTTPS
>     with an unknown CA certificate, please add `--insecure-registry
>     dhe.yourdomain.com` to the daemon's arguments. In the case of HTTPS, if
>     you have access to the registry's CA certificate, no need for the flag;
>     simply place the CA certificate at
>     /etc/docker/certs.d/dhe.yourdomain.com/ca.crt

## Pulling images

You can retrieve an image with the
[`docker pull` command](https://docs.docker.com/reference/commandline/cli/#run),
or you can retrieve an image and run Docker to build the container with the
[`docker run`command](https://docs.docker.com/reference/commandline/cli/#run).

To retrieve an image from DHE and then run Docker to build the container, add
the needed info to `docker run`:

        $ docker run dhe.yourdomain.com/yourusername/hello-mine
        latest: Pulling from dhe.yourdomain.com/yourusername/hello-mine
        511136ea3c5a: Pull complete
        31cbccb51277: Pull complete
        e45a5af57b00: Already exists
        Digest: sha256:45f0de377f861694517a1440c74aa32eecc3295ea803261d62f950b1b757bed1
        Status: Downloaded newer image for dhe.yourdomain.com/demouser/hello-mine:latest

Note that if you don't specify a version, by default the `latest` version of an
image will be pulled.

If you run `docker images` after this you'll see a `hello-mine` image.

        $ docker images
        REPOSITORY                           TAG     IMAGE ID      CREATED       VIRTUAL SIZE
        dhe.yourdomain.com/yourusername/hello-mine  latest  e45a5af57b00  3 months ago  910 B

To pull an image without building the container, use `docker pull` and specify
your DHE registry by adding it to the command:

     $ docker pull dhe.yourdomain.com/yourusername/hello-mine


## Next Steps

For information on administering DHE, take a look at the
[Administrator's Guide](./adminguide.md).


<!--TODO:

* mention that image aliases that are not in the same repository are not updated - either on push or pull
* but that multiple tags in one repo are pushed if you don't specify the `:tag` (ie, `imagename` does not always mean `imagename:latest`)
* show what happens for non-latest, and when there are more than one tag in a repo
* explain the fully-qualified repo/image name
* explain how to remove an image from DHE -->
