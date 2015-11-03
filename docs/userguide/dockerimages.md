<!--[metadata]>
+++
title = "Build your own images"
description = "How to work with Docker images."
keywords = ["documentation, docs, the docker guide, docker guide, docker, docker platform, virtualization framework, docker.io, Docker images, Docker image, image management, Docker repos, Docker repositories, docker, docker tag, docker tags, Docker Hub,  collaboration"]
[menu.main]
parent = "smn_containers"
weight = -4
+++
<![end-metadata]-->

# Build your own images

Docker images are the basis of containers. Each time you've used `docker run`
you told it which image you wanted. In the previous sections of the guide you
used Docker images that already exist, for example the `ubuntu` image and the
`training/webapp` image.

You also discovered that Docker stores downloaded images on the Docker host. If
an image isn't already present on the host then it'll be downloaded from a
registry: by default the [Docker Hub Registry](https://registry.hub.docker.com).

In this section you're going to explore Docker images a bit more
including:

* Managing and working with images locally on your Docker host.
* Creating basic images.
* Uploading images to [Docker Hub Registry](https://registry.hub.docker.com).

## Listing images on the host

Let's start with listing the images you have locally on our host. You can
do this using the `docker images` command like so:

    $ docker images
    REPOSITORY          TAG                 IMAGE ID            CREATED             VIRTUAL SIZE
    ubuntu              14.04               1d073211c498        3 days ago          187.9 MB
    busybox             latest              2c5ac3f849df        5 days ago          1.113 MB
    training/webapp     latest              54bb4e8718e8        5 months ago        348.7 MB

You can see the images you've previously used in the user guide.
Each has been downloaded from [Docker Hub](https://hub.docker.com) when you
launched a container using that image. When you list images, you get three crucial pieces of information in the listing.

* What repository they came from, for example `ubuntu`.
* The tags for each image, for example `14.04`.
* The image ID of each image.

> **Tip:**
> You can use [a third-party dockviz tool](https://github.com/justone/dockviz)
> or the [Image layers site](https://imagelayers.io/) to display  
> visualizations of image data.

A repository potentially holds multiple variants of an image. In the case of
our `ubuntu` image you can see multiple variants covering Ubuntu 10.04, 12.04,
12.10, 13.04, 13.10 and 14.04. Each variant is identified by a tag and you can
refer to a tagged image like so:

    ubuntu:14.04

So when you run a container you refer to a tagged image like so:

    $ docker run -t -i ubuntu:14.04 /bin/bash

If instead you wanted to run an Ubuntu 12.04 image you'd use:

    $ docker run -t -i ubuntu:12.04 /bin/bash

If you don't specify a variant, for example you just use `ubuntu`, then Docker
will default to using the `ubuntu:latest` image.

> **Tip:**
> You recommend you always use a specific tagged image, for example
> `ubuntu:12.04`. That way you always know exactly what variant of an image is
> being used.

## Getting a new image

So how do you get new images? Well Docker will automatically download any image
you use that isn't already present on the Docker host. But this can potentially
add some time to the launch of a container. If you want to pre-load an image you
can download it using the `docker pull` command. Suppose you'd like to
download the `centos` image.

    $ docker pull centos
    Pulling repository centos
    b7de3133ff98: Pulling dependent layers
    5cc9e91966f7: Pulling fs layer
    511136ea3c5a: Download complete
    ef52fb1fe610: Download complete
    . . .

    Status: Downloaded newer image for centos

You can see that each layer of the image has been pulled down and now you
can run a container from this image and you won't have to wait to
download the image.

    $ docker run -t -i centos /bin/bash
    bash-4.1#

## Finding images

One of the features of Docker is that a lot of people have created Docker
images for a variety of purposes. Many of these have been uploaded to
[Docker Hub](https://hub.docker.com). You can search these images on the
[Docker Hub](https://hub.docker.com) website.

![indexsearch](search.png)

You can also search for images on the command line using the `docker search`
command. Suppose your team wants an image with Ruby and Sinatra installed on
which to do our web application development. You can search for a suitable image
by using the `docker search` command to find all the images that contain the
term `sinatra`.

    $ docker search sinatra
    NAME                                   DESCRIPTION                                     STARS     OFFICIAL   AUTOMATED
    training/sinatra                       Sinatra training image                          0                    [OK]
    marceldegraaf/sinatra                  Sinatra test app                                0
    mattwarren/docker-sinatra-demo                                                         0                    [OK]
    luisbebop/docker-sinatra-hello-world                                                   0                    [OK]
    bmorearty/handson-sinatra              handson-ruby + Sinatra for Hands on with D...   0
    subwiz/sinatra                                                                         0
    bmorearty/sinatra                                                                      0
    . . .

You can see the command returns a lot of images that use the term `sinatra`.
You've received a list of image names, descriptions, Stars (which measure the
social popularity of images - if a user likes an image then they can "star" it),
and the Official and Automated build statuses. [Official
Repositories](https://docs.docker.com/docker-hub/official_repos) are a carefully
curated set of Docker repositories supported by Docker, Inc.  Automated
repositories are [Automated Builds](dockerrepos.md#automated-builds) that allow
you to validate the source and content of an image.

You've reviewed the images available to use and you decided to use the
`training/sinatra` image. So far you've seen two types of images repositories,
images like `ubuntu`, which are called base or root images. These base images
are provided by Docker Inc and are built, validated and supported. These can be
identified by their single word names.

You've also seen user images, for example the `training/sinatra` image you've
chosen. A user image belongs to a member of the Docker community and is built
and maintained by them.  You can identify user images as they are always
prefixed with the user name, here `training`, of the user that created them.

## Pulling our image

You've identified a suitable image, `training/sinatra`, and now you can download it using the `docker pull` command.

    $ docker pull training/sinatra

The team can now use this image by running their own containers.

    $ docker run -t -i training/sinatra /bin/bash
    root@a8cb6ce02d85:/#

## Creating our own images

The team has found the `training/sinatra` image pretty useful but it's not quite
what they need and you need to make some changes to it. There are two ways you
can update and create images.

1. You can update a container created from an image and commit the results to an image.
2. You can use a `Dockerfile` to specify instructions to create an image.


### Updating and committing an image

To update an image you first need to create a container from the image
you'd like to update.

    $ docker run -t -i training/sinatra /bin/bash
    root@0b2616b0e5a8:/#

> **Note:**
> Take note of the container ID that has been created, `0b2616b0e5a8`, as you'll
> need it in a moment.

Inside our running container let's add the `json` gem.

    root@0b2616b0e5a8:/# gem install json

Once this has completed let's exit our container using the `exit`
command.

Now you have a container with the change you want to make. You can then
commit a copy of this container to an image using the `docker commit`
command.

    $ docker commit -m "Added json gem" -a "Kate Smith" \
    0b2616b0e5a8 ouruser/sinatra:v2
    4f177bd27a9ff0f6dc2a830403925b5360bfe0b93d476f7fc3231110e7f71b1c

Here you've used the `docker commit` command. You've specified two flags: `-m`
and `-a`. The `-m` flag allows us to specify a commit message, much like you
would with a commit on a version control system. The `-a` flag allows us to
specify an author for our update.

You've also specified the container you want to create this new image from,
`0b2616b0e5a8` (the ID you recorded earlier) and you've specified a target for
the image:

    ouruser/sinatra:v2

Break this target down. It consists of a new user, `ouruser`, that you're
writing this image to. You've also specified the name of the image, here you're
keeping the original image name `sinatra`. Finally you're specifying a tag for
the image: `v2`.

You can then look at our new `ouruser/sinatra` image using the `docker images`
command.

    $ docker images
    REPOSITORY          TAG     IMAGE ID       CREATED       VIRTUAL SIZE
    training/sinatra    latest  5bc342fa0b91   10 hours ago  446.7 MB
    ouruser/sinatra     v2      3c59e02ddd1a   10 hours ago  446.7 MB
    ouruser/sinatra     latest  5db5f8471261   10 hours ago  446.7 MB

To use our new image to create a container you can then:

    $ docker run -t -i ouruser/sinatra:v2 /bin/bash
    root@78e82f680994:/#

### Building an image from a `Dockerfile`

Using the `docker commit` command is a pretty simple way of extending an image
but it's a bit cumbersome and it's not easy to share a development process for
images amongst a team. Instead you can use a new command, `docker build`, to
build new images from scratch.

To do this you create a `Dockerfile` that contains a set of instructions that
tell Docker how to build our image.

First, create a directory and a `Dockerfile`.

    $ mkdir sinatra
    $ cd sinatra
    $ touch Dockerfile

If you are using Docker Machine on Windows, you may access your host
directory by `cd` to `/c/Users/your_user_name`.

Each instruction creates a new layer of the image. Try a simple example now for
building your own Sinatra image for your fictitious development team.

    # This is a comment
    FROM ubuntu:14.04
    MAINTAINER Kate Smith <ksmith@example.com>
    RUN apt-get update && apt-get install -y ruby ruby-dev
    RUN gem install sinatra

Examine what your `Dockerfile` does. Each instruction prefixes a statement and
is capitalized.

    INSTRUCTION statement

> **Note:** You use `#` to indicate a comment

The first instruction `FROM` tells Docker what the source of our image is, in
this case you're basing our new image on an Ubuntu 14.04 image. The instruction uses the `MAINTAINER` instruction to specify who maintains the new image.

Lastly, you've specified two `RUN` instructions. A `RUN` instruction executes
a command inside the image, for example installing a package. Here you're
updating our APT cache, installing Ruby and RubyGems and then installing the
Sinatra gem.



Now let's take our `Dockerfile` and use the `docker build` command to build an image.

    $ docker build -t ouruser/sinatra:v2 .
    Sending build context to Docker daemon 2.048 kB
    Sending build context to Docker daemon
    Step 1 : FROM ubuntu:14.04
     ---> e54ca5efa2e9
    Step 2 : MAINTAINER Kate Smith <ksmith@example.com>
     ---> Using cache
     ---> 851baf55332b
    Step 3 : RUN apt-get update && apt-get install -y ruby ruby-dev
     ---> Running in 3a2558904e9b
    Selecting previously unselected package libasan0:amd64.
    (Reading database ... 11518 files and directories currently installed.)
    Preparing to unpack .../libasan0_4.8.2-19ubuntu1_amd64.deb ...
    Unpacking libasan0:amd64 (4.8.2-19ubuntu1) ...
    Selecting previously unselected package libatomic1:amd64.
    Preparing to unpack .../libatomic1_4.8.2-19ubuntu1_amd64.deb ...
    Unpacking libatomic1:amd64 (4.8.2-19ubuntu1) ...
    Selecting previously unselected package libgmp10:amd64.
    Preparing to unpack .../libgmp10_2%3a5.1.3+dfsg-1ubuntu1_amd64.deb ...
    Unpacking libgmp10:amd64 (2:5.1.3+dfsg-1ubuntu1) ...
    Selecting previously unselected package libisl10:amd64.
    Preparing to unpack .../libisl10_0.12.2-1_amd64.deb ...
    Unpacking libisl10:amd64 (0.12.2-1) ...
    Selecting previously unselected package libcloog-isl4:amd64.
    Preparing to unpack .../libcloog-isl4_0.18.2-1_amd64.deb ...
    Unpacking libcloog-isl4:amd64 (0.18.2-1) ...
    Selecting previously unselected package libgomp1:amd64.
    Preparing to unpack .../libgomp1_4.8.2-19ubuntu1_amd64.deb ...
    Unpacking libgomp1:amd64 (4.8.2-19ubuntu1) ...
    Selecting previously unselected package libitm1:amd64.
    Preparing to unpack .../libitm1_4.8.2-19ubuntu1_amd64.deb ...
    Unpacking libitm1:amd64 (4.8.2-19ubuntu1) ...
    Selecting previously unselected package libmpfr4:amd64.
    Preparing to unpack .../libmpfr4_3.1.2-1_amd64.deb ...
    Unpacking libmpfr4:amd64 (3.1.2-1) ...
    Selecting previously unselected package libquadmath0:amd64.
    Preparing to unpack .../libquadmath0_4.8.2-19ubuntu1_amd64.deb ...
    Unpacking libquadmath0:amd64 (4.8.2-19ubuntu1) ...
    Selecting previously unselected package libtsan0:amd64.
    Preparing to unpack .../libtsan0_4.8.2-19ubuntu1_amd64.deb ...
    Unpacking libtsan0:amd64 (4.8.2-19ubuntu1) ...
    Selecting previously unselected package libyaml-0-2:amd64.
    Preparing to unpack .../libyaml-0-2_0.1.4-3ubuntu3_amd64.deb ...
    Unpacking libyaml-0-2:amd64 (0.1.4-3ubuntu3) ...
    Selecting previously unselected package libmpc3:amd64.
    Preparing to unpack .../libmpc3_1.0.1-1ubuntu1_amd64.deb ...
    Unpacking libmpc3:amd64 (1.0.1-1ubuntu1) ...
    Selecting previously unselected package openssl.
    Preparing to unpack .../openssl_1.0.1f-1ubuntu2.4_amd64.deb ...
    Unpacking openssl (1.0.1f-1ubuntu2.4) ...
    Selecting previously unselected package ca-certificates.
    Preparing to unpack .../ca-certificates_20130906ubuntu2_all.deb ...
    Unpacking ca-certificates (20130906ubuntu2) ...
    Selecting previously unselected package manpages.
    Preparing to unpack .../manpages_3.54-1ubuntu1_all.deb ...
    Unpacking manpages (3.54-1ubuntu1) ...
    Selecting previously unselected package binutils.
    Preparing to unpack .../binutils_2.24-5ubuntu3_amd64.deb ...
    Unpacking binutils (2.24-5ubuntu3) ...
    Selecting previously unselected package cpp-4.8.
    Preparing to unpack .../cpp-4.8_4.8.2-19ubuntu1_amd64.deb ...
    Unpacking cpp-4.8 (4.8.2-19ubuntu1) ...
    Selecting previously unselected package cpp.
    Preparing to unpack .../cpp_4%3a4.8.2-1ubuntu6_amd64.deb ...
    Unpacking cpp (4:4.8.2-1ubuntu6) ...
    Selecting previously unselected package libgcc-4.8-dev:amd64.
    Preparing to unpack .../libgcc-4.8-dev_4.8.2-19ubuntu1_amd64.deb ...
    Unpacking libgcc-4.8-dev:amd64 (4.8.2-19ubuntu1) ...
    Selecting previously unselected package gcc-4.8.
    Preparing to unpack .../gcc-4.8_4.8.2-19ubuntu1_amd64.deb ...
    Unpacking gcc-4.8 (4.8.2-19ubuntu1) ...
    Selecting previously unselected package gcc.
    Preparing to unpack .../gcc_4%3a4.8.2-1ubuntu6_amd64.deb ...
    Unpacking gcc (4:4.8.2-1ubuntu6) ...
    Selecting previously unselected package libc-dev-bin.
    Preparing to unpack .../libc-dev-bin_2.19-0ubuntu6_amd64.deb ...
    Unpacking libc-dev-bin (2.19-0ubuntu6) ...
    Selecting previously unselected package linux-libc-dev:amd64.
    Preparing to unpack .../linux-libc-dev_3.13.0-30.55_amd64.deb ...
    Unpacking linux-libc-dev:amd64 (3.13.0-30.55) ...
    Selecting previously unselected package libc6-dev:amd64.
    Preparing to unpack .../libc6-dev_2.19-0ubuntu6_amd64.deb ...
    Unpacking libc6-dev:amd64 (2.19-0ubuntu6) ...
    Selecting previously unselected package ruby.
    Preparing to unpack .../ruby_1%3a1.9.3.4_all.deb ...
    Unpacking ruby (1:1.9.3.4) ...
    Selecting previously unselected package ruby1.9.1.
    Preparing to unpack .../ruby1.9.1_1.9.3.484-2ubuntu1_amd64.deb ...
    Unpacking ruby1.9.1 (1.9.3.484-2ubuntu1) ...
    Selecting previously unselected package libruby1.9.1.
    Preparing to unpack .../libruby1.9.1_1.9.3.484-2ubuntu1_amd64.deb ...
    Unpacking libruby1.9.1 (1.9.3.484-2ubuntu1) ...
    Selecting previously unselected package manpages-dev.
    Preparing to unpack .../manpages-dev_3.54-1ubuntu1_all.deb ...
    Unpacking manpages-dev (3.54-1ubuntu1) ...
    Selecting previously unselected package ruby1.9.1-dev.
    Preparing to unpack .../ruby1.9.1-dev_1.9.3.484-2ubuntu1_amd64.deb ...
    Unpacking ruby1.9.1-dev (1.9.3.484-2ubuntu1) ...
    Selecting previously unselected package ruby-dev.
    Preparing to unpack .../ruby-dev_1%3a1.9.3.4_all.deb ...
    Unpacking ruby-dev (1:1.9.3.4) ...
    Setting up libasan0:amd64 (4.8.2-19ubuntu1) ...
    Setting up libatomic1:amd64 (4.8.2-19ubuntu1) ...
    Setting up libgmp10:amd64 (2:5.1.3+dfsg-1ubuntu1) ...
    Setting up libisl10:amd64 (0.12.2-1) ...
    Setting up libcloog-isl4:amd64 (0.18.2-1) ...
    Setting up libgomp1:amd64 (4.8.2-19ubuntu1) ...
    Setting up libitm1:amd64 (4.8.2-19ubuntu1) ...
    Setting up libmpfr4:amd64 (3.1.2-1) ...
    Setting up libquadmath0:amd64 (4.8.2-19ubuntu1) ...
    Setting up libtsan0:amd64 (4.8.2-19ubuntu1) ...
    Setting up libyaml-0-2:amd64 (0.1.4-3ubuntu3) ...
    Setting up libmpc3:amd64 (1.0.1-1ubuntu1) ...
    Setting up openssl (1.0.1f-1ubuntu2.4) ...
    Setting up ca-certificates (20130906ubuntu2) ...
    debconf: unable to initialize frontend: Dialog
    debconf: (TERM is not set, so the dialog frontend is not usable.)
    debconf: falling back to frontend: Readline
    debconf: unable to initialize frontend: Readline
    debconf: (This frontend requires a controlling tty.)
    debconf: falling back to frontend: Teletype
    Setting up manpages (3.54-1ubuntu1) ...
    Setting up binutils (2.24-5ubuntu3) ...
    Setting up cpp-4.8 (4.8.2-19ubuntu1) ...
    Setting up cpp (4:4.8.2-1ubuntu6) ...
    Setting up libgcc-4.8-dev:amd64 (4.8.2-19ubuntu1) ...
    Setting up gcc-4.8 (4.8.2-19ubuntu1) ...
    Setting up gcc (4:4.8.2-1ubuntu6) ...
    Setting up libc-dev-bin (2.19-0ubuntu6) ...
    Setting up linux-libc-dev:amd64 (3.13.0-30.55) ...
    Setting up libc6-dev:amd64 (2.19-0ubuntu6) ...
    Setting up manpages-dev (3.54-1ubuntu1) ...
    Setting up libruby1.9.1 (1.9.3.484-2ubuntu1) ...
    Setting up ruby1.9.1-dev (1.9.3.484-2ubuntu1) ...
    Setting up ruby-dev (1:1.9.3.4) ...
    Setting up ruby (1:1.9.3.4) ...
    Setting up ruby1.9.1 (1.9.3.484-2ubuntu1) ...
    Processing triggers for libc-bin (2.19-0ubuntu6) ...
    Processing triggers for ca-certificates (20130906ubuntu2) ...
    Updating certificates in /etc/ssl/certs... 164 added, 0 removed; done.
    Running hooks in /etc/ca-certificates/update.d....done.
     ---> c55c31703134
    Removing intermediate container 3a2558904e9b
    Step 4 : RUN gem install sinatra
     ---> Running in 6b81cb6313e5
    unable to convert "\xC3" to UTF-8 in conversion from ASCII-8BIT to UTF-8 to US-ASCII for README.rdoc, skipping
    unable to convert "\xC3" to UTF-8 in conversion from ASCII-8BIT to UTF-8 to US-ASCII for README.rdoc, skipping
    Successfully installed rack-1.5.2
    Successfully installed tilt-1.4.1
    Successfully installed rack-protection-1.5.3
    Successfully installed sinatra-1.4.5
    4 gems installed
    Installing ri documentation for rack-1.5.2...
    Installing ri documentation for tilt-1.4.1...
    Installing ri documentation for rack-protection-1.5.3...
    Installing ri documentation for sinatra-1.4.5...
    Installing RDoc documentation for rack-1.5.2...
    Installing RDoc documentation for tilt-1.4.1...
    Installing RDoc documentation for rack-protection-1.5.3...
    Installing RDoc documentation for sinatra-1.4.5...
     ---> 97feabe5d2ed
    Removing intermediate container 6b81cb6313e5
    Successfully built 97feabe5d2ed

You've specified our `docker build` command and used the `-t` flag to identify
our new image as belonging to the user `ouruser`, the repository name `sinatra`
and given it the tag `v2`.

You've also specified the location of our `Dockerfile` using the `.` to
indicate a `Dockerfile` in the current directory.

> **Note:**
> You can also specify a path to a `Dockerfile`.

Now you can see the build process at work. The first thing Docker does is
upload the build context: basically the contents of the directory you're
building in. This is done because the Docker daemon does the actual
build of the image and it needs the local context to do it.

Next you can see each instruction in the `Dockerfile` being executed
step-by-step. You can see that each step creates a new container, runs
the instruction inside that container and then commits that change -
just like the `docker commit` work flow you saw earlier. When all the
instructions have executed you're left with the `97feabe5d2ed` image
(also helpfully tagged as `ouruser/sinatra:v2`) and all intermediate
containers will get removed to clean things up.

> **Note:**
> An image can't have more than 127 layers regardless of the storage driver.
> This limitation is set globally to encourage optimization of the overall
> size of images.

You can then create a container from our new image.

    $ docker run -t -i ouruser/sinatra:v2 /bin/bash
    root@8196968dac35:/#

> **Note:**
> This is just a brief introduction to creating images. We've
> skipped a whole bunch of other instructions that you can use. We'll see more of
> those instructions in later sections of the Guide or you can refer to the
> [`Dockerfile`](../reference/builder.md) reference for a
> detailed description and examples of every instruction.
> To help you write a clear, readable, maintainable `Dockerfile`, you've also
> written a [`Dockerfile` Best Practices guide](../articles/dockerfile_best-practices.md).


## Setting tags on an image

You can also add a tag to an existing image after you commit or build it. We
can do this using the `docker tag` command. Now, add a new tag to your
`ouruser/sinatra` image.

    $ docker tag 5db5f8471261 ouruser/sinatra:devel

The `docker tag` command takes the ID of the image, here `5db5f8471261`, and our
user name, the repository name and the new tag.

Now, see your new tag using the `docker images` command.

    $ docker images ouruser/sinatra
    REPOSITORY          TAG     IMAGE ID      CREATED        VIRTUAL SIZE
    ouruser/sinatra     latest  5db5f8471261  11 hours ago   446.7 MB
    ouruser/sinatra     devel   5db5f8471261  11 hours ago   446.7 MB
    ouruser/sinatra     v2      5db5f8471261  11 hours ago   446.7 MB

## Image Digests

Images that use the v2 or later format have a content-addressable identifier
called a `digest`. As long as the input used to generate the image is
unchanged, the digest value is predictable. To list image digest values, use
the `--digests` flag:

    $ docker images --digests | head
    REPOSITORY                         TAG                 DIGEST                                                                     IMAGE ID            CREATED             VIRTUAL SIZE
    ouruser/sinatra                    latest              sha256:cbbf2f9a99b47fc460d422812b6a5adff7dfee951d8fa2e4a98caa0382cfbdbf    5db5f8471261        11 hours ago        446.7 MB

When pushing or pulling to a 2.0 registry, the `push` or `pull` command
output includes the image digest. You can `pull` using a digest value.

    $ docker pull ouruser/sinatra@cbbf2f9a99b47fc460d422812b6a5adff7dfee951d8fa2e4a98caa0382cfbdbf

You can also reference by digest in `create`, `run`, and `rmi` commands, as well as the
`FROM` image reference in a Dockerfile.

## Push an image to Docker Hub

Once you've built or created a new image you can push it to [Docker
Hub](https://hub.docker.com) using the `docker push` command. This
allows you to share it with others, either publicly, or push it into [a
private repository](https://registry.hub.docker.com/plans/).

    $ docker push ouruser/sinatra
    The push refers to a repository [ouruser/sinatra] (len: 1)
    Sending image list
    Pushing repository ouruser/sinatra (3 tags)
    . . .

## Remove an image from the host

You can also remove images on your Docker host in a way [similar to
containers](usingdocker.md) using the `docker rmi` command.

Delete the `training/sinatra` image as you don't need it anymore.

    $ docker rmi training/sinatra
    Untagged: training/sinatra:latest
    Deleted: 5bc342fa0b91cabf65246837015197eecfa24b2213ed6a51a8974ae250fedd8d
    Deleted: ed0fffdcdae5eb2c3a55549857a8be7fc8bc4241fb19ad714364cbfd7a56b22f
    Deleted: 5c58979d73ae448df5af1d8142436d81116187a7633082650549c52c3a2418f0

> **Note:** To remove an image from the host, please make sure
> that there are no containers actively based on it.

# Next steps

Until now you've seen how to build individual applications inside Docker
containers. Now learn how to build whole application stacks with Docker
by networking together multiple Docker containers.

Go to [Network containers](networkingcontainers.md).
