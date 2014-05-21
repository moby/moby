page_title: Share Images via Repositories
page_description: Repositories allow users to share images.
page_keywords: repo, repositories, usage, pull image, push image, image, documentation

# Share Images via Repositories

## Introduction

Docker is not only a tool for creating and managing your own
[*containers*](/terms/container/#container-def) – **Docker is also a
tool for sharing**. A *repository* is a shareable collection of tagged
[*images*](/terms/image/#image-def) that together create the file
systems for containers. The repository's name is a label that indicates
the provenance of the repository, i.e. who created it and where the
original copy is located.

You can find one or more repositories hosted on a *registry*. There are
two types of *registry*: public and private. There's also a default
*registry* that Docker uses which is called
[Docker.io](http://index.docker.io).
[Docker.io](http://index.docker.io) is the home of "top-level"
repositories and public "user" repositories.  The Docker project
provides [Docker.io](http://index.docker.io) to host public and [private
repositories](https://index.docker.io/plans/), namespaced by user. We
provide user authentication and search over all the public repositories.

Docker acts as a client for these services via the `docker search`,
`pull`, `login` and `push` commands.

## Repositories

### Local Repositories

Docker images which have been created and labeled on your local Docker
server need to be pushed to a Public (by default they are pushed to
[Docker.io](http://index.docker.io)) or Private registry to be shared.

### Public Repositories

There are two types of public repositories: *top-level* repositories
which are controlled by the Docker team, and *user* repositories created
by individual contributors. Anyone can read from these repositories –
they really help people get started quickly! You could also use
[*Trusted Builds*](#trusted-builds) if you need to keep control of who
accesses your images.

- Top-level repositories can easily be recognized by **not** having a
  `/` (slash) in their name. These repositories represent trusted images
  provided by the Docker team.
- User repositories always come in the form of `<username>/<repo_name>`.
  This is what your published images will look like if you push to the
  public [Docker.io](http://index.docker.io) registry.
- Only the authenticated user can push to their *username* namespace on
  a [Docker.io](http://index.docker.io) repository.
- User images are not curated, it is therefore up to you whether or not
  you trust the creator of this image.

### Private repositories

You can also create private repositories on
[Docker.io](https://index.docker.io/plans/). These allow you to store
images that you don't want to share publicly. Only authenticated users
can push to private repositories.

## Find Public Images on Docker.io

You can search the [Docker.io](https://index.docker.io) registry or
using the command line interface. Searching can find images by name,
user name or description:

    $ sudo docker help search
    Usage: docker search NAME

    Search the docker index for images

      --no-trunc=false: Don᾿t truncate output
    $ sudo docker search centos
    Found 25 results matching your query ("centos")
    NAME                             DESCRIPTION
    centos
    slantview/centos-chef-solo       CentOS 6.4 with chef-solo.
    ...

There you can see two example results: `centos` and
`slantview/centos-chef-solo`. The second result shows that it comes from
the public repository of a user, `slantview/`, while the first result
(`centos`) doesn't explicitly list a repository so it comes from the
trusted top-level namespace. The `/` character separates a user's
repository and the image name.

Once you have found the image name, you can download it:

    # sudo docker pull <value>
    $ sudo docker pull centos
    Pulling repository centos
    539c0211cd76: Download complete

What can you do with that image? Check out the
[*Examples*](/examples/#example-list) and, when you're ready with your
own image, come back here to learn how to share it.

## Contributing to Docker.io

Anyone can pull public images from the
[Docker.io](http://index.docker.io) registry, but if you would like to
share one of your own images, then you must register a unique user name
first. You can create your username and login on
[Docker.io](https://index.docker.io/account/signup/), or by running

    $ sudo docker login

This will prompt you for a username, which will become a public
namespace for your public repositories.

If your username is available then `docker` will also prompt you to
enter a password and your e-mail address. It will then automatically log
you in. Now you're ready to commit and push your own images!

> **Note:**
> Your authentication credentials will be stored in the [`.dockercfg`
> authentication file](#authentication-file).

## Committing a Container to a Named Image

When you make changes to an existing image, those changes get saved to a
container's file system. You can then promote that container to become
an image by making a `commit`. In addition to converting the container
to an image, this is also your opportunity to name the image,
specifically a name that includes your user name from
[Docker.io](http://index.docker.io) (as you did a `login` above) and a
meaningful name for the image.

    # format is "sudo docker commit <container_id> <username>/<imagename>"
    $ sudo docker commit $CONTAINER_ID myname/kickassapp

## Pushing a repository to its registry

In order to push an repository to its registry you need to have named an
image, or committed your container to a named image (see above)

Now you can push this repository to the registry designated by its name
or tag.

    # format is "docker push <username>/<repo_name>"
    $ sudo docker push myname/kickassapp

## Trusted Builds

Trusted Builds automate the building and updating of images from GitHub
or BitBucket, directly on Docker.io. It works by adding a commit hook to
your selected repository, triggering a build and update when you push a
commit.

### To setup a trusted build

1.  Create a [Docker.io account](https://index.docker.io/) and login.
2.  Link your GitHub or BitBucket account through the [`Link Accounts`](https://index.docker.io/account/accounts/) menu.
3.  [Configure a Trusted build](https://index.docker.io/builds/).
4.  Pick a GitHub or BitBucket project that has a `Dockerfile` that you want to build.
5.  Pick the branch you want to build (the default is the `master` branch).
6.  Give the Trusted Build a name.
7.  Assign an optional Docker tag to the Build.
8.  Specify where the `Dockerfile` is located. The default is `/`.

Once the Trusted Build is configured it will automatically trigger a
build, and in a few minutes, if there are no errors, you will see your
new trusted build on the [Docker.io](https://index.docker.io) Registry.
It will stay in sync with your GitHub and BitBucket repository until you
deactivate the Trusted Build.

If you want to see the status of your Trusted Builds you can go to your
[Trusted Builds page](https://index.docker.io/builds/) on the Docker.io,
and it will show you the status of your builds, and the build history.

Once you've created a Trusted Build you can deactivate or delete it. You
cannot however push to a Trusted Build with the `docker push` command.
You can only manage it by committing code to your GitHub or BitBucket
repository.

You can create multiple Trusted Builds per repository and configure them
to point to specific `Dockerfile`'s or Git branches.

## Private Registry

Private registries are possible by hosting [your own
registry](https://github.com/dotcloud/docker-registry).

> **Note**:
> You can also use private repositories on
> [Docker.io](https://index.docker.io/plans/).

To push or pull to a repository on your own registry, you must prefix
the tag with the address of the registry's host (a `.` or `:` is used to
identify a host), like this:

    # Tag to create a repository with the full registry location.
    # The location (e.g. localhost.localdomain:5000) becomes
    # a permanent part of the repository name
    $ sudo docker tag 0u812deadbeef localhost.localdomain:5000/repo_name

    # Push the new repository to its home location on localhost
    $ sudo docker push localhost.localdomain:5000/repo_name

Once a repository has your registry's host name as part of the tag, you
can push and pull it like any other repository, but it will **not** be
searchable (or indexed at all) on [Docker.io](http://index.docker.io),
and there will be no user name checking performed. Your registry will
function completely independently from the
[Docker.io](http://index.docker.io) registry.

<iframe width="640" height="360" src="//www.youtube.com/embed/CAewZCBT4PI?rel=0" frameborder="0" allowfullscreen></iframe>

See also

[Docker Blog: How to use your own registry](
http://blog.docker.io/2013/07/how-to-use-your-own-registry/)

## Authentication File

The authentication is stored in a JSON file, `.dockercfg`, located in
your home directory. It supports multiple registry URLs.

The `docker login` command will create the:

    [https://index.docker.io/v1/](https://index.docker.io/v1/)

key.

The `docker login https://my-registry.com` command will create the:

    [https://my-registry.com](https://my-registry.com)

key.

For example:

    {
         "https://index.docker.io/v1/": {
                 "auth": "xXxXxXxXxXx=",
                 "email": "email@example.com"
         },
         "https://my-registry.com": {
                 "auth": "XxXxXxXxXxX=",
                 "email": "email@my-registry.com"
         }
    }

The `auth` field represents

    base64(<username>:<password>)

