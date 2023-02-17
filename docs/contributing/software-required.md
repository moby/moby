# Get the required software for Linux or macOS

This page explains how to get the software you need to use a Linux or macOS
machine for Moby development. Before you begin contributing you must have:

*  a GitHub account
* `git`
* `make`
* `docker`

You'll notice that `go`, the language that Moby is written in, is not listed.
That's because you don't need it installed; Moby's development environment
provides it for you. You'll learn more about the development environment later.

## Task 1. Get a GitHub account

To contribute to the Moby project, you will need a <a
href="https://github.com" target="_blank">GitHub account</a>. A free account is
fine. All the Moby project repositories are public and visible to everyone.

You should also have some experience using both the GitHub application and `git`
on the command line.

## Task 2. Install git

Install `git` on your local system. You can check if `git` is on already on your
system and properly installed with the following command:

```bash
$ git --version
```

This documentation is written using `git` version 2.2.2. Your version may be
different depending on your OS.

## Task 3. Install make

Install `make`. You can check if `make` is on your system with the following
command:

```bash
$ make -v
```

This documentation is written using GNU Make 3.81. Your version may be different
depending on your OS.

## Task 4. Install or upgrade Docker

If you haven't already, [install the Docker Engine](https://docs.docker.com/engine/install/).
If you have an existing installation, check your version and make sure you have
the latest version of Docker.

To check if `docker` is already installed on Linux:

```bash
docker --version
Docker version 17.10.0-ce, build f4ffd25
```

On macOS or Windows, you should have installed Docker for Mac or
Docker for Windows.

```bash
$ docker --version
Docker version 17.10.0-ce, build f4ffd25
```

## Tip for Linux users

This guide assumes you have added your user to the `docker` group on your system.
To check, list the group's contents:

```
$ getent group docker
docker:x:999:ubuntu
```

If the command returns no matches, you have two choices. You can preface this
guide's `docker` commands with `sudo` as you work. Alternatively, you can add
your user to the `docker` group as follows:

```bash
$ sudo usermod -aG docker ubuntu
```

You must log out and log back in for this modification to take effect.


## Where to go next

In the next section, you'll [configure the project to be properly recognized by the IDE](set-up-ide.md).

If you don't want to use IDE, you can proceed to the next step
and [learn how to set up and configure Git for contributing to Moby](set-up-git.md).