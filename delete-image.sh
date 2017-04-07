#!/bin/bash
#!/usr/bin/python
#################################################################################
#Python script to delete an image
#Written by= Arif Ahmed (arifch2009@gmail.com)
#
##################################################################################

#print the image name
echo "Image to be deleted:$1"

#command to delete the containers
docker rm $(docker ps -q -a --filter ancestor=$1)

#command to delete the image
docker rmi $1




