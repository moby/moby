#!/bin/sh

set -ex

HERE=${HERE:="."}
mkdir -p "${HERE}"/deploy

export DOCKER_BUILDKIT=1

docker build \
	--target=tests \
	--platform=linux/amd64 \
	--progress=plain \
	"${HERE}"

# make build-in-container/store-artefacts happy
cp ${HERE}/VERSION ${HERE}/deploy/VERSION
