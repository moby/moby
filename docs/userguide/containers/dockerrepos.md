<!--[metadata]>
+++
aliases = ["/engine/userguide/dockerrepos/"]
title = "Store images on Docker Hub"
description = "Learn how to use the Docker Hub to manage Docker images and work flow"
keywords = ["repo, Docker Hub, Docker Hub, registry, index, repositories, usage, pull image, push image, image,  documentation"]
[menu.main]
parent = "engine_learn"
+++
<![end-metadata]-->

# Store images on Docker Hub

So far you've learned how to use the command line to run Docker on your local
host. You've learned how to [pull down images](usingdocker.md) to build
containers from existing images and you've learned how to [create your own
images](dockerimages.md).

Next, you're going to learn how to use the [Docker Hub](https://hub.docker.com)
to simplify and enhance your Docker workflows.

The [Docker Hub](https://hub.docker.com) is a public registry maintained by
Docker, Inc. It contains images you can download and use to build
containers. It also provides authentication, work group structure, workflow
tools like webhooks and build triggers, and privacy tools like private
repositories for storing images you don't want to share publicly.

## Docker commands and Docker Hub

Docker itself provides access to Docker Hub services via the `docker search`,
`pull`, `login`, and `push` commands. This page will show you how these commands work.

### Account creation and login
Typically, you'll want to start by creating an account on Docker Hub (if you haven't
already) and logging in. You can create your account directly on
[Docker Hub](https://hub.docker.com/account/signup/).

    $ docker login

You can now commit and push your own images up to your repos on Docker Hub.

> **Note:**
> Your authentication credentials will be stored in the `~/.docker/config.json`
> authentication file in your home directory.

## Searching for images

You can search the [Docker Hub](https://hub.docker.com) registry via its search
interface or by using the command line interface. Searching can find images by image
name, user name, or description:

    $ docker search centos
    NAME           DESCRIPTION                                     STARS     OFFICIAL   AUTOMATED
    centos         The official build of CentOS                    1223      [OK]
    tianon/centos  CentOS 5 and 6, created using rinse instea...   33
    ...

There you can see two example results: `centos` and `tianon/centos`. The second
result shows that it comes from the public repository of a user, named
`tianon/`, while the first result, `centos`, doesn't explicitly list a
repository which means that it comes from the trusted top-level namespace for
[Official Repositories](https://docs.docker.com/docker-hub/official_repos/). The `/` character separates
a user's repository from the image name.

Once you've found the image you want, you can download it with `docker pull <imagename>`:

    $ docker pull centos
    Using default tag: latest
    latest: Pulling from library/centos
    f1b10cd84249: Pull complete
    c852f6d61e65: Pull complete
    7322fbe74aa5: Pull complete
    Digest: sha256:90305c9112250c7e3746425477f1c4ef112b03b4abe78c612e092037bfecc3b7
    Status: Downloaded newer image for centos:latest

You now have an image from which you can run containers.

### Specific Versions or Latest
Using `docker pull centos` is equivalent to using `docker pull centos:latest`.
To pull an image that is not the default latest image you can be more precise
with the image that you want.

For example, to pull version 5 of `centos` use `docker pull centos:centos5`.
In this example, `centos5` is the tag labeling an image in the `centos`
repository for a version of `centos`.

To find a list of tags pointing to currently available versions of a repository
see the [Docker Hub](https://hub.docker.com) registry.

## Contributing to Docker Hub

Anyone can pull public images from the [Docker Hub](https://hub.docker.com)
registry, but if you would like to share your own images, then you must
[register first](https://docs.docker.com/docker-hub/accounts).

## Pushing a repository to Docker Hub

In order to push a repository to its registry, you need to have named an image
or committed your container to a named image as we saw
[here](dockerimages.md).

Now you can push this repository to the registry designated by its name or tag.

    $ docker push yourname/newimage

The image will then be uploaded and available for use by your team-mates and/or the
community.

## Features of Docker Hub

Let's take a closer look at some of the features of Docker Hub. You can find more
information [here](https://docs.docker.com/docker-hub/).

* Private repositories
* Organizations and teams
* Automated Builds
* Webhooks

### Private repositories

Sometimes you have images you don't want to make public and share with
everyone. So Docker Hub allows you to have private repositories. You can
sign up for a plan [here](https://registry.hub.docker.com/plans/).

### Organizations and teams

One of the useful aspects of private repositories is that you can share
them only with members of your organization or team. Docker Hub lets you
create organizations where you can collaborate with your colleagues and
manage private repositories. You can learn how to create and manage an organization
[here](https://registry.hub.docker.com/account/organizations/).

### Automated Builds

Automated Builds automate the building and updating of images from
[GitHub](https://www.github.com) or [Bitbucket](http://bitbucket.com), directly on Docker
Hub. It works by adding a commit hook to your selected GitHub or Bitbucket repository,
triggering a build and update when you push a commit.

#### To setup an Automated Build

1.  Create a [Docker Hub account](https://hub.docker.com/) and login.
2.  Link your GitHub or Bitbucket account through the ["Link Accounts"](https://registry.hub.docker.com/account/accounts/) menu.
3.  [Configure an Automated Build](https://registry.hub.docker.com/builds/add/).
4.  Pick a GitHub or Bitbucket project that has a `Dockerfile` that you want to build.
5.  Pick the branch you want to build (the default is the `master` branch).
6.  Give the Automated Build a name.
7.  Assign an optional Docker tag to the Build.
8.  Specify where the `Dockerfile` is located. The default is `/`.

Once the Automated Build is configured it will automatically trigger a
build and, in a few minutes, you should see your new Automated Build on the [Docker Hub](https://hub.docker.com)
Registry. It will stay in sync with your GitHub and Bitbucket repository until you
deactivate the Automated Build.

To check the output and status of your Automated Build repositories, click on a repository name within the ["Your Repositories" page](https://registry.hub.docker.com/repos/). Automated Builds are indicated by a check-mark icon next to the repository name. Within the repository details page, you may click on the "Build Details" tab to view the status and output of all builds triggered by the Docker Hub.

Once you've created an Automated Build you can deactivate or delete it. You
cannot, however, push to an Automated Build with the `docker push` command.
You can only manage it by committing code to your GitHub or Bitbucket
repository.

You can create multiple Automated Builds per repository and configure them
to point to specific `Dockerfile`'s or Git branches.

#### Build triggers

Automated Builds can also be triggered via a URL on Docker Hub. This
allows you to rebuild an Automated build image on demand.

### Webhooks

Webhooks are attached to your repositories and allow you to trigger an
event when an image or updated image is pushed to the repository. With
a webhook you can specify a target URL and a JSON payload that will be
delivered when the image is pushed.

See the Docker Hub documentation for [more information on
webhooks](https://docs.docker.com/docker-hub/repos/#webhooks)

## Next steps

Go and use Docker!
