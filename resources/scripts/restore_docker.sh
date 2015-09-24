#!/bin/bash
#wget https://get.docker.com/builds/Linux/x86_64/docker-latest
sudo service docker stop
sudo cp docker-latest $(which docker)
sudo service docker start
