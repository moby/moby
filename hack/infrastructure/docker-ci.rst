docker-ci
=========

docker-ci is our buildbot continuous integration server,
building and testing docker, hosted on EC2 and reachable at
http://docker-ci.dotcloud.com


Deployment
==========

# Load AWS credentials
export AWS_ACCESS_KEY_ID=''
export AWS_SECRET_ACCESS_KEY=''
export AWS_KEYPAIR_NAME=''
export AWS_SSH_PRIVKEY=''

# Load buildbot credentials and config
export BUILDBOT_PWD=''
export IRC_PWD=''
export IRC_CHANNEL='docker-dev'
export SMTP_USER=''
export SMTP_PWD=''
export EMAIL_RCP=''

# Load registry test credentials
export REGISTRY_USER=''
export REGISTRY_PWD=''

cd docker/testing
vagrant up --provider=aws


github pull request
===================

The entire docker pull request test workflow is event driven by github. Its
usage is fully automatic and the results are logged in docker-ci.dotcloud.com

Each time there is a pull request on docker's github project, github connects
to docker-ci using github's rest API documented in http://developer.github.com/v3/repos/hooks
The issued command to program github's notification PR event was:
curl -u GITHUB_USER:GITHUB_PASSWORD -d '{"name":"web","active":true,"events":["pull_request"],"config":{"url":"http://docker-ci.dotcloud.com:8011/change_hook/github?project=docker"}}' https://api.github.com/repos/dotcloud/docker/hooks

buildbot (0.8.7p1) was patched using ./testing/buildbot/github.py, so it
can understand the PR data github sends to it. Originally PR #1603 (ee64e099e0)
implemented this capability. Also we added a new scheduler to exclusively filter
PRs. and the 'pullrequest' builder to rebase the PR on top of master and test it.


nighthly release
================

The nightly release process is done by buildbot, running a DinD container that downloads
the docker repository and builds the release container. The resulting
docker binary is then tested, and if everything is fine the release is done.

Building the release DinD Container
-----------------------------------

# Log into docker-ci
ssh ubuntu@docker-ci.dotcloud.com
cd /data/docker/testing/nightlyrelease
# Add release_credentials.json as specified in ./Dockerfile
cat  > release_credentials.json << EOF
EOF
sudo docker build -t dockerbuilder .
# Now that the container is built release_credentials.json is not needed anymore
git checkout release_credentials.json
