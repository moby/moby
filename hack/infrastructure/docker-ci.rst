docker-ci github pull request
=============================

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
