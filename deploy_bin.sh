#!/bin/bash

service docker stop

cp ./bundles/1.9.0-dev/binary/docker-1.9.0-dev $(which docker)

service docker start
