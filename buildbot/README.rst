Buildbot
========

Buildbot is a continuous integration system designed to automate the
build/test cycle. By automatically rebuilding and testing the tree each time
something has changed, build problems are pinpointed quickly, before other
developers are inconvenienced by the failure.

When running 'make hack' at the docker root directory, it spawns a virtual
machine in the background running a buildbot instance and adds a git
post-commit hook that automatically run docker tests for you.

You can check your buildbot instance at http://192.168.33.21:8010/waterfall


Buildbot dependencies
---------------------

vagrant, virtualbox packages and python package requests

