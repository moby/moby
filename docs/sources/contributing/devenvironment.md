page_title: Setting Up a Dev Environment
page_description: Guides on how to contribute to docker
page_keywords: Docker, documentation, developers, contributing, dev environment

# Setting Up a Dev Environment

To make it easier to contribute to Docker, we provide a standard
development environment. It is important that the same environment be
used for all tests, builds and releases. The standard development
environment defines all build dependencies: system libraries and
binaries, go environment, go dependencies, etc.

## Install Docker

Docker’s build environment itself is a Docker container, so the first
step is to install Docker on your system.

You can follow the [install instructions most relevant to your
system](https://docs.docker.io/en/latest/installation/). Make sure you
have a working, up-to-date docker installation, then continue to the
next step.

## Install tools used for this tutorial

Install `git`; honest, it’s very good. You can use
other ways to get the Docker source, but they’re not anywhere near as
easy.

Install `make`. This tutorial uses our base Makefile
to kick off the docker containers in a repeatable and consistent way.
Again, you can do it in other ways but you need to do more work.

## Check out the Source

    git clone http://git@github.com/dotcloud/docker
    cd docker

To checkout a different revision just use `git checkout`
with the name of branch or revision number.

## Build the Environment

This following command will build a development environment using the
Dockerfile in the current directory. Essentially, it will install all
the build and runtime dependencies necessary to build and test Docker.
This command will take some time to complete when you first execute it.

    sudo make build

If the build is successful, congratulations! You have produced a clean
build of docker, neatly encapsulated in a standard build environment.

## Build the Docker Binary

To create the Docker binary, run this command:

    sudo make binary

This will create the Docker binary in
`./bundles/<version>-dev/binary/`

### Using your built Docker binary

The binary is available outside the container in the directory
`./bundles/<version>-dev/binary/`. You can swap your
host docker executable with this binary for live testing - for example,
on ubuntu:

    sudo service docker stop ; sudo cp $(which docker) $(which docker)_ ; sudo cp ./bundles/<version>-dev/binary/docker-<version>-dev $(which docker);sudo service docker start

Note

Its safer to run the tests below before swapping your hosts docker
binary.

## Run the Tests

To execute the test cases, run this command:

    sudo make test

If the test are successful then the tail of the output should look
something like this

    --- PASS: TestWriteBroadcaster (0.00 seconds)
    === RUN TestRaceWriteBroadcaster
    --- PASS: TestRaceWriteBroadcaster (0.00 seconds)
    === RUN TestTruncIndex
    --- PASS: TestTruncIndex (0.00 seconds)
    === RUN TestCompareKernelVersion
    --- PASS: TestCompareKernelVersion (0.00 seconds)
    === RUN TestHumanSize
    --- PASS: TestHumanSize (0.00 seconds)
    === RUN TestParseHost
    --- PASS: TestParseHost (0.00 seconds)
    === RUN TestParseRepositoryTag
    --- PASS: TestParseRepositoryTag (0.00 seconds)
    === RUN TestGetResolvConf
    --- PASS: TestGetResolvConf (0.00 seconds)
    === RUN TestCheckLocalDns
    --- PASS: TestCheckLocalDns (0.00 seconds)
    === RUN TestParseRelease
    --- PASS: TestParseRelease (0.00 seconds)
    === RUN TestDependencyGraphCircular
    --- PASS: TestDependencyGraphCircular (0.00 seconds)
    === RUN TestDependencyGraph
    --- PASS: TestDependencyGraph (0.00 seconds)
    PASS
    ok      github.com/dotcloud/docker/utils        0.017s

If $TESTFLAGS is set in the environment, it is passed as extra
arguments to ‘go test’. You can use this to select certain tests to run,
eg.

> TESTFLAGS=’-run \^TestBuild\$’ make test

If the output indicates "FAIL" and you see errors like this:

    server.go:1302 Error: Insertion failed because database is full: database or disk is full

    utils_test.go:179: Error copy: exit status 1 (cp: writing '/tmp/docker-testd5c9-[...]': No space left on device

Then you likely don’t have enough memory available the test suite. 2GB
is recommended.

## Use Docker

You can run an interactive session in the newly built container:

    sudo make shell

    # type 'exit' or Ctrl-D to exit

## Build And View The Documentation

If you want to read the documentation from a local website, or are
making changes to it, you can build the documentation and then serve it
by:

        sudo make docs
    # when its done, you can point your browser to http://yourdockerhost:8000
        # type Ctrl-C to exit

**Need More Help?**

If you need more help then hop on to the [\#docker-dev IRC
channel](irc://chat.freenode.net#docker-dev) or post a message on the
[Docker developer mailing
list](https://groups.google.com/d/forum/docker-dev).
