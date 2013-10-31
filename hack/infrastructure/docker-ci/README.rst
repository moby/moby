=======
testing
=======

This directory contains docker-ci testing related files.


Buildbot
========

Buildbot is a continuous integration system designed to automate the
build/test cycle. By automatically rebuilding and testing the tree each time
something has changed, build problems are pinpointed quickly, before other
developers are inconvenienced by the failure.

We are running buildbot in Amazon's EC2 to verify docker passes all
tests when commits get pushed to the master branch and building
nightly releases using Docker in Docker awesome implementation made
by Jerome Petazzoni.

https://github.com/jpetazzo/dind

Docker's buildbot instance is at http://docker-ci.dotcloud.com/waterfall

For deployment instructions, please take a look at
hack/infrastructure/docker-ci/Dockerfile
