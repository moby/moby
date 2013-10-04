#!/bin/bash

# This script runs the docker tests.
#
# Requirements: 
# - prepare_tests.sh has been run to build the docker-test image

set -e

# stop apparmor interfering with us
sudo /etc/init.d/apparmor stop
sudo /etc/init.d/apparmor teardown

# run the tests!
sudo docker run -dns=8.8.8.8 -lxc-conf=lxc.aa_profile=unconfined -privileged -v `pwd`:/go/src/github.com/dotcloud/docker docker-test hack/make.sh test
