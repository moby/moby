page_title: Working with Docker.io
page_description: Learning how to use Docker.io to manage images and work flow
page_keywords: repo, Docker.io, Docker Hub, registry, index, repositories, usage, pull image, push image, image, documentation

# Working with Docker.io

So far we've seen a lot about how to use Docker on the command line and
your local host. We've seen [how to pull down
images](/userguide/usingdocker/) that you can run your containers from
and we've seen how to [create your own images](/userguide/dockerimages).

Now we're going to learn a bit more about
[Docker.io](https://index.docker.io) and how you can use it to enhance
your Docker work flows.

[Docker.io](https://index.docker.io) is the public registry that Docker
Inc maintains. It contains a huge collection of images, over 15,000,
that you can download and use to build your containers. It also provides
authentication, structure (you can setup teams and organizations), work
flow tools like webhooks and build triggers as well as privacy features
like private repositories for storing images you don't want to publicly
share.

## Docker commands and Docker.io

Docker acts as a client for these services via the `docker search`,
`pull`, `login` and `push` commands.

## Searching for images

As we've already seen we can search the
[Docker.io](https://index.docker.io) registry via it's search interface
or using the command line interface. Searching can find images by name,
user name or description:

    $ sudo docker search centos
    NAME           DESCRIPTION                                     STARS     OFFICIAL   TRUSTED
    centos         Official CentOS 6 Image as of 12 April 2014     88
    tianon/centos  CentOS 5 and 6, created using rinse instea...   21
    ...

There you can see two example results: `centos` and
`tianon/centos`. The second result shows that it comes from
the public repository of a user, `tianon/`, while the first result,
`centos`, doesn't explicitly list a repository so it comes from the
trusted top-level namespace. The `/` character separates a user's
repository and the image name.

Once you have found the image you want, you can download it:

    $ sudo docker pull centos
    Pulling repository centos
    0b443ba03958: Download complete
    539c0211cd76: Download complete
    511136ea3c5a: Download complete
    7064731afe90: Download complete

The image is now available to run a container from.

## Contributing to Docker.io

Anyone can pull public images from the [Docker.io](http://index.docker.io)
registry, but if you would like to share your own images, then you must
register a user first as we saw in the [first section of the Docker User
Guide](/userguide/dockerio/).

To refresh your memory, you can create your user name and login to
[Docker.io](https://index.docker.io/account/signup/), or by running:

    $ sudo docker login

This will prompt you for a user name, which will become a public
namespace for your public repositories, for example:

    training/webapp

Here `training` is the user name and `webapp` is a repository owned by
that user.

If your user name is available then `docker` will also prompt you to
enter a password and your e-mail address. It will then automatically log
you in. Now you're ready to commit and push your own images!

> **Note:**
> Your authentication credentials will be stored in the [`.dockercfg`
> authentication file](#authentication-file) in your home directory.

## Pushing a repository to Docker.io

In order to push an repository to its registry you need to have named an image,
or committed your container to a named image as we saw
[here](/userguide/dockerimages).

Now you can push this repository to the registry designated by its name
or tag.

    $ sudo docker push yourname/newimage

The image will then be uploaded and available for use.

## Features of Docker.io

Now let's look at some of the features of Docker.io. You can find more
information [here](/docker-io/).

* Private repositories
* Organizations and teams
* Automated Builds
* Webhooks

## Private Repositories

Sometimes you have images you don't want to make public and share with
everyone. So Docker.io allows you to have private repositories. You can
sign up for a plan [here](https://index.docker.io/plans/).

## Organizations and teams

One of the useful aspects of private repositories is that you can share
them only with members of your organization or team. Docker.io lets you
create organizations where you can collaborate with your colleagues and
manage private repositories. You can create and manage an organization
[here](https://index.docker.io/account/organizations/).

## Automated Builds

Automated Builds automate the building and updating of images from [GitHub](https://www.github.com)
or [BitBucket](http://bitbucket.com), directly on Docker.io. It works by adding a commit hook to
your selected GitHub or BitBucket repository, triggering a build and update when you push a
commit.

### To setup an Automated Build

1.  Create a [Docker.io account](https://index.docker.io/) and login.
2.  Link your GitHub or BitBucket account through the [`Link Accounts`](https://index.docker.io/account/accounts/) menu.
3.  [Configure an Automated Build](https://index.docker.io/builds/).
4.  Pick a GitHub or BitBucket project that has a `Dockerfile` that you want to build.
5.  Pick the branch you want to build (the default is the `master` branch).
6.  Give the Automated Build a name.
7.  Assign an optional Docker tag to the Build.
8.  Specify where the `Dockerfile` is located. The default is `/`.

Once the Automated Build is configured it will automatically trigger a
build, and in a few minutes, if there are no errors, you will see your
new Automated Build on the [Docker.io](https://index.docker.io) Registry.
It will stay in sync with your GitHub and BitBucket repository until you
deactivate the Automated Build.

If you want to see the status of your Automated Builds you can go to your
[Automated Builds page](https://index.docker.io/builds/) on the Docker.io,
and it will show you the status of your builds, and the build history.

Once you've created an Automated Build you can deactivate or delete it. You
cannot however push to an Automated Build with the `docker push` command.
You can only manage it by committing code to your GitHub or BitBucket
repository.

You can create multiple Automated Builds per repository and configure them
to point to specific `Dockerfile`'s or Git branches.

### Build Triggers

Automated Builds can also be triggered via a URL on Docker.io. This
allows you to rebuild an Automated build image on demand.

## Webhooks

Webhooks are attached to your repositories and allow you to trigger an
event when an image or updated image is pushed to the repository. With
a webhook you can specify a target URL and a JSON payload will be
delivered when the image is pushed.

## Next steps

Go and use Docker!

