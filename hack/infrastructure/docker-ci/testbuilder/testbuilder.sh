#!/usr/bin/env bash
# Download,  build and run a docker project tests
# Environment variables: DEPLOYMENT

cat $0
set -e
set -x

PROJECT=$1
COMMIT=${2-HEAD}
REPO=${3-https://github.com/dotcloud/$PROJECT}
BRANCH=${4-master}
REPO_PROJ="https://github.com/docker-test/$PROJECT"
if [ "$DEPLOYMENT" == "production" ]; then
    REPO_PROJ="https://github.com/dotcloud/$PROJECT"
fi
set +x

# Generate a random string of $1 characters
function random {
    cat /dev/urandom | tr -cd 'a-f0-9' | head -c $1
}

PROJECT_PATH="$PROJECT-tmp-$(random 12)"

# Set docker-test git user
set -x
git config --global user.email "docker-test@docker.io"
git config --global user.name "docker-test"

# Fetch project
git clone -q $REPO_PROJ -b master /data/$PROJECT_PATH
cd /data/$PROJECT_PATH
echo "Git commit: $(git rev-parse HEAD)"
git fetch -q $REPO $BRANCH
git merge --no-edit $COMMIT

# Build the project dockertest
/testbuilder/$PROJECT.sh $PROJECT_PATH
rm -rf /data/$PROJECT_PATH
