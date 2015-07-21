#!/usr/bin/env bash

PR=$1
GH_TOKEN=$2
REPO_NAME="docker"
REPO_OWNER="docker"

for scnr in golint
do
    joker -repo=$REPO_NAME \
	  -owner=$REPO_OWNER \
	  -token=$GH_TOKEN \
	  -pr=$PR \
	  -scanner=$scnr
done
