=======
testing
=======

This directory contains testing related files.


Buildbot
========

Buildbot is a continuous integration system designed to automate the
build/test cycle. By automatically rebuilding and testing the tree each time
something has changed, build problems are pinpointed quickly, before other
developers are inconvenienced by the failure.

We are running buildbot in an AWS instance to verify docker passes all tests
when commits get pushed to the master branch.

You can check docker's buildbot instance at http://docker-ci.dotcloud.com/waterfall


Deployment
~~~~~~~~~~

::

  # Define AWS credential environment variables
  export AWS_ACCESS_KEY_ID=xxxxxxxxxxxx
  export AWS_SECRET_ACCESS_KEY=xxxxxxxxxxxx
  export AWS_KEYPAIR_NAME=xxxxxxxxxxxx
  export AWS_SSH_PRIVKEY=xxxxxxxxxxxx

  # Checkout docker
  git clone git://github.com/dotcloud/docker.git

  # Deploy docker on AWS
  cd docker/testing
  vagrant up --provider=aws


Buildbot AWS dependencies
-------------------------

vagrant, virtualbox packages and vagrant aws plugin
