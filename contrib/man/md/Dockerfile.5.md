% DOCKERFILE(5) Docker User Manuals
% Zac Dover
% May 2014
# NAME

Dockerfile - automate the steps of creating a Docker image

# INTRODUCTION
**Dockerfile** is a configuration file that automates the steps of creating a
Docker image. It is similar to a Makefile. Docker reads instructions from
**Dockerfile** to automate the steps otherwise performed manually to create 
an image. To build an image, create a file called **Dockerfile**.  The 
**Dockerfile** describes the steps taken to assemble the image. When the 
**Dockerfile** has been created, call the **docker build** command, using the 
path of directory that contains **Dockerfile** as the argument.

# SYNOPSIS

INSTRUCTION arguments

For example:

FROM image

# DESCRIPTION

A Dockerfile is a file that automates the steps of creating a Docker image. 
A Dockerfile is similar to a Makefile.

# USAGE

**sudo docker build .**
 -- runs the steps and commits them, building a final image
    The path to the source repository defines where to find the context of the
    build. The build is run by the docker daemon, not the CLI. The whole 
    context must be transferred to the daemon. The Docker CLI reports 
    "Uploading context" when the context is sent to the daemon.
    
** sudo docker build -t repository/tag .**
 -- specifies a repository and tag at which to save the new image if the build 
    succeeds. The Docker daemon runs the steps one-by-one, commiting the result 
    to a new image if necessary before finally outputting the ID of the new 
    image. The Docker daemon automatically cleans up the context it is given.

Docker re-uses intermediate images whenever possible. This significantly 
accelerates the *docker build* process.
 
# FORMAT

**FROM image**
or
**FROM image:tag**
 -- The FROM instruction sets the base image for subsequent instructions. A
 valid Dockerfile must have FROM as its first instruction. The image can be any
 valid image. It is easy to start by pulling an image from the public
 repositories.
 -- FROM must be he first non-comment instruction in Dockerfile.
 -- FROM may appear multiple times within a single Dockerfile in order to create
 multiple images. Make a note of the last image id output by the commit before
 each new FROM command.
 -- If no tag is given to the FROM instruction, latest is assumed. If the used
 tag does not exist, an error is returned.

**MAINTAINER**
 --The MAINTAINER instruction sets the Author field for the generated images.

**RUN**
 --RUN has two forms:
 **RUN <command>**
 -- (the command is run in a shell - /bin/sh -c)
 **RUN ["executable", "param1", "param2"]**
 --The above is executable form.
 --The RUN instruction executes any commands in a new layer on top of the
 current image and commits the results. The committed image is used for the next
 step in Dockerfile.
 --Layering RUN instructions and generating commits conforms to the core
 concepts of Docker where commits are cheap and containers can be created from
 any point in the history of an image. This is similar to source control.  The
 exec form makes it possible to avoid shell string munging. The exec form makes
 it possible to RUN commands using a base image that does not contain /bin/sh.

**CMD**
 --CMD has three forms:
  **CMD ["executable", "param1", "param2"]** This is the preferred form, the
  exec form.
  **CMD ["param1", "param2"]** This command provides default parameters to
  ENTRYPOINT)
  **CMD command param1 param2** This command is run as a shell.
  --There can be only one CMD in a Dockerfile. If more than one CMD is listed, only
  the last CMD takes effect.
  The main purpose of a CMD is to provide defaults for an executing container.
  These defaults may include an executable, or they can omit the executable. If
  they omit the executable, an ENTRYPOINT must be specified.
  When used in the shell or exec formats, the CMD instruction sets the command to
  be executed when running the image.
  If you use the shell form of of the CMD, the <command> executes in /bin/sh -c:
  **FROM ubuntu**
  **CMD echo "This is a test." | wc -**
  If you run <command> wihtout a shell, then you must express the command as a
  JSON arry and give the full path to the executable. This array form is the
  preferred form of CMD. All additional parameters must be individually expressed
  as strings in the array:
  **FROM ubuntu**
  **CMD ["/usr/bin/wc","--help"]**
  To make the container run the same executable every time, use ENTRYPOINT in
  combination with CMD.
  If the user specifies arguments to  docker run, the specified commands override
  the default in CMD.
  Do not confuse **RUN** with **CMD**. RUN runs a command and commits the result. CMD
  executes nothing at build time, but specifies the intended command for the
  image.

**EXPOSE**
 --EXPOSE <port> [<port>...]
 The **EXPOSE** instruction informs Docker that the container listens on the
 specified network ports at runtime. Docker uses this information to
 interconnect containers using links, and to set up port redirection on the host
 system.

# HISTORY
The MAINTAINER instruction sets the Author field for the generated images.
May 2014, Compiled by Zac Dover (zdover at redhat dot com) based on docker.io Dockerfile documentation.
