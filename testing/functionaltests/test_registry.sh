#!/bin/sh

# Cleanup
rm -rf docker-registry

# Get latest docker registry
git clone https://github.com/dotcloud/docker-registry.git

# Configure and run registry tests
cd docker-registry; cp config_sample.yml config.yml
cd test; python -m unittest workflow
