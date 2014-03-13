#!/usr/bin/env bash
set -x
set -e
PROJECT_PATH=$1

# Build the docker project
cd /data/$PROJECT_PATH
sg docker -c "docker build -q -t docker ."

if [ "$DOCKER_RELEASE" == "1" ]; then
    # Do nightly release
    echo sg docker -c "docker run --rm --privileged -v /run:/var/socket -e AWS_S3_BUCKET=$AWS_S3_BUCKET -e AWS_ACCESS_KEY= -e AWS_SECRET_KEY= -e GPG_PASSPHRASE= docker hack/release.sh"
    set +x
    sg docker -c "docker run --rm --privileged -v /run:/var/socket -e AWS_S3_BUCKET=$AWS_S3_BUCKET -e AWS_ACCESS_KEY=$AWS_ACCESS_KEY -e AWS_SECRET_KEY=$AWS_SECRET_KEY -e GPG_PASSPHRASE=$GPG_PASSPHRASE docker hack/release.sh"
else
    # Run the tests
    sg docker -c "docker run --rm --privileged -v /home/docker-ci/coverage/docker:/data docker ./hack/infrastructure/docker-ci/docker-coverage/gocoverage.sh"
fi
