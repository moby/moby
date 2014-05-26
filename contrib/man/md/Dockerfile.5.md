% DOCKERFILE(1) Docker User Manuals
% Zac Dover
% May 2014
# NAME

Dockerfile - automate the steps of creating a Docker image

# INTRODUCTION
**Dockerfile** is a configuration file that automates the steps of creating a Docker image. Docker can act as a builder and can read instructions from **Dockerfile** to automate the steps that you would otherwise manually perform to create an image. To build an image from a source repository, create a description file called **Dockerfile** at the root of your repository. This file describes the steps that will be taken to assemble the image. When **Dockerfile** has been created, call **docker build** with the path of the source repository as the argument.

# SYNOPSIS

INSTRUCTION arguments

For example:

FROM image

# DESCRIPTION

Dockerfile is a file that automates the steps of creating a Docker image.

# USAGE

$ sudo docker build .
 -- runs the steps and commits them, building a final image
    The path to the source repository defines where to find the context of the build.
    The build is run by the docker daemon, not the CLI. The whole context must be
    transferred to the daemon. The Docker CLI reports "Uploading context" when the
    context is sent to the daemon.
    
$ sudo docker build -t repository/tag .
 -- specifies a repository and tag at which to save the new image if the build succeeds.
    The Docker daemon runs the steps one-by-one, commiting the result to a new image
    if necessary before finally outputting the ID of the new image. The Docker
    daemon automatically cleans up the context it is given.

Docker re-uses intermediate images whenever possible. This significantly accelerates the *docker build* process.
 
# HISTORY
May 2014, Compiled by Zac Dover (zdover at redhat dot com) based on docker.io Dockerfile documentation.
