page_title: Working with Docker and the Dockerfile
page_description: Working with Docker and The Dockerfile explained in depth
page_keywords: docker, introduction, documentation, about, technology, understanding, Dockerfile

# Working with Docker and the Dockerfile

*How to use and work with Docker and Docker's main element?*

> **Warning! Don't let this long page bore you.**
> If you prefer a summary and would like to get started **_very 
> quickly_**, you can check out the glossary of all available client
> commands on our [User's Manual: Commands Reference](
> http://docs.docker.io/en/latest/reference/commandline/cli).

## Introduction

On the last page (i.e., [Understanding the Technology](technology.md)) we covered the
parts forming Docker (e.g. the client and the daemon), studied the
underlying technology and *how* everything works (i.e., the underlying
technology). It should be clear how beneficial containers are compared
to virtual machines.

Now, it is time to get practical and see *how to work with* the Docker client,
Docker images and the `Dockerfile`.

> **Note:** You are encouraged to take a good look at the container,
> image and Dockerfile explanations here to have a better understanding
> on what exactly they are and to get an overall idea on how to work with
> them. On the next page (i.e., [Get Docker](get-docker.md)), you will be
> able to find links for platform-centric installation instructions and
> also, more goal-oriented usage tutorials based on `Dockerfile`s.

## Elements of Docker

As we mentioned on the previous page (i.e., [Understanding the Technology](technology.md)), main
elements of Docker are:

 - Containers;
 - Images, and;
 - The Dockerfile.

> **Note:** This page is more *practical* than *technical*. If you are
> interested in understanding how these tools work behind the scenes
> and do their job, you can always read more on
> [Understanding the Technology](technology.md).

## Working with the Docker client

In order to work with the Docker client, you need to have a host
set up with the Docker daemon (i.e., have Docker installed).

> **Tip:** Depending on your platform and the way you choose to install
> Docker, you might need to manually start the daemon: `sudo docker -d &`,
> or, a VM running the Docker daemon (e.g. on 
> [Mac OS X](http://docs.docker.io/en/latest/installation/mac)).

### How to use the client

The client provides you a command-line interface (i.e., CLI) to use Docker
and it works like almost any other regular command-line application.

> **Tip:** The below instructions can be considered a summary of our
> *interactive tutorial*. If you prefer a more hands-on approach without
> installing anything, why not give that a shot and check out the
> [Docker Interactive Tutorial](http://www.docker.io/interactivetutorial).

Usage consists of passing a chain of arguments:

    # Usage:  [sudo] docker [option] [command] [arguments] ..
    # Example:
    docker run -i -t ubuntu /bin/bash
    
### Versions and status

Perhaps the most performed operation with any application is to check 
the version and status.

    # Usage: [sudo] docker version
    # Example:
    docker version
    
This command will not only provide you the version of Docker client 
you are using, but also the version of Go (the programming language powering Docker).

    Client version: 0.8.0
    Go version (client): go1.2

    Git commit (client): cc3a8c8
    Server version: 0.8.0

    Git commit (server): cc3a8c8
    Go version (server): go1.2

    Last stable version: 0.8.0
    
### Finding out all available commands

The user-centric nature of Docker means providing you a constant stream
of helpful instructions. This begins with the client itself.

In order to get a full list of available commands, call the client:

    # Usage: [sudo] docker
    # Example:
    docker

You will get an output with all currently available commands.

    # Commands:
    #     attach    Attach to a running container
    #     build     Build a container from a Dockerfile
    #     commit    Create a new image from a container's changes
    # ..
    # A good long list of commands are available ;-)
    # .

### Command usage instructions

The same way used to learn all available commands can be repeated to find
out usage instructions for a specific command.

Try typing Docker followed with a `[command]` to see the instructions:

    # Usage: [sudo] docker [command] [--help]
    # Example:
    docker attach
    
    # Example:
    
    docker images --help

You will get an output with all available options:

    Usage: docker attach [OPTIONS] CONTAINER
    
    Attach to a running container
    
      --no-stdin=false: Do not attach stdin
      --sig-proxy=true: Proxify all received signal to the process (even in non-tty mode)

    # We do realise "proxify" is not a real word.
    # We do though like keeping things concise and efficient for you. ;-)

## Working with images

### Docker Images

In terms of IT, an image is a frozen-in-time collection of all files,
folders, programs etc. of a computer. In fact, it is the entirety of
everything important powering the computer.

As we have explained in our [introduction](../index.md), applications
inside the containers depend on libraries and tools supplied by
the operating system to work. They also depend on drivers, again
supplied by the OS to communicate with the hardware. This can mean
writing or reading data from the disk, or communicating over a network.

Any application running inside a container, therefore, needs to be
based on a default image. Any Linux distribution can be used as a
base (i.e., base Ubuntu), and any image can be used as a starting point
to power a container (i.e., Nginx installed on Ubuntu).

> **Tip:** Containing everything, including the base of an operating-system
> is the key to isolation. This provides the application everything it
> needs and allows blocking the outside access (i.e., isolation).

### Searching for images

Searching for images consists of using the `docker search` command. Depending
on what you are looking for, you might have a very long list returned,
or an empty one. All image information is supplied with some additional
ones, such as a *description*, *stars* and whether the image is *trusted*
or not.

    # Usage: [sudo] docker search [image name]
    # Example:
    docker search nginx
    
Output:

    # NAME                                     DESCRIPTION                                     STARS     OFFICIAL   TRUSTED
    # dockerfile/nginx                         Trusted Nginx (http://nginx.org/) Build         6                    [OK]
    # paintedfox/nginx-php5                    A docker image for running Nginx with PHP5.     3                    [OK]
    # dockerfiles/django-uwsgi-nginx           Dockerfile and configuration files to buil...   2                    [OK]
    # ..
    
> **Note:** To learn more about trusted builds, check out [this]
(http://blog.docker.io/2013/11/introducing-trusted-builds) blog post.

### Downloading an image

Downloading an image can be considered as *pulling* one from the repository.
Therefore, the `pull` command is used.

    # Usage: [sudo] docker pull [image name]
    # Example:
    docker pull dockerfile/nginx

Output:

    # Pulling repository dockerfile/nginx
    # 0ade68db1d05: Pulling dependent layers 
    # 27cf78414709: Download complete 
    # b750fe79269d: Download complete 
    # ..

As you will see, Docker will download, one by one, all the layers forming
the final image. This demonstrates the *building block* philosophy.

### Committing an image

Docker uses base images to create containers. However, when an application
is running, a read-write mode enabled filesystem is also provided.

Whenever you would like to take a snap-shot and save the changes of a
container, you need to `commit`.

    # Usage: [sudo] docker commit [container ID] [image name]
    # Example:
    docker commit 9565c1aeabea nginx

If you plan to share an image, you need to commit it with a specified username
which you can obtain by registering to the [Docker Index](https://index.docker.io).
Your username acts like a repository where images are collected as a set.

    # Usage: [sudo] docker commit [container ID] [user name]/[image name]
    # Example:
    docker commit 9565c1aeabea myUserName/nginx

### Sharing an image

Similar to Git, when you would like to share your commits, you can use
the `push` command.

    # Usage: [sudo] docker push [container ID] [user name]/[image name]
    # Example:
    docker push username/nginx

### Listing available images

In order to get a full list of available images, you can use the
`images` command.

    # Usage: [sudo] docker images
    # Example:
    docker images

Output:

    # REPOSITORY          TAG                 IMAGE ID            CREATED             VIRTUAL SIZE
    # myUserName/nginx    latest              a0d6c70867d2        41 seconds ago      578.8 MB
    # nginx               latest              173c2dd28ab2        3 minutes ago       578.8 MB
    # dockerfile/nginx    latest              0ade68db1d05        3 weeks ago         578.8 MB
    
## Working with containers

### Docker Containers

Docker containers are little more than an actual directory on your system.
Yes, they do contain literally everything for an application to run. And
yes, you can build them, extend them, manage their lifecycle and pull them
apart and continue building from any saved (i.e., *committed*) stage.
However, they still remain very lightweight, since Docker makes use of
the Linux kernel features to provide all the important functionality to
*decorate* the actual application running process (i.e., to contain, ship, 
share, sand-box and isolate an application).

> **Tip:** Containers, despite holding everything inside, are nothing like 
> VMs. To learn more about their differences, see the detailed comparison 
> in [*Docker Compared Against Virtual-Machines*](understanding-docker.md).

In order to create or start a container, you need an image. This could be an
empty Ubuntu instance, or, someone's *committed* personal image that is designed
to run a web-application.

Every action you take on an image immediately forms a new one. Working 
with Docker images means working with building blocks where images are 
*yours* to create anything, in any direction, from any point taken in time.

**Note:** A container can exist in two main states – *running* and *stopped*. 

### Running a new container from an image

One way to create a new container is to directly use an image.

    # Usage: [sudo] docker run [arguments] ..
    # Example:
    # docker run -name [new container name] -p [outside port:container port] -d [image name] 
    docker run -name nginx -p 80:80 -d dockerfile/nginx

Upon the successful creation of a container, a new image gets created.

### Listing all available containers

Listing containers depends on the `ps` command. Obtaining a full list, including
stopped containers requires the `-a` (or `--all`) flagged to be passed with the command.

    # Usage: [sudo] docker ps [-a]
    # Example:
    docker ps

Output:

    # CONTAINER ID        IMAGE                     COMMAND             CREATED             STATUS              PORTS                NAMES
    # 842a50a13032        dockerfile/nginx:latest   nginx               35 minutes ago      Up 30 minutes       0.0.0.0:80->80/tcp   nginx_web1

### Stopping a container

You can use the `stop` command to stop an active container. This will gracefully
end the active process.

    # Usage: [sudo] docker stop [container ID]
    # Example:
    docker stop nginx_web1

Output:
    
    # The output is the ID of the container stopped
    # nginx_web1

### Starting a Container

Stopped containers can start back again. They will run the application specified
during creation.

    # Usage: [sudo] docker start [container ID]
    # Example:
    docker start nginx_web1

Output:
    
    # The output is the ID of the container stopped
    # nginx_web1

### Saving the container state

As we have also previously covered, in order to save the current state 
of a container, the `commit` command must be used.

> *Tip:* When you `stop` a container, its state does not get lost. However,
> to form an image, it must be committed.

## Working with the Dockerfile

The Dockerfile provides an automation process to create and build new images 
and containers. This procedure consists of *successively* describing the steps
to be taken to build inside a text file, called `Dockerfile` and letting 
Docker go through them one by one.

> **Tip:** Below is a short summary of our full Dockerfile tutorial. 
> In order to get a better-grasp of how to work with these automation 
> scripts, check out the 
> [Dockerfile step-by-step tutorial](http://www.docker.io/learn/dockerfile).

### The Dockerfile

You can streamline and automate the whole Docker image creation process
successively, in a maintainable and predictable way by using Dockerfiles.

A Dockerfile is a script (i.e., a set of instructions) to be read and 
run by the Docker client to build a brand new container image with 
your exact specifications.

Dockerfiles are powerful. They allow you to attach/detach directories 
from the host, copy files and specify which process to run when a 
container gets started *from* the formed image and even link containers
intelligently.

> **Tip:** Dockerfiles are flexible and allow you to achieve a great 
> deal of things through a collection of directives / commands. To learn 
> about the Dockerfile auto-builder, check out the documentation page
> [Dockerfile Reference](http://docs.docker.io/en/latest/reference/builder).

### Dockerfile Format

A `Dockerfile` contains instructions written in the below format:

    # Usage: Instruction [arguments / command] ..
    # Example:
    FROM ubuntu

All arguments provided after an instruction, unless it is a Docker
specific one (e.g. `FROM`), are passed directly to the container to be
executed.

Although there is not a script requirement, instructions are generally
advised to be written in all capitals for the purposes of clarity.

A `#` sign is used to comment-out text:

    # Comments ..

### First steps with the Dockerfile

Dockerfiles shall be supplied with necessary explanations, instructions
along with appropriate meta-data, such as the name of the creator
(i.e., maintainer) of the document.

Therefore, it is advisable to place a block of code, similar to the one
below, at the beginning of each Dockerfile:

    ###############################
    # Dockerfile to install Nginx #
    # VERSION 2 - EDITION 1       #
    ###############################
    
As the first instruction, they should always state the name of the base
image to be used with the `FROM` instruction, i.e.,

    # Base image used is Ubuntu:
    FROM ubuntu
    
Next, for brevity, the name of the `Maintainer` can be declared by typing
the name and an email address, separated by a comma, after the instruction:

    # Maintainer: O.S. Tezer <ostezer at gmail com> (@ostezer)
    MAINTAINER O.S. Tezer, ostezer@gmail.com

Once the meta-data and the name of the base image is declared appropriately,
it is time to start listing the commands (i.e., instructions).

### Listing instructions in a Dockerfile

Following up from the previous section, the current final state of our
Dockerfile should be similar to:

    ###############################
    # Dockerfile to install Nginx #
    # VERSION 2 - EDITION 1       #
    ###############################
    
    # Base image used is Ubuntu:
    FROM ubuntu

    # Maintainer: O.S. Tezer <ostezer at gmail com> (@ostezer)
    MAINTAINER O.S. Tezer, ostezer@gmail.com

Now we can continue adding instructions, which are to be executed successively.
They should be added following the below example:

    # Instruction (command) explanation
    # Instruction argument (command)

Therefore;

    # Install Nginx on Ubuntu
    
    # Add application repository
    RUN echo "deb http://archive.ubuntu.com/ubuntu/ raring main universe" >> /etc/apt/sources.list

    # Update the server repository list
    RUN apt-get update
    
    # Install some necessary libraries and tools
    RUN apt-get install -y net-tools wget nano curl dialog
    
    # Finally, download and install Nginx
    RUN apt-get install -y nginx
    
    # Turn daemon mode off
    RUN echo "\ndaemon off;" >> /etc/nginx/nginx.conf
    
    # Attach volumes
    # Volumes are regular OS directories that are not 
    # part of an image's layers.
    # You can easily access them from the outside and
    # keep them as they are.
    # To learn more about volumes, visit:
    # http://docs.docker.io/en/latest/use/working_with_volumes/
    VOLUME /etc/nginx/sites-enabled
    VOLUME /var/log/nginx
    
    # Or;
    
    # Alternatively, copy a custom nginx.conf
    # RUN rm -v /etc/nginx/nginx.conf
    # ADD nginx.conf /etc/nginx/
    
    # Expose ports
    EXPOSE 80

    # Set the command to be executed when
    # running (i.e., creating) a new container
    CMD service nginx start

You can now save this file and use it to build images and create containers.

### Using a Dockerfile

Docker uses the Dockerfile to build an image. Therefore, using a Dockerfile is
achieved with the `build` command.

    # Use the Dockerfile at the current location
    # Usage: [sudo] docker build .
    # Example:
    docker build -t my_nginx_img .
    
Output:

    # Uploading context 25.09 kB
    # Uploading context 
    # Step 0 : FROM ubuntu
    #  ---> 9cd978db300e
    # Step 1 : MAINTAINER O.S. Tezer, ostezer@gmail.com
    #  ---> Using cache
    #  ---> 467542d0cdd3
    # Step 2 : RUN echo "deb http://archive.ubuntu.com/ubuntu/ raring main universe" >> /etc/apt/sources.list
    #  ---> Using cache
    #  ---> 0a688bd2a48c
    # Step 3 : RUN apt-get update
    #  ---> Running in de2937e8915a
    
    # ..
    # ..
    # Step 10 : CMD service nginx start
    #  ---> Running in b4908b9b9868
    #  ---> 626e92c5fab1
    # Successfully built 626e92c5fab1

    docker images
    
    # REPOSITORY          TAG                 IMAGE ID            CREATED             VIRTUAL SIZE
    # my_nginx_img       latest              626e92c5fab1        57 seconds ago      337.6 MB

## Where to go from here

### Understanding Docker

Visit [Understanding Docker](understanding-docker.md) in our Getting Started manual.

### Learn about parts of Docker and the underlying technology

Visit [Understanding the Technology](technology.md) in our Getting Started manual.

### Get the product and go hands-on

Visit [Get Docker](get-docker.md) in our Getting Started manual.

### Get the whole story

[https://www.docker.io/the_whole_story/](https://www.docker.io/the_whole_story/)
