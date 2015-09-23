#!/bin/bash

sudo service docker stop
sudo cp ./bundles/1.9.0-dev/binary/docker-1.9.0-dev $(which docker)
sudo service docker start
